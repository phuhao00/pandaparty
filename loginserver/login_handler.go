package loginserver

import (
	"io"
	"log"
	"net/http"

	"sync" // Added for sync.Pool

	pb "github.com/phuhao00/pandaparty/infra/pb/protocol/login"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto" // Added for proto.Reset
)

var (
	loginRequestPool = sync.Pool{
		New: func() interface{} {
			return &pb.LoginRequest{}
		},
	}
	loginResponsePool = sync.Pool{
		New: func() interface{} {
			return &pb.LoginResponse{}
		},
	}
	validateSessionRequestPool = sync.Pool{
		New: func() interface{} {
			return &pb.ValidateSessionRequest{}
		},
	}
	validateSessionResponsePool = sync.Pool{
		New: func() interface{} {
			return &pb.ValidateSessionResponse{}
		},
	}
)

// LoginHandler handles HTTP requests for login.
type LoginHandler struct {
	loginImpl *LoginImpl
}

// NewLoginHandler creates a new instance of LoginHandler.
func NewLoginHandler(impl *LoginImpl) *LoginHandler {
	if impl == nil {
		log.Fatalf("NewLoginHandler received a nil LoginImpl")
	}
	return &LoginHandler{
		loginImpl: impl,
	}
}

// HandleLogin is the HTTP coordinator function for the /api/login endpoint.
func (h *LoginHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	loginReq := loginRequestPool.Get().(*pb.LoginRequest)
	defer func() {
		proto.Reset(loginReq)
		loginRequestPool.Put(loginReq)
	}()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Using protojson to unmarshal directly into the protobuf struct
	if err := protojson.Unmarshal(body, loginReq); err != nil {
		log.Printf("Error unmarshalling request JSON to LoginRequest: %v", err)
		http.Error(w, "Invalid request format: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received login request via HTTP: Username=%s", loginReq.Username)

	// Call the core logic
	// The ProcessLogin method is expected to return a *pb.LoginResponse.
	// We'll get an object from the pool, pass it to ProcessLogin (or ProcessLogin creates one),
	// then marshal it, and finally put it back.
	// For simplicity, let's assume ProcessLogin populates a given response object or returns a new one.
	// If ProcessLogin allocates, we can't use a pool for its direct return value easily without modifying it.
	// Let's assume ProcessLogin is modified to take a *pb.LoginResponse to populate, or we handle its return value.
	// Current ProcessLogin returns (resp, err). We will use a pooled object for what ProcessLogin returns.

	loginRes := loginResponsePool.Get().(*pb.LoginResponse)
	// It's crucial that loginRes is reset and put back into the pool in all execution paths.

	returnedResp, err := h.loginImpl.ProcessLogin(r.Context(), loginReq)
	if err != nil {
		log.Printf("Error processing login for username %s: %v", loginReq.Username, err)
		http.Error(w, "Internal server error during login processing", http.StatusInternalServerError)
		proto.Reset(loginRes) // Reset before putting back, even on error path
		loginResponsePool.Put(loginRes)
		return
	}

	// Copy data from returnedResp to our pooled loginRes
	// This is if ProcessLogin cannot directly use a pooled object.
	// If ProcessLogin *could* take loginRes as an argument to populate, that would be more efficient.
	// For now, let's assume ProcessLogin returns a new object, and we copy to our pooled one.
	// Better: Modify ProcessLogin to accept *pb.LoginResponse as an argument to fill.
	// However, sticking to the current ProcessLogin signature:
	proto.Merge(loginRes, returnedResp) // Copy content from returnedResp to pooled loginRes

	w.Header().Set("Content-Type", "application/json")
	if !loginRes.Success {
		if loginRes.ErrorMessage == "Username and password are required" {
			w.WriteHeader(http.StatusBadRequest)
		} else if loginRes.ErrorMessage == "Failed to create new player account." || loginRes.ErrorMessage == "Database error while finding player." {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	} else {
		w.WriteHeader(http.StatusOK)
	}

	jsonBytes, marshalErr := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(loginRes)

	proto.Reset(loginRes)           // Reset after use (marshal or error)
	loginResponsePool.Put(loginRes) // Put back after use

	if marshalErr != nil {
		log.Printf("Error marshalling LoginResponse to JSON: %v", marshalErr)
		http.Error(w, "Internal server error creating response", http.StatusInternalServerError)
		return
	}

	_, writeErr := w.Write(jsonBytes)
	if writeErr != nil {
		log.Printf("Error writing JSON response: %v", writeErr)
	}
	// Note: loginReq.Username might be misleading here if loginReq was reset by defer already,
	// but for logging it's minor. The actual username was logged earlier.
	log.Printf("Sent login response for username %s: Success=%t, UserID=%s", loginReq.Username, loginRes.Success, loginRes.UserId)
}

// HandleValidateSession is the HTTP coordinator function for the /api/validate_session endpoint.
func (h *LoginHandler) HandleValidateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed for session validation", http.StatusMethodNotAllowed)
		return
	}

	validateReq := validateSessionRequestPool.Get().(*pb.ValidateSessionRequest)
	defer func() {
		proto.Reset(validateReq)
		validateSessionRequestPool.Put(validateReq)
	}()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading ValidateSession request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if err := protojson.Unmarshal(body, validateReq); err != nil {
		log.Printf("Error unmarshalling ValidateSessionRequest JSON: %v", err)
		http.Error(w, "Invalid request format for session validation: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received session validation request for token: %s", validateReq.SessionToken)

	validateRes := validateSessionResponsePool.Get().(*pb.ValidateSessionResponse)
	// Manually manage putting validateRes back due to multiple return paths.

	userID, validationErr := h.loginImpl.ValidateSession(r.Context(), validateReq.SessionToken)

	if validationErr != nil {
		log.Printf("Session validation failed for token %s: %v", validateReq.SessionToken, validationErr)
		validateRes.IsValid = false
		validateRes.ErrorMessage = validationErr.Error()
		w.WriteHeader(http.StatusUnauthorized)
	} else {
		log.Printf("Session token %s validated successfully for UserID: %s", validateReq.SessionToken, userID)
		validateRes.UserId = userID
		validateRes.IsValid = true
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	jsonBytes, marshalErr := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(validateRes)

	proto.Reset(validateRes)
	validateSessionResponsePool.Put(validateRes)

	if marshalErr != nil {
		log.Printf("Error marshalling ValidateSessionResponse to JSON: %v", marshalErr)
		http.Error(w, "Internal server error creating session validation response", http.StatusInternalServerError)
		return
	}

	_, writeErr := w.Write(jsonBytes)
	if writeErr != nil {
		log.Printf("Error writing ValidateSessionResponse JSON: %v", writeErr)
	}
	log.Printf("Sent session validation response for token %s: IsValid=%t, UserID=%s", validateReq.SessionToken, validateRes.IsValid, validateRes.UserId)
}
