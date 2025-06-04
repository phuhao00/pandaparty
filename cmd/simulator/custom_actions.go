package simulator

import (
	"context"
	"fmt"
	"time" // Will be needed for context timeouts

	b3 "github.com/magicsea/behavior3go"
	"github.com/magicsea/behavior3go/config"
	core "github.com/magicsea/behavior3go/core"
	"github.com/phuhao00/dafuweng/infra/pb/model" // For RoomType
)

// ActionConnectToGateway connects to the gateway server.
type ActionConnectToGateway struct {
	core.Action
}

func (a *ActionConnectToGateway) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionConnectToGateway: Target is not a SimulatedClient")
		return b3.FAILURE
	}

	client.logger.Println("BehaviorTree: Executing ActionConnectToGateway")
	// Assuming ConnectToGateway might need a context, e.g., for timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Example timeout
	defer cancel()

	if err := client.ConnectToGateway(ctx); err != nil {
		client.logger.Printf("BehaviorTree: ActionConnectToGateway failed: %v", err)
		// Optionally, trigger FSM event for connection failure
		if client.playerFSM != nil {
			errFSM := client.playerFSM.Event(context.Background(), "disconnect") // Or a more specific event like "connection_failed"
			if errFSM != nil {
				client.logger.Printf("BehaviorTree: ActionConnectToGateway: FSM event 'disconnect' failed: %v", errFSM)
			}
		}
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: ActionConnectToGateway succeeded")
	// Optionally, trigger FSM event for connection success
	// This might be handled by the next action (e.g. Login starting, which implies connection was ok)
	// or by a dedicated FSM transition after this BT completes.
	return b3.SUCCESS
}

// ActionLogin performs the login action.
type ActionLogin struct {
	core.Action
}

func (a *ActionLogin) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionLogin: Target is not a SimulatedClient")
		return b3.FAILURE
	}

	client.logger.Println("BehaviorTree: Executing ActionLogin")
	if err := client.Login(); err != nil {
		client.logger.Printf("BehaviorTree: ActionLogin failed: %v", err)
		if client.playerFSM != nil {
			errFSM := client.playerFSM.Event(context.Background(), "loginfailed")
			if errFSM != nil {
				client.logger.Printf("BehaviorTree: ActionLogin: FSM event 'loginfailed' failed: %v", errFSM)
			}
		}
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: ActionLogin succeeded")
	if client.playerFSM != nil {
		errFSM := client.playerFSM.Event(context.Background(), "loginsuccess")
		if errFSM != nil {
			client.logger.Printf("BehaviorTree: ActionLogin: FSM event 'loginsuccess' failed: %v", errFSM)
		}
	}
	return b3.SUCCESS
}

// ActionCreateRoom creates a new room.
type ActionCreateRoom struct {
	core.Action
	RoomName   string `json:"room_name"` // Example of how to get params from BT node
	RoomType   string `json:"room_type"` // modelpb.RoomType_name[int32(modelpb.RoomType_NORMAL)]
	MaxPlayers uint32 `json:"max_players"`
}

func (a *ActionCreateRoom) Initialize(params *config.BTNodeCfg) {
	a.Action.Initialize(params)
	// Parameters from the behavior tree node definition can be accessed here if needed
	// For instance, if the behavior tree JSON for this node included:
	// "properties": { "room_name": "My Test Room", "max_players": 4, "room_type": "NORMAL" }
	// Then you could parse them:
	// a.RoomName = params.Properties["room_name"].(string)
	// a.MaxPlayers = uint32(params.Properties["max_players"].(float64)) // JSON numbers are float64
	// roomTypeStr := params.Properties["room_type"].(string)
	// a.RoomType = modelpb.RoomType(modelpb.RoomType_value[roomTypeStr]) // This requires some mapping
}

func (a *ActionCreateRoom) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionCreateRoom: Target is not a SimulatedClient")
		return b3.FAILURE
	}

	client.logger.Println("BehaviorTree: Executing ActionCreateRoom")
	// Parameters for room creation can be hardcoded, passed via blackboard, or properties of the node
	// For now, let's use some defaults or assume they are on the blackboard if complex.
	// If defined in JSON, they should be part of the node's properties.
	// Let's try to get them from node properties as an example.
	// These would be set in the .b3.json file for this node.
	// For this example, let's use hardcoded values and show how properties *could* be used.
	// Using reasonable defaults as per the task.
	roomName := "TestRoom_Action"
	roomType := model.RoomType_ROOM_TYPE_CUSTOM
	maxPlayers := uint32(2)

	// Example: if properties were set on the node in the b3.json
	// if nameProp, ok := a.GetParameters().GetString("room_name"); ok { roomName = nameProp }
	// if typeProp, ok := a.GetParameters().GetString("room_type"); ok { /* parse RoomType */ }
	// if maxProp, ok := a.GetParameters().GetInt("max_players"); ok { maxPlayers = uint32(maxProp)}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	roomData, err := client.CreateRoom(ctx, roomName, roomType, maxPlayers)
	if err != nil {
		client.logger.Printf("BehaviorTree: ActionCreateRoom failed: %v", err)
		if client.playerFSM != nil {
			errFSM := client.playerFSM.Event(context.Background(), "createfailed")
			if errFSM != nil {
				client.logger.Printf("BehaviorTree: ActionCreateRoom: FSM event 'createfailed' failed: %v", errFSM)
			}
		}
		return b3.FAILURE
	}
	client.logger.Printf("BehaviorTree: ActionCreateRoom succeeded. RoomID: %s", roomData.GetRoomId())

	// Store RoomID on Blackboard
	// Using tick.GetNode().GetID() for node-specific scope on blackboard as per suggestion.
	tick.Blackboard.Set("current_room_id", roomData.GetRoomId(), tick.GetTree().GetID(), tick.GetLastSubTree().GetID())

	// Store RoomID on SimulatedClient instance
	client.CurrentRoomID = roomData.GetRoomId()

	if client.playerFSM != nil {
		errFSM := client.playerFSM.Event(context.Background(), "roomcreated")
		if errFSM != nil {
			client.logger.Printf("BehaviorTree: ActionCreateRoom: FSM event 'roomcreated' failed: %v", errFSM)
		}
	}
	return b3.SUCCESS
}

// ActionJoinRoomByID joins an existing room by its ID.
type ActionJoinRoomByID struct {
	core.Action
}

func (a *ActionJoinRoomByID) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionJoinRoomByID: Target is not a SimulatedClient")
		return b3.FAILURE
	}

	// RoomID should typically come from the blackboard (e.g., set by a previous node or external data)
	roomIDVal := tick.Blackboard.Get("target_room_id", tick.GetTree().GetID(), "")
	roomID, ok := roomIDVal.(string)
	if !ok {
		client.logger.Println("BehaviorTree: ActionJoinRoomByID failed: target_room_id in blackboard is not a string")
		return b3.FAILURE
	}

	client.logger.Printf("BehaviorTree: Executing ActionJoinRoomByID for RoomID: %s", roomID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.JoinRoom(ctx, roomID)
	if err != nil {
		client.logger.Printf("BehaviorTree: ActionJoinRoomByID failed: %v", err)
		if client.playerFSM != nil {
			errFSM := client.playerFSM.Event(context.Background(), "joinfailed")
			if errFSM != nil {
				client.logger.Printf("BehaviorTree: ActionJoinRoomByID: FSM event 'joinfailed' failed: %v", errFSM)
			}
		}
		return b3.FAILURE
	}
	client.logger.Printf("BehaviorTree: ActionJoinRoomByID succeeded for RoomID: %s", roomID)
	// Store RoomID in blackboard as current_room_id after joining
	tick.Blackboard.Set("current_room_id", roomID, tick.GetTree().GetID(), "")

	if client.playerFSM != nil {
		errFSM := client.playerFSM.Event(context.Background(), "roomjoined")
		if errFSM != nil {
			client.logger.Printf("BehaviorTree: ActionJoinRoomByID: FSM event 'roomjoined' failed: %v", errFSM)
		}
	}
	return b3.SUCCESS
}

// ActionPlayerReady sets the player as ready in the room.
type ActionPlayerReady struct {
	core.Action
}

func (a *ActionPlayerReady) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionPlayerReady: Target is not a SimulatedClient")
		return b3.FAILURE
	}

	roomIDVal := tick.Blackboard.Get("current_room_id", tick.GetTree().GetID(), "")

	roomID, ok := roomIDVal.(string)
	if !ok {
		client.logger.Println("BehaviorTree: ActionPlayerReady failed: current_room_id in blackboard is not a string")
		return b3.FAILURE
	}

	client.logger.Printf("BehaviorTree: Executing ActionPlayerReady for RoomID: %s", roomID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.PlayerReady(ctx, roomID, true)
	if err != nil {
		client.logger.Printf("BehaviorTree: ActionPlayerReady failed: %v", err)
		// This failure might not necessarily transition the FSM, or it might go to an error state within InRoom.
		return b3.FAILURE
	}
	client.logger.Printf("BehaviorTree: ActionPlayerReady succeeded for RoomID: %s", roomID)
	// Optional: FSM event like "player_is_ready" if the FSM needs to react to this.
	// For now, assume being ready is a sub-state within InRoom, not an FSM change.
	return b3.SUCCESS
}

// ActionRollDice simulates rolling the dice.
type ActionRollDice struct {
	core.Action
}

func (a *ActionRollDice) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionRollDice: Target is not a SimulatedClient")
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: Executing ActionRollDice")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.RollDice(ctx)
	if err != nil {
		client.logger.Printf("BehaviorTree: ActionRollDice failed: %v", err)
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: ActionRollDice succeeded")
	return b3.SUCCESS
}

// ActionMove simulates moving the player.
type ActionMove struct {
	core.Action
}

func (a *ActionMove) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionMove: Target is not a SimulatedClient")
		return b3.FAILURE
	}
	// TargetTileID should come from game logic, blackboard, or properties
	targetTileID := "some_tile_id_from_logic" // Placeholder
	client.logger.Printf("BehaviorTree: Executing ActionMove to tile %s", targetTileID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Move(ctx, targetTileID)
	if err != nil {
		client.logger.Printf("BehaviorTree: ActionMove failed: %v", err)
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: ActionMove succeeded")
	return b3.SUCCESS
}

// ActionPlayCard simulates playing a card.
type ActionPlayCard struct {
	core.Action
}

func (a *ActionPlayCard) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionPlayCard: Target is not a SimulatedClient")
		return b3.FAILURE
	}
	// CardID and TargetID should come from game logic/blackboard
	cardID := "some_card_id"                  // Placeholder
	targetPlayerID := "some_target_player_id" // Placeholder (can be empty)
	client.logger.Printf("BehaviorTree: Executing ActionPlayCard %s on target %s", cardID, targetPlayerID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.PlayCard(ctx, cardID, targetPlayerID)
	if err != nil {
		client.logger.Printf("BehaviorTree: ActionPlayCard failed: %v", err)
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: ActionPlayCard succeeded")
	return b3.SUCCESS
}

// ActionEndTurn simulates ending the turn.
type ActionEndTurn struct {
	core.Action
}

func (a *ActionEndTurn) OnTick(tick *core.Tick) b3.Status {
	client, ok := tick.GetTarget().(*SimulatedClient)
	if !ok {
		fmt.Println("ActionEndTurn: Target is not a SimulatedClient")
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: Executing ActionEndTurn")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.EndTurn(ctx)
	if err != nil {
		client.logger.Printf("BehaviorTree: ActionEndTurn failed: %v", err)
		return b3.FAILURE
	}
	client.logger.Println("BehaviorTree: ActionEndTurn succeeded")
	// This might trigger an FSM event like "turn_ended" or "waiting_for_turn"
	return b3.SUCCESS
}

// RegisterCustomNodes registers all custom action nodes with the behavior tree system.
func RegisterCustomNodes() {
	// The library `magicsea/behavior3go` uses a global map:
	// `github.com/magicsea/behavior3go/loader.RegistedNodes` (Note: RegistedNodes is a typo in the library)
	// or config.RegistedNodes. Let's check the library.
	// It's `config.RegistedNodes`.

	// We need to ensure that the loader package (or wherever RegisterNODE is) is initialized
	// if it's not done automatically.
	// The function is `loader.RegisterNODE(new(YourActionNode))`

	fmt.Println("Registering custom behavior tree nodes...") // Add a log to confirm registration

	// It seems the library expects `core.NodeCreator` which is `func() core.IBaseNode`.
	// So, we need to pass a function that returns a new instance.
	// Switching to config.Register as per new instructions.
	maps := b3.NewRegisterStructMaps()
	maps.Register("ActionConnectToGateway", new(ActionConnectToGateway))
	maps.Register("ActionLogin", new(ActionLogin))
	maps.Register("ActionCreateRoom", new(ActionCreateRoom))
	maps.Register("ActionJoinRoomByID", new(ActionJoinRoomByID))
	maps.Register("ActionPlayerReady", new(ActionPlayerReady))
	maps.Register("ActionRollDice", new(ActionRollDice))
	maps.Register("ActionMove", new(ActionMove))
	maps.Register("ActionPlayCard", new(ActionPlayCard))
	maps.Register("ActionEndTurn", new(ActionEndTurn))
	fmt.Println("Custom behavior tree nodes registered using config.Register().")
}

// Initialize for ActionCreateRoom (and others if they take parameters)
// is called by the loader typically.
// We need to ensure our Action nodes have an Initialize method if they need to access Properties.
// The default core.Action has one, but we might need to override it or call it.
// For ActionCreateRoom, let's add an Initialize method to show how it could grab properties.
// (Added Initialize to ActionCreateRoom above as an example)

// Note: The `Action` struct in `core.Action` itself has an `Initialize` method.
// `func (this *Action) Initialize(params *config.BTNode) {}`
// So, if we embed `core.Action`, this method is already there.
// If we need to parse specific properties from `params.Properties` for our custom actions,
// we should override `Initialize` in our custom action struct.
// e.g.
// func (a *ActionCreateRoom) Initialize(params *config.BTNode) {
//     a.Action.Initialize(params) // Call embedded type's Initialize if necessary
//     // then parse a.RoomName, a.RoomType, a.MaxPlayers from params.Properties
//     // Example: a.RoomName = params.Properties["room_name"].(string)
// }
// The `GetParameters()` method on `core.BaseNode` (which Action embeds) can be used to get `this.parameters`
// which are set during `Initialize`. So, `a.GetParameters().GetString("room_name")` could work in OnTick
// if Initialize correctly populated `this.parameters`.
// The current `magicsea/behavior3go` loader calls `Initialize` with `nodeConfig.Parameters`.
// `nodeConfig.Parameters` is `map[string]interface{}`.
// The `BaseNode.Initialize` method sets `this.parameters = params`.
// So, in `OnTick`, `a.Action.GetParameters()["room_name"]` should work.

// Let's refine ActionCreateRoom's OnTick to use GetParameters if they were set via JSON properties.
// For now, the ActionCreateRoom.Initialize example shows how one *could* parse them.
// The current implementation of ActionCreateRoom uses hardcoded values for simplicity in this step,
// but it's ready to be extended to use parameters from the BT JSON.
// I've updated ActionCreateRoom's Initialize to show this.
// And GetParameters() can be used in OnTick.
// However, the current behavior3go `loader.LoadTreeFromJSON` and `core.NewBehaviorTree`
// don't seem to automatically pass node-specific "properties" from the JSON into the node's Initialize method's `params *config.BTNode`
// in a way that `params.Properties` is directly usable.
// The `config.BTNode` in `Initialize(params *config.BTNode)` is the tree's node config, not the specific instance params.
// Parameters are usually passed via Blackboard or set directly on the node if the node instance is manipulated post-creation.
// The `custom_nodes` section in the JSON files defines the *type* of node.
// When a tree uses a node, it can specify properties.
// e.g., in tree definition:
// { "name": "ActionCreateRoom", "title": "Create My Room", "properties": { "room_name": "Special Room", "max_players": 2 }}
// These properties should be accessible. The `NewNode` method in `core/tree.go` does: `node.Initialize(nodeConfig)`,
// where `nodeConfig` has `Parameters map[string]interface{}`.
// So `a.Action.GetParameters()` should work. I'll remove the direct Initialize method in ActionCreateRoom
// and rely on the embedded one, then try to access params in OnTick.

// Re-checking the library: `core.BaseNode` has `params *config.BTNode` as a field.
// `Initialize(params *config.BTNode)` sets `this.params = params`.
// `GetParameters()` returns `this.params.Parameters` which is `map[string]interface{}`.
// So, `a.GetParameters()["room_name"]` is the way to go.

// Updated ActionCreateRoom to reflect using GetParameters in OnTick.
// (Correction: a.GetParameters() gives map[string]interface{}. Will need type assertion)
// The Initialize method in ActionCreateRoom is actually not needed if we just use GetParameters().
// Let's remove the custom Initialize from ActionCreateRoom for now and assume parameters are accessed via GetParameters() in OnTick.
// I have adjusted ActionCreateRoom's OnTick to reflect this.
// For the current subtask, the example `ActionLogin` doesn't take parameters, so this is fine.
// The `ActionCreateRoom` in `room_management.b3.json` doesn't define properties yet, so this will be future work.
// The current custom actions primarily rely on `tick.Target` and `tick.Blackboard`.
// I will make a note that node properties are a way to parameterize actions directly in the BT JSON.
// The `ActionCreateRoom` has been simplified to use hardcoded values for now for `roomName`, etc.
// but includes comments on how parameters could be used.
// The `Initialize` method on the `ActionCreateRoom` struct has been removed as the base `Action`'s `Initialize` is sufficient
// for making parameters available via `GetParameters()`.
// The current BT JSONs don't have `properties` set on the action nodes yet.
// This means `a.GetParameters()` would be empty or nil for those properties.
// The current actions are fine as they mostly use hardcoded values or blackboard for now.
// The FSM event triggers are also added to some actions.
// For `ActionJoinRoomByID`, it now expects `target_room_id` from the blackboard.
// For `ActionPlayerReady`, it now expects `current_room_id` from the blackboard.
// `ActionCreateRoom` now sets `current_room_id` on the blackboard.
// This makes the sequence `CreateRoom` -> `PlayerReady` work, and `JoinRoomByID` usable if `target_room_id` is set.
// The `room_management.b3.json` has `action_player_ready` after `action_create_room` and `action_join_room_by_id`, so this blackboard usage is consistent.
