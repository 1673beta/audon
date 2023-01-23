package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

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

// Updates the avatar of the current user.
func updateAvatar(ctx context.Context, c *mastodon.Client, filename string) (*mastodon.Account, error) {
	u, err := url.Parse(c.Config.Server)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, "/api/v1/accounts/update_credentials")

	avatar, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)

	// h := make(textproto.MIMEHeader)
	// h.Set("Content-Disposition", "form-data; name=\"avatar\"; filename=\"blob\"")
	// h.Set("Content-Type", mimetype.Detect(avatar).String())
	// part, err := mw.CreatePart(h)
	part, err := mw.CreateFormFile("avatar", filepath.Base(filename))
	if err != nil {
		return nil, err
	}
	io.Copy(part, avatar)
	mw.Close()

	req, err := http.NewRequest(http.MethodPatch, u.String(), buf)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+c.Config.AccessToken)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.UserAgent = USER_AGENT
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %d", resp.StatusCode)
	}

	account := new(mastodon.Account)
	err = json.NewDecoder(resp.Body).Decode(account)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func getMastodonClient(data *SessionData) *mastodon.Client {
	if data == nil || data.MastodonConfig.AccessToken == "" {
		return nil
	}
	mastoClient := mastodon.NewClient(data.MastodonConfig)
	mastoClient.UserAgent = USER_AGENT

	return mastoClient
}
