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
	if err = mainValidator.StructExcept(room, "RoomID"); err != nil { // New RoomID will be created, so one in request doesn't matter
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

	// If CoHosts are already registered, retrieve their AudonID
	for i, cohost := range room.CoHost {
		cohostUser, err := findUserByRemote(c.Request().Context(), cohost.RemoteID, cohost.RemoteURL)
		if err == nil {
			room.CoHost[i].AudonID = cohostUser.AudonID
		}
	}

	roomToken, err := getHostToken(room.RoomID, host.AudonID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusCreated, roomToken)
}

func getHostToken(room, identity string) (string, error) {
	at := auth.NewAccessToken(mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)
	grant := &auth.VideoGrant{
		Room:       room,
		RoomJoin:   true,
		RoomRecord: true,
	}
	at.AddGrant(grant).
		SetIdentity(identity).
		SetValidFor(24 * time.Hour)

	return at.ToJWT()
}
