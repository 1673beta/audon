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

	now := time.Now().UTC()
	if now.Sub(room.ScheduledAt) > 0 {
		room.ScheduledAt = now
	}

	// If CoHosts are already registered, retrieve their AudonID
	for i, cohost := range room.CoHost {
		cohostUser, err := findUserByRemote(c.Request().Context(), cohost.RemoteID, cohost.RemoteURL)
		if err == nil {
			room.CoHost[i] = cohostUser
		}
	}

	room.CreatedAt = now

	coll := mainDB.Collection(COLLECTION_ROOM)
	if _, insertErr := coll.InsertOne(c.Request().Context(), room); insertErr != nil {
		c.Logger().Error(insertErr)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.String(http.StatusCreated, room.RoomID)
}

func getHostToken(room *Room) (string, error) {
	at := auth.NewAccessToken(mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)
	grant := &auth.VideoGrant{
		Room:     room.RoomID,
		RoomJoin: true,
	}
	at.AddGrant(grant).SetIdentity(room.Host.AudonID).SetValidFor(10 * time.Minute)

	return at.ToJWT()
}
