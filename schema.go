package main

import (
	"context"
	"time"

	"github.com/livekit/protocol/livekit"
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
		AudonID   string    `bson:"audon_id" json:"audon_id" validate:"alphanum"`
		RemoteID  string    `bson:"remote_id" json:"remote_id" validate:"printascii"`
		RemoteURL string    `bson:"remote_url" json:"remote_url" validate:"url"`
		CreatedAt time.Time `bson:"created_at" json:"created_at"`
	}

	Room struct {
		RoomID        string       `bson:"room_id" json:"room_id" validate:"required,printascii"`
		Title         string       `bson:"title" json:"title" validate:"required,max=100,printascii|multibyte"`
		Description   string       `bson:"description" json:"description" validate:"max=500,ascii|multibyte"`
		Host          *AudonUser   `bson:"host" json:"host"`
		CoHosts       []*AudonUser `bson:"cohost" json:"cohosts,omitempty"`
		FollowingOnly bool         `bson:"following_only" json:"following_only"`
		FollowerOnly  bool         `bson:"follower_only" json:"follower_only"`
		MutualOnly    bool         `bson:"mutual_only" json:"mutual_only"`
		Kicked        []*AudonUser `bson:"kicked" json:"kicked,omitempty"`
		ScheduledAt   time.Time    `bson:"scheduled_at" json:"scheduled_at"`
		EndedAt       time.Time    `bson:"ended_at" json:"ended_at"`
		CreatedAt     time.Time    `bson:"created_at" json:"created_at"`
	}

	TokenResponse struct {
		Url     string `json:"url"`
		Token   string `json:"token"`
		AudonID string `json:"audon_id"`
	}
)

const (
	COLLECTION_USER = "user"
	COLLECTION_ROOM = "room"
)

func (a *AudonUser) Equal(u *AudonUser) bool {
	if a == nil {
		return false
	}

	return a.AudonID == u.AudonID || (a.RemoteID == u.RemoteID && a.RemoteURL == u.RemoteURL)
}

func (r *Room) IsCoHost(u *AudonUser) bool {
	if r == nil {
		return false
	}

	for _, cohost := range r.CoHosts {
		if cohost.Equal(u) {
			return true
		}
	}

	return false
}

func (r *Room) IsHost(u *AudonUser) bool {
	return r != nil && r.Host.Equal(u)
}

func (r *Room) IsUserInLivekitRoom(ctx context.Context, userID string) bool {
	participantsInfo, _ := lkRoomServiceClient.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: r.RoomID})
	participants := participantsInfo.GetParticipants()

	for _, info := range participants {
		if info.GetIdentity() == userID {
			return true
		}
	}

	return false
}

func (r *Room) IsAnyomeInLivekitRoom(ctx context.Context) bool {
	participantsInfo, _ := lkRoomServiceClient.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: r.RoomID})
	participants := participantsInfo.GetParticipants()

	return len(participants) > 0
}

func createIndexes(ctx context.Context) error {
	userColl := mainDB.Collection(COLLECTION_USER)
	userIndexes, err := userColl.Indexes().ListSpecifications(ctx)
	if err != nil {
		return err
	}

	if len(userIndexes) < 3 {
		_, err := userColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "audon_id", Value: 1}},
				Options: options.Index().SetUnique(true),
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

	roomColl := mainDB.Collection(COLLECTION_ROOM)
	roomIndexes, err := roomColl.Indexes().ListSpecifications(ctx)
	if err != nil {
		return err
	}

	if len(roomIndexes) < 3 {
		_, err := roomColl.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				Keys:    bson.D{{Key: "room_id", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
			{
				Keys: bson.D{{Key: "host.audon_id", Value: 1}},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
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
	if err := coll.FindOne(ctx, bson.D{{Key: "audon_id", Value: audonID}}).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
