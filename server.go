package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/gob"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v9"
	"github.com/gorilla/sessions"
	"github.com/jellydator/ttlcache/v3"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/mattn/go-mastodon"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rbcervilla/redisstore/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	Template struct {
		templates *template.Template
	}

	TemplateData struct {
		Config *AppConfigBase
		Room   *Room
	}

	CustomValidator struct {
		validator *validator.Validate
	}

	M map[string]interface{}
)

var (
	// mastAppConfigBase   *mastodon.AppConfig = nil
	mainDB              *mongo.Database = nil
	mainValidator                       = validator.New()
	mainConfig          *AppConfig
	lkRoomServiceClient *lksdk.RoomServiceClient
	localeBundle        *i18n.Bundle
	roomSessionCache    *ttlcache.Cache[string, *SessionData]
	webhookTimerCache   *ttlcache.Cache[string, *time.Timer]
)

func init() {
	gob.Register(&SessionData{})
	gob.Register(&M{})
}

func main() {
	var err error

	log.Println("Audon server started.")

	// Load config from environment variables and .env
	log.Println("Loading Audon config values")
	mainConfig, err = loadConfig(os.Getenv("AUDON_ENV"))
	if err != nil {
		log.Fatalf("Failed loading config values: %s\n", err.Error())
	}

	// Load locales
	localeBundle = initLocaleBundle()

	// Setup Livekit RoomService Client
	lkURL := &url.URL{
		Scheme: "https",
		Host:   mainConfig.Livekit.Host,
	}
	if mainConfig.Environment == "development" {
		lkURL.Scheme = "http"
	}
	lkRoomServiceClient = lksdk.NewRoomServiceClient(lkURL.String(), mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)

	backContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup database client
	log.Println("Connecting to DB")
	dbClient, err := mongo.Connect(backContext, options.Client().ApplyURI(mainConfig.MongoURL.String()))
	defer dbClient.Disconnect(backContext)
	if err != nil {
		log.Fatalf("Failed connecting to DB: %s\n", err.Error())
	}
	mainDB = dbClient.Database(mainConfig.Database.Name)
	err = createIndexes(backContext)
	if err != nil {
		log.Fatalf("Failed creating indexes: %s\n", err.Error())
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

	e.Validator = &CustomValidator{validator: mainValidator}
	e.Renderer = &Template{
		templates: template.Must(template.New("tmpl").Delims("{%", "%}").ParseGlob("audon-fe/dist/index.html")),
	}

	// Setup session middleware (currently Audon stores all client data in cookie)
	log.Println("Connecting to Redis")
	redisStore, err := redisstore.NewRedisStore(backContext, redisClient)
	if err != nil {
		log.Fatalf("Failed connecting to Redis: %s\n", err.Error())
	}
	defer redisStore.Close()
	redisStore.KeyGen(func() (string, error) {
		k := make([]byte, 64)
		if _, err := rand.Read(k); err != nil {
			return "", err
		}
		return strings.TrimRight(base32.StdEncoding.EncodeToString(k), "="), nil
	})
	redisStore.KeyPrefix("session_")
	sessionOptions := sessions.Options{
		Path:     "/",
		Domain:   mainConfig.LocalDomain,
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteDefaultMode,
		Secure:   true,
	}
	if mainConfig.Environment == "development" {
		sessionOptions.Domain = ""
		sessionOptions.SameSite = http.SameSiteDefaultMode
		sessionOptions.Secure = false
		sessionOptions.MaxAge = 3600 * 24
		sessionOptions.HttpOnly = false
	}
	redisStore.Options(sessionOptions)
	e.Use(session.Middleware(redisStore))

	// Setup caches
	roomSessionCache = ttlcache.New(ttlcache.WithTTL[string, *SessionData](24 * time.Hour))
	webhookTimerCache = ttlcache.New(ttlcache.WithTTL[string, *time.Timer](60 * time.Second))
	go roomSessionCache.Start()
	go webhookTimerCache.Start()

	e.POST("/app/login", loginHandler)
	e.GET("/app/oauth", oauthHandler)
	e.GET("/app/verify", verifyHandler)
	e.POST("/app/logout", logoutHandler)
	e.GET("/app/preview/:id", previewRoomHandler)
	e.GET("/app/user/:id", getUserHandler)

	e.POST("/app/webhook", livekitWebhookHandler)

	api := e.Group("/api", authMiddleware)
	api.GET("/token", getUserTokenHandler)
	api.GET("/room", getStatusHandler)
	api.POST("/room", createRoomHandler)
	api.DELETE("/room", leaveRoomHandler)
	api.POST("/room/:id", joinRoomHandler)
	api.PATCH("/room/:id", updateRoomHandler)
	api.DELETE("/room/:id", closeRoomHandler)
	api.PUT("/room/:room/:user", updatePermissionHandler)

	e.Static("/assets", "audon-fe/dist/assets")
	e.Static("/static", "audon-fe/dist/static")
	e.Static("/storage", mainConfig.StorageDir)
	e.GET("/r/:id", renderRoomHandler)
	e.GET("/*", func(c echo.Context) error {
		return c.Render(http.StatusOK, "tmpl", &TemplateData{Config: &mainConfig.AppConfigBase})
	})
	// e.File("/*", "audon-fe/dist/index.html")

	// use anonymous func to support graceful shutdown
	go func() {
		if err := e.Start(":8100"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatalf("Shutting down the server: %s\n", err.Error())
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	// Use a buffered channel to avoid missing signals as recommended for signal.Notify
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	e.Logger.Print("Attempting graceful shutdown")
	defer shutdownCancel()
	roomSessionCache.DeleteAll()
	webhookTimerCache.DeleteAll()
	if err := e.Shutdown(shutdownCtx); err != nil {
		e.Logger.Fatalf("Failed shutting down gracefully: %s\n", err.Error())
	}
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
	if redirPath == "" {
		redirPath = "/"
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
		Scopes:       "read:accounts read:follows write:accounts",
		Website:      "https://codeberg.org/nmkj/audon",
		RedirectURIs: redirectURI,
	}

	// mastAppConfigBase = conf

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

	if data == nil {
		sess.Values[SESSION_DATASTORE_NAME] = ""
	} else {
		sess.Values[SESSION_DATASTORE_NAME] = data
	}

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
