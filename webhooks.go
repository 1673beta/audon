package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/webhook"
)

func livekitWebhookHandler(c echo.Context) error {
	authProvider := auth.NewSimpleKeyProvider(mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)
	event, err := webhook.ReceiveWebhookEvent(c.Request(), authProvider)

	if err == webhook.ErrNoAuthHeader {
		return echo.NewHTTPError(http.StatusForbidden)
	}

	if event.GetEvent() == webhook.EventRoomFinished {
		roomID := event.GetRoom().GetName()
		if err := mainValidator.Var(&roomID, "required,printascii"); err == nil {
			room, err := findRoomByID(c.Request().Context(), roomID)
			if err == nil {
				if err := endRoom(c.Request().Context(), room); err != nil {
					c.Logger().Error(err)
					return echo.NewHTTPError(http.StatusInternalServerError)
				}
			}
		}

		return c.NoContent(http.StatusOK)
	}

	return echo.NewHTTPError(http.StatusNotFound)
}
