package simulator // Changed from package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/looplab/fsm"
	"github.com/phuhao00/dafuweng/config"
	consulx "github.com/phuhao00/pandaparty/infra/consul"
	"log"
	"net"
	"net/http"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/phuhao00/dafuweng/infra/network"
	modelpb "github.com/phuhao00/pandaparty/infra/pb/model"
	pbgs "github.com/phuhao00/pandaparty/infra/pb/protocol/gameserver"
	pbroom "github.com/phuhao00/pandaparty/infra/pb/protocol/room" // Added for room operations
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pbmodel "github.com/phuhao00/pandaparty/infra/pb/model" // Ensure alias for modelpb
)

// LoginRequest mirrors the JSON structure for the login request.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse mirrors the JSON structure for the login response.
type LoginResponse struct {
	UserID       string `json:"user_id"`
	Nickname     string `json:"nickname"`
	SessionToken string `json:"session_token"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message"`
}

// SimulatedClient holds state and methods for a single simulated client.
type SimulatedClient struct {
	UserID           string
	SessionToken     string
	Username         string
	Password         string
	LoginServerAddr  string
	ConsulServerAddr string
	// RoomClient      roompb.RoomServiceClient // Removed
	// grpcConn        *grpc.ClientConn           // Removed
	logger             *log.Logger
	GatewayServiceName string
	GameServiceName    string // Retained as per instructions
	gatewayConn        net.Conn
	// gameServiceGrpcConn *grpc.ClientConn // Removed: Tunneled connection
	// GameServiceClient pbgs.GameServiceClient // Removed: Tunneled client
	directGameServiceConn   *grpc.ClientConn       // Added: Direct gRPC connection to gameserver
	directGameServiceClient pbgs.GameServiceClient // Added: Client for direct gameserver communication
	rpcClient               *network.RPCClient
	consulClient            *consulapi.Client
	gatewayServiceAddress   string           // Added to store discovered gateway address
	RoomServiceName         string           // Added for direct room server calls
	playerFSM               *fsm.FSM         // Added for FSM
	behaviorManager         *BehaviorManager // Added for Behavior Trees
	CurrentRoomID           string           // Added: ID of the room the client is currently in
}

// NewSimulatedClient creates and initializes a new SimulatedClient.
func NewSimulatedClient(loginAddr, consulAddr, username, password, gatewayServiceName, gameServiceName, roomServiceName string, logger *log.Logger, bm *BehaviorManager) (*SimulatedClient, error) {
	consulCfg := consulapi.DefaultConfig()
	consulCfg.Address = consulAddr
	//cClient, err := consulapi.NewClient(consulCfg)
	consulClient, err := consulx.NewConsulClient(config.ConsulConfig{Addr: consulAddr})
	if err != nil {
		logger.Printf("Error creating Consul client: %v", err)
		return nil, fmt.Errorf("failed to create Consul client at %s: %w", consulAddr, err)
	}

	// Initialize RPCClient. Assuming NewRPCClient requires a non-nil consul client.
	// And that it doesn't return an error itself, but might panic if consul client is nil.
	// The check above for cClient error should prevent passing nil cClient.
	rpcClient := network.NewRPCClient(consulClient, 0, 0) // Using 0,0 for default maxConns and timeout.
	if rpcClient == nil {
		// This case implies NewRPCClient might return nil on other failures not just nil consulClient.
		// Or if it's a simple constructor, this might not be reachable if cClient is guaranteed non-nil.
		logger.Printf("Error: NewRPCClient returned nil even with a valid Consul client.")
		return nil, fmt.Errorf("failed to initialize RPCClient")
	}

	sc := &SimulatedClient{
		Username:           username,
		Password:           password,
		LoginServerAddr:    loginAddr,
		ConsulServerAddr:   consulAddr,
		GatewayServiceName: gatewayServiceName,
		GameServiceName:    gameServiceName,
		RoomServiceName:    roomServiceName, // Ensure RoomServiceName is initialized
		logger:             logger,
		consulClient:       consulClient.GetReal(), // Store the initialized Consul client
		rpcClient:          rpcClient,              // Store the initialized RPC client
		behaviorManager:    bm,                     // Initialize BehaviorManager
		CurrentRoomID:      "",                     // Initialize CurrentRoomID
	}

	// Initialize FSM
	// Important: Pass 'sc' (the SimulatedClient instance being created) to NewPlayerFSM.
	// This allows FSM callbacks to access the client's logger and other fields.
	sc.playerFSM = NewPlayerFSM("Idle", sc)

	// Discover and connect to the GameService directly
	if sc.GameServiceName != "" && sc.consulClient != nil {
		serviceEntries, _, err := sc.consulClient.Health().Service(sc.GameServiceName, "", true, nil)
		if err != nil {
			logger.Printf("Error discovering GameService '%s' via Consul: %v", sc.GameServiceName, err)
			// Depending on requirements, this could be a fatal error for client setup
			return nil, fmt.Errorf("failed to query consul for GameService '%s': %w", sc.GameServiceName, err)
		} else if len(serviceEntries) == 0 {
			logger.Printf("No healthy instances found for GameService '%s' via Consul.", sc.GameServiceName)
			return nil, fmt.Errorf("no healthy instances found for GameService '%s'", sc.GameServiceName)
		} else {
			// Use the first healthy instance
			instance := serviceEntries[0]
			address := instance.Service.Address
			if address == "" {
				address = instance.Node.Address
			}
			port := instance.Service.Port
			gameServiceAddress := fmt.Sprintf("%s:%d", address, port)
			logger.Printf("GameService '%s' discovered at %s. Attempting direct gRPC connection.", sc.GameServiceName, gameServiceAddress)

			// Establish direct gRPC connection
			// Consider context for dial, e.g., with timeout for setup
			dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second) // Example timeout
			defer dialCancel()

			conn, err := grpc.DialContext(dialCtx, gameServiceAddress,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock(),
			)
			if err != nil {
				logger.Printf("Failed to directly connect to GameService '%s' at '%s': %v", sc.GameServiceName, gameServiceAddress, err)
				return nil, fmt.Errorf("failed to dial GameService '%s' at '%s': %w", sc.GameServiceName, gameServiceAddress, err)
			} else {
				logger.Printf("Successfully connected directly to GameService '%s' at '%s'.", sc.GameServiceName, gameServiceAddress)
				sc.directGameServiceConn = conn
				sc.directGameServiceClient = pbgs.NewGameServiceClient(sc.directGameServiceConn)
			}
		}
	} else {
		if sc.GameServiceName == "" {
			logger.Println("GameServiceName is empty, skipping direct GameService connection setup.")
		}
		if sc.consulClient == nil {
			logger.Println("ConsulClient is nil, skipping direct GameService connection setup.")
		}
	}

	return sc, nil
}

// Login performs the login operation for the client.
func (sc *SimulatedClient) Login() error {
	sc.logger.Printf("Attempting to login to server '%s'", sc.LoginServerAddr)
	requestBody := LoginRequest{
		Username: sc.Username,
		Password: sc.Password,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %w", err)
	}

	resp, err := http.Post(sc.LoginServerAddr+"/api/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("http post request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResponse LoginResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err == nil && errorResponse.ErrorMessage != "" {
			return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, errorResponse.ErrorMessage)
		}
		return fmt.Errorf("login failed with status code: %d", resp.StatusCode)
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	if !loginResp.Success {
		return fmt.Errorf("login unsuccessful via API: %s", loginResp.ErrorMessage)
	}

	sc.UserID = loginResp.UserID
	sc.SessionToken = loginResp.SessionToken
	sc.logger.Printf("Login successful! UserID: %s", sc.UserID)

	return nil
}

/*
// SetupGameServiceRPC was removed as it established a gRPC connection to the GameService over the gatewayConn.
// It is replaced by a direct gRPC connection in NewSimulatedClient.
*/

/*
// DiscoverAndConnectRoomService discovers the RoomService using Consul and establishes a gRPC connection.
func (sc *SimulatedClient) DiscoverAndConnectRoomService() error {
	sc.logger.Printf("Attempting to discover service '%s' via Consul at '%s'", sc.GameServiceName, sc.ConsulServerAddr)

	// This method would now use sc.consulClient if it were to be kept for discovery,
	// or rely on sc.rpcClient which handles discovery internally.
	// For now, it's commented out as per instructions.

	// Example using existing sc.consulClient:
	// if sc.consulClient == nil {
	// 	return fmt.Errorf("Consul client not initialized")
	// }
	// serviceEntries, _, err := sc.consulClient.Health().Service(sc.GameServiceName, "", true, nil)
	// if err != nil {
	// 	return fmt.Errorf("failed to query consul for service '%s': %w", sc.GameServiceName, err)
	// }
	// if len(serviceEntries) == 0 {
	// 	return fmt.Errorf("no healthy instances found for service '%s'", sc.GameServiceName)
	// }
	// ... rest of the logic to pick an instance ...

	// However, the rpcClient is intended to replace this direct gRPC setup.
	// So, this entire method is deprecated in favor of using rpcClient.Call for room operations.
	return fmt.Errorf("DiscoverAndConnectRoomService is deprecated; use rpcClient for room operations")
}
*/

// DiscoverGatewayService discovers the Gateway service using Consul.
// This method now uses the sc.consulClient field.
func (sc *SimulatedClient) DiscoverGatewayService() (string, error) {
	if sc.consulClient == nil {
		return "", fmt.Errorf("Consul client not initialized, cannot discover gateway service")
	}
	sc.logger.Printf("Attempting to discover service '%s' via Consul at '%s'", sc.GatewayServiceName, sc.ConsulServerAddr)

	serviceEntries, _, err := sc.consulClient.Health().Service(sc.GatewayServiceName, "", true, nil)
	if err != nil {
		return "", fmt.Errorf("failed to query consul for service '%s': %w", sc.GatewayServiceName, err)
	}
	if len(serviceEntries) == 0 {
		return "", fmt.Errorf("no healthy instances found for service '%s'", sc.GatewayServiceName)
	}

	// Use the first healthy instance
	instance := serviceEntries[0]
	address := instance.Service.Address
	// If Service.Address is empty, use Node.Address. This is common in some Consul setups.
	if address == "" {
		address = instance.Node.Address
	}
	port := instance.Service.Port
	serviceAddress := fmt.Sprintf("%s:%d", address, port)

	sc.logger.Printf("Service '%s' discovered at %s", sc.GatewayServiceName, serviceAddress)
	return serviceAddress, nil
}

// ConnectToGateway discovers and connects to the Gateway service.
func (sc *SimulatedClient) ConnectToGateway(ctx context.Context) error {
	gatewayAddr, err := sc.DiscoverGatewayService()
	if err != nil {
		return fmt.Errorf("failed to discover gateway service: %w", err)
	}

	sc.logger.Printf("Attempting to connect to Gateway service at %s", gatewayAddr)

	// Determine timeout: use context deadline if available, otherwise default to 10 seconds.
	var timeout time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	} else {
		timeout = 10 * time.Second
	}

	conn, err := net.DialTimeout("tcp", gatewayAddr, timeout)
	if err != nil {
		sc.logger.Printf("Failed to connect to Gateway service at %s: %v", gatewayAddr, err)
		return fmt.Errorf("failed to dial gateway service at %s: %w", gatewayAddr, err)
	}

	sc.logger.Printf("Successfully connected to Gateway service at %s", gatewayAddr)
	sc.gatewayConn = conn
	sc.gatewayServiceAddress = gatewayAddr // Store the discovered address

	// Perform handshake
	if sc.UserID == "" || sc.SessionToken == "" {
		sc.logger.Println("UserID or SessionToken is empty. Cannot perform handshake.")
		// Close the connection as handshake is not possible
		if sc.gatewayConn != nil {
			sc.gatewayConn.Close()
			sc.gatewayConn = nil
		}
		return fmt.Errorf("cannot perform handshake: UserID or SessionToken is missing")
	}

	// Using a simple JSON structure for the handshake message
	handshakeData := map[string]string{
		"userId": sc.UserID,
		"token":  sc.SessionToken,
	}
	handshakePayload, err := json.Marshal(handshakeData)
	if err != nil {
		sc.logger.Printf("Failed to marshal handshake data: %v", err)
		if sc.gatewayConn != nil {
			sc.gatewayConn.Close()
			sc.gatewayConn = nil
		}
		return fmt.Errorf("failed to marshal handshake data: %w", err)
	}

	// Append a newline character to delimit the message, common for socket programming
	fullMessage := append(handshakePayload, '\n')

	// Set a write deadline for the handshake
	// This is important to prevent indefinite blocking if the server doesn't read or the network is slow.
	// Adjust the timeout as needed.
	if err := sc.gatewayConn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		sc.logger.Printf("Failed to set write deadline for handshake: %v", err)
		// Proceed without deadline, or handle as critical error
	}
	defer sc.gatewayConn.SetWriteDeadline(time.Time{}) // Clear the deadline

	n, err := sc.gatewayConn.Write(fullMessage)
	if err != nil {
		sc.logger.Printf("Failed to send handshake message to Gateway: %v", err)
		if sc.gatewayConn != nil {
			sc.gatewayConn.Close() // Attempt to close, though it might also fail if already broken
			sc.gatewayConn = nil
		}
		return fmt.Errorf("failed to send handshake to gateway: %w", err)
	}
	sc.logger.Printf("Successfully sent handshake message to Gateway (%d bytes): %s", n, string(handshakePayload)) // Log payload without newline for readability

	return nil
}

// CreateRoom uses rpcClient to call the CreateRoom method on the gateway.
func (sc *SimulatedClient) CreateRoom(ctx context.Context, roomName string, roomType modelpb.RoomType, maxPlayers uint32) (*modelpb.RoomData, error) {
	if sc.rpcClient == nil {
		return nil, fmt.Errorf("RPCClient not initialized. Cannot call CreateRoom")
	}
	if sc.gatewayServiceAddress == "" {
		return nil, fmt.Errorf("Gateway service address not discovered. Call ConnectToGateway first.")
	}

	req := &pbroom.CreateRoomRequest{
		PlayerId: sc.UserID,
		RoomName: roomName,
		// RoomType is of type modelpb.RoomType (from infra/pb/model).
		// The field pbroom.CreateRoomRequest.RoomType is also expected to be compatible.
		// This direct assignment works because protobuf generates these enums as type aliases
		// of int32, or they are defined such that they are directly assignable in Go.
		RoomType:   roomType,
		MaxPlayers: maxPlayers,
	}
	resp := &pbroom.CreateRoomResponse{}

	sc.logger.Printf("Attempting to CreateRoom '%s' via RPC to gateway %s", roomName, sc.gatewayServiceAddress)
	err := sc.rpcClient.Call(sc.GameServiceName, "CreateRoom", req, resp)
	if err != nil {
		sc.logger.Printf("CreateRoom RPC call to gateway failed: %v", err)
		return nil, fmt.Errorf("rpcClient.Call to CreateRoom (via gateway) failed: %w", err)
	}

	if resp.GetRoomData() == nil {
		// This case might indicate a server-side issue or an unexpected response structure.
		sc.logger.Printf("CreateRoom RPC call successful but RoomInfo is nil in response. Response: %v", resp)
		return nil, fmt.Errorf("CreateRoom response or RoomInfo is nil from rpcClient call")
	}

	sc.logger.Printf("CreateRoom successful via RPC. RoomID: %s, Name: %s", resp.GetRoomData().GetRoomId(), resp.GetRoomData().GetRoomName())
	return resp.GetRoomData(), nil
}

// JoinRoom uses rpcClient to call the JoinRoom method on the gateway.
func (sc *SimulatedClient) JoinRoom(ctx context.Context, roomID string) (*pbroom.JoinRoomResponse, error) {
	if sc.rpcClient == nil {
		return nil, fmt.Errorf("RPCClient not initialized. Cannot call JoinRoom")
	}
	if sc.gatewayServiceAddress == "" {
		return nil, fmt.Errorf("Gateway service address not discovered. Call ConnectToGateway first.")
	}

	req := &pbroom.JoinRoomRequest{
		PlayerId: sc.UserID,
		RoomId:   roomID,
	}
	resp := &pbroom.JoinRoomResponse{}

	sc.logger.Printf("Attempting to JoinRoom %s via RPC to gateway %s", roomID, sc.gatewayServiceAddress)
	err := sc.rpcClient.Call(sc.GatewayServiceName, "JoinRoom", req, resp)
	if err != nil {
		// JoinRoom can have legitimate failures (e.g., room full, already joined)
		sc.logger.Printf("JoinRoom RPC call to gateway returned: %v (this may be expected)", err)
		return resp, err
	}

	sc.logger.Printf("JoinRoom call to gateway successful for room %s. Response: %v", roomID, resp)
	return resp, nil
}

// PlayerReady uses rpcClient to call the PlayerReady method on the room server.
func (sc *SimulatedClient) PlayerReady(ctx context.Context, roomID string, isReady bool) (*pbroom.PlayerReadyResponse, error) {
	if sc.rpcClient == nil {
		return nil, fmt.Errorf("RPCClient not initialized. Cannot call PlayerReady")
	}
	if sc.RoomServiceName == "" {
		return nil, fmt.Errorf("RoomServiceName not configured. Cannot call PlayerReady directly.")
	}

	req := &pbroom.PlayerReadyRequest{
		PlayerId: sc.UserID,
		RoomId:   roomID,
		IsReady:  isReady,
	}
	resp := &pbroom.PlayerReadyResponse{}

	sc.logger.Printf("Attempting to set PlayerReady (isReady: %v) for room %s via RPC to room server %s", isReady, roomID, sc.RoomServiceName)
	err := sc.rpcClient.Call(sc.RoomServiceName, "PlayerReady", req, resp) // Using sc.RoomServiceName
	if err != nil {
		sc.logger.Printf("PlayerReady RPC call failed: %v", err)
		return nil, fmt.Errorf("rpcClient.Call to PlayerReady failed: %w", err)
	}

	sc.logger.Printf("PlayerReady call successful for room %s, status %v. Response: %v", roomID, isReady, resp)
	return resp, nil
}

// Close closes all managed connections.
func (sc *SimulatedClient) Close() {
	// if sc.grpcConn != nil { // Removed
	// 	sc.logger.Println("Closing gRPC connection to RoomService.")
	// 	if err := sc.grpcConn.Close(); err != nil {
	// 		sc.logger.Printf("Error closing gRPC connection: %v", err)
	// 	}
	// }
	if sc.rpcClient != nil {
		sc.logger.Println("Closing RPCClient connections.")
		// Assuming CloseAllConnections does not return an error or handles logging internally.
		// If it does return an error, it should be logged.
		sc.rpcClient.CloseAllConnections()
	}
	if sc.gatewayConn != nil {
		sc.logger.Println("Closing TCP connection to GatewayService.")
		if err := sc.gatewayConn.Close(); err != nil {
			sc.logger.Printf("Error closing gateway connection: %v", err)
		}
	}
	// if sc.gameServiceGrpcConn != nil { // Removed: Tunneled connection
	// 	sc.logger.Println("Closing gRPC connection to GameService (via Gateway).")
	// 	if err := sc.gameServiceGrpcConn.Close(); err != nil {
	// 		sc.logger.Printf("Error closing GameService gRPC connection: %v", err)
	// 	}
	// }
	if sc.directGameServiceConn != nil {
		sc.logger.Println("Closing direct gRPC connection to GameService.")
		if err := sc.directGameServiceConn.Close(); err != nil {
			sc.logger.Printf("Error closing direct GameService gRPC connection: %v", err)
		}
	}
}

// GetRoomList uses rpcClient to call the GetRoomList method on the room server.
func (sc *SimulatedClient) GetRoomList(ctx context.Context, req *pbroom.GetRoomListRequest) (*pbroom.GetRoomListResponse, error) {
	if sc.rpcClient == nil {
		return nil, fmt.Errorf("RPCClient not initialized. Cannot call GetRoomList")
	}
	if sc.RoomServiceName == "" {
		return nil, fmt.Errorf("RoomServiceName not configured. Cannot call GetRoomList directly.")
	}

	resp := &pbroom.GetRoomListResponse{}
	sc.logger.Printf("Attempting to GetRoomList via RPC to room server %s with request: %v", sc.RoomServiceName, req)
	err := sc.rpcClient.Call(sc.RoomServiceName, "GetRoomList", req, resp)
	if err != nil {
		sc.logger.Printf("GetRoomList RPC call failed: %v", err)
		return nil, fmt.Errorf("rpcClient.Call to GetRoomList failed: %w", err)
	}

	if resp != nil {
		sc.logger.Printf("GetRoomList call successful. Found %d rooms.", len(resp.GetRooms()))
	} else {
		sc.logger.Println("GetRoomList call successful but response is nil (no rooms or error).")
	}
	return resp, nil
}

// LeaveRoom uses rpcClient to call the LeaveRoom method on the room server.
func (sc *SimulatedClient) LeaveRoom(ctx context.Context, roomID string) (*pbroom.LeaveRoomResponse, error) {
	if sc.rpcClient == nil {
		return nil, fmt.Errorf("RPCClient not initialized. Cannot call LeaveRoom")
	}
	if sc.RoomServiceName == "" {
		return nil, fmt.Errorf("RoomServiceName not configured. Cannot call LeaveRoom directly.")
	}

	req := &pbroom.LeaveRoomRequest{
		PlayerId: sc.UserID,
		RoomId:   roomID,
	}
	resp := &pbroom.LeaveRoomResponse{}

	sc.logger.Printf("Attempting to LeaveRoom %s via RPC to gateway %s", roomID, sc.gatewayServiceAddress)
	err := sc.rpcClient.Call(sc.GatewayServiceName, "LeaveRoom", req, resp)
	if err != nil {
		sc.logger.Printf("LeaveRoom RPC call to gateway failed: %v", err)
		return nil, fmt.Errorf("rpcClient.Call to LeaveRoom (via gateway) failed: %w", err)
	}

	sc.logger.Printf("LeaveRoom call to gateway successful for room %s. Response: %v", roomID, resp)
	return resp, nil
}

// StartGame uses rpcClient to call the StartGame method on the room server.
func (sc *SimulatedClient) StartGame(ctx context.Context, roomID string) (*pbroom.StartGameResponse, error) {
	if sc.rpcClient == nil {
		return nil, fmt.Errorf("RPCClient not initialized. Cannot call StartGame")
	}
	if sc.RoomServiceName == "" {
		return nil, fmt.Errorf("RoomServiceName not configured. Cannot call StartGame directly.")
	}

	req := &pbroom.StartGameRequest{
		PlayerId: sc.UserID, // Assuming PlayerId is the one initiating the start
		RoomId:   roomID,
	}
	resp := &pbroom.StartGameResponse{}

	sc.logger.Printf("Attempting to StartGame for room %s via RPC to room server %s", roomID, sc.RoomServiceName)
	err := sc.rpcClient.Call(sc.RoomServiceName, "StartGame", req, resp)
	if err != nil {
		sc.logger.Printf("StartGame RPC call failed: %v", err)
		return nil, fmt.Errorf("rpcClient.Call to StartGame failed: %w", err)
	}

	sc.logger.Printf("StartGame call successful for room %s. Response: %v", roomID, resp)
	return resp, nil
}

// RollDice calls the PlayerRollDice gRPC method on the GameService.
func (sc *SimulatedClient) RollDice(ctx context.Context) (*pbgs.RollDiceResponse, error) {
	if sc.directGameServiceClient == nil {
		return nil, fmt.Errorf("directGameServiceClient not initialized")
	}
	req := &pbgs.RollDiceRequest{
		PlayerId: sc.UserID,
	}
	sc.logger.Printf("Attempting to RollDice for player %s", sc.UserID)
	resp, err := sc.directGameServiceClient.PlayerRollDice(ctx, req)
	if err != nil {
		sc.logger.Printf("RollDice failed for player %s: %v", sc.UserID, err)
		return nil, fmt.Errorf("GameService.PlayerRollDice failed: %w", err)
	}
	// Assuming RollDiceResponse has a field like DiceValue or similar. Logging basic success.
	sc.logger.Printf("RollDice successful for player %s. Response: %v", sc.UserID, resp)
	return resp, nil
}

// Move calls the PlayerMove gRPC method on the GameService.
func (sc *SimulatedClient) Move(ctx context.Context, targetTileID string) (*pbgs.MoveResponse, error) {
	if sc.directGameServiceClient == nil {
		return nil, fmt.Errorf("directGameServiceClient not initialized")
	}
	req := &pbgs.MoveRequest{
		PlayerId:     sc.UserID,
		TargetTileId: targetTileID,
	}
	sc.logger.Printf("Attempting to Move player %s to tile %s", sc.UserID, targetTileID)
	resp, err := sc.directGameServiceClient.PlayerMove(ctx, req)
	if err != nil {
		sc.logger.Printf("Move failed for player %s to tile %s: %v", sc.UserID, targetTileID, err)
		return nil, fmt.Errorf("GameService.PlayerMove failed: %w", err)
	}
	sc.logger.Printf("Move successful for player %s to tile %s. Response: %v", sc.UserID, targetTileID, resp)
	return resp, nil
}

// PlayCard calls the PlayerPlayCard gRPC method on the GameService.
func (sc *SimulatedClient) PlayCard(ctx context.Context, cardID string, targetID string) (*pbgs.PlayCardResponse, error) {
	if sc.directGameServiceClient == nil {
		return nil, fmt.Errorf("directGameServiceClient not initialized")
	}
	req := &pbgs.PlayCardRequest{
		PlayerId: sc.UserID,
		CardId:   cardID,
		TargetId: targetID,
	}
	sc.logger.Printf("Attempting to PlayCard %s for player %s (target: %s)", cardID, sc.UserID, targetID)
	resp, err := sc.directGameServiceClient.PlayerPlayCard(ctx, req)
	if err != nil {
		sc.logger.Printf("PlayCard %s failed for player %s: %v", cardID, sc.UserID, err)
		return nil, fmt.Errorf("GameService.PlayerPlayCard failed: %w", err)
	}
	sc.logger.Printf("PlayCard %s successful for player %s. Response: %v", cardID, sc.UserID, resp)
	return resp, nil
}

// EndTurn calls the PlayerEndTurn gRPC method on the GameService.
func (sc *SimulatedClient) EndTurn(ctx context.Context) (*pbgs.EndTurnResponse, error) {
	if sc.directGameServiceClient == nil {
		return nil, fmt.Errorf("directGameServiceClient not initialized")
	}
	req := &pbgs.EndTurnRequest{
		PlayerId: sc.UserID,
	}
	sc.logger.Printf("Attempting to EndTurn for player %s", sc.UserID)
	resp, err := sc.directGameServiceClient.PlayerEndTurn(ctx, req)
	if err != nil {
		sc.logger.Printf("EndTurn failed for player %s: %v", sc.UserID, err)
		return nil, fmt.Errorf("GameService.PlayerEndTurn failed: %w", err)
	}
	sc.logger.Printf("EndTurn successful for player %s. Response: %v", sc.UserID, resp)
	return resp, nil
}

// SendChatMessage calls the SendChatMessageGame gRPC method on the GameService.
func (sc *SimulatedClient) SendChatMessage(ctx context.Context, channel pbmodel.ChatChannel, receiverID string, content string) (*pbgs.SendChatMessageGameResponse, error) {
	if sc.directGameServiceClient == nil {
		return nil, fmt.Errorf("directGameServiceClient not initialized")
	}
	req := &pbgs.SendChatMessageGameRequest{
		PlayerId:   sc.UserID, // Assuming SenderId is the PlayerId for game messages
		Channel:    channel,
		ReceiverId: receiverID,
		Content:    content,
	}
	sc.logger.Printf("Attempting to SendChatMessage from player %s (channel: %s, receiver: %s): %s", sc.UserID, channel, receiverID, content)
	resp, err := sc.directGameServiceClient.PlayerSendChatMessage(ctx, req)
	if err != nil {
		sc.logger.Printf("SendChatMessage failed for player %s: %v", sc.UserID, err)
		return nil, fmt.Errorf("GameService.SendChatMessageGame failed: %w", err)
	}
	sc.logger.Printf("SendChatMessage successful for player %s. Response: %v", sc.UserID, resp)
	return resp, nil
}
