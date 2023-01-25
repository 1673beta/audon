package main

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	mastodon "github.com/mattn/go-mastodon"
	"go.mongodb.org/mongo-driver/bson"
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

	ids, err := u.GetCurrentRoomIDs(c.Request().Context())
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, ids)
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

func (a *AudonUser) GetCurrentRoomIDs(ctx context.Context) ([]string, error) {
	rooms, err := a.GetCurrentLivekitRooms(ctx)
	if err != nil {
		return nil, err
	}
	roomIDs := make([]string, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.GetName()
	}
	return roomIDs, nil
}
