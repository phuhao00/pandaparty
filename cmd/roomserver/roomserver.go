package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal" // Added for signal handling
	"syscall"   // Added for signal handling

	"google.golang.org/grpc"

	"github.com/phuhao00/pandaparty/config"
	"github.com/phuhao00/pandaparty/infra/mongo"
	pbroom "github.com/phuhao00/pandaparty/infra/pb/protocol/room"
	redisx "github.com/phuhao00/pandaparty/infra/redis"
	"github.com/phuhao00/pandaparty/internal/roomserver/coordinator"
)

const serverName = "roomserver"

func main() {
	log.Printf("%s starting...", serverName)

	// Parse Configuration
	cfg := config.GetServerConfig()
	if cfg == nil {
		log.Printf("%s config is nil", serverName)
		return
	}
	// Get RPC port from config
	rpcPort, ok := cfg.Server.ServiceRpcPorts[serverName]
	if !ok || rpcPort == 0 {
		log.Fatalf("RPC port for %s not configured or is zero in server.yaml", serverName)
	}

	// Initialize Logger (using standard log package for now)
	log.SetOutput(os.Stdout) // Example: log to stdout
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Configuration loaded successfully")

	// Initialize MongoDB Connection
	mongoClient, err := mongo.NewMongoClient(cfg.Mongo)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
	} else {
		log.Println("Connected to MongoDB successfully")
	}

	// Initialize Redis Connection
	redisClient, err := redisx.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
	} else {
		log.Println("Connected to Redis successfully")
	}
	// Create module instances
	roomCoordinator := coordinator.NewRoomCoordinator(mongoClient, redisClient)
	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", rpcPort))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", rpcPort, err)
	}
	grpcServer := grpc.NewServer()
	pbroom.RegisterRoomServiceServer(grpcServer, roomCoordinator)
	log.Printf("%s started successfully on port %d", serverName, rpcPort)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()
	defer grpcServer.Stop() // Explicitly closed during graceful shutdown

	// Final status log
	if mongoClient != nil && redisClient != nil {
		log.Printf("%s started successfully with all components (DB, Redis)", serverName)
	} else {
		log.Printf("%s started with one or more components missing or failed to initialize.", serverName)
	}
	// Keep the server running (e.g., by blocking or starting a server)
	log.Printf("%s running...", serverName)
	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan // Block until a signal is received
	log.Printf("Shutting down %s...", serverName)
	// Close gRPC Server
	if grpcServer != nil {
		log.Println("Stopping gRPC server...")
		grpcServer.GracefulStop()
		log.Println("gRPC server stopped.")
	}

	// Disconnect MongoDB
	if mongoClient != nil {
		log.Println("Disconnecting from MongoDB...")
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Printf("Failed to disconnect from MongoDB: %v", err)
		} else {
			log.Println("Disconnected from MongoDB successfully.")
		}
	}

	// Close Redis Client
	if redisClient != nil {
		log.Println("Closing Redis client...")
		if err := redisClient.Close(); err != nil {
			log.Printf("Failed to close Redis client: %v", err)
		} else {
			log.Println("Redis client closed successfully.")
		}
	}

	log.Printf("%s shut down gracefully.", serverName)
	os.Exit(0)
}
