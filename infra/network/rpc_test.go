package network

import (
	"errors"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	// Import the generated protobuf package for PingRequest/PingResponse
	pb "github.com/phuhao00/pandaparty/infra/pb/protocol/rpctest"
)

// Helper function to start an RPCServer on a dynamic port for testing.
func startTestRPCServer(t *testing.T) (*RPCServer, string) {
	server, err := NewRPCServer(nil) // No Consul client needed for server in this test
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "localhost:0") // Dynamic port
	require.NoError(t, err)

	addr := listener.Addr().String()
	log.Printf("Test RPCServer listening on %s", addr)

	go func() {
		// Override server.listener with the one we created so we know the address
		// This is a bit of a hack for testing. A cleaner way would be for Listen to return the listener or address.
		// For now, we will manually assign it if the Listen method in RPCServer is not directly using the passed address string to make a new listener
		// but rather expects s.listener to be pre-set or uses the address to create one internally.
		// The current RPCServer.Listen does `s.listener, err = net.Listen("tcp", address)`. So we just need to pass the address.
		// The issue is that `s.listener.Accept()` will be called.
		// The provided `Listen` method in `RPCServer` creates its own listener.
		// So, we need to ensure the server's internal listener is used.
		// The test needs the address. One way is to let server listen, then try to get addr.
		// For simplicity, the current Listen method in RPCServer takes an address string.
		// We'll let the OS pick a port, then pass that specific address to Listen.

		// This setup is slightly different from the actual Listen method which takes an address string.
		// To test with a dynamic port, we listen on "localhost:0", get the address,
		// then the server should listen on that specific address.
		// However, the RPCServer's Listen method itself calls net.Listen.
		// So, we will create a listener, get its address, close it, and then pass this address to RPCServer.Listen.
		// This is to ensure we know the port.

		// Simpler: Let the server listen and if it logs the address, use that.
		// For robust testing, it's better if Listen could return the address or if server had an Addr() method.
		// Given current structure, we'll use "localhost:0" for the server to pick a port,
		// but the client needs to know this port.
		// The dynamic port from listener.Addr().String() is what we need.
		// The server's Listen method will then re-listen. This is slightly redundant but works for testing.

		// Let's modify the test setup:
		// 1. Create listener on localhost:0
		// 2. Get addr
		// 3. Close this listener (as RPCServer.Listen will create its own)
		// 4. Pass addr to RPCServer.Listen

		// The current RPCServer.Listen takes an address. If we pass "localhost:0", it will pick a port,
		// but we won't know it directly from the calling test to give to the client.
		// For testing, it's common to have the server start and report its listening address.
		// Let's assume we can make RPCServer.Listen more test-friendly or use a fixed port for testing (less ideal).

		// For now, we'll use a fixed, hopefully available, port for the test server.
		// Or, start the server, and the client will use Consul (if we set up a mock Consul).
		// The instruction was to bypass Consul for client by using direct address.
		// So, the client needs to know the server's address.

		// Re-evaluating: The listener created here is what the server should use.
		// This requires a small modification to RPCServer to allow injecting a listener,
		// or for Listen to return the address.
		// Let's try to make RPCServer.Listen use the passed address string.
		// The challenge is that the client part of the test needs this address.

		// Simplest for now: use a known test port. If it conflicts, test will fail.
		// testServerAddr := "localhost:55555" // Example fixed port
		// For dynamic port:
		// Listener is created above. Server should use this listener.
		// Modify RPCServer to accept a listener (ideal) or make Listen report its port (hacky).
		// Let's try with the dynamic port and assume the server's Listen is robust.
		// The client will use listener.Addr().String().

		// The server's Listen method will be called with listener.Addr().String().
		// This means the server will try to listen on the *same port* we just got.
		// This should work if the initial listener is closed before the server's Listen is called.

		// Actually, RPCServer.Listen will internally call net.Listen.
		// So, we create a listener to find a free port, get its address, close it,
		// then tell the server to listen on that specific address.

		initialListener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err, "Failed to create initial listener to find free port")
		dynamicAddr := initialListener.Addr().String()
		require.NoError(t, initialListener.Close(), "Failed to close initial listener")

		// Now start the actual server on this dynamic address
		go func() {
			if err := server.Listen(dynamicAddr); err != nil && !errors.Is(err, net.ErrClosed) {
				// Don't fail test here from goroutine, just log
				log.Printf("Test RPCServer Listen error: %v", err)
			}
		}()

		// This setup is still a bit complex. A channel to signal server readiness would be better.
		// For now, a short sleep to allow the server to start.
		time.Sleep(100 * time.Millisecond)

	}() // This outer go func isn't needed if we manage server start properly.
	// The inner go func for server.Listen is correct.

	// The address is dynamicAddr from above.
	// The server is started in a goroutine. We need its address.
	// Let's refine the server start logic to be more testable.

	// Corrected server start for testing:
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	serverAddr := lis.Addr().String()

	// We need to make RPCServer use this listener.
	// This implies a change to RPCServer or a different Listen method for tests.
	// For now, assume we can't change RPCServer.Listen's internals for this task.
	// So, we close `lis` and tell the server to listen on `serverAddr`.
	// This is a common pattern if the server doesn't support listener injection.
	lis.Close()

	go func() {
		if errS := server.Listen(serverAddr); errS != nil {
			if !errors.Is(errS, net.ErrClosed) { // ErrClosed is expected on server.Close()
				t.Logf("RPCServer Listen error: %v", errS) // Use t.Logf for test logging
			}
		}
	}()
	time.Sleep(50 * time.Millisecond) // Give server a moment to start

	return server, serverAddr
}

func TestRPCCall_Success(t *testing.T) {
	server, serverAddr := startTestRPCServer(t)
	defer server.Close()

	// Register Ping coordinator
	server.RegisterHandler("Ping", func(reqPayload []byte) (resPayload []byte, err error) {
		var req pb.PingRequest
		if err := proto.Unmarshal(reqPayload, &req); err != nil {
			return nil, fmt.Errorf("coordinator: failed to unmarshal PingRequest: %w", err)
		}
		t.Logf("Ping coordinator received: %s", req.Message)
		resp := &pb.PingResponse{Reply: "Pong: " + req.Message}
		return proto.Marshal(resp)
	})

	// Client side
	// No Consul client needed as we're using direct address
	rpcClient := NewRPCClient(nil, 5, 2*time.Second)
	defer rpcClient.CloseAllConnections()

	pingReq := &pb.PingRequest{Message: "Hello RPC"}
	pingResp := &pb.PingResponse{}

	// First call
	err := rpcClient.Call(serverAddr, "Ping", pingReq, pingResp)
	require.NoError(t, err, "First RPC call failed")
	assert.Equal(t, "Pong: Hello RPC", pingResp.Reply, "Unexpected reply in first call")
	t.Logf("First call response: %s", pingResp.Reply)

	// (Optional Bonus) Test Connection Pooling Behavior (Simplified)
	// Make a second call. If pooling works, it might reuse the connection.
	// We can't directly assert reuse without logs/metrics from RPCClient.
	// But we can ensure it still works.
	pingReq2 := &pb.PingRequest{Message: "Hello RPC Again"}
	pingResp2 := &pb.PingResponse{}
	err = rpcClient.Call(serverAddr, "Ping", pingReq2, pingResp2)
	require.NoError(t, err, "Second RPC call failed")
	assert.Equal(t, "Pong: Hello RPC Again", pingResp2.Reply, "Unexpected reply in second call")
	t.Logf("Second call response: %s", pingResp2.Reply)

	// TODO: Add logging in RPCClient's getConnection/returnConnection to observe pooling in test logs.
	// For now, this test just ensures multiple calls work.
}

func TestRPCCall_HandlerError(t *testing.T) {
	server, serverAddr := startTestRPCServer(t)
	defer server.Close()

	// Register Ping coordinator that returns an error
	server.RegisterHandler("PingError", func(reqPayload []byte) (resPayload []byte, err error) {
		t.Logf("PingError coordinator called, will return an error.")
		return nil, errors.New("coordinator error: something went wrong")
	})

	// Client side
	rpcClient := NewRPCClient(nil, 5, 2*time.Second)
	defer rpcClient.CloseAllConnections()

	pingReq := &pb.PingRequest{Message: "Trigger Error"}
	pingResp := &pb.PingResponse{} // Response proto not strictly needed if error is expected

	err := rpcClient.Call(serverAddr, "PingError", pingReq, pingResp)
	require.Error(t, err, "RPC call should have returned an error")
	assert.Contains(t, err.Error(), "coordinator error: something went wrong", "Error message does not match expected coordinator error")
	t.Logf("Received expected error: %v", err)
}

func TestRPCCall_ServerNotAvailable(t *testing.T) {
	// Client side
	rpcClient := NewRPCClient(nil, 1, 100*time.Millisecond) // Short timeout
	defer rpcClient.CloseAllConnections()

	pingReq := &pb.PingRequest{Message: "Test No Server"}
	pingResp := &pb.PingResponse{}

	// Attempt to call a non-existent server
	nonExistentServerAddr := "localhost:12345" // Assume this port is not in use
	err := rpcClient.Call(nonExistentServerAddr, "Ping", pingReq, pingResp)

	require.Error(t, err, "RPC call to non-existent server should fail")
	// Error might be "connection refused" or "i/o timeout" depending on system and timing
	assert.Contains(t, err.Error(), "failed to get connection", "Error message should indicate connection failure")
	t.Logf("Received expected error for non-existent server: %v", err)
}

func TestRPCCall_MethodNotFound(t *testing.T) {
	server, serverAddr := startTestRPCServer(t)
	defer server.Close()

	// No coordinator registered for "MethodDoesNotExist"

	// Client side
	rpcClient := NewRPCClient(nil, 5, 2*time.Second)
	defer rpcClient.CloseAllConnections()

	pingReq := &pb.PingRequest{Message: "Test Method Not Found"}
	pingResp := &pb.PingResponse{}

	err := rpcClient.Call(serverAddr, "MethodDoesNotExist", pingReq, pingResp)
	require.Error(t, err, "RPC call to non-existent method should fail")
	assert.Contains(t, err.Error(), "no coordinator found for method: MethodDoesNotExist", "Error message should indicate method not found")
	t.Logf("Received expected error for method not found: %v", err)
}
