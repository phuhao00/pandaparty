package mongo

import (
	"context"
	"time"

	"github.com/phuhao00/pandaparty/config" // Added import for config
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoClient struct {
	client     *mongo.Client
	collection *mongo.Collection // This specific collection can be for default or specific use like 'config'
	// For more general use, database and collection might be passed to methods.
}

func (m *MongoClient) GetReal() *mongo.Client {
	return m.client
}

func NewMongoClient(cfg config.MongoConfig) (*MongoClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Default timeout for the connect operation itself
	defer cancel()

	clientOptions := options.Client()

	if cfg.URI != "" {
		clientOptions.ApplyURI(cfg.URI)
	} else if len(cfg.Hosts) > 0 {
		clientOptions.SetHosts(cfg.Hosts)
	} else {
		// It's a good idea to have a default or return an error if no connection info is provided
		// For now, assume URI or Hosts will be provided, or it will fail later.
	}

	if cfg.ReplicaSet != "" {
		clientOptions.SetReplicaSet(cfg.ReplicaSet)
	}

	if cfg.Username != "" && cfg.Password != "" {
		cred := options.Credential{
			AuthSource: cfg.AuthSource, // AuthSource can be empty if the user is defined in the default 'admin' db or the db being connected to.
			Username:   cfg.Username,
			Password:   cfg.Password,
		}
		clientOptions.SetAuth(cred)
	}

	if cfg.ConnectTimeoutMS > 0 {
		clientOptions.SetConnectTimeout(time.Duration(cfg.ConnectTimeoutMS) * time.Millisecond)
	}

	if cfg.MaxPoolSize > 0 {
		clientOptions.SetMaxPoolSize(cfg.MaxPoolSize)
	}

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the primary to verify connection.
	// ctxPing, cancelPing := context.WithTimeout(context.Background(), 2*time.Second) // Shorter timeout for ping
	// defer cancelPing()
	// err = client.Ping(ctxPing, readpref.Primary())
	// if err != nil {
	//     client.Disconnect(ctx) // Disconnect if ping fails
	//     return nil, fmt.Errorf("failed to ping mongo: %w", err)
	// }
	// Note: Ping is removed as per current file structure, but good practice for robust connections.

	// The default collection is now set based on cfg.Database and cfg.Collection.
	// Specific operations can still target other databases/collections using client.Database("otherDB").Collection("otherColl")
	collection := client.Database(cfg.Database).Collection(cfg.Collection)

	return &MongoClient{client: client, collection: collection}, nil
}

func (m *MongoClient) InsertConfig(ctx context.Context, doc interface{}) error {
	_, err := m.collection.InsertOne(ctx, doc)
	return err
}

func (m *MongoClient) FindConfig(ctx context.Context, filter interface{}) (*mongo.SingleResult, error) {
	result := m.collection.FindOne(ctx, filter)
	return result, result.Err()
}

func (m *MongoClient) Disconnect(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}
