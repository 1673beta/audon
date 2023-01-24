package main

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
)

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
	// os.Remove(a.getAvatarImagePath(a.AvatarFile))
	coll := mainDB.Collection(COLLECTION_USER)
	_, err := coll.UpdateOne(ctx,
		bson.D{{Key: "audon_id", Value: a.AudonID}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "avatar", Value: ""}}},
		})
	return err
}
