package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jaevor/go-nanoid"
	"github.com/jellydator/ttlcache/v3"
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

	// check if user is already hosting or cohosting
	lkRooms, err := host.GetCurrentLivekitRooms(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	for _, r := range lkRooms {
		meta, err := getRoomMetadataFromLivekitRoom(r)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		if meta.IsHost(host) || meta.IsCoHost(host) {
			return ErrOperationNotPermitted
		}
	}

	coll := mainDB.Collection(COLLECTION_ROOM)

	now := time.Now().UTC()

	// TODO: use a job scheduler to manage rooms?

	room.EndedAt = time.Time{}

	canonic, err := nanoid.Standard(16)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	room.RoomID = canonic()

	room.CreatedAt = now

	// if cohosts are already registered, retrieve their data from DB
	for i, cohost := range room.CoHosts {
		cohostUser, err := findUserByWebfinger(c.Request().Context(), cohost.Webfinger)
		if err == nil {
			room.CoHosts[i] = cohostUser
		}
	}

	if _, insertErr := coll.InsertOne(c.Request().Context(), room); insertErr != nil {
		c.Logger().Error(insertErr)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// Create livekit room
	roomMetadata := &RoomMetadata{Room: room, Speakers: []*AudonUser{}, Kicked: []*AudonUser{}, MastodonAccounts: make(map[string]*MastodonAccount)}
	metadata, _ := json.Marshal(roomMetadata)
	_, err = lkRoomServiceClient.CreateRoom(c.Request().Context(), &livekit.CreateRoomRequest{
		Name:     room.RoomID,
		Metadata: string(metadata),
	})
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusConflict)
	}
	countdown := time.NewTimer(mainConfig.Livekit.EmptyRoomTimeout)
	orphanRooms.Set(room.RoomID, true, ttlcache.DefaultTTL)

	go func(r *Room, logger echo.Logger) {
		<-countdown.C

		if orphaned := orphanRooms.Get(r.RoomID); orphaned == nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if !r.IsAnyomeInLivekitRoom(ctx) {
			if err := endRoom(ctx, r); err != nil {
				logger.Error(err)
			}
		}
	}(room, c.Logger())

	return c.String(http.StatusCreated, room.RoomID)
}

type RoomUpdateRequest struct {
	Title       string          `bson:"title" json:"title" validate:"required,max=100,printascii|multibyte"`
	Description string          `bson:"description" json:"description" validate:"max=500,ascii|multibyte"`
	Restriction JoinRestriction `bson:"restriction" json:"restriction"`
}

func updateRoomHandler(c echo.Context) (err error) {
	roomID := c.Param("id")
	if err := mainValidator.Var(&roomID, "required,printascii"); err != nil {
		return wrapValidationError(err)
	}

	user := c.Get("user").(*AudonUser)

	var room *RoomMetadata
	lkRoom, _ := getRoomInLivekit(c.Request().Context(), roomID)
	if lkRoom != nil {
		room, _ = getRoomMetadataFromLivekitRoom(lkRoom)
	} else {
		dbRoom, err := findRoomByID(c.Request().Context(), roomID)
		if err != nil {
			return ErrRoomNotFound
		}
		room = new(RoomMetadata)
		room.Room = dbRoom
	}

	if !room.IsHost(user) {
		return ErrOperationNotPermitted
	}

	req := new(RoomUpdateRequest)
	if err = c.Bind(req); err != nil {
		return ErrInvalidRequestFormat
	}
	if err = mainValidator.Struct(req); err != nil {
		return wrapValidationError(err)
	}

	coll := mainDB.Collection(COLLECTION_ROOM)
	if _, err = coll.UpdateOne(c.Request().Context(),
		bson.D{{Key: "room_id", Value: roomID}},
		bson.D{{Key: "$set", Value: req}}); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	if lkRoom != nil {
		room.Title = req.Title
		room.Description = req.Description
		room.Restriction = req.Restriction
		newMetadata, _ := json.Marshal(room)
		if _, err := lkRoomServiceClient.UpdateRoomMetadata(c.Request().Context(), &livekit.UpdateRoomMetadataRequest{Room: roomID, Metadata: string(newMetadata)}); err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
	}

	return c.JSON(http.StatusOK, room)
}

// handler for GET to /r/:id
func renderRoomHandler(c echo.Context) error {
	roomID := c.Param("id")
	if err := mainValidator.Var(&roomID, "required,printascii"); err != nil {
		return wrapValidationError(err)
	}

	room, err := findRoomByID(c.Request().Context(), roomID)
	if err != nil {
		return echo.NotFoundHandler(c)
	}

	return c.Render(http.StatusOK, "tmpl", &TemplateData{Config: &mainConfig.AppConfigBase, Room: room})
}

// for preview, this bypasses authentication
func previewRoomHandler(c echo.Context) (err error) {
	roomID := c.Param("id")
	if err := mainValidator.Var(&roomID, "required,printascii"); err != nil {
		return wrapValidationError(err)
	}

	room, _ := findRoomByID(c.Request().Context(), roomID)
	if room != nil && !room.EndedAt.IsZero() && room.EndedAt.Before(time.Now()) {
		return ErrAlreadyEnded
	}

	lkRoom, _ := getRoomInLivekit(c.Request().Context(), roomID)
	if lkRoom == nil {
		return ErrRoomNotFound
	}

	roomMetadata, err := getRoomMetadataFromLivekitRoom(lkRoom)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	if roomMetadata.Restriction != EVERYONE {
		return ErrOperationNotPermitted
	}

	participants, err := lkRoomServiceClient.ListParticipants(c.Request().Context(), &livekit.ListParticipantsRequest{Room: roomID})
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	userMetadata := map[string]*AudonUser{}

	for _, part := range participants.GetParticipants() {
		user := new(AudonUser)
		if err := json.Unmarshal([]byte(part.GetMetadata()), user); err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		userMetadata[part.Identity] = user
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"roomInfo": roomMetadata, "participants": userMetadata})
}

func joinRoomHandler(c echo.Context) (err error) {
	roomID := c.Param("id")
	if err := mainValidator.Var(&roomID, "required,printascii"); err != nil {
		return wrapValidationError(err)
	}

	user := c.Get("user").(*AudonUser)

	room, err := findRoomByID(c.Request().Context(), roomID)
	if err != nil {
		return ErrRoomNotFound
	}

	now := time.Now().UTC()

	// check if room has already ended
	if !room.EndedAt.IsZero() && room.EndedAt.Before(now) {
		return ErrAlreadyEnded
	}

	canTalk := room.IsHost(user) || room.IsCoHost(user) // host and cohost can talk from the beginning

	// check room restriction
	if room.IsPrivate() && !canTalk {
		return c.String(http.StatusForbidden, string(room.Restriction))
	}
	if !canTalk && (room.IsFollowingOnly() || room.IsFollowerOnly() || room.IsFollowingOrFollowerOnly() || room.IsMutualOnly()) {
		data, _ := getSessionData(c)
		mastoClient := getMastodonClient(data)
		if mastoClient == nil {
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		search, err := mastoClient.AccountsSearch(c.Request().Context(), room.Host.Webfinger, 1)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		if len(search) != 1 {
			return ErrOperationNotPermitted
		}
		rels, err := mastoClient.GetAccountRelationships(c.Request().Context(), []string{string(search[0].ID)})
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		if len(rels) != 1 {
			return ErrOperationNotPermitted
		}
		rel := rels[0]
		if (room.IsFollowingOnly() && !rel.FollowedBy) ||
			(room.IsFollowerOnly() && !rel.Following) ||
			(room.IsFollowingOrFollowerOnly() && !(rel.FollowedBy || rel.Following)) ||
			(room.IsMutualOnly() && !(rel.FollowedBy && rel.Following)) {
			return c.String(http.StatusForbidden, string(room.Restriction))
		}
	}

	lkRoom, _ := getRoomInLivekit(c.Request().Context(), room.RoomID) // lkRoom will be nil if it doesn't exist
	if lkRoom == nil {
		return ErrRoomNotFound
	}
	roomMetadata, _ := getRoomMetadataFromLivekitRoom(lkRoom)

	// return 403 if one has been kicked
	for _, kicked := range roomMetadata.Kicked {
		if kicked.Equal(user) {
			return echo.NewHTTPError(http.StatusForbidden)
		}
	}

	// Allows the user to talk if the user is a speaker
	for _, speaker := range roomMetadata.Speakers {
		if speaker.AudonID == user.AudonID {
			canTalk = true
			break
		}
	}

	token, err := getRoomToken(room, user, canTalk)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	resp := &TokenResponse{
		Url:   mainConfig.Livekit.URL.String(),
		Token: token,
		Audon: user,
	}

	mastoAccount := new(MastodonAccount)
	if err := c.Bind(&mastoAccount); err != nil {
		c.Logger().Error(err)
		return ErrInvalidRequestFormat
	}

	roomMetadata.MastodonAccounts[user.AudonID] = mastoAccount

	// Get user's stored avatar if exists
	if user.AvatarFile != "" {
		orig, err := os.ReadFile(user.getAvatarImagePath(user.AvatarFile))
		if err == nil && orig != nil {
			resp.Original = fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(orig))
		} else if orig == nil {
			user.AvatarFile = ""
		}
		// icon, err := os.ReadFile(user.GetGIFAvatarPath())
		// if err == nil && icon != nil {
		// 	resp.Indicator = fmt.Sprintf("data:image/gif;base64,%s", base64.StdEncoding.EncodeToString(icon))
		// }
	}
	avatarLink := mastoAccount.Avatar
	if err := mainValidator.Var(&avatarLink, "required"); err != nil {
		return wrapValidationError(err)
	}
	avatarURL, err := url.Parse(avatarLink)
	if err != nil {
		c.Logger().Error(err)
		return ErrInvalidRequestFormat
	}

	// Retrieve user's current avatar if the old one doesn't exist in Audon.
	// Skips if user is still in another room.
	if already, err := user.InLivekit(c.Request().Context()); !already && err == nil && user.AvatarFile == "" {
		// Download user's avatar
		req, err := http.NewRequest(http.MethodGet, avatarURL.String(), nil)
		if err != nil {
			c.Logger().Error(err)
			return ErrInvalidRequestFormat
		}
		req.Header.Set("User-Agent", USER_AGENT)

		avatarResp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.Logger().Error(err)
			return ErrInvalidRequestFormat
		}
		defer avatarResp.Body.Close()

		fnew, err := io.ReadAll(avatarResp.Body)
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}

		// Generate indicator GIF
		indicator, original, isGIF, err := user.GetIndicator(c.Request().Context(), fnew, room)
		origMime := "image/png"
		if isGIF {
			origMime = "image/gif"
		}
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		resp.Original = fmt.Sprintf("data:%s;base64,%s", origMime, base64.StdEncoding.EncodeToString(original))
		resp.Indicator = fmt.Sprintf("data:image/gif;base64,%s", base64.StdEncoding.EncodeToString(indicator))
	} else if err != nil {
		c.Logger().Error(err)
	}

	// Update room metadata
	roomMetadata.MastodonAccounts[user.AudonID] = mastoAccount
	newMetadata, err := json.Marshal(roomMetadata)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	_, err = lkRoomServiceClient.UpdateRoomMetadata(c.Request().Context(), &livekit.UpdateRoomMetadataRequest{
		Room:     roomID,
		Metadata: string(newMetadata),
	})
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// Store user's session data in cache
	data, _ := getSessionData(c)
	userSessionCache.Set(user.AudonID, data, ttlcache.DefaultTTL)
	orphanRooms.Delete(roomID)

	return c.JSON(http.StatusOK, resp)
}

// intended to be called by room's host or cohost
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

	meta := room.getRoomMetadata(c.Request().Context())
	if meta == nil {
		return ErrRoomNotFound
	}

	// only host or cohost can close the room
	user := c.Get("user").(*AudonUser)
	if !meta.IsHost(user) && !meta.IsCoHost(user) {
		return ErrOperationNotPermitted
	}

	if err := endRoom(c.Request().Context(), room); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

// Client notifies server that user left room
func leaveRoomHandler(c echo.Context) error {
	user := c.Get("user").(*AudonUser)
	still, err := user.InLivekit(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	} else if still {
		return c.NoContent(http.StatusConflict)
	}
	if err := user.ClearUserAvatar(c.Request().Context()); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

func updateRoleHandler(c echo.Context) error {
	roomID := c.Param("id")

	// look up lkRoom in livekit
	lkRoom, exists := getRoomInLivekit(c.Request().Context(), roomID)
	if !exists {
		return ErrRoomNotFound
	}

	lkRoomMetadata, err := getRoomMetadataFromLivekitRoom(lkRoom)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	iam := c.Get("user").(*AudonUser)

	if !(lkRoomMetadata.IsHost(iam) || lkRoomMetadata.IsCoHost(iam)) {
		return ErrOperationNotPermitted
	}

	params := make(map[string]string)
	if err := c.Bind(&params); err != nil {
		return ErrInvalidRequestFormat
	}
	audonID := params["identity"]
	operation := params["op"]
	if !lkRoomMetadata.IsUserInLivekitRoom(c.Request().Context(), audonID) {
		return ErrUserNotFound
	}
	tgtUser, err := findUserByID(c.Request().Context(), audonID)
	if err != nil {
		return ErrUserNotFound
	}
	if lkRoomMetadata.IsHost(tgtUser) || lkRoomMetadata.IsCoHost(tgtUser) {
		return ErrOperationNotPermitted
	}

	newPermission := &livekit.ParticipantPermission{
		CanPublishData: true,
		CanSubscribe:   true,
		CanPublish:     true,
	}

	if operation == "speaker" {
		for _, speaker := range lkRoomMetadata.Speakers {
			if speaker.Equal(tgtUser) {
				return echo.NewHTTPError(http.StatusConflict, "already_speaking")
			}
		}
		lkRoomMetadata.Speakers = append(lkRoomMetadata.Speakers, tgtUser)
	} else if operation == "cohost" {
		lkRoomMetadata.CoHosts = append(lkRoomMetadata.CoHosts, tgtUser)
		coll := mainDB.Collection(COLLECTION_ROOM)
		if _, err = coll.UpdateOne(c.Request().Context(),
			bson.D{{Key: "room_id", Value: roomID}},
			bson.D{{Key: "$set", Value: bson.D{{
				Key:   "cohosts",
				Value: lkRoomMetadata.CoHosts,
			}}}}); err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
	} else if operation == "kick" {
		lkRoomMetadata.Kicked = append(lkRoomMetadata.Kicked, tgtUser)
		lkRoomServiceClient.RemoveParticipant(c.Request().Context(), &livekit.RoomParticipantIdentity{
			Room:     roomID,
			Identity: tgtUser.AudonID,
		})
	} else if operation == "demote" {
		newPermission.CanPublish = false
	} else {
		return ErrInvalidRequestFormat
	}

	if operation == "demote" || operation == "cohost" {
		newSpeakers := make([]*AudonUser, 0, len(lkRoomMetadata.Speakers))
		for _, v := range lkRoomMetadata.Speakers {
			if v.AudonID != tgtUser.AudonID {
				newSpeakers = append(newSpeakers, v)
			}
		}
		lkRoomMetadata.Speakers = newSpeakers
	}

	newMetadata, err := json.Marshal(lkRoomMetadata)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	if operation != "kick" {
		_, err = lkRoomServiceClient.UpdateParticipant(c.Request().Context(), &livekit.UpdateParticipantRequest{
			Room:       roomID,
			Identity:   audonID,
			Permission: newPermission,
		})
		if err != nil {
			c.Logger().Error(err)
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
	}

	metadataRequest := &livekit.UpdateRoomMetadataRequest{Room: roomID, Metadata: string(newMetadata)}

	_, err = lkRoomServiceClient.UpdateRoomMetadata(context.Background(), metadataRequest)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
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
	metadata, _ := json.Marshal(user)

	at.AddGrant(grant).SetIdentity(user.AudonID).SetValidFor(24 * time.Hour).SetMetadata(string(metadata))

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
	if err := mainValidator.Var(&roomID, "required,printascii"); err != nil {
		return nil, err
	}

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
