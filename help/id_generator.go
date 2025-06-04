package help

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// Snowflake-like ID generator for generating unique IDs
// This is a simplified version that generates 64-bit integers
type IDGenerator struct {
	mutex    sync.Mutex
	epoch    int64 // Custom epoch (e.g., 2020-01-01 00:00:00 UTC)
	nodeID   int64 // Node/machine ID (0-1023)
	sequence int64 // Sequence number (0-4095)
	lastTime int64 // Last timestamp
}

const (
	// Bit lengths
	sequenceBits  = 12
	nodeIDBits    = 10
	timestampBits = 41

	// Max values
	maxNodeID   = (1 << nodeIDBits) - 1   // 1023
	maxSequence = (1 << sequenceBits) - 1 // 4095

	// Bit shifts
	nodeIDShift    = sequenceBits
	timestampShift = sequenceBits + nodeIDBits

	// Custom epoch: 2020-01-01 00:00:00 UTC
	customEpoch = 1577836800000 // milliseconds
)

var (
	defaultGenerator *IDGenerator
	once             sync.Once
)

// GetDefaultIDGenerator returns the default ID generator instance
func GetDefaultIDGenerator() *IDGenerator {
	once.Do(func() {
		defaultGenerator = NewIDGenerator(1) // Default node ID is 1
	})
	return defaultGenerator
}

// NewIDGenerator creates a new ID generator with the specified node ID
func NewIDGenerator(nodeID int64) *IDGenerator {
	if nodeID < 0 || nodeID > maxNodeID {
		panic(fmt.Sprintf("node ID must be between 0 and %d", maxNodeID))
	}

	return &IDGenerator{
		epoch:    customEpoch,
		nodeID:   nodeID,
		sequence: 0,
		lastTime: 0,
	}
}

// GenerateID generates a new unique ID
func (g *IDGenerator) GenerateID() uint64 {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	now := time.Now().UnixMilli()

	if now < g.lastTime {
		panic("clock moved backwards")
	}

	if now == g.lastTime {
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			// Sequence overflow, wait for next millisecond
			for now <= g.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTime = now

	// Generate the ID
	timestamp := now - g.epoch
	id := (timestamp << timestampShift) | (g.nodeID << nodeIDShift) | g.sequence

	return uint64(id)
}

// GenerateIDString generates a new unique ID as string
func (g *IDGenerator) GenerateIDString() string {
	return Uint64ToString(g.GenerateID())
}

// GenerateUint64ID generates a new unique uint64 ID (alias for GenerateID)
func (g *IDGenerator) GenerateUint64ID() uint64 {
	return g.GenerateID()
}

// ID generation functions with prefixes

// GeneratePlayerID generates a unique player ID with "P" prefix
func GeneratePlayerID() string {
	id := GetDefaultIDGenerator().GenerateID()
	return fmt.Sprintf("P%d", id)
}

// GeneratePlayerUint64ID generates a unique player ID as uint64
func GeneratePlayerUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateRoomID generates a unique room ID with "R" prefix
func GenerateRoomID() string {
	id := GetDefaultIDGenerator().GenerateID()
	return fmt.Sprintf("R%d", id)
}

// GenerateRoomUint64ID generates a unique room ID as uint64
func GenerateRoomUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateMatchID generates a unique match ID with "M" prefix
func GenerateMatchID() string {
	id := GetDefaultIDGenerator().GenerateID()
	return fmt.Sprintf("M%d", id)
}

// GenerateMatchUint64ID generates a unique match ID as uint64
func GenerateMatchUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateSessionID generates a unique session ID with "S" prefix
func GenerateSessionID() string {
	id := GetDefaultIDGenerator().GenerateID()
	return fmt.Sprintf("S%d", id)
}

// GenerateSessionUint64ID generates a unique session ID as uint64
func GenerateSessionUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateOrderID generates a unique order ID with "O" prefix and timestamp
func GenerateOrderID() string {
	id := GetDefaultIDGenerator().GenerateID()
	timestamp := time.Now().Format("20060102")
	return fmt.Sprintf("O%s%d", timestamp, id)
}

// GenerateOrderUint64ID generates a unique order ID as uint64
func GenerateOrderUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateOrderInt64ID generates a unique order ID as int64
func GenerateOrderInt64ID() int64 {
	return Uint64ToInt64(GetDefaultIDGenerator().GenerateID())
}

// Int64 ID generation functions

// GenerateInt64ID generates a new unique int64 ID
func (g *IDGenerator) GenerateInt64ID() int64 {
	return Uint64ToInt64(g.GenerateID())
}

// GeneratePlayerInt64ID generates a unique player ID as int64
func GeneratePlayerInt64ID() int64 {
	return Uint64ToInt64(GetDefaultIDGenerator().GenerateID())
}

// GenerateSessionInt64ID generates a unique session ID as int64
func GenerateSessionInt64ID() int64 {
	return Uint64ToInt64(GetDefaultIDGenerator().GenerateID())
}

// Buff-specific ID generators

// GenerateBuffInt64ID generates a unique buff ID as int64
func GenerateBuffInt64ID() int64 {
	return GenerateShortInt64ID()
}

// GenerateBuffUint64ID generates a unique buff ID as uint64
func GenerateBuffUint64ID() uint64 {
	return GenerateShortUint64ID()
}

// GenerateBuffStringID generates a unique buff ID as string
func GenerateBuffStringID() string {
	return Int64ToString(GenerateBuffInt64ID())
}

// GenerateMatchInt64ID generates a unique match ID as int64
func GenerateMatchInt64ID() int64 {
	return GenerateShortInt64ID()
}

// GenerateRoomInt64ID generates a unique room ID as int64
func GenerateRoomInt64ID() int64 {
	return GenerateShortInt64ID()
}

// GenerateActionInt64ID generates a unique action ID as int64
func GenerateActionInt64ID() int64 {
	return GenerateShortInt64ID()
}

// Utility functions for uint64 and string conversion
func Uint64ToString(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func StringToUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// Utility functions for int64 and string conversion
func Int64ToString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func StringToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// Convert between uint64 and int64
func Uint64ToInt64(id uint64) int64 {
	if id > 9223372036854775807 { // max int64
		return int64(id - 9223372036854775808) // wrap around
	}
	return int64(id)
}



// Alternative simple incremental ID generator for testing/development
type SimpleIDGenerator struct {
	mutex   sync.Mutex
	counter uint64
	prefix  string
}

// NewSimpleIDGenerator creates a simple incremental ID generator
func NewSimpleIDGenerator(prefix string, startFrom uint64) *SimpleIDGenerator {
	return &SimpleIDGenerator{
		counter: startFrom,
		prefix:  prefix,
	}
}

// Next generates the next incremental ID
func (s *SimpleIDGenerator) Next() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.counter++
	if s.prefix != "" {
		return fmt.Sprintf("%s%d", s.prefix, s.counter)
	}
	return strconv.FormatUint(s.counter, 10)
}

// NextUint64 generates the next incremental ID as uint64
func (s *SimpleIDGenerator) NextUint64() uint64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.counter++
	return s.counter
}

// NextInt64 generates the next incremental ID as int64
func (s *SimpleIDGenerator) NextInt64() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.counter++
	return int64(s.counter)
}

// Utility functions for common ID generation patterns

// GenerateUniqueID generates a timestamp-based unique ID
func GenerateUniqueID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateUniqueIDString generates a timestamp-based unique ID as string
func GenerateUniqueIDString() string {
	return GetDefaultIDGenerator().GenerateIDString()
}

// GenerateShortID generates a shorter ID for less critical use cases
func GenerateShortID() string {
	now := time.Now().Unix()
	seq := GetDefaultIDGenerator().GenerateID() & 0xFFFF // Use lower 16 bits as sequence
	return fmt.Sprintf("%d%04d", now, seq)
}

// GenerateShortUint64ID generates a shorter uint64 ID for less critical use cases
func GenerateShortUint64ID() uint64 {
	now := time.Now().Unix()
	seq := GetDefaultIDGenerator().GenerateID() & 0xFFFF // Use lower 16 bits as sequence
	return uint64(now*100000 + int64(seq))
}

// GenerateShortInt64ID generates a shorter int64 ID for less critical use cases
func GenerateShortInt64ID() int64 {
	now := time.Now().Unix()
	seq := GetDefaultIDGenerator().GenerateID() & 0xFFFF // Use lower 16 bits as sequence
	return now*100000 + int64(seq)
}

// Batch ID generation functions

// GenerateBatchUint64IDs generates multiple unique uint64 IDs
func GenerateBatchUint64IDs(count int) []uint64 {
	ids := make([]uint64, count)
	generator := GetDefaultIDGenerator()

	for i := 0; i < count; i++ {
		ids[i] = generator.GenerateID()
	}

	return ids
}

// GenerateBatchStringIDs generates multiple unique string IDs
func GenerateBatchStringIDs(count int) []string {
	ids := make([]string, count)
	generator := GetDefaultIDGenerator()

	for i := 0; i < count; i++ {
		ids[i] = Uint64ToString(generator.GenerateID())
	}

	return ids
}

// GenerateBatchInt64IDs generates multiple unique int64 IDs
func GenerateBatchInt64IDs(count int) []int64 {
	ids := make([]int64, count)
	generator := GetDefaultIDGenerator()

	for i := 0; i < count; i++ {
		ids[i] = Uint64ToInt64(generator.GenerateID())
	}

	return ids
}

// Specific game-related ID generators

// GenerateTransactionUint64ID generates a unique transaction ID as uint64
func GenerateTransactionUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateLogUint64ID generates a unique log entry ID as uint64
func GenerateLogUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateMessageUint64ID generates a unique message ID as uint64
func GenerateMessageUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// GenerateNotificationUint64ID generates a unique notification ID as uint64
func GenerateNotificationUint64ID() uint64 {
	return GetDefaultIDGenerator().GenerateID()
}

// ID validation functions

// IsValidUint64ID checks if an ID is in valid uint64 range
func IsValidUint64ID(id uint64) bool {
	return id > 0 && id <= ^uint64(0) // Check if it's a positive number within uint64 range
}

// IsValidIDString checks if a string represents a valid uint64 ID
func IsValidIDString(idStr string) bool {
	if idStr == "" {
		return false
	}

	id, err := StringToUint64(idStr)
	if err != nil {
		return false
	}

	return IsValidUint64ID(id)
}

// IsValidInt64ID checks if an ID is in valid int64 range
func IsValidInt64ID(id int64) bool {
	return id > 0 // Check if it's a positive number
}

// IsValidInt64IDString checks if a string represents a valid int64 ID
func IsValidInt64IDString(idStr string) bool {
	if idStr == "" {
		return false
	}

	id, err := StringToInt64(idStr)
	if err != nil {
		return false
	}

	return IsValidInt64ID(id)
}

// ID range generation for testing

// GenerateIDRange generates a range of consecutive uint64 IDs for testing
func GenerateIDRange(start, count uint64) []uint64 {
	ids := make([]uint64, count)
	for i := uint64(0); i < count; i++ {
		ids[i] = start + i
	}
	return ids
}

// GenerateInt64IDRange generates a range of consecutive int64 IDs for testing
func GenerateInt64IDRange(start int64, count int64) []int64 {
	ids := make([]int64, count)
	for i := int64(0); i < count; i++ {
		ids[i] = start + i
	}
	return ids
}

// ParsePlayerID extracts the numeric ID from a player ID string
func ParsePlayerID(playerIDStr string) (uint64, error) {
	if len(playerIDStr) < 2 || playerIDStr[0] != 'P' {
		return 0, fmt.Errorf("invalid player ID format: %s", playerIDStr)
	}

	return StringToUint64(playerIDStr[1:])
}

// ParseRoomID extracts the numeric ID from a room ID string
func ParseRoomID(roomIDStr string) (uint64, error) {
	if len(roomIDStr) < 2 || roomIDStr[0] != 'R' {
		return 0, fmt.Errorf("invalid room ID format: %s", roomIDStr)
	}

	return StringToUint64(roomIDStr[1:])
}

// ParseMatchID extracts the numeric ID from a match ID string
func ParseMatchID(matchIDStr string) (uint64, error) {
	if len(matchIDStr) < 2 || matchIDStr[0] != 'M' {
		return 0, fmt.Errorf("invalid match ID format: %s", matchIDStr)
	}

	return StringToUint64(matchIDStr[1:])
}

// ParseSessionID extracts the numeric ID from a session ID string
func ParseSessionID(sessionIDStr string) (uint64, error) {
	if len(sessionIDStr) < 2 || sessionIDStr[0] != 'S' {
		return 0, fmt.Errorf("invalid session ID format: %s", sessionIDStr)
	}

	return StringToUint64(sessionIDStr[1:])
}

// ParsePlayerIDToInt64 extracts the numeric ID from a player ID string as int64
func ParsePlayerIDToInt64(playerIDStr string) (int64, error) {
	if len(playerIDStr) < 2 || playerIDStr[0] != 'P' {
		return 0, fmt.Errorf("invalid player ID format: %s", playerIDStr)
	}

	return StringToInt64(playerIDStr[1:])
}

// ParseRoomIDToInt64 extracts the numeric ID from a room ID string as int64
func ParseRoomIDToInt64(roomIDStr string) (int64, error) {
	if len(roomIDStr) < 2 || roomIDStr[0] != 'R' {
		return 0, fmt.Errorf("invalid room ID format: %s", roomIDStr)
	}

	return StringToInt64(roomIDStr[1:])
}

// ParseMatchIDToInt64 extracts the numeric ID from a match ID string as int64
func ParseMatchIDToInt64(matchIDStr string) (int64, error) {
	if len(matchIDStr) < 2 || matchIDStr[0] != 'M' {
		return 0, fmt.Errorf("invalid match ID format: %s", matchIDStr)
	}

	return StringToInt64(matchIDStr[1:])
}

// ParseSessionIDToInt64 extracts the numeric ID from a session ID string as int64
func ParseSessionIDToInt64(sessionIDStr string) (int64, error) {
	if len(sessionIDStr) < 2 || sessionIDStr[0] != 'S' {
		return 0, fmt.Errorf("invalid session ID format: %s", sessionIDStr)
	}

	return StringToInt64(sessionIDStr[1:])
}
