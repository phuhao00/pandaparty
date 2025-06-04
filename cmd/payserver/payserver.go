package main

import (
	"context" // Added for MongoDB context
	"log"
	"os"
	"os/signal" // Added for signal handling
	"syscall"   // Added for signal handling

	"gopkg.in/yaml.v3"

	"fmt" // Added for RPC server address formatting
	"github.com/phuhao00/pandaparty/config"
	consulx "github.com/phuhao00/pandaparty/infra/consul"
	"github.com/phuhao00/pandaparty/infra/mongo"
	"github.com/phuhao00/pandaparty/infra/network" // Added for RPC Server
	nsqx "github.com/phuhao00/pandaparty/infra/nsq"
	redisx "github.com/phuhao00/pandaparty/infra/redis"
	"github.com/phuhao00/pandaparty/internal/payserver" // Added for RPC Handler
)

const serverName = "payserver" // Define serverName constant

func main() {
	log.Println("PayServer starting...")

	// Parse Configuration
	cfg, err := loadConfig("config/server.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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

	// Initialize NSQ Producer
	nsqProducer, err := nsqx.NewProducer(cfg.NSQ)
	if err != nil {
		log.Printf("Failed to initialize NSQ Producer: %v", err)
	} else {
		log.Println("NSQ Producer initialized successfully")
	}
	payServerRPCPort := 0
	var serviceID string // Declare serviceID to be accessible in shutdown
	// Initialize Consul Client
	consulClient, err := consulx.NewConsulClient(cfg.Consul)
	if err != nil {
		log.Printf("Failed to initialize Consul client: %v", err)
	} else {
		log.Println("Consul client initialized successfully")
		// Register PayServer service with Consul, using specific RPC port
		payServerRPCPort, ok := cfg.Server.ServiceRpcPorts[serverName]
		if !ok || payServerRPCPort == 0 {
			log.Fatalf("RPC port for %s not configured in server.yaml (server.servicerpcports.%s)", serverName, serverName)
		}
		if !ok || payServerRPCPort == 0 { // Ensure payServerRPCPort is defined before use
			log.Fatalf("RPC port for %s not configured in server.yaml (server.servicerpcports.%s)", serverName, serverName)
		}

		registrationHost := cfg.Server.Host // Default
		if cfg.Server.RegisterSelfAsHost {
			registrationHost = serverName // Override with the server's own name
		}

		if registrationHost == "" {
			log.Fatalf("Registration host is empty for %s after config evaluation", serverName)
		}

		// serviceID is already declared above
		serviceID = serverName + "-rpc"
		serviceNameStr := serverName + "-rpc" // Using serviceNameStr to avoid conflict with const
		err = consulClient.RegisterService(serviceID, serviceNameStr, registrationHost, payServerRPCPort)
		if err != nil {
			log.Printf("Failed to register %s with Consul: %v", serviceNameStr, err)
		} else {
			log.Printf("%s registered with Consul successfully on port %d with host %s", serviceNameStr, payServerRPCPort, registrationHost)
		}
	}

	// Initialize RPC Server and Handlers for PayServer
	rpcServer, err := network.NewRPCServer(consulClient) // Pass consulClient
	if err != nil {
		log.Fatalf("Failed to create RPC server for %s: %v", serverName, err)
	}

	if mongoClient == nil {
		log.Fatalf("MongoDB client is nil. Cannot initialize PayServerRPCHandler.")
	}
	rpcHandler := payserver.NewPayServerRPCHandler(mongoClient.GetReal(), cfg.Mongo.Database)
	rpcServer.RegisterHandler("GetPaymentStatus", rpcHandler.GetPaymentStatus)
	// Register other payserver RPC handlers here if any

	// payServerRPCPort is already defined and checked above.
	rpcListenAddr := fmt.Sprintf("0.0.0.0:%d", payServerRPCPort)
	log.Printf("Starting RPC server for %s on %s", serverName, rpcListenAddr)
	go func() {
		if err := rpcServer.Listen(rpcListenAddr); err != nil {
			log.Fatalf("RPC server for %s failed to listen: %v", serverName, err)
		}
	}()
	// defer rpcServer.Close() // Explicitly closed during graceful shutdown

	// Initialize pay-specific services here
	log.Println("Initializing pay-specific services...") // Placeholder for pay-specific logic

	// Final status log
	if mongoClient != nil && redisClient != nil && nsqProducer != nil && consulClient != nil && rpcServer != nil {
		log.Printf("%s started successfully with all components (DB, Redis, NSQ, Consul, RPC)", serverName)
	} else {
		log.Printf("%s started with one or more components missing or failed to initialize.", serverName)
	}

	// Keep the server running
	log.Printf("%s running...", serverName)

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan // Block until a signal is received

	log.Printf("Shutting down %s...", serverName)

	// Close RPC Server
	if rpcServer != nil {
		log.Println("Closing RPC server...")
		rpcServer.Close()
		log.Println("RPC server closed.")
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

	// Stop NSQ Producer
	if nsqProducer != nil {
		log.Println("Stopping NSQ producer...")
		nsqProducer.Stop()
		log.Println("NSQ producer stopped.")
	}

	// Deregister from Consul
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

func loadConfig(path string) (*config.ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg config.ServerConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
