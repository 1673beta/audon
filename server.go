package main

import (
	"context"
	"encoding/gob"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/mattn/go-mastodon"
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
	err_invalid_cookie error               = errors.New("invalid cookie")
	mastAppConfigBase  *mastodon.AppConfig = nil
	mainDB             *mongo.Database     = nil
	mainValidator                          = validator.New()
	mainConfig         *AppConfig
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
		os.Exit(1)
	}

	backContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Setup database client
	dbClient, err := mongo.Connect(backContext, options.Client().ApplyURI(mainConfig.MongoURL.String()))
	if err != nil {
		log.Fatalln(err)
		os.Exit(2)
	}
	mainDB = dbClient.Database(mainConfig.Database.Name)
	err = createIndexes(backContext)
	if err != nil {
		log.Fatalln(err)
		os.Exit(3)
	}

	e := echo.New()
	defer e.Close()

	t := &Template{
		templates: template.Must(template.ParseFiles("audon-fe/index.html", "audon-fe/dist/index.html")),
	}
	e.Renderer = t
	e.Validator = &CustomValidator{validator: mainValidator}
	cookieStore := sessions.NewCookieStore([]byte(mainConfig.SeesionSecret))
	cookieStore.Options = &sessions.Options{
		Path:     "/",
		Domain:   mainConfig.LocalDomain,
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
	}
	if mainConfig.Environment == "development" {
		cookieStore.Options.Domain = ""
		cookieStore.Options.SameSite = http.SameSiteNoneMode
		cookieStore.Options.Secure = false
		cookieStore.Options.MaxAge = 3600 * 24
		cookieStore.Options.HttpOnly = false
	}
	e.Use(session.Middleware(cookieStore))

	e.Static("/", "audon-fe/dist/assets")

	e.POST("/app/login", loginHandler)
	e.GET("/app/oauth", oauthHandler)
	e.GET("/app/verify", verifyHandler)

	api := e.Group("/api", authMiddleware)
	api.POST("/room", createRoomHandler)

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

func getAppConfig(server string) (*mastodon.AppConfig, error) {
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
	valid, _ := verifyTokenInSession(c)
	if !valid {
		return c.NoContent(http.StatusUnauthorized)
	}

	return c.NoContent(http.StatusOK)
}
