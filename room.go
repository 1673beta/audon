package main

import (
	"context"
	"encoding/json"
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

	host := c.Get("user").(*AudonUser)
	room.Host = host

	coll := mainDB.Collection(COLLECTION_ROOM)

	now := time.Now().UTC()
	if now.After(room.ScheduledAt) {
		// host is trying to create an instant room even though there is another instant room that wasn't used, assumed that host won't use such rooms
		if cur, err := coll.Find(c.Request().Context(),
			bson.D{
				{Key: "host.audon_id", Value: host.AudonID},
				{Key: "ended_at", Value: time.Time{}}, // host didn't close
				{Key: "$expr", Value: bson.D{ // instant room
					{Key: "$eq", Value: bson.A{"$created_at", "$scheduled_at"}},
				}},
			}); err == nil {
			defer cur.Close(c.Request().Context())

			roomIDsToBeDeleted := []string{}
			for cur.Next(c.Request().Context()) {
				emptyRoom := new(Room)
				if err := cur.Decode(emptyRoom); err == nil {
					if !emptyRoom.IsAnyomeInLivekitRoom(c.Request().Context()) {
						roomIDsToBeDeleted = append(roomIDsToBeDeleted, emptyRoom.RoomID)
					}
				}
			}
			if len(roomIDsToBeDeleted) > 0 {
				coll.DeleteMany(c.Request().Context(), bson.D{{
					Key:   "room_id",
					Value: bson.D{{Key: "$in", Value: roomIDsToBeDeleted}}},
				})
			}
		}

		room.ScheduledAt = now
	} else {
		// TODO: limit the number of rooms one can schedule?
	}

	// TODO: use a job scheduler to manage rooms?

	room.EndedAt = time.Time{}

	canonic, err := nanoid.Standard(16)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	room.RoomID = canonic()

	// if cohosts are already registered, retrieve their data from DB
	for i, cohost := range room.CoHosts {
		cohostUser, err := findUserByRemote(c.Request().Context(), cohost.RemoteID, cohost.RemoteURL)
		if err == nil {
			room.CoHosts[i] = cohostUser
		}
	}

	room.CreatedAt = now
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

	// decline the request if user is already in the room
	if room.IsUserInLivekitRoom(c.Request().Context(), user.AudonID) {
		return echo.NewHTTPError(http.StatusNotAcceptable, "already_in_room")
	}

	now := time.Now().UTC()

	// check if room is not yet started
	if room.ScheduledAt.After(now) {
		return echo.NewHTTPError(http.StatusConflict, "not_yet_started")
	}

	// check if room has already ended
	if !room.EndedAt.IsZero() && room.EndedAt.Before(now) {
		return ErrAlreadyEnded
	}

	// return 403 if one has been kicked
	for _, kicked := range room.Kicked {
		if kicked.Equal(user) {
			return echo.NewHTTPError(http.StatusForbidden)
		}
	}

	token, err := getRoomToken(room, user, room.IsHost(user) || room.IsCoHost(user)) // host and cohost can talk from the beginning
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	resp := &TokenResponse{
		Url:     mainConfig.Livekit.URL.String(),
		Token:   token,
		AudonID: user.AudonID,
	}

	// Create room in LiveKit if it doesn't exist
	metadata, _ := json.Marshal(room)

	_, err = lkRoomServiceClient.CreateRoom(c.Request().Context(), &livekit.CreateRoomRequest{
		Name:     room.RoomID,
		Metadata: string(metadata),
	})
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusConflict)
	}

	return c.JSON(http.StatusOK, resp)
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
		return ErrRoomNotFound
	} else if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// return 410 if the room has already ended
	if !room.EndedAt.IsZero() {
		return ErrAlreadyEnded
	}

	// only host can close the room
	user := c.Get("user").(*AudonUser)
	if !room.IsHost(user) {
		return ErrOperationNotPermitted
	}

	if err := endRoom(c.Request().Context(), room); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

func updatePermissionHandler(c echo.Context) error {
	roomID := c.Param("room")

	// look up room in livekit
	room, exists := getRoomInLivekit(c.Request().Context(), roomID)
	if !exists {
		return ErrRoomNotFound
	}

	audonRoom := new(Room)
	err := json.Unmarshal([]byte(room.Metadata), audonRoom)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	iam := c.Get("user").(*AudonUser)

	if !(audonRoom.IsHost(iam) || audonRoom.IsCoHost(iam)) {
		return ErrOperationNotPermitted
	}

	tgtAudonID := c.Param("user")

	if !audonRoom.IsUserInLivekitRoom(c.Request().Context(), tgtAudonID) {
		return ErrUserNotFound
	}

	permission := new(livekit.ParticipantPermission)
	if err := c.Bind(permission); err != nil {
		return ErrInvalidRequestFormat
	}

	info, err := lkRoomServiceClient.UpdateParticipant(c.Request().Context(), &livekit.UpdateParticipantRequest{
		Room:       roomID,
		Identity:   tgtAudonID,
		Permission: permission,
	})
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, info)
}

func getRoomToken(room *Room, user *AudonUser, canTalk bool) (string, error) {
	at := auth.NewAccessToken(mainConfig.Livekit.APIKey, mainConfig.Livekit.APISecret)
	canPublishData := true
	grant := &auth.VideoGrant{
		Room:           room.RoomID,
		RoomJoin:       true,
		RoomCreate:     false,
		CanPublish:     &canTalk,
		CanPublishData: &canPublishData,
	}
	metadata, _ := json.Marshal(map[string]string{
		"remote_id":  user.RemoteID,
		"remote_url": user.RemoteURL,
	})

	at.AddGrant(grant).SetIdentity(user.AudonID).SetValidFor(10 * time.Minute).SetMetadata(string(metadata))

	return at.ToJWT()
}

func getRoomInLivekit(ctx context.Context, roomID string) (*livekit.Room, bool) {
	rooms, _ := lkRoomServiceClient.ListRooms(ctx, &livekit.ListRoomsRequest{Names: []string{roomID}})
	if len(rooms.GetRooms()) == 0 {
		return nil, false
	}

	return rooms.GetRooms()[0], true
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
