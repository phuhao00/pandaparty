package simulator

import (
	"context"
	"fmt"

	"github.com/looplab/fsm"
)

// PlayerFSMEvents is a list of events that can trigger state transitions.
var PlayerFSMEvents = []fsm.EventDesc{
	{Name: "connect", Src: []string{"Idle", "Disconnected"}, Dst: "Connecting"},
	{Name: "login", Src: []string{"Connecting"}, Dst: "LoggingIn"},
	{Name: "loginsuccess", Src: []string{"LoggingIn"}, Dst: "LoggedIn"},
	{Name: "loginfailed", Src: []string{"LoggingIn"}, Dst: "Disconnected"},
	{Name: "createroom", Src: []string{"LoggedIn"}, Dst: "CreatingRoom"},
	{Name: "joinroom", Src: []string{"LoggedIn"}, Dst: "JoiningRoom"},
	{Name: "roomcreated", Src: []string{"CreatingRoom"}, Dst: "InRoom"},
	{Name: "createfailed", Src: []string{"CreatingRoom"}, Dst: "LoggedIn"},
	{Name: "roomjoined", Src: []string{"JoiningRoom"}, Dst: "InRoom"},
	{Name: "joinfailed", Src: []string{"JoiningRoom"}, Dst: "LoggedIn"},
	{Name: "gamestart", Src: []string{"InRoom"}, Dst: "InGame"},
	{Name: "leaveroom", Src: []string{"InRoom", "InGame"}, Dst: "LeavingRoom"},
	{Name: "roomleft", Src: []string{"LeavingRoom"}, Dst: "LoggedIn"},
	{Name: "disconnect", Src: []string{"Idle", "Connecting", "LoggingIn", "LoggedIn", "CreatingRoom", "JoiningRoom", "InRoom", "InGame", "LeavingRoom"}, Dst: "Disconnected"},
}

// PlayerFSMStates is a list of states for the player FSM.
// While not strictly needed by the fsm library itself for definition (events define states),
// it's good for documentation and potentially for validation.
var PlayerFSMStates = []string{
	"Idle",
	"Connecting",
	"LoggingIn",
	"LoggedIn",
	"CreatingRoom",
	"JoiningRoom",
	"InRoom",
	"InGame",
	"LeavingRoom",
	"Disconnected",
}

// NewPlayerFSM creates a new FSM for a player.
func NewPlayerFSM(initialState string, client *SimulatedClient) *fsm.FSM {
	callbacks := fsm.Callbacks{
		"enter_state": func(_ context.Context, e *fsm.Event) {
			if client != nil && client.logger != nil {
				client.logger.Printf("FSM: Client %s transitioning from %s to %s (event: %s)", client.Username, e.Src, e.Dst, e.Event)
			} else {
				fmt.Printf("FSM: Client %s transitioning from %s to %s (event: %s)\n", client.Username, e.Src, e.Dst, e.Event)
			}
		},
		// Example of specific state entry if needed later:
		// "enter_LoggedIn": func(_ context.Context, e *fsm.Event) {
		// 	 fmt.Printf("FSM: Client %s has successfully logged in.\n", client.Username)
		// },
		"leave_state": func(_ context.Context, e *fsm.Event) {
			// This callback is called when leaving any state.
			// Useful for generic cleanup or logging before transitioning.
			if client != nil && client.logger != nil {
				client.logger.Printf("FSM: Client %s leaving state %s (event: %s, next state: %s)", client.Username, e.Src, e.Event, e.Dst)
			} else {
				fmt.Printf("FSM: Client %s leaving state %s (event: %s, next state: %s)\n", client.Username, e.Src, e.Event, e.Dst)
			}
		},
		// Callbacks for specific transitions if fine-grained control or logging is needed
		"before_connect": func(_ context.Context, e *fsm.Event) {
			if client != nil && client.logger != nil {
				client.logger.Printf("FSM: Client %s about to connect (from %s). Event: %s", client.Username, e.Src, e.Event)
			} else {
				fmt.Printf("FSM: Client %s about to connect (from %s). Event: %s\n", client.Username, e.Src, e.Event)
			}
		},
		"after_disconnect": func(_ context.Context, e *fsm.Event) {
			if client != nil && client.logger != nil {
				client.logger.Printf("FSM: Client %s has disconnected. Was in state %s. Event: %s", client.Username, e.Src, e.Event)
			} else {
				fmt.Printf("FSM: Client %s has disconnected. Was in state %s. Event: %s\n", client.Username, e.Src, e.Event)
			}
		},
	}

	return fsm.NewFSM(
		initialState,
		PlayerFSMEvents,
		callbacks,
	)
}
