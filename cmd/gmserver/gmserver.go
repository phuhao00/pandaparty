package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure" // Add this import

	"github.com/phuhao00/pandaparty/config"               // Added for config loading
	consulx "github.com/phuhao00/pandaparty/infra/consul" // Added for Consul client
	"github.com/phuhao00/pandaparty/infra/pb/protocol/gm" // Corrected import path for GM protocol messages
	internalgm "github.com/phuhao00/pandaparty/internal/gmserver"
)

const (
	// defaultGMServerPort = "8088" // Removed in favor of config
	serverName = "gmserver"
	// gatewayBaseURL = "http://localhost:8080/gateway" // Placeholder, actual calls via gameServiceClient
)

// GMServer holds dependencies for the GM HTTP server.
type GMServer struct {
	gmService  *internalgm.GMServiceImpl
	httpClient *http.Client // Kept for any direct HTTP calls if necessary, but GM logic uses gmService
	// config *config.ServerBaseConfig // Keep if base server config is used directly
}

// NewGMServer creates a new GMServer instance.
func NewGMServer(gameClient gm.GMServiceClient) *GMServer {
	gmServiceImpl := internalgm.NewGMServiceImpl(gameClient)
	return &GMServer{
		gmService:  gmServiceImpl,
		httpClient: &http.Client{Timeout: 10 * time.Second}, // Kept for other potential uses
	}
}

// writeJSONResponse is a helper to marshal data to JSON and write it to the response.
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Error encoding JSON response: %v", err)
			// http.Error already sent header, this is just for server log
		}
	}
}

// writeErrorResponse is a helper to send a JSON error message.
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	type ErrorResponse struct {
		Error string `json:"error"`
	}
	writeJSONResponse(w, statusCode, ErrorResponse{Error: message})
}

// handleGetPlayerInfo handles requests for player information.
func (gs *GMServer) handleGetPlayerInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMGetPlayerInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received GetPlayerInfo request: %+v", req)
	resp, err := gs.gmService.GetPlayerInfo(r.Context(), &req)
	if err != nil {
		// This error is from the service client itself (e.g. network issue if it were real)
		// GMServiceImpl currently returns responses with RetCode for validation errors, not transport errors.
		log.Printf("GMServer: Error from gmService.GetPlayerInfo: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing GetPlayerInfo: %v", err))
		return
	}
	log.Printf("GMServer: Responding to GetPlayerInfo: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleSendItemToPlayer handles requests to send items to a player.
func (gs *GMServer) handleSendItemToPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMSendItemToPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received SendItemToPlayer request: %+v", req)
	resp, err := gs.gmService.SendItemToPlayer(r.Context(), &req)
	if err != nil {
		log.Printf("GMServer: Error from gmService.SendItemToPlayer: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing SendItemToPlayer: %v", err))
		return
	}
	log.Printf("GMServer: Responding to SendItemToPlayer: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleCreateNotice handles requests to create a new game notice.
func (gs *GMServer) handleCreateNotice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMCreateNoticeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received CreateNotice request: %+v", req)
	resp, err := gs.gmService.CreateNotice(r.Context(), &req)
	if err != nil {
		log.Printf("GMServer: Error from gmService.CreateNotice: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing CreateNotice: %v", err))
		return
	}
	log.Printf("GMServer: Responding to CreateNotice: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleSetPlayerAttribute handles setting a player attribute.
func (gs *GMServer) handleSetPlayerAttribute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMSetPlayerAttributeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received SetPlayerAttribute request: %+v", req)
	resp, err := gs.gmService.SetPlayerAttribute(r.Context(), &req)
	if err != nil {
		log.Printf("GMServer: Error from gmService.SetPlayerAttribute: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing SetPlayerAttribute: %v", err))
		return
	}
	log.Printf("GMServer: Responding to SetPlayerAttribute: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleBanPlayer handles banning a player.
func (gs *GMServer) handleBanPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMBanPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received BanPlayer request: %+v", req)
	resp, err := gs.gmService.BanPlayer(r.Context(), &req)
	if err != nil {
		log.Printf("GMServer: Error from gmService.BanPlayer: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing BanPlayer: %v", err))
		return
	}
	log.Printf("GMServer: Responding to BanPlayer: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleUnbanPlayer handles unbanning a player.
func (gs *GMServer) handleUnbanPlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMUnbanPlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received UnbanPlayer request: %+v", req)
	resp, err := gs.gmService.UnbanPlayer(r.Context(), &req)
	if err != nil {
		log.Printf("GMServer: Error from gmService.UnbanPlayer: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing UnbanPlayer: %v", err))
		return
	}
	log.Printf("GMServer: Responding to UnbanPlayer: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleUpdateNotice handles updating a notice.
func (gs *GMServer) handleUpdateNotice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMUpdateNoticeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received UpdateNotice request: %+v", req)
	resp, err := gs.gmService.UpdateNotice(r.Context(), &req)
	if err != nil {
		log.Printf("GMServer: Error from gmService.UpdateNotice: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing UpdateNotice: %v", err))
		return
	}
	log.Printf("GMServer: Responding to UpdateNotice: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleDeleteNotice handles deleting a notice.
func (gs *GMServer) handleDeleteNotice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}
	var req gm.GMDeleteNoticeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Error decoding request: %v", err))
		return
	}
	defer r.Body.Close()

	log.Printf("GMServer: Received DeleteNotice request: %+v", req)
	resp, err := gs.gmService.DeleteNotice(r.Context(), &req)
	if err != nil {
		log.Printf("GMServer: Error from gmService.DeleteNotice: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing DeleteNotice: %v", err))
		return
	}
	log.Printf("GMServer: Responding to DeleteNotice: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleServerStatus handles getting server status.
func (gs *GMServer) handleServerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { // Typically GET for status, but can be POST if needs body
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Only GET method is allowed")
		return
	}
	// GMServerStatusRequest is empty, so no body decoding needed for GET.
	// If it were POST with body:
	// var req gm.GMServerStatusRequest
	// if err := json.NewDecoder(r.Body).Decode(&req); err != nil { ... }
	// defer r.Body.Close()

	log.Printf("GMServer: Received ServerStatus request")
	resp, err := gs.gmService.GetServerStatus(r.Context(), &gm.GMServerStatusRequest{}) // Pass empty request
	if err != nil {
		log.Printf("GMServer: Error from gmService.GetServerStatus: %v", err)
		writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Error processing ServerStatus: %v", err))
		return
	}
	log.Printf("GMServer: Responding to ServerStatus: %+v", resp)
	writeJSONResponse(w, http.StatusOK, resp)
}

func main() {
	log.Printf("%s starting...", serverName)

	// Load configuration
	cfg := config.GetServerConfig()
	if cfg == nil {
		log.Printf("%s failed to load config", serverName)
		return
	}
	log.Println("Configuration loaded successfully") // Ensure this log is present

	// Initialize Consul Client
	consulClient, err := consulx.NewConsulClient(cfg.Consul)
	if err != nil {
		log.Printf("Failed to initialize Consul client for %s: %v", serverName, err)
		// Non-fatal for now, server continues
	} else {
		log.Printf("Consul client initialized successfully for %s", serverName)
		// Already loaded and checked below
		// httpPort is defined below, ensure it's available for this block
		httpPort := cfg.Server.GMServerHTTPPort
		if httpPort == 0 { // Check early if needed for registration logic
			log.Fatalf("HTTP port for %s not configured or is zero in server.yaml", serverName)
		}

		registrationHost := cfg.Server.Host // Default
		if cfg.Server.RegisterSelfAsHost {
			registrationHost = serverName // Override with the server's own name
		}

		if registrationHost == "" {
			log.Fatalf("Registration host is empty for %s after config evaluation", serverName)
		}

		serviceID := serverName + "-http"
		serviceNameStr := serverName + "-http" // Renamed to avoid conflict with const

		err = consulClient.RegisterService(serviceID, serviceNameStr, registrationHost, httpPort)
		if err != nil {
			log.Printf("Failed to register %s HTTP service with Consul: %v", serviceNameStr, err)
		} else {
			log.Printf("%s HTTP service registered with Consul successfully on port %d with host %s", serviceNameStr, httpPort, registrationHost)
		}
	}

	// Get the target service address for gRPC connection
	// This should be the gameserver or gateway that provides GM services
	gameServerAddr := "localhost:8081" // Default, should come from config
	if cfg.Server.ServiceRpcPorts != nil {
		if port, exists := cfg.Server.ServiceRpcPorts["gameserver"]; exists && port > 0 {
			gameServerAddr = fmt.Sprintf("localhost:%d", port)
		}
	}

	log.Printf("Connecting to game server at: %s", gameServerAddr)

	// Fix: Add insecure credentials to the gRPC client connection
	grpcClient, err := grpc.NewClient(gameServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to game server: %v", err)
	}
	defer func() {
		if err := grpcClient.Close(); err != nil {
			log.Printf("Error closing gRPC connection: %v", err)
		}
	}()

	serviceClient := gm.NewGMServiceClient(grpcClient)
	gmServer := NewGMServer(serviceClient)

	// Use GMServerHTTPPort for HTTP server
	httpPort := cfg.Server.GMServerHTTPPort
	if httpPort == 0 {
		log.Fatalf("HTTP port for %s not configured or is zero in server.yaml", serverName)
	}
	listenAddr := fmt.Sprintf("0.0.0.0:%d", httpPort)

	mux := http.NewServeMux()
	mux.HandleFunc("/gm/getPlayerInfo", gmServer.handleGetPlayerInfo)
	mux.HandleFunc("/gm/sendItemToPlayer", gmServer.handleSendItemToPlayer)
	mux.HandleFunc("/gm/createNotice", gmServer.handleCreateNotice)

	// Add new handlers
	mux.HandleFunc("/gm/setPlayerAttribute", gmServer.handleSetPlayerAttribute)
	mux.HandleFunc("/gm/banPlayer", gmServer.handleBanPlayer)
	mux.HandleFunc("/gm/unbanPlayer", gmServer.handleUnbanPlayer)
	mux.HandleFunc("/gm/updateNotice", gmServer.handleUpdateNotice)
	mux.HandleFunc("/gm/deleteNotice", gmServer.handleDeleteNotice)
	mux.HandleFunc("/gm/serverStatus", gmServer.handleServerStatus)

	log.Printf("%s starting on port %d\n", serverName, httpPort)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("Failed to start %s: %v", serverName, err)
	}
}
