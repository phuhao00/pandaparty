package main

import (
	"fmt"
	"github.com/phuhao00/pandaparty/internal/gatewayserver"
	"log"
	"net"
	"os"
	"os/signal" // Added for signal handling
	"syscall"   // Added for signal handling

	"github.com/phuhao00/pandaparty/config"
	consulx "github.com/phuhao00/pandaparty/infra/consul" // Added for Consul
)

const serverName = "gatewayserver"

func main() {
	log.Printf("%s starting...", serverName)
	// Load configuration
	cfg := config.GetServerConfig()
	if cfg == nil {
		log.Printf("%s config is nil", serverName)
		return
	}
	// Initialize Logger
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Configuration loaded successfully")

	// Initialize Consul Client
	consulClient, err := consulx.NewConsulClient(cfg.Consul)
	if err != nil {
		log.Printf("Failed to initialize Consul client for %s: %v", serverName, err)
		// Making this non-fatal for now, but logging the error.
	} else {
		log.Printf("Consul client initialized successfully for %s", serverName)
	}

	// TCP Port Setup
	gameServerTCPPort := cfg.Server.GatewayGameServerTCPPort
	if gameServerTCPPort == 0 {
		log.Fatalf("TCP port %d for %s not configured or is zero in server.yaml", gameServerTCPPort, serverName)
	}
	// TCP Port Setup
	roomServerTCPPort := cfg.Server.GatewayRoomServerTCPPort
	if roomServerTCPPort == 0 {
		log.Fatalf("TCP port:%d for %s not configured or is zero in server.yaml", roomServerTCPPort, serverName)
	}

	// Actual connection handling loop for TCP would go here
	// TCP Service Registration
	var tcpServiceIDGame string // Declare outside to be accessible in shutdown
	var tcpServiceIDRoom string // Declare outside to be accessible in shutdown
	if consulClient != nil {
		registrationHost := cfg.Server.Host // Default
		if cfg.Server.RegisterSelfAsHost {
			registrationHost = serverName // Override with the server's own name
		}

		if registrationHost == "" {
			log.Fatalf("Registration host is empty for %s TCP service after config evaluation", serverName)
		}

		tcpServiceIDGame = serverName + "-tcp" + "-game"
		tcpServiceNameGame := serverName + "-tcp" + "-game"
		// gameServerTCPPort is already available and validated
		err = consulClient.RegisterService(tcpServiceIDGame, tcpServiceNameGame, registrationHost, gameServerTCPPort)
		if err != nil {
			log.Printf("Failed to register %s TCP service with Consul: %v", tcpServiceNameGame, err)
		} else {
			log.Printf("%s TCP service registered with Consul successfully on port %d with host %s", tcpServiceNameGame, gameServerTCPPort, registrationHost)
		}
		tcpServiceIDRoom = serverName + "-tcp" + "-room"
		tcpServiceNameRoom := serverName + "-tcp" + "-room"
		// gameServerTCPPort is already available and validated
		err = consulClient.RegisterService(tcpServiceIDRoom, tcpServiceNameRoom, registrationHost, gameServerTCPPort)
		if err != nil {
			log.Printf("Failed to register %s TCP service with Consul: %v", tcpServiceNameRoom, err)
		} else {
			log.Printf("%s TCP service registered with Consul successfully on port %d with host %s", tcpServiceNameRoom, gameServerTCPPort, registrationHost)
		}
	}

	// RPC Port Setup
	rpcPort, ok := cfg.Server.ServiceRpcPorts[serverName]
	if !ok || rpcPort == 0 {
		log.Fatalf("RPC port for %s not configured or is zero in server.yaml", serverName)
	}
	rpcListenAddr := fmt.Sprintf("0.0.0.0:%d", rpcPort)
	log.Printf("Starting Gateway RPC listener on %s", rpcListenAddr)
	// Placeholder for actual RPC server setup
	// Simulating listening on the RPC port for now
	rpcLis, err := net.Listen("tcp", rpcListenAddr) // Using net.Listen for simulation
	if err != nil {
		log.Fatalf("Failed to start (simulated) RPC listener for %s: %v", serverName, err)
	}
	// defer rpcLis.Close() // Explicitly closed during graceful shutdown
	log.Printf("Gateway RPC listener (simulated) active on %s", rpcListenAddr)
	// Actual RPC server (e.g. gRPC) would be started here

	// RPC Service Registration
	var rpcServiceID string // Declare outside to be accessible in shutdown
	if consulClient != nil {
		registrationHost := cfg.Server.Host // Default
		if cfg.Server.RegisterSelfAsHost {
			registrationHost = serverName // Override with the server's own name
		}

		if registrationHost == "" {
			log.Fatalf("Registration host is empty for %s RPC service after config evaluation", serverName)
		}

		rpcServiceID := serverName + "-rpc"
		rpcServiceName := serverName + "-rpc"
		// rpcPort is already available and validated
		err = consulClient.RegisterService(rpcServiceID, rpcServiceName, registrationHost, rpcPort)
		if err != nil {
			log.Printf("Failed to register %s RPC service with Consul: %v", rpcServiceName, err)
		} else {
			log.Printf("%s RPC service registered with Consul successfully on port %d with host %s", rpcServiceName, rpcPort, registrationHost)
		}
	}
	tcpListenGameAddr := fmt.Sprintf("0.0.0.0:%d", gameServerTCPPort)
	tcpListenRoomAddr := fmt.Sprintf("0.0.0.0:%d", roomServerTCPPort)
	log.Printf("Starting Gateway   TCP listener on %s,", tcpListenGameAddr)
	gateway, err := gatewayserver.NewGateway(tcpListenGameAddr, tcpListenRoomAddr,
		nil, nil, consulClient, rpcLis, "")
	if err != nil {
		log.Fatalf("Failed to initialize gateway server for %s: %v", serverName, err)
	}
	err = gateway.Start()
	if err != nil {
		log.Fatalf("Failed to start gateway server for %s: %v", serverName, err)
	}

	log.Printf("%s running...", serverName)

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan // Block until a signal is received

	log.Printf("%s shutting down...", serverName)
	err = gateway.Stop()
	if err != nil {
		log.Printf("Failed to stop gateway server for %s: %v", serverName, err)
	}
	// Deregister from Consul
	if consulClient != nil {
		if tcpServiceIDGame != "" {
			log.Printf("Deregistering TCP game service %s from Consul...", tcpServiceIDGame)
			if err := consulClient.DeregisterService(tcpServiceIDGame); err != nil {
				log.Printf("Failed to deregister TCP game service %s from Consul: %v", tcpServiceIDGame, err)
			} else {
				log.Println("TCP game service deregistered from Consul successfully.")
			}
		}
		if tcpServiceIDRoom != "" {
			log.Printf("Deregistering TCP room  service %s from Consul...", tcpServiceIDRoom)
			if err := consulClient.DeregisterService(tcpServiceIDRoom); err != nil {
				log.Printf("Failed to deregister TCP room service %s from Consul: %v", tcpServiceIDRoom, err)
			} else {
				log.Println("TCP room service deregistered from Consul successfully.")
			}
		}
		if rpcServiceID != "" {
			log.Printf("Deregistering RPC service %s from Consul...", rpcServiceID)
			if err := consulClient.DeregisterService(rpcServiceID); err != nil {
				log.Printf("Failed to deregister RPC service %s from Consul: %v", rpcServiceID, err)
			} else {
				log.Println("RPC service deregistered from Consul successfully.")
			}
		}
	}

	log.Printf("%s shut down gracefully.", serverName)
	os.Exit(0)
}
