package main

import (
	"context"
	"encoding/json"
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
		AudonID    string    `bson:"audon_id" json:"audon_id" validate:"alphanum"`
		RemoteID   string    `bson:"remote_id" json:"remote_id" validate:"printascii"`
		RemoteURL  string    `bson:"remote_url" json:"remote_url" validate:"url"`
		Webfinger  string    `bson:"webfinger" json:"webfinger" validate:"email"`
		AvatarFile string    `bson:"avatar" json:"avatar"`
		CreatedAt  time.Time `bson:"created_at" json:"created_at"`
	}

	RoomMetadata struct {
		*Room
		Speakers         []*AudonUser                `json:"speakers"`
		MastodonAccounts map[string]*MastodonAccount `json:"accounts"`
	}

	Room struct {
		RoomID      string          `bson:"room_id" json:"room_id" validate:"required,printascii"`
		Title       string          `bson:"title" json:"title" validate:"required,max=100,printascii|multibyte"`
		Description string          `bson:"description" json:"description" validate:"max=500,ascii|multibyte"`
		Host        *AudonUser      `bson:"host" json:"host"`
		CoHosts     []*AudonUser    `bson:"cohosts" json:"cohosts"`
		Restriction JoinRestriction `bson:"restriction" json:"restriction"`
		Kicked      []*AudonUser    `bson:"kicked" json:"kicked"`
		EndedAt     time.Time       `bson:"ended_at" json:"ended_at"`
		CreatedAt   time.Time       `bson:"created_at" json:"created_at"`
		Advertise   string          `bson:"advertise" json:"advertise"`
	}

	TokenResponse struct {
		Url       string     `json:"url"`
		Token     string     `json:"token"`
		Audon     *AudonUser `json:"audon"`
		Indicator string     `json:"indicator"`
		Original  string     `json:"original"`
	}
)

type JoinRestriction string

const (
	COLLECTION_USER = "user"
	COLLECTION_ROOM = "room"

	EVERYONE              JoinRestriction = "everyone"
	FOLLOWING             JoinRestriction = "following"
	FOLLOWER              JoinRestriction = "follower"
	FOLLOWING_OR_FOLLOWER JoinRestriction = "knowing"
	MUTUAL                JoinRestriction = "mutual"
	PRIVATE               JoinRestriction = "private"
)

func (a *AudonUser) GetCurrentLivekitRooms(ctx context.Context) ([]*livekit.Room, error) {
	resp, err := lkRoomServiceClient.ListRooms(ctx, &livekit.ListRoomsRequest{})
	if err != nil {
		return nil, err
	}
	rooms := resp.GetRooms()
	current := []*livekit.Room{}
	for _, r := range rooms {
		partResp, err := lkRoomServiceClient.ListParticipants(ctx, &livekit.ListParticipantsRequest{
			Room: r.Name,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range partResp.GetParticipants() {
			if p.Identity == a.AudonID {
				current = append(current, r)
				break
			}
		}
	}
	return current, nil
}

func (r *Room) IsFollowingOnly() bool {
	return r.Restriction == FOLLOWING
}

func (r *Room) IsFollowerOnly() bool {
	return r.Restriction == FOLLOWER
}

func (r *Room) IsFollowingOrFollowerOnly() bool {
	return r.Restriction == FOLLOWING_OR_FOLLOWER
}

func (r *Room) IsMutualOnly() bool {
	return r.Restriction == MUTUAL
}

func (r *Room) IsPrivate() bool {
	return r.Restriction == PRIVATE
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

func (r *RoomMetadata) IsSpeaker(u *AudonUser) bool {
	for _, s := range r.Speakers {
		if s.Equal(u) {
			return true
		}
	}
	return false
}

func getRoomMetadataFromLivekitRoom(lkRoom *livekit.Room) (*RoomMetadata, error) {
	metadata := new(RoomMetadata)
	if err := json.Unmarshal([]byte(lkRoom.GetMetadata()), metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}

func (r *Room) ExistsInLivekit(ctx context.Context) bool {
	lkRooms, _ := lkRoomServiceClient.ListRooms(ctx, &livekit.ListRoomsRequest{Names: []string{r.RoomID}})

	return len(lkRooms.GetRooms()) > 0
}

func (r *Room) IsUserInLivekitRoom(ctx context.Context, userID string) bool {
	if r == nil {
		return false
	}
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
				Keys:    bson.D{{Key: "webfinger", Value: 1}},
				Options: options.Index().SetUnique(true),
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

func findUserByWebfinger(ctx context.Context, webfinger string) (*AudonUser, error) {
	var result AudonUser
	coll := mainDB.Collection(COLLECTION_USER)
	if err := coll.FindOne(ctx, bson.D{{Key: "webfinger", Value: webfinger}}).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
