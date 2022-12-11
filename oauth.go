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
	mastoClient.UserAgent = USER_AGENT

	acc, err := mastoClient.GetAccountCurrentUser(c.Request().Context())
	user, dbErr := findUserByID(c.Request().Context(), data.AudonID)

	if err != nil || dbErr != nil || string(acc.ID) != user.RemoteID {
		return false, nil, err
	}

	return true, acc, nil
}

type LoginRequest struct {
	ServerHost string `validate:"required,hostname,fqdn" form:"server"`
	Redirect   string `validate:"url_encoded" form:"redir"`
}

// handler for POST to /app/login
func loginHandler(c echo.Context) (err error) {
	req := new(LoginRequest)

	if err = c.Bind(req); err != nil {
		return ErrInvalidRequestFormat
	}
	if err = mainValidator.Struct(req); err != nil {
		return wrapValidationError(err)
	}

	valid, _, _ := verifyTokenInSession(c)
	if !valid {
		serverURL := &url.URL{
			Host:   req.ServerHost,
			Scheme: "https",
			Path:   "/",
		}
		if req.Redirect == "" {
			req.Redirect = "/"
		}

		appConfig, err := getAppConfig(serverURL.String(), req.Redirect)
		if err != nil {
			return ErrInvalidRequestFormat
		}
		// mastApp, err := mastodon.RegisterApp(c.Request().Context(), appConfig)
		mastApp, err := registerApp(c.Request().Context(), appConfig)
		if err != nil {
			c.Logger().Error(err)
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

type OAuthRequest struct {
	Code     string `query:"code"`
	Redirect string `query:"redir"`
}

// handler for GET to /app/oauth?code=****
func oauthHandler(c echo.Context) (err error) {
	req := new(OAuthRequest)

	if err = c.Bind(req); err != nil {
		return ErrInvalidRequestFormat
	}

	if req.Code == "" {
		if errMsg := c.QueryParam("error"); errMsg == "access_denied" {
			return c.Redirect(http.StatusFound, "/login")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "auth_code_required")
	}
	// if req.Redirect == "" {
	// 	req.Redirect = "/"
	// }

	data, err := getSessionData(c)
	if err != nil {
		return err
	}
	appConf, err := getAppConfig(data.MastodonConfig.Server, req.Redirect)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	data.AuthCode = req.Code
	client := mastodon.NewClient(data.MastodonConfig)
	client.UserAgent = USER_AGENT
	err = client.AuthenticateToken(c.Request().Context(), req.Code, appConf.RedirectURIs)
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

	return c.Redirect(http.StatusFound, req.Redirect)
	// return c.Redirect(http.StatusFound, "http://localhost:5173")
}

func getOAuthTokenHandler(c echo.Context) (err error) {
	data, ok := c.Get("data").(*SessionData)
	if !ok {
		return ErrInvalidSession
	}

	return c.JSON(http.StatusOK, &TokenResponse{
		Url:     data.MastodonConfig.Server,
		Token:   data.MastodonConfig.AccessToken,
		AudonID: data.AudonID,
	})
}

func logoutHandler(c echo.Context) (err error) {
	data, err := getSessionData(c)
	if err == nil && data.AudonID != "" {
		mastoURL, err := url.Parse(data.MastodonConfig.Server)
		if err != nil {
			return ErrInvalidRequestFormat
		}
		mastoURL = mastoURL.JoinPath("oauth", "revoke")
		formValues := url.Values{}
		formValues.Add("client_id", data.MastodonConfig.ClientID)
		formValues.Add("client_secret", data.MastodonConfig.ClientSecret)
		formValues.Add("token", data.MastodonConfig.AccessToken)
		resp, err := http.PostForm(mastoURL.String(), formValues)
		if err == nil && resp.StatusCode == http.StatusOK {
			return c.NoContent(http.StatusOK)
		}
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	return echo.NewHTTPError(http.StatusUnauthorized, "login_required")
}

func authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		data, err := getSessionData(c)
		if err == nil && data.AudonID != "" {
			if user, err := findUserByID(c.Request().Context(), data.AudonID); err == nil {
				c.Set("user", user)
				c.Set("data", data)
				return next(c)
			}
		}

		return echo.NewHTTPError(http.StatusUnauthorized, "login_required")
	}
}
