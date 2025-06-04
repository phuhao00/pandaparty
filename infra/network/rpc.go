package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	// Assuming your consul package is aliased or directly usable.
	// Adjust the import path if your consul package is located elsewhere or named differently.
	// For example, if it's "github.com/phuhao00/pandaparty/infra/consulx" use that.
	// For now, using the path provided in the project structure.
	"sync"
	"time"

	consul "github.com/hashicorp/consul/api"
	consulx "github.com/phuhao00/pandaparty/infra/consul"
	"google.golang.org/protobuf/proto"
)

// RPCRequest defines the structure for an RPC call.
// This is primarily for conceptual understanding; the server receives methodName and payload separately.
type RPCRequest struct {
	MethodName string // Name of the RPC method to be invoked.
	Payload    []byte // Protobuf-marshaled payload for the request.
}

// RPCResponse defines the structure for an RPC response.
// This is used by the server to structure its reply.
type RPCResponse struct {
	Payload []byte // Protobuf-marshaled payload of the response.
	Error   string // Application-level error message, if any. Empty if the call was successful.
}

// RPCServer manages RPC handlers and listens for incoming TCP connections.
// It allows registration of methods and their corresponding coordinator functions.
// Each incoming connection is handled in a separate goroutine.
// The server uses a length-prefixed framing protocol:
//
//	Request Frame: TotalFrameLength (int32) | MethodNameLength (int32) | MethodName ([]byte) | PayloadLength (int32) | Payload ([]byte)
//	Response Frame: TotalFrameLength (int32) | ErrorLength (int32) | ErrorString ([]byte) | PayloadLength (int32) | Payload ([]byte)
type RPCServer struct {
	handlers     map[string]func(reqPayload []byte) (resPayload []byte, err error) // Map of method names to coordinator functions.
	listener     net.Listener                                                      // TCP listener.
	consulClient *consulx.ConsulClient                                             // Optional Consul client for potential future use (e.g., dynamic re-registration).
}

// NewRPCServer creates a new RPC server instance.
// The provided consulClient is stored for potential future extensions but is not
// actively used by the server's core listening/handling logic currently.
func NewRPCServer(client *consulx.ConsulClient) (*RPCServer, error) {
	return &RPCServer{
		handlers:     make(map[string]func(reqPayload []byte) (resPayload []byte, err error)),
		consulClient: client,
	}, nil
}

// RegisterHandler adds a new coordinator function for a given RPC method name.
// If a coordinator for the methodName already exists, it will be overwritten.
// The coordinator function takes the raw byte payload of the request and is expected
// to return the raw byte payload of the response and an application-level error if any.
func (s *RPCServer) RegisterHandler(methodName string, handler func(reqPayload []byte) (resPayload []byte, err error)) {
	if s.handlers == nil {
		s.handlers = make(map[string]func(reqPayload []byte) (resPayload []byte, err error))
	}
	s.handlers[methodName] = handler
	log.Printf("Registered coordinator for method: %s", methodName)
}

// Listen starts the TCP listener on the specified address and begins accepting incoming connections.
// Each connection is processed in a new goroutine by the handleConnection method.
// This method blocks until the listener fails with a non-recoverable error or is closed.
// Example address: "0.0.0.0:50051".
func (s *RPCServer) Listen(address string) error {
	var err error
	s.listener, err = net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	log.Printf("RPC Server listening on %s", address)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Handle listener errors (e.g., if listener is closed)
			if opError, ok := err.(*net.OpError); ok && opError.Err.Error() == "use of closed network connection" {
				log.Printf("RPC Server listener on %s closed.", address)
				return nil // Graceful shutdown
			}
			log.Printf("Failed to accept connection: %v", err)
			// Consider whether to continue or stop based on the error type
			if opError, ok := err.(*net.OpError); ok && !opError.Temporary() {
				log.Printf("Permanent error accepting connections; stopping listener: %v", err)
				return err // Stop listening on permanent errors
			}
			continue // Continue on temporary errors
		}
		log.Printf("Accepted new connection from %s", conn.RemoteAddr())
		go s.handleConnection(conn)
	}
}

// handleConnection reads requests, calls handlers, and sends responses
func (s *RPCServer) handleConnection(conn net.Conn) {
	defer func() {
		log.Printf("Closing connection from %s", conn.RemoteAddr())
		conn.Close()
	}()

	for {
		// Frame format for request:
		// TotalFrameLength (int32) - Length of (MethodNameLength + MethodName + PayloadLength + Payload)
		// MethodNameLength (int32)
		// MethodName ([]byte)
		// PayloadLength (int32)
		// Payload ([]byte)

		var totalFrameLen int32
		if err := binary.Read(conn, binary.BigEndian, &totalFrameLen); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				log.Printf("Connection closed by client %s (EOF reading total frame length)", conn.RemoteAddr())
				return
			}
			log.Printf("Error reading total frame length from %s: %v", conn.RemoteAddr(), err)
			return
		}

		if totalFrameLen <= 0 {
			log.Printf("Invalid total frame length %d received from %s", totalFrameLen, conn.RemoteAddr())
			return
		}

		frameData := make([]byte, totalFrameLen)
		if _, err := io.ReadFull(conn, frameData); err != nil {
			log.Printf("Error reading frame data from %s: %v", conn.RemoteAddr(), err)
			return
		}
		frameReader := bytes.NewReader(frameData)

		// 1. Read MethodName length (int32)
		var methodNameLen int32
		if err := binary.Read(frameReader, binary.BigEndian, &methodNameLen); err != nil {
			log.Printf("Error reading method name length from %s: %v", conn.RemoteAddr(), err)
			return
		}

		// 2. Read MethodName string
		methodNameBytes := make([]byte, methodNameLen)
		if _, err := io.ReadFull(frameReader, methodNameBytes); err != nil {
			log.Printf("Error reading method name from %s: %v", conn.RemoteAddr(), err)
			return
		}
		methodName := string(methodNameBytes)

		// 3. Read Payload length (int32)
		var payloadLen int32
		if err := binary.Read(frameReader, binary.BigEndian, &payloadLen); err != nil {
			log.Printf("Error reading payload length for method %s from %s: %v", methodName, conn.RemoteAddr(), err)
			return
		}

		// 4. Read Payload
		reqPayload := make([]byte, payloadLen)
		if _, err := io.ReadFull(frameReader, reqPayload); err != nil {
			log.Printf("Error reading payload for method %s from %s: %v", methodName, conn.RemoteAddr(), err)
			return
		}

		log.Printf("Received request for method '%s' with payload size %d from %s", methodName, payloadLen, conn.RemoteAddr())

		handler, ok := s.handlers[methodName]
		var resPayload []byte
		var appErr error
		rpcResp := RPCResponse{}

		if !ok {
			errMsg := fmt.Sprintf("no coordinator found for method: %s", methodName)
			log.Println(errMsg)
			rpcResp.Error = errMsg
		} else {
			resPayload, appErr = handler(reqPayload)
			if appErr != nil {
				log.Printf("Handler for method '%s' returned error: %v", methodName, appErr)
				rpcResp.Error = appErr.Error()
			}
			rpcResp.Payload = resPayload
		}

		// Serialize and send RPCResponse
		// Frame format for response:
		// TotalFrameLength (int32) - Length of (ErrorLength + Error + PayloadLength + Payload)
		// ErrorLength (int32)
		// Error ([]byte)
		// PayloadLength (int32)
		// Payload ([]byte)
		var responseBuffer bytes.Buffer

		errorBytes := []byte(rpcResp.Error)
		// Write Error string length
		if err := binary.Write(&responseBuffer, binary.BigEndian, int32(len(errorBytes))); err != nil {
			log.Printf("Error writing error length to buffer for %s: %v", conn.RemoteAddr(), err)
			return
		}
		// Write Error string
		if _, err := responseBuffer.Write(errorBytes); err != nil {
			log.Printf("Error writing error to buffer for %s: %v", conn.RemoteAddr(), err)
			return
		}

		// Write Payload length
		if err := binary.Write(&responseBuffer, binary.BigEndian, int32(len(rpcResp.Payload))); err != nil {
			log.Printf("Error writing payload length to buffer for %s: %v", conn.RemoteAddr(), err)
			return
		}
		// Write Payload
		if _, err := responseBuffer.Write(rpcResp.Payload); err != nil {
			log.Printf("Error writing payload to buffer for %s: %v", conn.RemoteAddr(), err)
			return
		}

		// Send the total length of the response frame first
		resTotalFrameLen := int32(responseBuffer.Len())
		if err := binary.Write(conn, binary.BigEndian, resTotalFrameLen); err != nil {
			log.Printf("Error sending response total frame length for method %s to %s: %v", methodName, conn.RemoteAddr(), err)
			return
		}
		// Send the response frame itself
		if _, err := conn.Write(responseBuffer.Bytes()); err != nil {
			log.Printf("Error sending response frame for method %s to %s: %v", methodName, conn.RemoteAddr(), err)
			return
		}
		log.Printf("Sent response for method '%s' to %s (Error: '%s', PayloadSize: %d)", methodName, conn.RemoteAddr(), rpcResp.Error, len(rpcResp.Payload))
	}
}

// Call performs an RPC call to a specified service and method.
// consulClient is required for service discovery.
// The config.ConsulConfig is passed to potentially initialize a new Consul client if one is not provided.
// Default values for RPCClient configuration.
const (
	defaultMaxConnsPerEndpoint = 10              // Default maximum idle connections per target endpoint.
	defaultDialTimeout         = 5 * time.Second // Default timeout for establishing a new connection.
)

// RPCClient provides a client for making RPC calls to services.
// It features a connection pooling mechanism to reuse TCP connections, reducing
// the overhead of connection establishment for frequent calls to the same endpoint.
// Service discovery is handled via a Consul client. If a serviceName provided to Call
// resembles a direct "host:port" address, Consul discovery is bypassed for testing or direct connections.
//
// The client uses a length-prefixed framing protocol identical to RPCServer:
//
//	Request Frame: TotalFrameLength (int32) | MethodNameLength (int32) | MethodName ([]byte) | PayloadLength (int32) | Payload ([]byte)
//	Response Frame: TotalFrameLength (int32) | ErrorLength (int32) | ErrorString ([]byte) | PayloadLength (int32) | Payload ([]byte)
type RPCClient struct {
	pools               map[string]chan net.Conn // Map of endpointAddress (host:port) to a buffered channel of net.Conn (connection pool).
	maxConnsPerEndpoint int                      // Maximum number of idle connections to keep in the pool for each endpoint.
	mu                  sync.Mutex               // Protects access to the pools map.
	consulClient        *consulx.ConsulClient    // Client for Consul service discovery.
	dialTimeout         time.Duration            // Timeout for dialing new TCP connections.
	nextInstance        map[string]uint64        // Stores the next index for round-robin per serviceName
	nextInstanceMu      sync.Mutex               // Protects access to nextInstance map
}

// NewRPCClient creates a new RPC client with connection pooling capabilities.
// Parameters:
//   - cc: A *consul.ConsulClient for service discovery. Can be nil if direct addressing is always used (not recommended for production).
//   - maxConns: Maximum number of idle connections to maintain per endpoint. If <= 0, defaults to defaultMaxConnsPerEndpoint.
//   - timeout: Timeout for establishing new connections. If <= 0, defaults to defaultDialTimeout.
func NewRPCClient(cc *consulx.ConsulClient, maxConns int, timeout time.Duration) *RPCClient {
	if maxConns <= 0 {
		maxConns = defaultMaxConnsPerEndpoint
	}
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}
	return &RPCClient{
		pools:               make(map[string]chan net.Conn),
		maxConnsPerEndpoint: maxConns,
		consulClient:        cc,
		dialTimeout:         timeout,
		nextInstance:        make(map[string]uint64), // Initialize nextInstance
	}
}

// getConnection retrieves an existing connection from the pool for the given endpointAddress
// or creates a new one if the pool is empty.
// It manages the creation of pool channels on-demand.
func (c *RPCClient) getConnection(endpointAddress string) (net.Conn, error) {
	c.mu.Lock() // Lock to safely access/create the pool channel for the endpoint.
	pool, ok := c.pools[endpointAddress]
	if !ok {
		pool = make(chan net.Conn, c.maxConnsPerEndpoint)
		c.pools[endpointAddress] = pool
	}
	c.mu.Unlock()

	select {
	case conn := <-pool: // Try to get an existing connection from the pool.
		log.Printf("RPCClient: Reusing connection to %s from pool.", endpointAddress)
		// TODO: Implement a health check for pooled connections (e.g., a test read/write)
		// before returning, to ensure the connection is still alive.
		return conn, nil
	default: // Pool is empty or no connections available immediately.
		log.Printf("RPCClient: Pool for %s is empty, dialing new connection.", endpointAddress)
		conn, err := net.DialTimeout("tcp", endpointAddress, c.dialTimeout)
		if err != nil {
			return nil, fmt.Errorf("failed to dial %s: %w", endpointAddress, err)
		}
		log.Printf("RPCClient: Successfully dialed new connection to %s", endpointAddress)
		return conn, nil
	}
}

// returnConnection returns a connection to the pool for the specified endpointAddress.
// If the pool is full (i.e., the buffered channel is at capacity), the connection is closed.
func (c *RPCClient) returnConnection(endpointAddress string, conn net.Conn) {
	if conn == nil {
		return
	}
	c.mu.Lock() // Lock to safely access the pool channel.
	pool, ok := c.pools[endpointAddress]
	if !ok {
		// This case should ideally not happen if getConnection was called correctly.
		// It implies the pool was removed or not created.
		c.mu.Unlock()
		log.Printf("RPCClient: Pool for %s not found on returnConnection. Closing connection.", endpointAddress)
		conn.Close()
		return
	}
	c.mu.Unlock()

	select {
	case pool <- conn: // Attempt to return the connection to the pool.
		log.Printf("RPCClient: Connection returned to pool for %s.", endpointAddress)
	default: // Pool is full.
		log.Printf("RPCClient: Pool for %s is full. Closing surplus connection.", endpointAddress)
		conn.Close()
	}
}

// Call performs an RPC to a specified service and method using a pooled connection.
// Parameters:
//   - serviceName: The logical name of the service to call (e.g., "roomserver") or a direct "host:port" address.
//     If it's a service name, Consul will be used for discovery. If it's a direct address, Consul is bypassed.
//   - methodName: The name of the RPC method to invoke on the service.
//   - requestProto: The protobuf message representing the request payload.
//   - responseProto: A pointer to a protobuf message where the response payload will be unmarshaled.
//
// Returns:
//   - An error if the call fails at any stage (discovery, connection, sending, receiving, unmarshaling, or application-level error from server).
//   - Nil error on successful call, with responseProto populated.
//
// Connection Management:
//   - Connections are obtained from a pool specific to the target endpoint.
//   - If a network error occurs during the request/response transmission, the connection is considered unhealthy and closed, not returned to the pool.
//   - Healthy connections are returned to the pool for reuse.
func (c *RPCClient) Call(serviceName string, methodName string, requestProto proto.Message, responseProto proto.Message) error {
	var targetAddr string
	var err error

	// Check if serviceName resembles a direct address (e.g., "localhost:1234")
	if _, _, errNet := net.SplitHostPort(serviceName); errNet == nil {
		targetAddr = serviceName
		log.Printf("RPCClient: Service name '%s' appears to be a direct address. Bypassing Consul discovery.", serviceName)
	} else {
		if c.consulClient == nil {
			return fmt.Errorf("RPCClient: Consul client is not initialized and service name '%s' is not a direct address", serviceName)
		}
		instances, errDiscover := c.consulClient.DiscoverService(serviceName)
		if errDiscover != nil {
			return fmt.Errorf("RPCClient: failed to discover service %s: %w", serviceName, errDiscover)
		}
		if len(instances) == 0 {
			return fmt.Errorf("RPCClient: no instances found for service %s", serviceName)
		}

		var selectedInstance *consul.ServiceEntry // Use consul.ServiceEntry which should be []*api.ServiceEntry

		// Round-robin selection
		c.nextInstanceMu.Lock()
		currentIndex := c.nextInstance[serviceName] // Get current index/counter for this service
		selectedInstance = instances[currentIndex%uint64(len(instances))]
		c.nextInstance[serviceName] = currentIndex + 1 // Increment for next call
		c.nextInstanceMu.Unlock()

		if selectedInstance.Service == nil {
			// This check might be redundant if DiscoverService filters unhealthy, but good for safety
			return fmt.Errorf("RPCClient: selected service instance for %s has nil Service data after round-robin", serviceName)
		}
		targetAddr = fmt.Sprintf("%s:%d", selectedInstance.Service.Address, selectedInstance.Service.Port)
	}

	log.Printf("RPCClient: Attempting to call service/address '%s' method '%s' at resolved address %s", serviceName, methodName, targetAddr)

	conn, err := c.getConnection(targetAddr)
	if err != nil {
		return fmt.Errorf("RPCClient: failed to get connection to %s for service %s: %w", targetAddr, serviceName, err)
	}

	// Defer returning the connection. If a network error occurs, conn will be set to nil.
	// This ensures we don't return a bad connection.
	connHealthy := true
	defer func() {
		if connHealthy {
			c.returnConnection(targetAddr, conn)
		} else if conn != nil {
			// If not healthy, but conn is not nil, it means we had it but it broke. Close it.
			log.Printf("RPCClient: Closing unhealthy connection to %s after failed call.", targetAddr)
			conn.Close()
		}
	}()

	log.Printf("RPCClient: Using connection to %s for RPC call to method '%s'", targetAddr, methodName)

	// Prepare request payload
	reqPayloadBytes, err := proto.Marshal(requestProto)
	if err != nil {
		return fmt.Errorf("failed to marshal request protobuf for method %s: %w", methodName, err)
	}

	var requestFrameBuffer bytes.Buffer
	methodNameBytes := []byte(methodName)

	// Write MethodName length and MethodName
	if err := binary.Write(&requestFrameBuffer, binary.BigEndian, int32(len(methodNameBytes))); err != nil {
		return fmt.Errorf("failed to write method name length: %w", err)
	}
	if _, err := requestFrameBuffer.Write(methodNameBytes); err != nil {
		return fmt.Errorf("failed to write method name: %w", err)
	}

	// Write Payload length and Payload
	if err := binary.Write(&requestFrameBuffer, binary.BigEndian, int32(len(reqPayloadBytes))); err != nil {
		return fmt.Errorf("failed to write payload length: %w", err)
	}
	if _, err := requestFrameBuffer.Write(reqPayloadBytes); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}

	// Send the total length of the request frame first
	reqTotalFrameLen := int32(requestFrameBuffer.Len())
	if err = binary.Write(conn, binary.BigEndian, reqTotalFrameLen); err != nil {
		connHealthy = false
		return fmt.Errorf("RPCClient: failed to send request total frame length for method %s to %s: %w", methodName, targetAddr, err)
	}
	// Send the request frame itself
	if _, err = conn.Write(requestFrameBuffer.Bytes()); err != nil {
		connHealthy = false
		return fmt.Errorf("RPCClient: failed to send request frame for method %s to %s: %w", methodName, targetAddr, err)
	}
	log.Printf("RPCClient: Sent request for method '%s' to %s", methodName, targetAddr)

	// Receive Response
	var resTotalFrameLen int32
	if err = binary.Read(conn, binary.BigEndian, &resTotalFrameLen); err != nil {
		connHealthy = false
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return fmt.Errorf("RPCClient: connection closed by server %s while reading response total frame length for method %s: %w", targetAddr, methodName, err)
		}
		return fmt.Errorf("RPCClient: failed to read response total frame length from %s for method %s: %w", targetAddr, methodName, err)
	}

	if resTotalFrameLen <= 0 {
		connHealthy = false // Or perhaps the connection is fine but response is corrupt
		return fmt.Errorf("RPCClient: invalid response total frame length %d received from %s for method %s", resTotalFrameLen, targetAddr, methodName)
	}

	resFrameData := make([]byte, resTotalFrameLen)
	if _, err = io.ReadFull(conn, resFrameData); err != nil {
		connHealthy = false
		return fmt.Errorf("RPCClient: failed to read response frame data from %s for method %s: %w", targetAddr, methodName, err)
	}
	responseReader := bytes.NewReader(resFrameData)

	var errLen int32
	if err = binary.Read(responseReader, binary.BigEndian, &errLen); err != nil {
		connHealthy = false // Connection might be okay, but data is malformed. For safety, mark unhealthy.
		return fmt.Errorf("RPCClient: failed to read error string length from %s: %w", targetAddr, err)
	}

	errBytes := make([]byte, errLen)
	if _, err = io.ReadFull(responseReader, errBytes); err != nil {
		connHealthy = false
		return fmt.Errorf("RPCClient: failed to read error string from %s: %w", targetAddr, err)
	}
	rpcErrStr := string(errBytes)

	var resPayloadLen int32
	if err = binary.Read(responseReader, binary.BigEndian, &resPayloadLen); err != nil {
		connHealthy = false
		return fmt.Errorf("RPCClient: failed to read response payload length from %s: %w", targetAddr, err)
	}

	resPayloadBytes := make([]byte, resPayloadLen)
	if _, err = io.ReadFull(responseReader, resPayloadBytes); err != nil {
		connHealthy = false
		return fmt.Errorf("RPCClient: failed to read response payload from %s: %w", targetAddr, err)
	}

	if rpcErrStr != "" {
		// This is an application-level error from the server, not necessarily a connection error.
		return fmt.Errorf("RPC call to method '%s' on service '%s' at '%s' failed: %s", methodName, serviceName, targetAddr, rpcErrStr)
	}

	if resPayloadLen > 0 {
		if err = proto.Unmarshal(resPayloadBytes, responseProto); err != nil {
			// This is a deserialization error, not a network error. Connection might still be okay.
			// However, if the server sends garbage, it's safer to assume the channel might be problematic.
			// For now, we don't mark connHealthy = false here, but this is debatable.
			return fmt.Errorf("RPCClient: failed to unmarshal response protobuf for method '%s' from service '%s' at '%s': %w", methodName, serviceName, targetAddr, err)
		}
	} else if responseProto != nil && responseProto.ProtoReflect().IsValid() {
		log.Printf("RPCClient: Received empty payload for method '%s' on service '%s' at '%s', but responseProto object was provided.", methodName, serviceName, targetAddr)
	}

	log.Printf("RPCClient: Successfully called method '%s' on service '%s' at %s", methodName, serviceName, targetAddr)
	return nil
}

// Close gracefully shuts down the RPC server.
func (s *RPCServer) Close() error {
	if s.listener != nil {
		log.Printf("Closing RPC server listener on %s", s.listener.Addr())
		return s.listener.Close()
	}
	return nil
}

// Dummy config struct, replace with your actual config.ConsulConfig
// type ConsulConfig struct { // This was a dummy, actual config.ConsulConfig is used
// 	Address string
// 	Scheme  string
// 	Token   string
// }

// CloseAllConnections closes all connections in all pools managed by the RPCClient.
// This should be called when the application is shutting down.
func (c *RPCClient) CloseAllConnections() {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Println("RPCClient: Closing all pooled connections.")
	for endpoint, pool := range c.pools {
		close(pool)              // Close the channel to signal no more connections will be added/retrieved via it.
		for conn := range pool { // Drain and close existing idle connections
			log.Printf("RPCClient: Closing idle connection to %s from pool.", endpoint)
			conn.Close()
		}
		delete(c.pools, endpoint) // Remove the pool from the map
	}
}
