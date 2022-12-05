package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jaevor/go-nanoid"
	"github.com/labstack/echo/v4"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// handler for POST to /api/room
func createRoomHandler(c echo.Context) error {
	room := new(Room)
	if err := c.Bind(room); err != nil {
		return ErrInvalidRequestFormat
	}
	if err := mainValidator.StructExcept(room, "RoomID"); err != nil { // New RoomID will be created, so one in request doesn't matter
		return wrapValidationError(err)
	}

	canonic, err := nanoid.Standard(16)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	room.RoomID = canonic()

	host := c.Get("user").(*AudonUser)
	room.Host = host

	now := time.Now().UTC()
	if now.After(room.ScheduledAt) {
		room.ScheduledAt = now
	}

	// if cohosts are already registered, retrieve their data from DB
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

func joinRoomHandler(c echo.Context) (err error) {
	roomID := c.Param("id")
	if err := mainValidator.Var(&roomID, "required,printascii"); err != nil {
		return wrapValidationError(err)
	}

	user := c.Get("user").(*AudonUser)

	room, err := findRoomByID(c.Request().Context(), roomID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	now := time.Now().UTC()

	// check if room is not yet started
	if room.ScheduledAt.After(now) {
		return echo.NewHTTPError(http.StatusConflict, "not_yet_started")
	}

	// check if room has already ended
	if !room.EndedAt.IsZero() && room.EndedAt.Before(now) {
		return echo.NewHTTPError(http.StatusGone, "already_ended")
	}

	// return 403 if one has been kicked
	for _, kicked := range room.Kicked {
		if kicked.Equal(user) {
			return echo.NewHTTPError(http.StatusForbidden)
		}
	}

	token, err := getRoomToken(room, user.AudonID, room.IsHost(user) || room.IsCoHost(user)) // host and cohost can talk from the beginning
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// Create room in LiveKit

	return c.JSON(http.StatusOK, token)
}

// intended to be called by room's host
func closeRoomHandler(c echo.Context) error {
	roomID := c.Param("id")
	if err := mainValidator.Var(&roomID, "required,printascii"); err != nil {
		return wrapValidationError(err)
	}

	// retrieve room info from the given room ID
	room, err := findRoomByID(c.Request().Context(), roomID)
	if err == mongo.ErrNoDocuments {
		return c.String(http.StatusNotFound, "room_not_found")
	} else if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// only host can close the room
	user := c.Get("user").(*AudonUser)
	if !room.IsHost(user) {
		return c.String(http.StatusForbidden, "must_be_host")
	}

	if err := endRoom(c.Request().Context(), room); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func getRoomToken(room *Room, identity string, canTalk bool) (string, error) {
	at := auth.NewAccessToken(mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)
	grant := &auth.VideoGrant{
		Room:       room.RoomID,
		RoomJoin:   true,
		CanPublish: &canTalk,
	}
	at.AddGrant(grant).SetIdentity(identity).SetValidFor(10 * time.Minute)

	return at.ToJWT()
}

func findRoomByID(ctx context.Context, roomID string) (*Room, error) {
	var room Room
	collRoom := mainDB.Collection(COLLECTION_ROOM)
	if err := collRoom.FindOne(ctx, bson.D{{Key: "room_id", Value: roomID}}).Decode(&room); err != nil {
		return nil, err
	}
	return &room, nil
}

func endRoom(ctx context.Context, room *Room) error {
	if room == nil {
		return errors.New("room cannot be nil")
	}

	if !room.EndedAt.IsZero() {
		return nil
	}

	now := time.Now().UTC()

	collRoom := mainDB.Collection(COLLECTION_ROOM)
	if _, err := collRoom.UpdateOne(ctx,
		bson.D{{Key: "room_id", Value: room.RoomID}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "ended_at", Value: now}}},
		}); err != nil {
		return err
	}

	rooms, err := lkRoomServiceClient.ListRooms(ctx, &livekit.ListRoomsRequest{Names: []string{room.RoomID}})
	if err == nil && len(rooms.Rooms) != 0 {
		_, err := lkRoomServiceClient.DeleteRoom(ctx, &livekit.DeleteRoomRequest{Room: room.RoomID})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}
