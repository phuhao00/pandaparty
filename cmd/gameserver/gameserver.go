package main

import (
	"context" // Added for MongoDB context
	"github.com/phuhao00/pandaparty/internal/gameserver/coordinator"
	"log"
	"os"
	"os/signal" // Added for signal handling
	"syscall"   // Added for signal handling

	"time" // Added for RPCClient timeout configuration

	"github.com/phuhao00/pandaparty/config"
	consulx "github.com/phuhao00/pandaparty/infra/consul" // Added for Consul client initialization
	"github.com/phuhao00/pandaparty/infra/mongo"
	"github.com/phuhao00/pandaparty/infra/network" // RPCClient, keep if other client calls are made
	redisx "github.com/phuhao00/pandaparty/infra/redis"
	"github.com/phuhao00/pandaparty/internal/gameserver/service" // Added for RoomService

	"fmt"                                                              // For formatting listener address
	pbgs "github.com/phuhao00/pandaparty/infra/pb/protocol/gameserver" // For registering GameService
	"google.golang.org/grpc"                                           // Standard gRPC
	"net"                                                              // For net.Listen
)

const serverName = "gameserver"

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

	// Initialize Consul Client (needed for RoomService to make RPC calls)
	consulClient, err := consulx.NewConsulClient(cfg.Consul)
	serviceID := ""
	if err != nil {
		log.Printf("Failed to initialize Consul client: %v. RPC calls to other services might fail.", err)
		// Depending on requirements, might choose to exit or continue.
		// For now, RoomService can attempt to create its own client if this one is nil.
	} else {
		log.Println("Consul client initialized successfully for GameServer.")
		// Register GameServer RPC service with Consul
		// rpcPort is already validated at the start of main

		registrationHost := cfg.Server.Host // Default
		if cfg.Server.RegisterSelfAsHost {
			registrationHost = serverName // Override with the server's own name
		}

		if registrationHost == "" {
			log.Fatalf("Registration host is empty for %s after config evaluation", serverName)
		}

		serviceID = serverName + "-rpc"
		serviceNameStr := serverName + "-rpc" // Using serviceNameStr to avoid conflict
		err = consulClient.RegisterService(serviceID, serviceNameStr, registrationHost, rpcPort)
		if err != nil {
			log.Printf("Failed to register %s RPC service with Consul: %v", serviceNameStr, err)
		} else {
			log.Printf("%s RPC service registered with Consul successfully on port %d with host %s", serviceNameStr, rpcPort, registrationHost)
		}
	}

	// Initialize RPCClient
	// Use defaultMaxConnsPerEndpoint (e.g., 10) and defaultDialTimeout (e.g., 5s)
	// These values can be made configurable later if needed.
	rpcClient := network.NewRPCClient(consulClient, 10, 5*time.Second)
	// defer rpcClient.CloseAllConnections() // Explicitly closed during graceful shutdown
	log.Println("RPCClient initialized.")

	// Initialize RoomService with RPCClient
	roomService := service.NewRoomService(rpcClient)
	service.Set(roomService)
	log.Println("RoomService initialized.")
	// mongoClient is already initialized above
	listenAddrFriendServer := fmt.Sprintf("0.0.0.0:%d", cfg.Server.ServiceRpcPorts["friendserver"])
	gameServiceHandler, err := coordinator.NewGameCoordinator(listenAddrFriendServer, mongoClient, redisClient.GetReal(), consulClient.GetReal(), nil) // Pass chatService
	log.Println("GameServiceHandler initialized.")
	// Using standard gRPC setup
	listenAddr := fmt.Sprintf("0.0.0.0:%d", rpcPort)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen for %s RPC: %v", serverName, err)
	}
	grpcServer := grpc.NewServer()
	pbgs.RegisterGameServiceServer(grpcServer, gameServiceHandler)
	log.Printf("%s RPC server listening on %s", serverName, listenAddr)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve %s RPC: %v", serverName, err)
		}
	}()

	if mongoClient != nil && redisClient != nil && consulClient != nil {
		log.Printf("%s started successfully with DB, Redis, and Consul.", serverName)
	} else {
		log.Printf("%s started with one or more core components missing (DB, Redis, or Consul).", serverName)
	}

	// Keep the server running
	log.Printf("%s fully initialized and running...", serverName)

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan // Block until a signal is received

	log.Printf("Shutting down %s...", serverName)

	// Stop gRPC server
	if grpcServer != nil {
		log.Println("Stopping gRPC server...")
		grpcServer.GracefulStop() // Or grpcServer.Stop() for immediate shutdown
		log.Println("gRPC server stopped.")
	}

	// Close all client connections
	if rpcClient != nil {
		log.Println("Closing RPC client connections...")
		rpcClient.CloseAllConnections()
		log.Println("RPC client connections closed.")
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

	// Deregister from Consul
	// serviceID was captured when the service was registered
	if consulClient != nil && serviceID != "" {
		log.Printf("Deregistering service %s from Consul...", serviceID)
		if err := consulClient.DeregisterService(serviceID); err != nil {
			log.Printf("Failed to deregister service %s from Consul: %v", serviceID, err)
		} else {
			log.Println("Service deregistered from Consul successfully.")
		}
	}

	log.Printf("%s shut down gracefully.", serverName)
	os.Exit(0)
}
