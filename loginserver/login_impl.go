package loginserver

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/phuhao00/pandaparty/help" // Add import for help package
	"github.com/phuhao00/pandaparty/infra/pb/model"

	"errors" // For ValidateSession error

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/go-redis/redis/v8" // Added for Redis
	"github.com/phuhao00/pandaparty/config"
	pb "github.com/phuhao00/pandaparty/infra/pb/protocol/login"
)

const (
	playersCollection    = "players"
	sessionKeyPrefix     = "session:"
	sessionKeyExpiration = 24 * time.Hour
)

// LoginImpl handles the core logic for login operations.
type LoginImpl struct {
	mongoClient *mongo.Client
	redisClient *redis.Client // Added Redis client
	dbName      string
}

// NewLoginImpl creates a new instance of LoginImpl.
func NewLoginImpl(mongoClient *mongo.Client, redisClient *redis.Client, cfg config.ServerConfig) *LoginImpl {
	if mongoClient == nil {
		log.Fatalf("NewLoginImpl received a nil mongoClient")
	}
	if redisClient == nil {
		log.Fatalf("NewLoginImpl received a nil redisClient")
	}
	return &LoginImpl{
		mongoClient: mongoClient,
		redisClient: redisClient,        // Store Redis client
		dbName:      cfg.Mongo.Database, // Get DB name from config
	}
}

// ProcessLogin processes the login request, interacting with the database.
// It assumes req.Username maps to the 'nick' field in the Player model.
func (impl *LoginImpl) ProcessLogin(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		log.Println("Login attempt with empty username or password.")
		return &pb.LoginResponse{Success: false, ErrorMessage: "Username and password are required"}, nil
	}

	// Simulate authentication: any non-empty username/password is valid for now.
	log.Printf("Processing login for username: %s", req.Username)

	collection := impl.mongoClient.Database(impl.dbName).Collection(playersCollection)
	filter := bson.M{"nick": req.Username} // Using 'nick' field for username lookup

	var playerDoc model.Player // Use the correctly imported Player type
	err := collection.FindOne(ctx, filter).Decode(&playerDoc)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Player not found, create a new one
			log.Printf("Player with username (nick) '%s' not found. Creating new player.", req.Username)

			// Use the new ID generator instead of UUID
			newPlayerID := help.GetDefaultIDGenerator().GenerateID() // Generate a new player ID with proper format

			playerDoc = model.Player{ // Use the correctly imported Player type
				PlayerId:    newPlayerID, // Player model's Id is string
				Nickname:    req.Username,
				Level:       1,
				Experience:  0,
				CreatedAt:   time.Now().Unix(),
				LastLoginAt: time.Now().Unix(),
				Status:      model.PlayerStatus_PLAYER_STATUS_ONLINE,
				// Initialize other fields as necessary (e.g., Avatar, Gold, etc.)
			}

			_, insertErr := collection.InsertOne(ctx, playerDoc)
			if insertErr != nil {
				log.Printf("Failed to insert new player %s: %v", req.Username, insertErr)
				return &pb.LoginResponse{Success: false, ErrorMessage: "Failed to create new player account."}, fmt.Errorf("failed to insert player: %w", insertErr)
			}
			log.Printf("New player %s created with ID %s", req.Username, newPlayerID)

			// Generate session token using the new ID generator
			sessionToken := help.GenerateSessionID()
			sessionKey := sessionKeyPrefix + sessionToken
			err = impl.redisClient.Set(ctx, sessionKey, newPlayerID, sessionKeyExpiration).Err()
			if err != nil {
				log.Printf("Failed to store session token in Redis for new player %s: %v", newPlayerID, err)
				// Decide if login should fail here. For now, returning error.
				return &pb.LoginResponse{Success: false, ErrorMessage: "Failed to store session for new player."}, fmt.Errorf("failed to store session in Redis: %w", err)
			}
			log.Printf("Session token %s stored in Redis for new player %s", sessionToken, newPlayerID)

			return &pb.LoginResponse{
				Success:      true,
				UserId:       newPlayerID,
				Nickname:     playerDoc.Nickname,
				SessionToken: sessionToken,
			}, nil
		}
		// Other FindOne error
		log.Printf("Error finding player %s: %v", req.Username, err)
		return &pb.LoginResponse{Success: false, ErrorMessage: "Database error while finding player."}, fmt.Errorf("failed to find player: %w", err)
	}

	// Player found
	log.Printf("Player %s (ID: %s) found. Last login: %d", playerDoc.Nickname, playerDoc.PlayerId, playerDoc.LastLoginAt)

	// Update LastLogin time and online status
	update := bson.M{"$set": bson.M{"lastlogin": time.Now().Unix(), "online": true}}
	_, updateErr := collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(false))
	if updateErr != nil {
		log.Printf("Failed to update last login for player %s (ID: %s): %v", playerDoc.Nickname, playerDoc.PlayerId, updateErr)
		// This is a non-critical error for the login flow itself, so we can proceed.
		// The error should be logged for monitoring.
	}

	// Generate session token for existing player using the new ID generator
	sessionToken := help.GenerateSessionID()
	sessionKey := sessionKeyPrefix + sessionToken
	err = impl.redisClient.Set(ctx, sessionKey, playerDoc.PlayerId, sessionKeyExpiration).Err()
	if err != nil {
		log.Printf("Failed to store session token in Redis for player %s: %v", playerDoc.PlayerId, err)
		// Decide if login should fail here. For now, returning error.
		return &pb.LoginResponse{Success: false, ErrorMessage: "Failed to store session for existing player."}, fmt.Errorf("failed to store session in Redis: %w", err)
	}
	log.Printf("Session token %s stored in Redis for player %s", sessionToken, playerDoc.PlayerId)

	return &pb.LoginResponse{
		Success:      true,
		UserId:       playerDoc.PlayerId,
		Nickname:     playerDoc.Nickname,
		SessionToken: sessionToken,
	}, nil
}

// ValidateSession checks if a session token is valid and returns the associated userID.
func (impl *LoginImpl) ValidateSession(ctx context.Context, sessionToken string) (string, error) {
	if sessionToken == "" {
		return "", errors.New("session token cannot be empty")
	}

	sessionKey := sessionKeyPrefix + sessionToken
	userID, err := impl.redisClient.Get(ctx, sessionKey).Result()

	if err == redis.Nil {
		log.Printf("Session token %s not found in Redis or expired.", sessionToken)
		return "", errors.New("session not found or expired")
	} else if err != nil {
		log.Printf("Error retrieving session token %s from Redis: %v", sessionToken, err)
		return "", fmt.Errorf("redis error validating session: %w", err)
	}

	// Optionally, refresh the session expiration here if desired
	// impl.redisClient.Expire(ctx, sessionKey, sessionKeyExpiration)

	log.Printf("Session token %s validated successfully for UserID: %s", sessionToken, userID)
	return userID, nil
}
