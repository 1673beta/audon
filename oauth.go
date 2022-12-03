package main

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jaevor/go-nanoid"
	"github.com/labstack/echo/v4"
	mastodon "github.com/mattn/go-mastodon"
	"go.mongodb.org/mongo-driver/mongo"
)

func verifyTokenInSession(c echo.Context, sess *sessions.Session) (valid bool, err error) {
	mastConf, err := getSessionData(sess)
	if err != nil {
		return false, err
	}

	if mastConf.MastodonConfig.AccessToken == "" {
		return false, nil
	}
	mastoClient := mastodon.NewClient(mastConf.MastodonConfig)

	_, err = mastoClient.VerifyAppCredentials(c.Request().Context())

	if err != nil {
		return false, err
	}

	return true, nil
}

// handler for POST to /login
func loginHandler(c echo.Context) (err error) {
	serverHost := c.FormValue("server")

	if err = mainValidator.Var(serverHost, "required,hostname|hostname_port"); err != nil {
		return wrapValidationError(err)
	}

	sess, err := getSession(c)
	if err != nil {
		c.Logger().Error(err)
		return ErrSessionNotAvailable
	}

	valid, _ := verifyTokenInSession(c, sess)
	if !valid {
		serverURL := &url.URL{
			Host:   serverHost,
			Scheme: "https",
			Path:   "/",
		}

		appConfig, err := getAppConfig(serverURL.String())
		if err != nil {
			return ErrInvalidRequestFormat
		}
		mastApp, err := mastodon.RegisterApp(c.Request().Context(), appConfig)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "server_not_found")
		}

		userSession := &SessionData{
			MastodonConfig: &mastodon.Config{
				Server:       serverURL.String(),
				ClientID:     mastApp.ClientID,
				ClientSecret: mastApp.ClientSecret,
			},
		}
		if err = writeSessionData(c, userSession); err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		return c.String(http.StatusCreated, mastApp.AuthURI)
	}

	return c.NoContent(http.StatusNoContent)
}

// handler for GET to /oauth?code=****
func oauthHandler(c echo.Context) (err error) {
	authCode := c.QueryParam("code")
	if authCode == "" {
		if errMsg := c.QueryParam("error"); errMsg == "access_denied" {
			return c.Redirect(http.StatusFound, "/login")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "authentication code needed")
	}

	sess, err := getSession(c)
	if err != nil {
		c.Logger().Error(err)
		return ErrSessionNotAvailable
	}

	data, err := getSessionData(sess)
	if err != nil {
		return ErrInvalidCookie
	}
	appConf, err := getAppConfig(data.MastodonConfig.Server)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	data.AuthCode = authCode
	client := mastodon.NewClient(data.MastodonConfig)
	err = client.AuthenticateToken(c.Request().Context(), authCode, appConf.RedirectURIs)
	if err != nil {
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	}
	data.MastodonConfig = client.Config

	acc, err := client.GetAccountCurrentUser(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	}

	coll := mainDB.Collection(COLLECTION_USER)
	if result, dbErr := findUserByRemote(c.Request().Context(), string(acc.ID), acc.URL); dbErr == mongo.ErrNoDocuments {
		// Create user if not yet registered
		canonic, err := nanoid.Standard(21) // Should AudonID be sortable?
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		data.AudonID = canonic()
		newUser := AudonUser{
			AudonID:   data.AudonID,
			RemoteID:  string(acc.ID),
			RemoteURL: acc.URL,
			CreatedAt: time.Now(),
		}
		if _, insertErr := coll.InsertOne(c.Request().Context(), newUser); insertErr != nil {
			c.Logger().Error(insertErr)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
	} else if dbErr != nil {
		c.Logger().Error(dbErr)
		return echo.NewHTTPError(http.StatusInternalServerError)
	} else if result != nil {
		// Set setssion's Audon ID if already registered
		data.AudonID = result.AudonID
	}

	err = writeSessionData(c, data)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// return c.Redirect(http.StatusFound, "/")
	return c.Redirect(http.StatusFound, "http://localhost:5173")
}
