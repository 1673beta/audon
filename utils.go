package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
	mastodon "github.com/mattn/go-mastodon"
)

const USER_AGENT = "Audon/0.1.0"

// RegisterApp returns the mastodon application.
func registerApp(ctx context.Context, appConfig *mastodon.AppConfig) (*mastodon.Application, error) {
	params := url.Values{}
	params.Set("client_name", appConfig.ClientName)
	if appConfig.RedirectURIs == "" {
		params.Set("redirect_uris", "urn:ietf:wg:oauth:2.0:oob")
	} else {
		params.Set("redirect_uris", appConfig.RedirectURIs)
	}
	params.Set("scopes", appConfig.Scopes)
	params.Set("website", appConfig.Website)

	u, err := url.Parse(appConfig.Server)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, "/api/v1/apps")

	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", USER_AGENT)
	resp, err := appConfig.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("bad request")
	}

	var app mastodon.Application
	err = json.NewDecoder(resp.Body).Decode(&app)
	if err != nil {
		return nil, err
	}

	u, err = url.Parse(appConfig.Server)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, "/oauth/authorize")
	u.RawQuery = url.Values{
		"scope":         {appConfig.Scopes},
		"response_type": {"code"},
		"redirect_uri":  {app.RedirectURI},
		"client_id":     {app.ClientID},
	}.Encode()

	app.AuthURI = u.String()

	return &app, nil
}

func getMastodonClient(c echo.Context) (*mastodon.Client, error) {
	data, err := getSessionData(c)
	if err != nil || data.MastodonConfig.AccessToken == "" {
		return nil, err
	}
	mastoClient := mastodon.NewClient(data.MastodonConfig)
	mastoClient.UserAgent = USER_AGENT

	return mastoClient, nil
}
