package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	mastodon "github.com/mattn/go-mastodon"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MastodonAccount struct {
	ID             mastodon.ID             `json:"id"`
	Username       string                  `json:"username"`
	Acct           string                  `json:"acct"`
	DisplayName    string                  `json:"displayName"`
	Locked         bool                    `json:"locked"`
	CreatedAt      time.Time               `json:"createdAt"`
	FollowersCount int64                   `json:"followersCount"`
	FollowingCount int64                   `json:"followingCount"`
	StatusesCount  int64                   `json:"statusesCount"`
	Note           string                  `json:"note"`
	URL            string                  `json:"url"`
	Avatar         string                  `json:"avatar"`
	AvatarStatic   string                  `json:"avatarStatic"`
	Header         string                  `json:"header"`
	HeaderStatic   string                  `json:"headerStatic"`
	Emojis         []mastodon.Emoji        `json:"emojis"`
	Moved          *MastodonAccount        `json:"moved"`
	Fields         []mastodon.Field        `json:"fields"`
	Bot            bool                    `json:"bot"`
	Discoverable   bool                    `json:"discoverable"`
	Source         *mastodon.AccountSource `json:"source"`
}

func getUserHandler(c echo.Context) error {
	audonID := c.Param("id")
	if err := mainValidator.Var(&audonID, "required,printascii"); err != nil {
		return wrapValidationError(err)
	}

	user, err := findUserByID(c.Request().Context(), audonID)
	if err != nil {
		return ErrUserNotFound
	}

	return c.JSON(http.StatusOK, user)
}

func getStatusHandler(c echo.Context) error {
	u := c.Get("user").(*AudonUser)

	status, err := u.GetCurrentRoomStatus(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, status)
}

func redirectUserHandler(c echo.Context) error {
	input := c.Param("webfinger")
	if err := mainValidator.Var(&input, "required,startswith=@,min=4"); err != nil {
		return wrapValidationError(err)
	}
	webfinger := input[1:]
	if err := mainValidator.Var(&webfinger, "email"); err != nil {
		return wrapValidationError(err)
	}

	user, err := findUserByWebfinger(c.Request().Context(), webfinger)
	if err != nil || user == nil {
		return ErrUserNotFound
	}

	coll := mainDB.Collection(COLLECTION_ROOM)
	opts := options.FindOne().SetSort(bson.D{{Key: "created_at", Value: -1}})
	var room Room

	if err := coll.FindOne(c.Request().Context(), bson.D{{Key: "host.audon_id", Value: user.AudonID}}, opts).Decode(&room); err == nil {
		if room.ExistsInLivekit(c.Request().Context()) {
			return c.Redirect(http.StatusFound, fmt.Sprintf("/r/%s", room.RoomID))
		}
	}

	query := make(url.Values)
	query.Add("url", user.RemoteURL)
	result := url.URL{
		Path:       "/error/offline",
		ForceQuery: true,
		OmitHost:   true,
		RawQuery:   query.Encode(),
	}

	return c.Redirect(http.StatusFound, result.String())
}

func (a *AudonUser) Equal(u *AudonUser) bool {
	if a == nil {
		return false
	}

	return a.AudonID == u.AudonID || (a.RemoteID == u.RemoteID && a.RemoteURL == u.RemoteURL)
}

func (a *AudonUser) InLivekit(ctx context.Context) (bool, error) {
	rooms, err := a.GetCurrentLivekitRooms(ctx)
	if err != nil {
		return false, err
	}

	return len(rooms) > 0, nil
}

func (a *AudonUser) ClearUserAvatar(ctx context.Context) error {
	coll := mainDB.Collection(COLLECTION_USER)
	_, err := coll.UpdateOne(ctx,
		bson.D{{Key: "audon_id", Value: a.AudonID}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "avatar", Value: ""}}},
		})
	// if err == nil {
	// 	os.Remove(a.getAvatarImagePath(a.AvatarFile))
	// }
	return err
}

type UserStatus struct {
	RoomID string `json:"roomID"`
	Role   string `json:"role"`
}

func (a *AudonUser) GetCurrentRoomStatus(ctx context.Context) ([]UserStatus, error) {
	rooms, err := a.GetCurrentLivekitRooms(ctx)
	if err != nil {
		return nil, err
	}
	roomList := make([]UserStatus, len(rooms))
	for i, r := range rooms {
		meta, _ := getRoomMetadataFromLivekitRoom(r)
		role := "listener"
		if meta.Room.IsHost(a) {
			role = "host"
		} else if meta.Room.IsCoHost(a) {
			role = "cohost"
		} else if meta.IsSpeaker(a) {
			role = "speaker"
		}
		roomList[i] = UserStatus{
			RoomID: r.GetName(),
			Role:   role,
		}
	}
	return roomList, nil
}
