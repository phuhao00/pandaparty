package config

import (
	"sync"
	"time"
)

// FriendConfig 好友系统配置结构
type FriendConfig struct {
	MaxFriends         int `yaml:"max_friends"`          // 最大好友数量
	MaxFriendRequests  int `yaml:"max_friend_requests"`  // 最大待处理好友申请数
	RequestExpireDays  int `yaml:"request_expire_days"`  // 好友申请过期天数
	MessageHistoryDays int `yaml:"message_history_days"` // 聊天记录保存天数
	MaxMessageLength   int `yaml:"max_message_length"`   // 单条消息最大长度
	MaxDailyMessages   int `yaml:"max_daily_messages"`   // 每日最大发送消息数
	BlockListMax       int `yaml:"block_list_max"`       // 黑名单最大数量

	// 好友功能开关
	EnableFriendChat   bool `yaml:"enable_friend_chat"`   // 是否启用好友聊天
	EnableFriendInvite bool `yaml:"enable_friend_invite"` // 是否启用好友邀请
	EnableFriendSearch bool `yaml:"enable_friend_search"` // 是否启用好友搜索
	EnableIntimacy     bool `yaml:"enable_intimacy"`      // 是否启用亲密度系统

	// 高级配置
	SearchResultLimit  int `yaml:"search_result_limit"`  // 搜索结果限制
	IntimacyDecayDays  int `yaml:"intimacy_decay_days"`  // 亲密度衰减天数
	AutoAcceptLevel    int `yaml:"auto_accept_level"`    // 自动接受好友申请的等级门槛
	VipMaxFriends      int `yaml:"vip_max_friends"`      // VIP玩家最大好友数
	MessageRetryTimes  int `yaml:"message_retry_times"`  // 消息发送重试次数
	OnlineStatusExpire int `yaml:"online_status_expire"` // 在线状态过期时间(秒)
}

// FriendConfigManager 好友配置管理器单例
type FriendConfigManager struct {
	config     *FriendConfig
	mu         sync.RWMutex
	lastUpdate time.Time
}

var (
	friendConfigManager *FriendConfigManager
	friendConfigOnce    sync.Once
)

// GetDefaultFriendConfig 返回默认的好友配置
func GetDefaultFriendConfig() *FriendConfig {
	return &FriendConfig{
		MaxFriends:         200,
		MaxFriendRequests:  50,
		RequestExpireDays:  7,
		MessageHistoryDays: 30,
		MaxMessageLength:   500,
		MaxDailyMessages:   1000,
		BlockListMax:       100,

		EnableFriendChat:   true,
		EnableFriendInvite: true,
		EnableFriendSearch: true,
		EnableIntimacy:     true,

		SearchResultLimit:  20,
		IntimacyDecayDays:  30,
		AutoAcceptLevel:    0, // 0表示不自动接受
		VipMaxFriends:      500,
		MessageRetryTimes:  3,
		OnlineStatusExpire: 3600, // 1小时
	}
}

// GetFriendConfigManager 获取好友配置管理器单例
func GetFriendConfigManager() *FriendConfigManager {
	friendConfigOnce.Do(func() {
		friendConfigManager = &FriendConfigManager{
			config: GetDefaultFriendConfig(),
		}
	})
	return friendConfigManager
}

// InitFriendConfig 初始化好友配置
func (fcm *FriendConfigManager) InitFriendConfig(serverConfig *ServerConfig) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()

	// 如果服务器配置中有好友配置，则使用它
	if serverConfig != nil {
		fcm.config = &serverConfig.Friend
		// 验证并设置默认值
		fcm.validateAndSetDefaults()
	}

	fcm.lastUpdate = time.Now()
}

// GetConfig 获取好友配置
func (fcm *FriendConfigManager) GetConfig() *FriendConfig {
	fcm.mu.RLock()
	defer fcm.mu.RUnlock()

	// 返回配置的副本，防止外部修改
	configCopy := *fcm.config
	return &configCopy
}

// UpdateConfig 更新好友配置
func (fcm *FriendConfigManager) UpdateConfig(newConfig *FriendConfig) {
	fcm.mu.Lock()
	defer fcm.mu.Unlock()

	fcm.config = newConfig
	fcm.validateAndSetDefaults()
	fcm.lastUpdate = time.Now()
}

// validateAndSetDefaults 验证配置并设置默认值
func (fcm *FriendConfigManager) validateAndSetDefaults() {
	if fcm.config.MaxFriends <= 0 {
		fcm.config.MaxFriends = 200
	}
	if fcm.config.MaxFriendRequests <= 0 {
		fcm.config.MaxFriendRequests = 50
	}
	if fcm.config.RequestExpireDays <= 0 {
		fcm.config.RequestExpireDays = 7
	}
	if fcm.config.MessageHistoryDays <= 0 {
		fcm.config.MessageHistoryDays = 30
	}
	if fcm.config.MaxMessageLength <= 0 {
		fcm.config.MaxMessageLength = 500
	}
	if fcm.config.MaxDailyMessages <= 0 {
		fcm.config.MaxDailyMessages = 1000
	}
	if fcm.config.BlockListMax <= 0 {
		fcm.config.BlockListMax = 100
	}
	if fcm.config.SearchResultLimit <= 0 {
		fcm.config.SearchResultLimit = 20
	}
	if fcm.config.IntimacyDecayDays <= 0 {
		fcm.config.IntimacyDecayDays = 30
	}
	if fcm.config.VipMaxFriends <= fcm.config.MaxFriends {
		fcm.config.VipMaxFriends = fcm.config.MaxFriends * 2
	}
	if fcm.config.MessageRetryTimes <= 0 {
		fcm.config.MessageRetryTimes = 3
	}
	if fcm.config.OnlineStatusExpire <= 0 {
		fcm.config.OnlineStatusExpire = 3600
	}
}

// GetLastUpdateTime 获取最后更新时间
func (fcm *FriendConfigManager) GetLastUpdateTime() time.Time {
	fcm.mu.RLock()
	defer fcm.mu.RUnlock()
	return fcm.lastUpdate
}

// 便捷方法：获取具体配置项

// GetMaxFriends 获取最大好友数量
func (fcm *FriendConfigManager) GetMaxFriends(isVip bool) int {
	config := fcm.GetConfig()
	if isVip && config.VipMaxFriends > config.MaxFriends {
		return config.VipMaxFriends
	}
	return config.MaxFriends
}

// GetMaxFriendRequests 获取最大好友申请数
func (fcm *FriendConfigManager) GetMaxFriendRequests() int {
	return fcm.GetConfig().MaxFriendRequests
}

// GetRequestExpireDuration 获取好友申请过期时间
func (fcm *FriendConfigManager) GetRequestExpireDuration() time.Duration {
	days := fcm.GetConfig().RequestExpireDays
	return time.Duration(days) * 24 * time.Hour
}

// GetMessageHistoryDuration 获取消息历史保存时间
func (fcm *FriendConfigManager) GetMessageHistoryDuration() time.Duration {
	days := fcm.GetConfig().MessageHistoryDays
	return time.Duration(days) * 24 * time.Hour
}

// GetMaxMessageLength 获取消息最大长度
func (fcm *FriendConfigManager) GetMaxMessageLength() int {
	return fcm.GetConfig().MaxMessageLength
}

// GetMaxDailyMessages 获取每日最大发送消息数
func (fcm *FriendConfigManager) GetMaxDailyMessages() int {
	return fcm.GetConfig().MaxDailyMessages
}

// GetBlockListMax 获取黑名单最大数量
func (fcm *FriendConfigManager) GetBlockListMax() int {
	return fcm.GetConfig().BlockListMax
}

// IsFeatureEnabled 检查功能是否启用
func (fcm *FriendConfigManager) IsFeatureEnabled(feature string) bool {
	config := fcm.GetConfig()
	switch feature {
	case "chat":
		return config.EnableFriendChat
	case "invite":
		return config.EnableFriendInvite
	case "search":
		return config.EnableFriendSearch
	case "intimacy":
		return config.EnableIntimacy
	default:
		return false
	}
}

// GetSearchResultLimit 获取搜索结果限制
func (fcm *FriendConfigManager) GetSearchResultLimit() int {
	return fcm.GetConfig().SearchResultLimit
}

// GetIntimacyDecayDuration 获取亲密度衰减时间
func (fcm *FriendConfigManager) GetIntimacyDecayDuration() time.Duration {
	days := fcm.GetConfig().IntimacyDecayDays
	return time.Duration(days) * 24 * time.Hour
}

// GetAutoAcceptLevel 获取自动接受好友申请的等级门槛
func (fcm *FriendConfigManager) GetAutoAcceptLevel() int {
	return fcm.GetConfig().AutoAcceptLevel
}

// GetMessageRetryTimes 获取消息重试次数
func (fcm *FriendConfigManager) GetMessageRetryTimes() int {
	return fcm.GetConfig().MessageRetryTimes
}

// GetOnlineStatusExpireDuration 获取在线状态过期时间
func (fcm *FriendConfigManager) GetOnlineStatusExpireDuration() time.Duration {
	seconds := fcm.GetConfig().OnlineStatusExpire
	return time.Duration(seconds) * time.Second
}

// ============= 全局便捷函数 =============

// GetFriendConfig 获取好友配置
func GetFriendConfig() *FriendConfig {
	return GetFriendConfigManager().GetConfig()
}

// GetMaxFriends 获取最大好友数量
func GetMaxFriends(isVip bool) int {
	return GetFriendConfigManager().GetMaxFriends(isVip)
}

// GetMaxFriendRequests 获取最大好友申请数
func GetMaxFriendRequests() int {
	return GetFriendConfigManager().GetMaxFriendRequests()
}

// GetRequestExpireDuration 获取好友申请过期时间
func GetRequestExpireDuration() time.Duration {
	return GetFriendConfigManager().GetRequestExpireDuration()
}

// GetMessageHistoryDuration 获取消息历史保存时间
func GetMessageHistoryDuration() time.Duration {
	return GetFriendConfigManager().GetMessageHistoryDuration()
}

// GetMaxMessageLength 获取消息最大长度
func GetMaxMessageLength() int {
	return GetFriendConfigManager().GetMaxMessageLength()
}

// GetMaxDailyMessages 获取每日最大发送消息数
func GetMaxDailyMessages() int {
	return GetFriendConfigManager().GetMaxDailyMessages()
}

// GetBlockListMax 获取黑名单最大数量
func GetBlockListMax() int {
	return GetFriendConfigManager().GetBlockListMax()
}

// IsFriendFeatureEnabled 检查好友功能是否启用
func IsFriendFeatureEnabled(feature string) bool {
	return GetFriendConfigManager().IsFeatureEnabled(feature)
}

// GetSearchResultLimit 获取搜索结果限制
func GetSearchResultLimit() int {
	return GetFriendConfigManager().GetSearchResultLimit()
}

// GetIntimacyDecayDuration 获取亲密度衰减时间
func GetIntimacyDecayDuration() time.Duration {
	return GetFriendConfigManager().GetIntimacyDecayDuration()
}

// GetAutoAcceptLevel 获取自动接受好友申请的等级门槛
func GetAutoAcceptLevel() int {
	return GetFriendConfigManager().GetAutoAcceptLevel()
}

// GetMessageRetryTimes 获取消息重试次数
func GetMessageRetryTimes() int {
	return GetFriendConfigManager().GetMessageRetryTimes()
}

// GetOnlineStatusExpireDuration 获取在线状态过期时间
func GetOnlineStatusExpireDuration() time.Duration {
	return GetFriendConfigManager().GetOnlineStatusExpireDuration()
}

// ============= 业务验证函数 =============

// ValidateMessageContent 验证消息内容
func ValidateMessageContent(content string) bool {
	if len(content) == 0 {
		return false
	}
	if len(content) > GetMaxMessageLength() {
		return false
	}
	return true
}

// CanAddMoreFriends 检查是否可以添加更多好友
func CanAddMoreFriends(currentCount int, isVip bool) bool {
	maxFriends := GetMaxFriends(isVip)
	return currentCount < maxFriends
}

// CanSendMoreMessages 检查今日是否可以发送更多消息
func CanSendMoreMessages(todayCount int) bool {
	return todayCount < GetMaxDailyMessages()
}

// CanAddMoreRequests 检查是否可以发送更多好友申请
func CanAddMoreRequests(currentCount int) bool {
	return currentCount < GetMaxFriendRequests()
}

// CanAddToBlackList 检查是否可以添加到黑名单
func CanAddToBlackList(currentCount int) bool {
	return currentCount < GetBlockListMax()
}

// ShouldAutoAcceptFriend 检查是否应该自动接受好友申请
func ShouldAutoAcceptFriend(fromPlayerLevel int) bool {
	autoLevel := GetAutoAcceptLevel()
	return autoLevel > 0 && fromPlayerLevel >= autoLevel
}

// GetFriendRequestExpireTime 获取好友申请的过期时间
func GetFriendRequestExpireTime() time.Time {
	return time.Now().Add(GetRequestExpireDuration())
}

// GetMessageCleanupTime 获取消息清理时间
func GetMessageCleanupTime() time.Time {
	return time.Now().Add(-GetMessageHistoryDuration())
}

// ============= 配置热更新支持 =============

// ReloadFriendConfig 重新加载好友配置
func ReloadFriendConfig(newConfig *FriendConfig) {
	GetFriendConfigManager().UpdateConfig(newConfig)
}

// GetFriendConfigLastUpdate 获取配置最后更新时间
func GetFriendConfigLastUpdate() time.Time {
	return GetFriendConfigManager().GetLastUpdateTime()
}

// InitializeFriendConfig 初始化好友配置（应在服务启动时调用）
func InitializeFriendConfig(serverConfig *ServerConfig) {
	GetFriendConfigManager().InitFriendConfig(serverConfig)
}

// ============= 配置状态检查 =============

// GetFriendConfigStatus 获取好友配置状态信息
func GetFriendConfigStatus() map[string]interface{} {
	config := GetFriendConfig()
	return map[string]interface{}{
		"max_friends":          config.MaxFriends,
		"vip_max_friends":      config.VipMaxFriends,
		"max_friend_requests":  config.MaxFriendRequests,
		"max_message_length":   config.MaxMessageLength,
		"max_daily_messages":   config.MaxDailyMessages,
		"enable_friend_chat":   config.EnableFriendChat,
		"enable_friend_invite": config.EnableFriendInvite,
		"enable_friend_search": config.EnableFriendSearch,
		"enable_intimacy":      config.EnableIntimacy,
		"last_update":          GetFriendConfigLastUpdate(),
	}
}
