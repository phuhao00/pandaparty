package gmserver

import (
	"context"
	"fmt"
	"github.com/phuhao00/pandaparty/infra/pb/protocol/gm"
	// Other necessary imports for RPC clients, config can be added later
)

// GMServiceImpl implements the core logic for GM commands.
type GMServiceImpl struct {
	// config *config.GMServerConfig // Specific GM server config if needed
	gameServiceClient gm.GMServiceClient
}

// NewGMServiceImpl creates a new instance of GMServiceImpl.
func NewGMServiceImpl(client gm.GMServiceClient) *GMServiceImpl {
	if client == nil {
		return nil
	}
	return &GMServiceImpl{gameServiceClient: client}
}

// GetPlayerInfo handles the request to get player information.
func (s *GMServiceImpl) GetPlayerInfo(ctx context.Context, req *gm.GMGetPlayerInfoRequest) (*gm.GMGetPlayerInfoResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetPlayerId() == "" && req.GetPlayerName() == "" {
		return nil, fmt.Errorf("validation error: either player_id or player_name must be provided")
	}
	return s.gameServiceClient.GMGetPlayerInfo(ctx, req)
}

// SendItemToPlayer handles sending an item to a player.
func (s *GMServiceImpl) SendItemToPlayer(ctx context.Context, req *gm.GMSendItemToPlayerRequest) (*gm.GMSendItemToPlayerResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetPlayerId() == "" {
		return nil, fmt.Errorf("validation error: player_id is required")
	}
	if req.GetItemId() == 0 || req.GetQuantity() <= 0 {
		return nil, fmt.Errorf("validation error: item_id and positive item_count are required")
	}
	return s.gameServiceClient.GMSendItemToPlayer(ctx, req)
}

// CreateNotice handles creating a new game notice.
func (s *GMServiceImpl) CreateNotice(ctx context.Context, req *gm.GMCreateNoticeRequest) (*gm.GMCreateNoticeResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetTitle() == "" || req.GetContent() == "" {
		return nil, fmt.Errorf("validation error: title and content are required for notice")
	}
	return s.gameServiceClient.GMCreateNotice(ctx, req)
}

// SetPlayerAttribute handles setting a player's attribute.
func (s *GMServiceImpl) SetPlayerAttribute(ctx context.Context, req *gm.GMSetPlayerAttributeRequest) (*gm.GMSetPlayerAttributeResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetPlayerId() == "" || req.GetAttributeName() == "" {
		return nil, fmt.Errorf("validation error: player_id and attribute_key are required")
	}
	return s.gameServiceClient.GMSetPlayerAttribute(ctx, req)
}

// BanPlayer handles banning a player.
func (s *GMServiceImpl) BanPlayer(ctx context.Context, req *gm.GMBanPlayerRequest) (*gm.GMBanPlayerResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetPlayerId() == "" {
		return nil, fmt.Errorf("validation error: player_id is required for banning")
	}
	if req.GetDurationHours() <= 0 {
		return nil, fmt.Errorf("validation error: ban duration must be positive")
	}
	return s.gameServiceClient.GMBanPlayer(ctx, req)
}

// UnbanPlayer handles unbanning a player.
func (s *GMServiceImpl) UnbanPlayer(ctx context.Context, req *gm.GMUnbanPlayerRequest) (*gm.GMUnbanPlayerResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetPlayerId() == "" {
		return nil, fmt.Errorf("validation error: player_id is required for unbanning")
	}
	return s.gameServiceClient.GMUnbanPlayer(ctx, req)
}

// UpdateNotice handles updating an existing game notice.
func (s *GMServiceImpl) UpdateNotice(ctx context.Context, req *gm.GMUpdateNoticeRequest) (*gm.GMUpdateNoticeResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetNoticeId() == "" {
		return nil, fmt.Errorf("validation error: notice_id is required for updating")
	}
	return s.gameServiceClient.GMUpdateNotice(ctx, req)
}

// DeleteNotice handles deleting a game notice.
func (s *GMServiceImpl) DeleteNotice(ctx context.Context, req *gm.GMDeleteNoticeRequest) (*gm.GMDeleteNoticeResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	if req.GetNoticeId() == "" {
		return nil, fmt.Errorf("validation error: notice_id is required for deleting")
	}
	return s.gameServiceClient.GMDeleteNotice(ctx, req)
}

// GetServerStatus handles requests for server status.
// No specific input validation here as GMServerStatusRequest is empty.
func (s *GMServiceImpl) GetServerStatus(ctx context.Context, req *gm.GMServerStatusRequest) (*gm.GMServerStatusResponse, error) {
	if s.gameServiceClient == nil {
		return nil, fmt.Errorf("game service client is not initialized in GMServiceImpl")
	}
	return s.gameServiceClient.GMGetServerStatus(ctx, req)
}
