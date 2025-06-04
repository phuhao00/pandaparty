# Client Simulator

This directory contains a Go application that simulates client activity for the dafuweng game server. It can be used for:

*   **System-level testing:** Simulating one or more clients performing typical user flows.
*   **Stress testing:** Simulating a large number of concurrent clients to load the server.
*   **Integration test library:** The `SimulatedClient` can be imported into other Go tests to programmatically drive client behavior for testing server-side features.

## Building

To build the simulator, navigate to this directory (`cmd/simulator/`) and run:

```bash
go build
```

This will produce a `simulator` executable (or `simulator.exe` on Windows). If you are in the root of the `dafuweng` repository, you can also build it using:
```bash
go build ./cmd/simulator
```

## Running

The simulator accepts several command-line flags:

*   `-loginServer <address>`: Address of the Login Server (default: `http://localhost:8081`).
*   `-consulServer <address>`: Address of the Consul server (default: `localhost:8500`).
*   `-numClients <count>`: Number of concurrent clients to simulate (default: `1`). If > 1, stress test mode is activated.
*   `-baseUsername <name>`: Base username for simulated clients (default: `simUser`). In stress mode, a numeric suffix is added (e.g., `simUser_0`, `simUser_1`).
*   `-password <password>`: Common password for all simulated clients (default: `simPass`).

### Examples

**Run a single client scenario:**

```bash
./simulator
```
(Uses all default values)

```bash
./simulator -loginServer http://my-login-server:8080 -baseUsername testUser001 -password securepassword
```

**Run a stress test with 50 concurrent clients:**

```bash
./simulator -numClients 50 -baseUsername loadTestUser -password stresstestpass
```

## Using as a Library

The `SimulatedClient` type and its methods can be imported into your Go tests. Ensure your Go environment is set up to resolve the import path to this package.

```go
package mytests

import (
	"context" // Required for client methods
	"log"
	"os"
	"testing"
	
	"github.com/phuhao00/dafuweng/cmd/simulator" // Adjust import path if necessary
	modelpb "github.com/phuhao00/pandaparty/infra/pb/model" // Assuming modelpb is accessible for RoomType
)

func TestMyServerFeature(t *testing.T) {
    // Standard Go logger, customize as needed
    logger := log.New(os.Stdout, "[TestMyServerFeature] ", log.LstdFlags|log.Lshortfile)
    
    // Create a new simulated client
    client := simulator.NewSimulatedClient(
        "http://localhost:8081", // Login server address
        "localhost:8500",        // Consul server address
        "testUserLib",           // Username for the test
        "testPassLib",           // Password for the test
        logger,                  // Logger instance
    )
    defer client.Close() // Ensure resources are cleaned up

    // Perform login
    if err := client.Login(); err != nil {
        t.Fatalf("Login failed: %v", err)
    }
    t.Logf("Client logged in successfully. UserID: %s", client.UserID)

    // Example: Create a room using context.Background()
    // The CreateRoom method expects a context, room name, room type, and max players.
    // modelpb.RoomType_NORMAL_ROOM is used as an example; ensure modelpb is correctly imported.
    roomData, err := client.CreateRoom(context.Background(), "MyTestRoom", modelpb.RoomType_NORMAL_ROOM, 2)
    if err != nil {
        t.Fatalf("CreateRoom failed: %v", err)
    }
    if roomData == nil {
        t.Fatalf("CreateRoom returned nil roomData despite no error")
    }
    // Access room details from roomData, e.g., roomData.GetRoomId(), roomData.GetRoomName()
    t.Logf("Room created successfully: ID %s, Name %s", roomData.GetRoomId(), roomData.GetRoomName())

    // ... further test logic using other client methods like JoinRoom, PlayerReady, etc.
    // Example:
    // joinResp, err := client.JoinRoom(context.Background(), roomData.GetRoomId())
    // if err != nil {
    //     t.Logf("JoinRoom attempt returned: %v (this might be expected)", err)
    // }
    // readyResp, err := client.PlayerReady(context.Background(), roomData.GetRoomId(), true)
    // if err != nil {
    //    t.Fatalf("PlayerReady failed: %v", err)
    // }
}
```
**Notes on using as a library:**
*   Make sure your test environment can resolve the import path to the simulator package (e.g., `github.com/phuhao00/dafuweng/cmd/simulator`). The simulator package itself is named `simulator`.
*   Your test code will also need access to any necessary protobuf definitions (like `modelpb` from `github.com/phuhao00/dafuweng/infra/pb/model`).
*   The simulator's `go.mod` file (in `cmd/simulator/go.mod`) includes a `replace github.com/phuhao00/dafuweng => ../../` directive. This is suitable for development within the `dafuweng` monorepo. If you intend to use the simulator as a standalone library in a different Go module, you might need to adjust your module's `go.mod` or how you vendor/manage this dependency. For instance, you might remove the `replace` directive if the simulator package is published and versioned correctly.
*   The example uses `context.Background()`. In real tests, you might want to use `context.WithTimeout` or other context management practices for more robust testing.

```
