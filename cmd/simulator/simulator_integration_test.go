package simulator

import (
	"context"
	"flag"
	b3 "github.com/magicsea/behavior3go"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/magicsea/behavior3go/core"
	// "github.com/phuhao00/pandaparty/infra/pb/model" // For room type, if needed later
	// "github.com/looplab/fsm" // If direct FSM assertions are made
)

// Helper to initialize flags if not already parsed
func ensureFlagsParsed() {
	if !flag.Parsed() {
		// Set default values for flags used by the simulator client or main logic
		// These might be the same defaults as in main.go's init()
		_ = flag.Set("numClients", "1")
		_ = flag.Set("baseUsername", "testSimUserInteg") // Use a distinct username for tests
		_ = flag.Set("loginServer", "http://localhost:8081")
		_ = flag.Set("consulServer", "localhost:8500")
		_ = flag.Set("password", "testSimPassInteg")
		_ = flag.Set("gatewayServiceName", "gatewayserver-tcp")
		_ = flag.Set("gameServiceName", "gameserver-rpc")
		_ = flag.Set("roomServiceName", "roomserver-rpc")
		_ = flag.Set("defaultTargetTile", "tile_default_target")
		_ = flag.Set("defaultCardID", "card_default_001")
		_ = flag.Set("actionDelayMs", "0")
		flag.Parse()
	}
}

func TestBasicLoginScenario(t *testing.T) {
	ensureFlagsParsed()

	// Correctly determine the path to the 'behaviors' directory.
	// This assumes the test is in 'cmd/simulator' and 'behaviors' is a subdirectory.
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)
	behaviorsPath := filepath.Join(basepath, "behaviors")
	// Check if behaviors directory exists
	if _, err := os.Stat(behaviorsPath); os.IsNotExist(err) {
		// Fallback if not found, try relative to current working dir as a guess
		// This might happen if `go test` is run from a different directory context.
		// For robustness, an absolute path or path relative to module root is better,
		// but for now, this is a common setup.
		behaviorsPath = "./behaviors" // Common if test run with cmd/simulator as CWD
		t.Logf("Behaviors path at '%s' not found, trying './behaviors/'", filepath.Join(basepath, "behaviors"))
		if _, errStat := os.Stat(behaviorsPath); os.IsNotExist(errStat) {
			t.Fatalf("Behaviors directory not found at %s or %s. Please ensure the path is correct.", filepath.Join(basepath, "behaviors"), behaviorsPath)
		}
	} else {
		t.Logf("Using behaviors path: %s", behaviorsPath)
	}

	logger := log.New(os.Stdout, "[TestBasicLoginScenario] ", log.LstdFlags|log.Lmicroseconds)
	logger.Println("Starting test: Basic Login Scenario")

	// 1. Register custom nodes
	RegisterCustomNodes()
	logger.Println("Custom nodes registered.")

	// 2. Initialize BehaviorManager
	bm, err := NewBehaviorManager(behaviorsPath)
	if err != nil {
		t.Fatalf("Failed to create BehaviorManager from path '%s': %v", behaviorsPath, err)
	}
	if len(bm.GetAllTreeConfigs()) == 0 {
		t.Fatalf("BehaviorManager initialized, but no tree configurations were loaded from %s. Check path and *.b3.json files.", behaviorsPath)
	}
	logger.Println("BehaviorManager created.")

	// 3. Create SimulatedClient
	// Use flag values for addresses, etc.
	// Note: The NewSimulatedClient signature in the prompt was:
	// client, err := NewSimulatedClient(loginAddr, consulAddr, username, password, gatewayServiceName, gameServiceName, roomServiceName, bm, logger)
	// I need to ensure this matches the actual current signature.
	// Based on previous steps, it is (..., logger, bm)
	client, err := NewSimulatedClient(
		*loginServerAddr,
		*consulAddr,
		*baseUsername, // This is a pointer, needs dereferencing
		*userPassword,
		*gatewayServiceName,
		*gameServiceName,
		*roomServiceName,
		logger, // logger first
		bm,     // then bm
	)
	if err != nil {
		t.Fatalf("Failed to create simulated client: %v", err)
	}
	defer client.Close()
	logger.Printf("SimulatedClient created for user %s.", client.Username)

	// 4. Basic FSM progression (Idle -> Connecting -> LoggingIn -> LoggedIn)
	scenarioCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Test timeout
	defer cancel()

	// Initial event to start the FSM
	// Ensure FSM is in "Idle" state first
	if client.playerFSM.Current() != "Idle" {
		t.Fatalf("Client FSM initial state is not 'Idle', it's '%s'", client.playerFSM.Current())
	}

	err = client.playerFSM.Event(scenarioCtx, "connect") // Event to move from Idle to Connecting
	if err != nil {
		t.Fatalf("FSM event 'connect' failed: %v. Current state: %s", err, client.playerFSM.Current())
	}
	logger.Printf("Initial FSM event 'connect' dispatched. Current state: %s", client.playerFSM.Current())
	if client.playerFSM.Current() != "Connecting" {
		t.Errorf("Expected FSM state 'Connecting' after 'connect' event, got '%s'", client.playerFSM.Current())
	}

	// Simplified main loop for testing - Part 1: Login
	maxLoginTicks := 10
	loggedIn := false
	for i := 0; i < maxLoginTicks; i++ {
		if scenarioCtx.Err() != nil {
			t.Fatalf("Scenario context timed out or was cancelled during login: %v. Last FSM state: %s", scenarioCtx.Err(), client.playerFSM.Current())
		}

		currentState := client.playerFSM.Current()
		logger.Printf("Login Tick %d: Current FSM state: %s", i+1, currentState)

		if currentState == "LoggedIn" {
			logger.Println("Successfully reached 'LoggedIn' state.")
			loggedIn = true
			break
		}
		if currentState == "Disconnected" {
			t.Fatalf("Client moved to 'Disconnected' state unexpectedly during login sequence. Tick %d", i+1)
		}

		var treeName string
		switch currentState {
		case "Connecting", "LoggingIn":
			treeName = "LoginSequence"
		default:
			t.Fatalf("Reached unexpected state '%s' during login test at tick %d.", currentState, i+1)
		}

		tree := client.behaviorManager.GetTree(treeName)
		if tree == nil {
			t.Fatalf("Behavior tree '%s' not found for state '%s' at tick %d", treeName, currentState, i+1)
		}

		board := core.NewBlackboard()
		status := tree.Tick(scenarioCtx, board)
		logger.Printf("Behavior tree '%s' (for login) ticked with status: %d. FSM state after tick: %s", tree.GetTitile(), status, client.playerFSM.Current())

		time.Sleep(100 * time.Millisecond)
	}

	if !loggedIn {
		t.Fatalf("Failed to reach 'LoggedIn' state within %d ticks. Final state: %s", maxLoginTicks, client.playerFSM.Current())
	}

	// Part 2: Room Operations and Basic Gameplay
	logger.Println("Client is LoggedIn. Proceeding to room creation and gameplay...")
	maxExtendedTicks := 20 // Additional ticks for room and game actions
	finalTargetStateReached := false

	for i := 0; i < maxExtendedTicks; i++ {
		if scenarioCtx.Err() != nil {
			t.Fatalf("Scenario context timed out or was cancelled during room/gameplay: %v. Last FSM state: %s", scenarioCtx.Err(), client.playerFSM.Current())
		}

		currentState := client.playerFSM.Current()
		logger.Printf("Extended Tick %d: Current FSM state: %s", i+1, currentState)

		// Define a target end state for this extended test.
		// For example, after executing one action in SimpleGameplay, we might consider it a success for this test.
		// Or after successfully leaving a room post-gameplay.
		// For this test, let's aim for having ticked SimpleGameplay at least once.
		// We will check if client.playerFSM.Current() was "InGame" and a tick happened.
		// A more robust check might be a custom flag set on the blackboard by the last game action.

		if currentState == "Disconnected" {
			t.Fatalf("Client moved to 'Disconnected' state unexpectedly during room/game sequence. Tick %d", i+1)
		}

		var treeName string
		processBT := true // Flag to indicate if a BT should be ticked in this iteration

		switch currentState {
		case "LoggedIn": // Try to create a room
			treeName = "RoomManagement"
			// ActionCreateRoom in RoomManagement BT should set current_room_id on blackboard
			// and transition FSM to CreatingRoom -> InRoom.
		case "CreatingRoom": // RoomManagement BT is handling this
			treeName = "RoomManagement"
		case "InRoom":
			// After room creation and player ready (handled by RoomManagement BT),
			// we need to trigger "gamestart". In a real scenario, this might be due to
			// host action or server logic. For this test, we'll attempt to trigger it.
			// Then, SimpleGameplay will run.
			logger.Println("InRoom state. Attempting to trigger 'gamestart'.")
			if client.playerFSM.Can("gamestart") {
				// Use client.CurrentRoomID to verify context if needed
				if client.CurrentRoomID == "" {
					t.Logf("Warning: client.CurrentRoomID is empty before attempting 'gamestart'. This might indicate room creation was not fully successful or RoomID was not propagated to the client struct.")
					// Consider if this should be t.Fatalf if a room ID is essential for gamestart.
				} else {
					logger.Printf("Using client.CurrentRoomID '%s' for 'gamestart' context (e.g. if server implicitly uses it via client identity).", client.CurrentRoomID)
				}

				// Simulate a StartGame action implicitly or call it if available as BT.
				// For this test, let's assume the condition to start game is met and trigger the event.
				// In a full test, an ActionStartGame would call client.StartGame and then this event.
				if err := client.playerFSM.Event(scenarioCtx, "gamestart"); err != nil {
					t.Fatalf("Failed to trigger 'gamestart' event from InRoom: %v", err)
				}
				logger.Printf("FSM event 'gamestart' triggered. New state: %s", client.playerFSM.Current())
				// We expect to be in InGame now, SimpleGameplay will be picked up in next iteration or switch case.
				// Avoid ticking another tree immediately after manual event.
				processBT = false
			} else {
				t.Fatalf("Cannot trigger 'gamestart' from InRoom state.")
			}
		case "InGame":
			treeName = "SimpleGameplay"
			// After SimpleGameplay runs (e.g., one sequence of RollDice, Move, etc.),
			// we might consider the main objective of this extended test achieved.
			// For a more complete test, SimpleGameplay would have an EndTurn that might lead to
			// another player's turn or game end.
			// Here, once SimpleGameplay's main sequence is ticked once, we'll set a flag.
			// finalTargetStateReached = true; // This is too simplistic. Let's check after the tick.
		default:
			t.Fatalf("Reached unexpected state '%s' during room/game test at tick %d.", currentState, i+1)
		}

		if processBT && treeName != "" {
			treeToTick := client.behaviorManager.GetTree(treeName)
			if treeToTick == nil {
				t.Fatalf("Behavior tree '%s' not found for state '%s' at tick %d", treeName, currentState, i+1)
			}

			board := core.NewBlackboard() // Fresh for each tree execution cycle in test
			// Populate blackboard if tree expects it (e.g. target_room_id for JoinRoom)
			// For ActionCreateRoom, it uses defaults or its own properties (not configured in this test).

			status := treeToTick.Tick(scenarioCtx, board)
			logger.Printf("Behavior tree '%s' (for room/game) ticked with status: %d. FSM state after tick: %s", treeToTick.GetTitile(), status, client.playerFSM.Current())

			if status == b3.FAILURE && client.playerFSM.Current() == currentState {
				t.Fatalf("BT %s failed and FSM state did not change from %s. Aborting.", treeName, currentState)
			}

			if treeName == "SimpleGameplay" && status == b3.SUCCESS {
				logger.Println("SimpleGameplay BT finished successfully. Test objective met.")
				finalTargetStateReached = true
				break // Exit extended loop
			}
		} else if !processBT {
			logger.Printf("Skipping BT tick in state %s due to manual FSM event this iteration.", currentState)
		}

		time.Sleep(200 * time.Millisecond) // Allow FSM to process events from actions / stabilize

		if i == maxExtendedTicks-1 && !finalTargetStateReached {
			t.Fatalf("Failed to complete desired room/gameplay sequence within %d ticks. Final state: %s", maxExtendedTicks, client.playerFSM.Current())
		}
	}

	if !finalTargetStateReached {
		t.Fatalf("Test completed, but desired room/gameplay sequence (e.g., SimpleGameplay success) was not achieved. Final state: %s", client.playerFSM.Current())
	}

	logger.Println("TestBasicLoginScenario with room/game operations completed successfully.")
}
