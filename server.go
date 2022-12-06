package main

import (
	"context"
	"encoding/gob"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v9"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/mattn/go-mastodon"
	"github.com/rbcervilla/redisstore/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	Template struct {
		templates *template.Template
	}

	CustomValidator struct {
		validator *validator.Validate
	}

	M map[string]interface{}
)

var (
	mastAppConfigBase   *mastodon.AppConfig = nil
	mainDB              *mongo.Database     = nil
	mainValidator                           = validator.New()
	mainConfig          *AppConfig
	lkRoomServiceClient *lksdk.RoomServiceClient
)

func init() {
	gob.Register(&SessionData{})
	gob.Register(&M{})
}

func main() {
	var err error

	// Load config from environment variables and .env
	mainConfig, err = loadConfig(os.Getenv("AUDON_ENV"))
	if err != nil {
		log.Fatalln(err)
	}

	// Setup Livekit RoomService Client
	lkURL := &url.URL{
		Scheme: "https",
		Host:   mainConfig.Livekit.Host,
	}
	lkRoomServiceClient = lksdk.NewRoomServiceClient(lkURL.String(), mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)

	backContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Setup database client
	dbClient, err := mongo.Connect(backContext, options.Client().ApplyURI(mainConfig.MongoURL.String()))
	if err != nil {
		log.Fatalln(err)
	}
	mainDB = dbClient.Database(mainConfig.Database.Name)
	err = createIndexes(backContext)
	if err != nil {
		log.Fatalln(err)
	}

	// Setup redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     mainConfig.Redis.Host,
		Username: mainConfig.Redis.User,
		Password: mainConfig.Redis.Password,
		DB:       0,
	})

	// Setup echo server
	e := echo.New()
	defer e.Close()

	t := &Template{
		templates: template.Must(template.ParseFiles("audon-fe/index.html", "audon-fe/dist/index.html")),
	}
	e.Renderer = t
	e.Validator = &CustomValidator{validator: mainValidator}

	// Setup session middleware (currently Audon stores all client data in cookie)
	redisStore, err := redisstore.NewRedisStore(backContext, redisClient)
	if err != nil {
		log.Fatalln(err)
	}
	redisStore.KeyPrefix("session_")
	sessionOptions := sessions.Options{
		Path:     "/",
		Domain:   mainConfig.LocalDomain,
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
	}
	if mainConfig.Environment == "development" {
		sessionOptions.Domain = ""
		sessionOptions.SameSite = http.SameSiteNoneMode
		sessionOptions.Secure = false
		sessionOptions.MaxAge = 3600 * 24
		sessionOptions.HttpOnly = false
	}
	redisStore.Options(sessionOptions)
	e.Use(session.Middleware(redisStore))

	e.POST("/app/login", loginHandler)
	e.GET("/app/oauth", oauthHandler)
	e.GET("/app/verify", verifyHandler)

	e.POST("/app/webhook", livekitWebhookHandler)

	api := e.Group("/api", authMiddleware)
	api.POST("/room", createRoomHandler)
	api.GET("/room/:id", joinRoomHandler)
	api.DELETE("/room/:id", closeRoomHandler)
	api.PATCH("/room/:room/:user", updatePermissionHandler)

	// e.Static("/", "audon-fe/dist/assets")

	e.Logger.Debug(e.Start(":1323"))
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return wrapValidationError(err)
	}
	return nil
}

func getAppConfig(server string, redirPath string) (*mastodon.AppConfig, error) {
	if mastAppConfigBase != nil {
		return &mastodon.AppConfig{
			Server:       server,
			ClientName:   mastAppConfigBase.ClientName,
			Scopes:       mastAppConfigBase.Scopes,
			Website:      mastAppConfigBase.Website,
			RedirectURIs: mastAppConfigBase.RedirectURIs,
		}, nil
	}

	redirectURI := "urn:ietf:wg:oauth:2.0:oob"
	u := &url.URL{
		Host:   mainConfig.LocalDomain,
		Scheme: "https",
		Path:   "/",
	}
	q := u.Query()
	q.Add("redir", redirPath)
	u.RawQuery = q.Encode()
	u = u.JoinPath("app", "oauth")
	redirectURI = u.String()

	conf := &mastodon.AppConfig{
		ClientName:   "Audon",
		Scopes:       "read:accounts read:follows",
		Website:      "https://github.com/nmkj-io/audon",
		RedirectURIs: redirectURI,
	}

	mastAppConfigBase = conf

	return &mastodon.AppConfig{
		Server:       server,
		ClientName:   conf.ClientName,
		Scopes:       conf.Scopes,
		Website:      conf.Website,
		RedirectURIs: conf.RedirectURIs,
	}, nil
}

func getSession(c echo.Context, sessionID string) (sess *sessions.Session, err error) {
	sess, err = session.Get(sessionID, c)
	if err != nil {
		return nil, err
	}

	return sess, nil
}

// retrieve user's session, returns invalid cookie error if failed
func getSessionData(c echo.Context) (data *SessionData, err error) {
	sess, err := getSession(c, SESSION_NAME)
	if err != nil {
		c.Logger().Error(err)
		return nil, ErrSessionNotAvailable
	}

	val := sess.Values[SESSION_DATASTORE_NAME]
	data, ok := val.(*SessionData)

	if !ok {
		return nil, ErrInvalidSession
	}

	return data, nil
}

// write user's session, returns error if failed
func writeSessionData(c echo.Context, data *SessionData) error {
	sess, err := getSession(c, SESSION_NAME)
	if err != nil {
		return err
	}

	sess.Values[SESSION_DATASTORE_NAME] = data

	return sess.Save(c.Request(), c.Response())
}

// handler for GET to /app/verify
func verifyHandler(c echo.Context) (err error) {
	valid, acc, _ := verifyTokenInSession(c)
	if !valid {
		return c.NoContent(http.StatusUnauthorized)
	}

	return c.JSON(http.StatusOK, acc)
}
