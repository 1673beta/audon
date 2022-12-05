package main

import (
	"crypto/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	mastodon "github.com/mattn/go-mastodon"
	"github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

func verifyTokenInSession(c echo.Context) (bool, *mastodon.Account, error) {
	data, err := getSessionData(c)
	if err != nil {
		return false, nil, err
	}

	if data.MastodonConfig.AccessToken == "" {
		return false, nil, nil
	}
	mastoClient := mastodon.NewClient(data.MastodonConfig)

	acc, err := mastoClient.GetAccountCurrentUser(c.Request().Context())
	user, dbErr := findUserByID(c.Request().Context(), data.AudonID)

	if err != nil || dbErr != nil || string(acc.ID) != user.RemoteID {
		return false, nil, err
	}

	return true, acc, nil
}

// handler for POST to /app/login
func loginHandler(c echo.Context) (err error) {
	serverHost := c.FormValue("server")

	if err = mainValidator.Var(serverHost, "required,hostname,fqdn"); err != nil {
		return wrapValidationError(err)
	}

	valid, _, _ := verifyTokenInSession(c)
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

// handler for GET to /app/oauth?code=****
func oauthHandler(c echo.Context) (err error) {
	authCode := c.QueryParam("code")
	if authCode == "" {
		if errMsg := c.QueryParam("error"); errMsg == "access_denied" {
			return c.Redirect(http.StatusFound, "/login")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "auth_code_required")
	}

	data, err := getSessionData(c)
	if err != nil {
		return err
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
		entropy := ulid.Monotonic(rand.Reader, 0)
		id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), entropy)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		data.AudonID = id.String()
		newUser := AudonUser{
			AudonID:   data.AudonID,
			RemoteID:  string(acc.ID),
			RemoteURL: acc.URL,
			CreatedAt: time.Now().UTC(),
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

	return c.Redirect(http.StatusFound, "/")
	// return c.Redirect(http.StatusFound, "http://localhost:5173")
}

func authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		data, err := getSessionData(c)
		if err != nil {
			return err
		}

		if data.AudonID != "" {
			if user, err := findUserByID(c.Request().Context(), data.AudonID); err == nil {
				c.Set("user", user)
				c.Set("session", data)
				return next(c)
			}
		}

		return echo.NewHTTPError(http.StatusUnauthorized, "login_required")
	}
}
