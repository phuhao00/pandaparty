package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/phuhao00/pandaparty/config"
	consulx "github.com/phuhao00/pandaparty/infra/consul"
	"github.com/phuhao00/pandaparty/infra/mongo"
	redisx "github.com/phuhao00/pandaparty/infra/redis"   // Added for Redis client
	"github.com/phuhao00/pandaparty/internal/loginserver" // Added import
)

const (
	serverName = "loginserver"
	// httpServerPort = 8081 // Removed in favor of config
)

func main() {
	log.Printf("%s starting...", serverName)

	// Parse Configuration
	cfg := config.GetServerConfig()
	if cfg == nil {
		log.Println("config is nil")
		return
	}

	// Get HTTP port from config
	httpPort := cfg.Server.LoginServerHTTPPort
	if httpPort == 0 {
		log.Fatalf("HTTP port for %s not configured in server.yaml or is zero", serverName)
	}

	// Initialize Logger
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Configuration loaded successfully")

	// Initialize MongoDB Connection
	mongoClient, err := mongo.NewMongoClient(cfg.Mongo)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		// Depending on requirements, might choose to exit or continue without DB
	} else {
		log.Println("Connected to MongoDB successfully")
		// Optionally, defer mongoClient.Disconnect(context.Background())
	}

	// Initialize Redis Connection
	redisClient, err := redisx.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		// Depending on requirements, might choose to exit or continue without Redis
	} else {
		log.Println("Connected to Redis successfully")
		// Optionally, defer redisClient.Close()
	}

	// Initialize Consul Client
	consulClient, err := consulx.NewConsulClient(cfg.Consul)
	if err != nil {
		log.Printf("Failed to initialize Consul client: %v", err)
	} else {
		log.Println("Consul client initialized successfully")
		// Register LoginServer service with Consul
		// Assuming cfg.Server.Host is the address where other services can reach this server.
		// If loginserver runs on a different machine or needs a specific externally visible IP,
		// that should be configured. For now, using cfg.Server.Host.
		// httpPort is already defined and validated

		registrationHost := cfg.Server.Host // Default
		if cfg.Server.RegisterSelfAsHost {
			registrationHost = serverName // Use "loginserver"
		}

		if registrationHost == "" { // Updated validation
			log.Fatalf("Registration host is empty for %s-http after config evaluation", serverName)
		}

		serviceID := serverName + "-http"
		serviceNameStr := serverName + "-http"                                                    // Using serviceNameStr to avoid conflict with const
		err = consulClient.RegisterService(serviceID, serviceNameStr, registrationHost, httpPort) // Use registrationHost
		if err != nil {
			log.Printf("Failed to register %s with Consul: %v", serviceNameStr, err)
		} else {
			log.Printf("%s registered with Consul successfully on port %d with host %s", serviceNameStr, httpPort, registrationHost) // Log the host used
		}
	}

	// Initialize Login Implementation and Handler
	if mongoClient == nil {
		log.Fatalf("MongoDB client is nil. LoginServer cannot start without a database connection.")
	}
	if redisClient == nil {
		log.Fatalf("Redis client is nil. LoginServer cannot start without Redis for session management.")
	}
	loginImpl := loginserver.NewLoginImpl(mongoClient.GetReal(), redisClient.GetReal(), *cfg) // Pass redisClient
	loginHandler := loginserver.NewLoginHandler(loginImpl)

	// HTTP Server Setup
	http.HandleFunc("/api/login", loginHandler.HandleLogin)
	http.HandleFunc("/api/validate_session", loginHandler.HandleValidateSession) // Register new endpoint

	log.Printf("Starting HTTP server for %s on port %d...", serverName, httpPort)
	// Start HTTP server
	if err := http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil); err != nil {
		log.Fatalf("Failed to start HTTP server for %s: %v", serverName, err)
	}

	log.Printf("%s running...", serverName) // This line might not be reached if ListenAndServe blocks indefinitely and successfully

	// Final status log update (optional, for clarity)
	if mongoClient != nil && redisClient != nil && consulClient != nil {
		log.Printf("%s started successfully with DB, Redis, and Consul.", serverName)
	} else {
		log.Printf("%s started with one or more core components missing.", serverName)
	}
}
