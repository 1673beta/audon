package main

import (
	"net/http"
	"time"

	"github.com/jaevor/go-nanoid"
	"github.com/labstack/echo/v4"
	"github.com/livekit/protocol/auth"
	"go.mongodb.org/mongo-driver/mongo"
)

func createRoomHandler(c echo.Context) (err error) {
	room := new(Room)
	if err = c.Bind(room); err != nil {
		return ErrInvalidRequestFormat
	}
	if err = mainValidator.StructExcept(room, "RoomID"); err != nil {
		return wrapValidationError(err)
	}

	canonic, err := nanoid.Standard(16)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	room.RoomID = canonic()

	sess, err := getSession(c)
	if err != nil {
		c.Logger().Error(err)
		return ErrSessionNotAvailable
	}
	sessData, err := getSessionData(sess)
	if err != nil {
		return ErrInvalidCookie
	}

	var host *AudonUser
	host, err = findUserByID(c.Request().Context(), sessData.AudonID)
	if err == mongo.ErrNoDocuments {
		return c.JSON(http.StatusNotFound, []string{sessData.AudonID})
	} else if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	room.Host = host

	for _, cohost := range room.CoHost {
		cohostUser, err := findUserByRemote(c.Request().Context(), cohost.RemoteID, cohost.RemoteURL)
		if err == nil {
			cohost.AudonID = cohostUser.AudonID
		}
	}
}

func getJoinToken(apiKey, apiSecret, room, identity string) (string, error) {
	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	at.AddGrant(grant).
		SetIdentity(identity).
		SetValidFor(time.Hour)

	return at.ToJWT()
}
