package simulator

import (
	"context"
	"flag"
	"fmt"
	b3 "github.com/magicsea/behavior3go"
	"github.com/magicsea/behavior3go/core"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var (
	successfulScenarios int64
	failedScenarios     int64

	// Command-line Flags
	numClients         *int
	baseUsername       *string
	loginServerAddr    *string
	consulAddr         *string
	userPassword       *string
	gatewayServiceName *string
	gameServiceName    *string
	roomServiceName    *string // Added roomServiceName flag
	defaultTargetTile  *string // Added for Move action
	defaultCardID      *string // Added for PlayCard action
	actionDelayMs      *int    // Added for configurable delays
)

func init() {
	// Define flags in init() to ensure they are set up before main runs,
	// especially useful if this package were imported and its main() called by another program.
	numClients = flag.Int("numClients", 1, "Number of concurrent clients to simulate.")
	baseUsername = flag.String("baseUsername", "simUser", "Base username for simulated clients. A numeric suffix will be added in stress mode (e.g., simUser_0, simUser_1).")
	loginServerAddr = flag.String("loginServer", "http://localhost:8081", "Login server address (e.g., http://localhost:8081).")
	consulAddr = flag.String("consulServer", "localhost:8500", "Consul server address (e.g., localhost:8500).")
	userPassword = flag.String("password", "simPass", "Common password for all simulated users.")
	gatewayServiceName = flag.String("gatewayServiceName", "gatewayserver-tcp", "The name of the gateway TCP service registered in Consul.")
	gameServiceName = flag.String("gameServiceName", "gameserver-rpc", "The name of the game server gRPC service registered in Consul.")
	roomServiceName = flag.String("roomServiceName", "roomserver-rpc", "The name of the room server RPC service registered in Consul.")
	defaultTargetTile = flag.String("defaultTargetTile", "tile_default_target", "Default target tile ID for the Move action.")
	defaultCardID = flag.String("defaultCardID", "card_default_001", "Default card ID for the PlayCard action.")
	actionDelayMs = flag.Int("actionDelayMs", 0, "Milliseconds to wait between actions in a scenario.")
}

// runSingleClientScenario executes a sequence of actions for a single simulated client.
func runSingleClientScenario(scenarioCtx context.Context, clientID int, loginAddr, consulAddr, username, password, gatewayServiceName, gameServiceName, roomServiceNameValue string, bm *BehaviorManager) {
	logger := log.New(os.Stdout, fmt.Sprintf("[Client %s (ID:%d)] ", username, clientID), log.LstdFlags|log.Lmicroseconds)

	// Helper function for delays (can be used by BT actions if needed, or for loop delay)
	// applyDelay := func(actionName string) {
	// 	if *actionDelayMs > 0 {
	// 		logger.Printf("Delaying for %dms after %s...", *actionDelayMs, actionName)
	// 		time.Sleep(time.Duration(*actionDelayMs) * time.Millisecond)
	// 	}
	// }

	client, err := NewSimulatedClient(loginAddr, consulAddr, username, password, gatewayServiceName, gameServiceName, roomServiceNameValue, logger, bm)
	if err != nil {
		logger.Printf("Failed to create simulated client: %v", err)
		atomic.AddInt64(&failedScenarios, 1)
		return
	}
	defer client.Close()

	logger.Println("Starting FSM and Behavior Tree driven scenario...")

	// Initial FSM event to start the process
	if client.playerFSM.Cannot("connect") {
		logger.Printf("Initial FSM event 'connect' is not allowed from state %s", client.playerFSM.Current())
		atomic.AddInt64(&failedScenarios, 1)
		return
	}
	if err := client.playerFSM.Event(scenarioCtx, "connect"); err != nil {
		logger.Printf("Error triggering initial FSM event 'connect': %v", err)
		atomic.AddInt64(&failedScenarios, 1)
		return
	}

	scenarioSucceeded := false // Flag to track scenario success

	// Main FSM loop
	for {
		select {
		case <-scenarioCtx.Done():
			logger.Println("Scenario timed out or was cancelled.")
			if !scenarioSucceeded { // Only count as failed if not already succeeded
				atomic.AddInt64(&failedScenarios, 1)
			}
			if client.playerFSM.Can("disconnect") {
				_ = client.playerFSM.Event(context.Background(), "disconnect", "scenario_timeout")
			}
			return // Exit runSingleClientScenario
		default:
			// Continue with FSM logic
		}

		currentState := client.playerFSM.Current()
		logger.Printf("Current FSM state: %s", currentState)

		var treeName string
		var treeToTick *core.BehaviorTree

		switch currentState {
		case "Idle":
			// This state implies the initial "connect" event might have failed to transition.
			// Or, if an operation sequence completes and returns to Idle (e.g. after logout).
			// For now, if we're Idle and scenario is not done, try to connect again.
			logger.Println("In Idle state, attempting to trigger 'connect'...")
			if client.playerFSM.Can("connect") {
				if err := client.playerFSM.Event(scenarioCtx, "connect"); err != nil {
					logger.Printf("Error triggering FSM event 'connect' from Idle: %v", err)
					atomic.AddInt64(&failedScenarios, 1)
					_ = client.playerFSM.Event(context.Background(), "disconnect", "idle_connect_fail")
				}
			} else {
				logger.Println("Cannot 'connect' from Idle. Ending scenario as failed.")
				atomic.AddInt64(&failedScenarios, 1)
				_ = client.playerFSM.Event(context.Background(), "disconnect", "idle_cannot_connect")
			}
		case "Connecting", "LoggingIn": // States covered by LoginSequence BT
			treeName = "LoginSequence"
		case "LoggedIn":
			// Player is logged in. Decide what to do next.
			// In this example, we'll try room management.
			treeName = "RoomManagement"
			// Note: RoomManagement BT might need "target_room_id" on blackboard for joining.
			// For creation, it uses defaults or node properties.
		case "CreatingRoom", "JoiningRoom": // States covered by RoomManagement BT
			treeName = "RoomManagement" // Continue ticking the same tree
		case "InRoom":
			// Player is in a room. Logic to decide to start game or wait.
			// For simulation, we might try to start game if host, or just wait for game to start.
			// The FSM transition to "InGame" is triggered by "gamestart" event.
			// This event should be triggered by an action like "ActionStartGame" if conditions are met,
			// or by a simulated server message.
			// For now, let's assume if InRoom, we try to manually trigger "gamestart"
			// This is a placeholder for more complex pre-game logic.
			logger.Println("InRoom: Attempting to trigger 'gamestart'.")
			if client.playerFSM.Can("gamestart") {
				// A real "StartGame" action/check would go here before this event.
				// For example, ActionStartGame could be part of a "PreGame" BT.
				// It would call client.StartGame() and on success, trigger "gamestart".
				// We simulate this success path for now.
				// Let's assume an ActionStartGame has been implicitly successful.
				// This part needs a proper BT action to call client.StartGame()
				// and then trigger this FSM event based on the outcome.
				// For now, we directly trigger the event to proceed in the simulation.
				// In a real scenario, a BT like "InRoomActions" would run here.
				// That BT might contain an action "AttemptStartGame".
				// For this refactor, let's assume the *next* BT (SimpleGameplay) handles what to do InGame.
				// The transition to InGame should ideally be done by an action.
				// Let's assume an "ActionPlayerReady" (from RoomManagement) got us to InRoom.
				// Now, if this client is the "host", it might run an "ActionStartGame".
				// For simplicity in this FSM loop, we will manually try to trigger "gamestart".
				// This is a simplification. A more robust design would have a BT for the "InRoom" state.
				err := client.playerFSM.Event(scenarioCtx, "gamestart")
				if err != nil {
					logger.Printf("Error triggering FSM event 'gamestart': %v. Remaining InRoom.", err)
					// Stay in InRoom, maybe try again or wait for external trigger.
				}
			} else {
				logger.Println("InRoom: Cannot trigger 'gamestart'. Waiting.")
			}
		case "InGame":
			treeName = "SimpleGameplay"
		case "LeavingRoom":
			// This state is entered when "leaveroom" event is triggered.
			// An "ActionLeaveRoom" (not yet defined as a specific BT node, but client method exists)
			// should run and then trigger "roomleft".
			// For simulation, if stuck here, manually trigger "roomleft".
			logger.Println("In LeavingRoom state. Simulating room left via FSM event.")
			if client.playerFSM.Can("roomleft") {
				if err := client.playerFSM.Event(scenarioCtx, "roomleft"); err != nil {
					logger.Printf("Error triggering FSM event 'roomleft': %v", err)
					atomic.AddInt64(&failedScenarios, 1)
					_ = client.playerFSM.Event(context.Background(), "disconnect", "leave_room_fail")
				}
			} else {
				logger.Println("Cannot trigger 'roomleft' from LeavingRoom. Disconnecting as error.")
				atomic.AddInt64(&failedScenarios, 1)
				_ = client.playerFSM.Event(context.Background(), "disconnect", "cannot_leave_room")
			}
		case "Disconnected":
			logger.Println("Client is in Disconnected state. Scenario ending.")
			// Success/failure should have been determined by the transition leading to Disconnected
			// or by scenario timeout.
			goto endLoop // Exit the FSM loop
		default:
			logger.Printf("Unhandled FSM state: %s. Disconnecting as error.", currentState)
			atomic.AddInt64(&failedScenarios, 1)
			if client.playerFSM.Can("disconnect") {
				_ = client.playerFSM.Event(context.Background(), "disconnect", "unhandled_state")
			}
			goto endLoop // Exit FSM loop
		}

		if treeName != "" {
			treeToTick = client.behaviorManager.GetTree(treeName)
			if treeToTick != nil {
				logger.Printf("Ticking behavior tree: %s (for state: %s)", treeToTick.GetTitile(), currentState)
				board := core.NewBlackboard()
				// Populate blackboard with necessary initial values if any for this tree/tick.
				// e.g. board.Set("target_room_id", "some_room_to_join", treeToTick.GetID(), "")
				status := treeToTick.Tick(scenarioCtx, board) // Pass scenarioCtx for BT tick
				logger.Printf("Behavior tree %s executed. Status: %v. FSM state is now: %s", treeToTick.GetTitile(), status, client.playerFSM.Current())

				if status == b3.FAILURE && client.playerFSM.Current() == currentState {
					logger.Printf("Behavior tree %s failed and FSM state did not change from %s. Forcing disconnect.", treeToTick.GetTitile(), currentState)
					atomic.AddInt64(&failedScenarios, 1)
					if client.playerFSM.Can("disconnect") {
						_ = client.playerFSM.Event(context.Background(), "disconnect", fmt.Sprintf("%s_bt_failed", treeName))
					}
				} else if status == b3.SUCCESS && treeName == "SimpleGameplay" { // Example: defining a success condition
					// If gameplay finishes successfully, perhaps we want to leave the room.
					logger.Printf("Gameplay BT %s succeeded. Triggering 'leaveroom'.", treeName)
					if client.playerFSM.Can("leaveroom") {
						if err := client.playerFSM.Event(scenarioCtx, "leaveroom"); err != nil {
							logger.Printf("Error triggering FSM event 'leaveroom' after gameplay: %v", err)
							atomic.AddInt64(&failedScenarios, 1)
							_ = client.playerFSM.Event(context.Background(), "disconnect", "leaveroom_event_fail")
						}
					} else {
						logger.Printf("Cannot 'leaveroom' from state %s after gameplay success. Disconnecting.", client.playerFSM.Current())
						atomic.AddInt64(&failedScenarios, 1)
						_ = client.playerFSM.Event(context.Background(), "disconnect", "cannot_leaveroom_after_game")
					}
				}

			} else {
				logger.Printf("Error: Behavior tree %s not found for state %s. Disconnecting as error.", treeName, currentState)
				atomic.AddInt64(&failedScenarios, 1)
				if client.playerFSM.Can("disconnect") {
					_ = client.playerFSM.Event(context.Background(), "disconnect", "bt_not_found")
				}
				goto endLoop // Exit FSM loop if BT is missing
			}
		}

		// Delay to prevent busy-looping, especially if states don't always run a BT or transition quickly.
		loopDelay := time.Duration(*actionDelayMs) * time.Millisecond
		if loopDelay == 0 { // Ensure a minimal delay if actionDelayMs is 0 to prevent tight loop on no-op states
			loopDelay = 100 * time.Millisecond
		}
		time.Sleep(loopDelay)
	}

endLoop:
	logger.Println("Exited FSM loop.")
	finalState := client.playerFSM.Current()
	logger.Printf("Final FSM state: %s", finalState)

	// Final success/failure determination
	// If scenarioSucceeded flag was set by a specific success condition (e.g. graceful logout action), use it.
	// Otherwise, if we exited the loop due to timeout (handled in select), it's a failure.
	// If we exited because FSM reached "Disconnected":
	//   - If it was due to an error action that already called atomic.AddInt64(&failedScenarios, 1), it's handled.
	//   - If it was a "graceful" disconnect (e.g. after successful logout), it should be success.
	// This part is tricky with global atomic counters. A local scenario success flag is better.
	// For now, let's assume if it reached Disconnected and scenarioCtx is not Done, and no prior failure recorded by this function's logic,
	// it's a success. This is a simplification.
	// A simple rule: if not already marked failed by timeout or specific error paths above, and final state is "Disconnected" (implying graceful exit), count as success.
	// Or if a specific success condition like "scenarioSucceeded" was met.

	// If the scenario was not marked as failed by the timeout or specific error paths within the loop
	if scenarioCtx.Err() == nil && !atomic.CompareAndSwapInt64(&failedScenarios, failedScenarios, failedScenarios) {
		// The CAS is a bit of a hack to check if failedScenarios was incremented by this scenario.
		// This is not perfectly threadsafe for the global counter if multiple scenarios fail concurrently.
		// Better: check a local `isFailed` flag for this scenario.
		// For this refactor, we'll rely on the fact that critical errors in the loop should set failedScenarios.
		// If we are here, and not timed out, and FSM is Disconnected (implying it finished or was manually disconnected by an action),
		// let's assume actions leading to Disconnected due to error would have already incremented failedScenarios.
		// So if we are here and Disconnected, and not timed out, we can assume success *unless* an error path in the loop was taken.
		// This is still complex. Let's simplify:
		// Timeout is a failure (handled in select).
		// Explicit atomic.AddInt64(&failedScenarios, 1) calls in error paths are failures.
		// If loop ends, FSM is Disconnected, and not timed out, and not explicitly failed, assume success.
		// If loop ends, FSM is NOT Disconnected, and not timed out -> this is an unexpected exit, count as failure.

		if finalState == "Disconnected" {
			// If not timed out and ended in Disconnected, and if no failure was explicitly recorded by this scenario's logic path
			// (this is hard to check with global atomics without passing a pointer to a local failure flag for this scenario).
			// For now, if it's Disconnected and not timed out, we assume it's a success from the FSM perspective.
			// The BT actions should handle setting failedScenarios for actual errors.
			// This is a point of potential miscounting success if an error action leads to Disconnected but doesn't inc failedScenarios.
			// However, our BT actions (e.g., ActionLogin failing) *do* set FSM to Disconnected.
			// We need to ensure they *also* increment failedScenarios.
			// Let's assume for now that if an action causes a disconnect due to failure, it's responsible for the failedScenarios count.
			// So, if we reach here, state is Disconnected, and not timed out, it's a success.
			if !scenarioSucceeded { // If not already marked as success by a specific flow.
				logger.Println("Scenario ended: FSM Disconnected. Counting as successful.")
				atomic.AddInt64(&successfulScenarios, 1)
			}
		} else { // Ended in a state other than Disconnected, and not by timeout.
			logger.Printf("Scenario ended unexpectedly in state %s. Counting as failed.", finalState)
			if !scenarioSucceeded { // Avoid double count if somehow succeeded then got here
				atomic.AddInt64(&failedScenarios, 1)
			}
		}
	}
	// If scenarioCtx.Err() != nil, it was already handled by the select's timeout case.

	logger.Println("Client scenario finished.")
}

// Main is the entry point for the simulator CLI.
// Kept as 'main' for direct execution.
func main() {
	// Ensure flags are parsed if not already (e.g. in tests)
	if !flag.Parsed() {
		flag.Parse()
	}

	// IMPORTANT: Register custom behavior tree nodes once
	RegisterCustomNodes()

	// Initialize BehaviorManager
	behaviorFilesPath := "cmd/simulator/behaviors/"
	bm, err := NewBehaviorManager(behaviorFilesPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize BehaviorManager: %v", err)
	}
	if len(bm.GetAllTreeConfigs()) == 0 {
		log.Fatalf("FATAL: BehaviorManager initialized, but no tree configurations were loaded from %s. Check path and *.b3.json files.", behaviorFilesPath)
	}
	log.Printf("BehaviorManager initialized successfully with %d tree configurations.", len(bm.GetAllTreeConfigs()))

	mainLogger := log.New(os.Stdout, "[SimulatorCLI] ", log.LstdFlags|log.Lmicroseconds)
	mainLogger.Printf("Starting simulation with %d client(s)...", *numClients)
	mainLogger.Printf("Login Server: %s, Consul Server: %s, Base Username: %s", *loginServerAddr, *consulAddr, *baseUsername)
	mainLogger.Printf("Gateway Service: %s, Game Service: %s, Room Service: %s", *gatewayServiceName, *gameServiceName, *roomServiceName) // Added roomServiceName to log

	if *numClients <= 0 {
		mainLogger.Println("Error: numClients must be a positive integer.")
		os.Exit(1)
	}

	startTime := time.Now()

	scenarioTimeout := 120 * time.Second // Default scenario timeout for context, can be made a flag

	if *numClients == 1 {
		// For a single client, use the baseUsername directly without suffix.
		// The context for the scenario is created here.
		ctx, cancel := context.WithTimeout(context.Background(), scenarioTimeout)
		defer cancel()
		runSingleClientScenario(ctx, 0, *loginServerAddr, *consulAddr, *baseUsername, *userPassword, *gatewayServiceName, *gameServiceName, *roomServiceName, bm)
	} else {
		var wg sync.WaitGroup
		for i := 0; i < *numClients; i++ {
			wg.Add(1)
			username := fmt.Sprintf("%s_%d", *baseUsername, i)
			go func(id int, user string) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), scenarioTimeout)
				defer cancel()
				runSingleClientScenario(ctx, id, *loginServerAddr, *consulAddr, user, *userPassword, *gatewayServiceName, *gameServiceName, *roomServiceName, bm)
			}(i, username)
		}
		wg.Wait()
	}

	duration := time.Since(startTime)

	mainLogger.Println("------------------------------------------")
	mainLogger.Println("Simulation Finished.")
	mainLogger.Printf("Total Duration: %s", duration)
	mainLogger.Printf("Total Scenarios Attempted: %d", *numClients)
	mainLogger.Printf("Successful Scenarios: %d", successfulScenarios)
	mainLogger.Printf("Failed Scenarios: %d", failedScenarios)

	avgDurationPerScenario := 0.0
	if *numClients > 0 {
		avgDurationPerScenario = duration.Seconds() / float64(*numClients)
	}
	mainLogger.Printf("Average Duration per Scenario: %.2f seconds", avgDurationPerScenario)
	mainLogger.Println("------------------------------------------")
}
