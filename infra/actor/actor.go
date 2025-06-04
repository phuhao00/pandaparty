package actor

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"sync"
)

type ActorID int64

func (receiver ActorID) String() string {
	return fmt.Sprintf("%d", receiver)
}

type IActor interface {
	Id() ActorID
	Name() string
	Tell(ctx context.Context, message proto.Message) error
	Ask(ctx context.Context, message proto.Message) (interface{}, error)
	Stop()
}

// --- Actor Implementation ---

// actorMessage is an internal wrapper for messages in the mailbox.
type actorMessage struct {
	ctx     context.Context // Context passed with Tell/Ask
	message proto.Message
	// For Ask pattern
	replyCh chan<- interface{} // channel to send the response back
	errorCh chan<- error       // channel to send an error back
}

// ActorProcessor defines the interface for message processing logic within an Actor.
// The user of the actor package implements this interface to define actor's behavior.
type ActorProcessor interface {
	// ProcessMessage is called for each message received by the actor.
	// It can return a response (for Ask) and/or an error.
	// For Tell, the response is ignored.
	ProcessMessage(actorCtx IActorContext, msg proto.Message) (response proto.Message, err error)
}

// IActorContext provides methods for the ActorProcessor to interact with its environment.
type IActorContext interface {
	Self() IActor // Gets a reference to the actor itself.
	// TODO: Add other methods like SpawnChild, Watch, etc. if needed.
}

// Actor is the concrete implementation of the IActor interface.
type Actor struct {
	id        ActorID
	name      string
	processor ActorProcessor
	mailbox   chan *actorMessage
	stopCh    chan struct{}  // Channel to signal the actor to stop
	wg        sync.WaitGroup // To wait for the processing goroutine to finish
	self      IActor         // Stores its own IActor interface reference
}

const defaultMailboxSize = 128

// NewActor creates and starts a new actor.
func NewActor(id ActorID, name string, processor ActorProcessor, mailboxSize ...int) *Actor {
	if processor == nil {
		log.Panic("ActorProcessor cannot be nil")
	}
	size := defaultMailboxSize
	if len(mailboxSize) > 0 && mailboxSize[0] > 0 {
		size = mailboxSize[0]
	}

	actor := &Actor{
		id:        id,
		name:      name,
		processor: processor,
		mailbox:   make(chan *actorMessage, size),
		stopCh:    make(chan struct{}),
	}
	actor.self = actor // Self-reference for IActorContext

	actor.wg.Add(1)
	go actor.run()

	return actor
}

// Id returns the actor's unique identifier.
func (a *Actor) Id() ActorID {
	return a.id
}

// Name returns the actor's name (primarily for debugging/logging).
func (a *Actor) Name() string {
	return a.name
}

// Tell sends an asynchronous message to the actor.
// The message is added to the actor's mailbox and processed sequentially.
// Returns an error if the message cannot be sent (e.g., mailbox full or actor stopped).
func (a *Actor) Tell(ctx context.Context, message proto.Message) error {
	if message == nil {
		return errors.New("cannot Tell a nil message")
	}
	msg := &actorMessage{
		ctx:     ctx,
		message: message,
	}

	// Non-blocking send to mailbox, or error if stopped/full
	select {
	case a.mailbox <- msg:
		return nil
	case <-a.stopCh:
		return errors.New("actor stopped, cannot Tell message")
	default:
		// Try one more time with a select that can also catch stopCh
		select {
		case a.mailbox <- msg:
			return nil
		case <-a.stopCh:
			return errors.New("actor stopped, cannot Tell message")
		default:
			// This indicates mailbox is full.
			// Depending on desired behavior, could block, return error, or drop.
			// For now, return error for full mailbox.
			log.Printf("Actor %s (%d) mailbox full. Message dropped: %T", a.name, a.id, message)
			return errors.New("actor mailbox is full")
		}
	}
}

// Ask sends a message to the actor and waits for a response.
// The response is the first return value. An error can also be returned.
// The provided context can be used for timeouts or cancellation.
func (a *Actor) Ask(ctx context.Context, message proto.Message) (interface{}, error) {
	if message == nil {
		return nil, errors.New("cannot Ask a nil message")
	}

	// Create channels for reply and error
	// Buffer them to prevent sender lockup if receiver (Ask) times out before sender (actor loop) replies.
	replyCh := make(chan interface{}, 1)
	errorCh := make(chan error, 1)

	msg := &actorMessage{
		ctx:     ctx, // Context for the message processing itself
		message: message,
		replyCh: replyCh,
		errorCh: errorCh,
	}

	// Send to mailbox, checking if actor is stopped.
	select {
	case a.mailbox <- msg:
		// Message sent, now wait for reply or context done.
	case <-a.stopCh:
		return nil, errors.New("actor stopped, cannot Ask message")
	case <-ctx.Done(): // Check caller's context before even sending if mailbox is full
		return nil, ctx.Err()
	}

	// Wait for the response, error, or context cancellation from the caller.
	select {
	case response := <-replyCh:
		return response, nil
	case err := <-errorCh:
		return nil, err
	case <-ctx.Done(): // Caller's context (e.g., for timeout)
		// TODO: How to notify the actor that the Ask caller has timed out and is no longer waiting?
		// This is important if the ProcessMessage is very long-running.
		// For now, the actor will still process and attempt to send to replyCh/errorCh,
		// but the send will not block due to buffered channels.
		log.Printf("Actor %s (%d) Ask call timed out by caller for message: %T", a.name, a.id, message)
		return nil, ctx.Err()
	case <-a.stopCh: // Actor itself stopped while Ask was waiting
		return nil, errors.New("actor stopped while Ask was waiting for a reply")
	}
}

// Stop signals the actor to terminate its processing goroutine.
// It waits for the goroutine to finish before returning.
func (a *Actor) Stop() {
	close(a.stopCh) // Signal the run loop to stop
	a.wg.Wait()     // Wait for the run loop to exit
	// Note: Mailbox is not explicitly closed here while run loop might be draining.
	// The run loop will exit when stopCh is closed and mailbox is eventually empty or handled.
}

// run is the actor's main processing loop.
// It should not be called directly. It's started by NewActor.
func (a *Actor) run() {
	defer a.wg.Done()
	defer func() {
		// Drain mailbox on stop, replying with errors for any pending Ask messages
		// This prevents senders of Ask from being stuck indefinitely if actor stops.
		close(a.mailbox)             // Close mailbox to signal no more messages will be accepted or processed
		for msg := range a.mailbox { // Range over remaining messages
			if msg.replyCh != nil { // This was an Ask
				select {
				case msg.errorCh <- errors.New("actor stopped before processing Ask message"):
				default: // Avoid blocking if errorCh is not listened to (e.g., Ask already timed out)
				}
			}
		}
		log.Printf("Actor %s (%d) processing loop stopped.", a.name, a.id)
	}()

	log.Printf("Actor %s (%d) processing loop started.", a.name, a.id)
	actorCtx := &actorContextImpl{actor: a}

	for {
		select {
		case msg, ok := <-a.mailbox:
			if !ok { // Mailbox closed, should only happen if stopCh was also closed.
				return
			}
			if msg == nil { // Should not happen with proper Tell/Ask
				continue
			}

			// Process the message using the provided processor
			// We need to handle the context of the message (msg.ctx) vs actor's lifecycle context.
			// For now, ProcessMessage gets the IActorContext which can provide actor's self-ref.
			// The msg.ctx could be used inside ProcessMessage if needed for that specific message's lifecycle.
			response, err := a.processor.ProcessMessage(actorCtx, msg.message)

			if msg.replyCh != nil { // This was an Ask message
				if err != nil {
					select {
					case msg.errorCh <- err:
					case <-a.stopCh: // Actor stopped while trying to send error
					case <-msg.ctx.Done(): // Original Ask context timed out/cancelled
					}
				} else {
					select {
					case msg.replyCh <- response: // response can be proto.Message or any interface{}
					case <-a.stopCh: // Actor stopped while trying to send reply
					case <-msg.ctx.Done(): // Original Ask context timed out/cancelled
					}
				}
				close(msg.replyCh)
				close(msg.errorCh)
			} else if err != nil { // This was a Tell message and processing resulted in an error
				log.Printf("Error processing Tell message for actor %s (%d): %v. Message: %T", a.name, a.id, err, msg.message)
				// For Tell, errors are typically logged or sent to a dead letter queue.
			}

		case <-a.stopCh: // Stop signal received
			return
		}
	}
}

// actorContextImpl implements IActorContext.
type actorContextImpl struct {
	actor *Actor // Reference to the actor instance
}

func (aci *actorContextImpl) Self() IActor {
	return aci.actor.self // Return the stored IActor interface
}

// --- Helper for proto.Message compatibility with Ask's interface{} response ---
// If ActorProcessor always returns proto.Message, Ask can be more type-safe internally.
// However, the IActor.Ask signature is interface{}.
// The current implementation correctly sends proto.Message (as interface{}) via replyCh.
