package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/webhook"
	mastodon "github.com/mattn/go-mastodon"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func livekitWebhookHandler(c echo.Context) error {
	authProvider := auth.NewSimpleKeyProvider(mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)
	event, err := webhook.ReceiveWebhookEvent(c.Request(), authProvider)

	if err == webhook.ErrNoAuthHeader {
		return echo.NewHTTPError(http.StatusForbidden)
	}

	if event.GetEvent() == webhook.EventRoomFinished {
		lkRoom := event.GetRoom()
		room, err := findRoomByID(c.Request().Context(), lkRoom.GetName())
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusNotFound)
		}
		if room.EndedAt.IsZero() {
			if err := endRoom(c.Request().Context(), room); err != nil {
				c.Logger().Error(err)
				return echo.NewHTTPError(http.StatusInternalServerError)
			}
		}
	} else if event.GetEvent() == webhook.EventParticipantLeft {
		audonID := event.GetParticipant().GetIdentity()
		user, err := findUserByID(c.Request().Context(), audonID)
		if user == nil || err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusNotFound)
		}
		still, err := user.InLivekit(c.Request().Context())
		if !still && err == nil {
			data := userSessionCache.Get(audonID)
			if data == nil {
				return echo.NewHTTPError(http.StatusGone)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			nextUser, err := findUserByID(ctx, audonID)
			if err != nil {
				log.Println(err)
			}
			nextUser.ClearUserAvatar(ctx)
		}
	} else if event.GetEvent() == webhook.EventRoomStarted {
		// Have the bot advertise the room
		room, err := findRoomByID(c.Request().Context(), event.GetRoom().GetName())
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusNotFound)
		}
		if err == nil && mainConfig.Bot.Enable && room.Advertise != "" && room.Restriction == EVERYONE {
			botClient := mastodon.NewClient(&mastodon.Config{
				Server:       mainConfig.Bot.Server.String(),
				ClientID:     mainConfig.Bot.ClientID,
				ClientSecret: mainConfig.Bot.ClientSecret,
				AccessToken:  mainConfig.Bot.AccessToken,
			})
			botClient.UserAgent = USER_AGENT

			localizer := i18n.NewLocalizer(localeBundle, room.Advertise)
			header := localizer.MustLocalize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "Advertise",
					Other: "@{{.Host}} is streaming now!",
				},
				TemplateData: map[string]string{
					"Host": room.Host.Webfinger,
				},
			})

			messages := []string{
				header,
				fmt.Sprintf(":udon: %s\n🎙️ https://%s/u/@%s", room.Title, mainConfig.LocalDomain, room.Host.Webfinger),
			}
			if room.Description != "" {
				messages = append(messages, room.Description)
			}
			messages = append(messages, "#Audon")
			message := strings.Join(messages, "\n\n")

			if _, err := botClient.PostStatus(c.Request().Context(), &mastodon.Toot{
				Status:     message,
				Language:   room.Advertise,
				Visibility: "public",
			}); err != nil {
				c.Logger().Error(err)
			}
		}
	}

	return c.NoContent(http.StatusOK)
}
