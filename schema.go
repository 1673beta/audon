package main

import (
	"context"
	"time"

	mastodon "github.com/mattn/go-mastodon"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	SessionData struct {
		MastodonConfig *mastodon.Config
		AuthCode       string
		AudonID        string
	}

	AudonUser struct {
		AudonID   string    `bson:"audon_id" json:"audon_id"`
		RemoteID  string    `bson:"remote_id" json:"remote_id"`
		RemoteURL string    `bson:"remote_url" json:"remote_url"`
		CreatedAt time.Time `bson:"created_at" json:"created_at"`
	}

	Room struct {
		RoomID        string       `bson:"room_id" json:"room_id" validate:"required,alphanum"`
		Title         string       `bson:"title" json:"title" validate:"required,alphanumunicode"`
		Description   string       `bson:"description" json:"description" validate:"alphanumunicode"`
		Host          *AudonUser   `bson:"host" json:"host"`
		CoHost        []*AudonUser `bson:"cohost" json:"cohost"`
		FollowingOnly bool         `bson:"following_only" json:"following_only" validate:"required"`
		FollowerOnly  bool         `bson:"follower_only" json:"follower_only" validate:"required"`
		InviteOnly    bool         `bson:"invite_only" json:"invite_only" validate:"required"`
		InviteToken   string       `bson:"invite_token" json:"invite_token"`
	}
)

const (
	COLLECTION_USER = "user"
	COLLECTION_ROOM = "room"
)

func createIndexes(ctx context.Context) error {
	userColl := mainDB.Collection(COLLECTION_USER)
	userIndexes, err := userColl.Indexes().ListSpecifications(ctx)

	if len(userIndexes) < 3 {
		_, err := userColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "audon_id", Value: 1}},
				Options: options.Index().SetName("audon_id_1").SetUnique(true),
			},
			{
				Keys: bson.D{
					{Key: "remote_url", Value: 1},
					{Key: "remote_id", Value: 1},
				},
			},
		})
		if err != nil {
			return err
		}
	}

	return err
}

func findUserByRemote(ctx context.Context, remoteID, remoteURL string) (*AudonUser, error) {
	var result AudonUser
	coll := mainDB.Collection(COLLECTION_USER)
	if err := coll.FindOne(ctx, bson.D{{Key: "remote_url", Value: remoteURL}, {Key: "remote_id", Value: remoteID}}).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func findUserByID(ctx context.Context, audonID string) (*AudonUser, error) {
	var result AudonUser
	coll := mainDB.Collection(COLLECTION_USER)
	if err := coll.FindOne(ctx, bson.D{{"audon_id", audonID}}).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
