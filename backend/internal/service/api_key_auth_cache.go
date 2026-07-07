package service

import "time"

// APIKeyAuthSnapshot API Key 认证缓存快照（仅包含认证所需字段）
type APIKeyAuthSnapshot struct {
	Version              int                      `json:"version"`
	APIKeyID             int64                    `json:"api_key_id"`
	UserID               int64                    `json:"user_id"`
	GroupID              *int64                   `json:"group_id,omitempty"`
	Name                 string                   `json:"name"`
	Status               string                   `json:"status"`
	IPWhitelist          []string                 `json:"ip_whitelist,omitempty"`
	IPBlacklist          []string                 `json:"ip_blacklist,omitempty"`
	MaxActiveIPs         int                      `json:"max_active_ips"`
	IPIdleTimeoutSeconds int                      `json:"ip_idle_timeout_seconds"`
	MaxConcurrency       int                      `json:"max_concurrency"`
	User                 APIKeyAuthUserSnapshot   `json:"user"`
	Group                *APIKeyAuthGroupSnapshot `json:"group,omitempty"`

	// Quota fields for API Key independent quota feature
	Quota     float64 `json:"quota"`      // Quota limit in USD (0 = unlimited)
	QuotaUsed float64 `json:"quota_used"` // Used quota amount

	// Expiration field for API Key expiration feature
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // Expiration time (nil = never expires)

	// Rate limit configuration (only limits, not usage - usage read from Redis at check time)
	RateLimit5h float64 `json:"rate_limit_5h"`
	RateLimit1d float64 `json:"rate_limit_1d"`
	RateLimit7d float64 `json:"rate_limit_7d"`
}

// APIKeyAuthUserSnapshot 用户快照
type APIKeyAuthUserSnapshot struct {
	ID            int64   `json:"id"`
	Status        string  `json:"status"`
	Role          string  `json:"role"`
	Balance       float64 `json:"balance"`
	Concurrency   int     `json:"concurrency"`
	AllowedGroups []int64 `json:"allowed_groups,omitempty"`

	// Balance notification fields (required for CheckBalanceAfterDeduction)
	Email                      string             `json:"email"`
	Username                   string             `json:"username"`
	BalanceNotifyEnabled       bool               `json:"balance_notify_enabled"`
	BalanceNotifyThresholdType string             `json:"balance_notify_threshold_type"`
	BalanceNotifyThreshold     *float64           `json:"balance_notify_threshold,omitempty"`
	BalanceNotifyExtraEmails   []NotifyEmailEntry `json:"balance_notify_extra_emails,omitempty"`
	TotalRecharged             float64            `json:"total_recharged"`

	// RPMLimit 用户级每分钟请求数上限（0 = 不限制）；用于 billing_cache_service.checkRPM 兜底判断。
	RPMLimit int `json:"rpm_limit"`

	// APIKeyMaxActiveIPs 用户级 API Key 活跃 IP 上限；热路径用于计算有效动态 IP 上限。
	APIKeyMaxActiveIPs        int  `json:"api_key_max_active_ips"`
	APIKeyMaxActiveIPsVisible bool `json:"api_key_max_active_ips_visible"`

	// UserGroupRPMOverride 该 API Key 对应的 (user, group) 专属 RPM 覆盖值。
	// nil = 无 override（回退到 group/user 级）；0 = 不限流；>0 = 专属上限。
	UserGroupRPMOverride *int `json:"user_group_rpm_override,omitempty"`
}

// APIKeyAuthGroupSnapshot 分组快照
type APIKeyAuthGroupSnapshot struct {
	ID                              int64    `json:"id"`
	Name                            string   `json:"name"`
	Platform                        string   `json:"platform"`
	IsExclusive                     bool     `json:"is_exclusive"`
	Status                          string   `json:"status"`
	SubscriptionType                string   `json:"subscription_type"`
	RateMultiplier                  float64  `json:"rate_multiplier"`
	DailyLimitUSD                   *float64 `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD                  *float64 `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD                 *float64 `json:"monthly_limit_usd,omitempty"`
	AllowImageGeneration            bool     `json:"allow_image_generation"`
	ImageRateIndependent            bool     `json:"image_rate_independent"`
	ImageRateMultiplier             float64  `json:"image_rate_multiplier"`
	ImagePrice1K                    *float64 `json:"image_price_1k,omitempty"`
	ImagePrice2K                    *float64 `json:"image_price_2k,omitempty"`
	ImagePrice4K                    *float64 `json:"image_price_4k,omitempty"`
	ClaudeCodeOnly                  bool     `json:"claude_code_only"`
	FallbackGroupID                 *int64   `json:"fallback_group_id,omitempty"`
	FallbackGroupIDOnInvalidRequest *int64   `json:"fallback_group_id_on_invalid_request,omitempty"`

	// Model routing is used by gateway account selection, so it must be part of auth cache snapshot.
	// Only anthropic groups use these fields; others may leave them empty.
	ModelRouting        map[string][]int64 `json:"model_routing,omitempty"`
	ModelRoutingEnabled bool               `json:"model_routing_enabled"`
	MCPXMLInject        bool               `json:"mcp_xml_inject"`

	// 支持的模型系列（仅 antigravity 平台使用）
	SupportedModelScopes []string `json:"supported_model_scopes,omitempty"`

	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       bool                              `json:"allow_messages_dispatch"`
	DefaultMappedModel          string                            `json:"default_mapped_model,omitempty"`
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig `json:"messages_dispatch_model_config,omitempty"`
	ModelsListConfig            GroupModelsListConfig             `json:"models_list_config,omitempty"`

	// RPMLimit 分组级每分钟请求数上限（0 = 不限制）；用于 billing_cache_service.checkRPM 级联判断。
	RPMLimit int `json:"rpm_limit"`

	// 优速通/随速通策略用于 OpenAI 请求热路径决策，必须随认证缓存保存。
	SpeedConfigEnabled         bool    `json:"speed_config_enabled"`
	UserSpeedConfigAllowed     bool    `json:"user_speed_config_allowed"`
	DefaultFastQuotaRatio      float64 `json:"default_fast_quota_ratio"`
	MinFastQuotaRatio          float64 `json:"min_fast_quota_ratio"`
	MaxFastQuotaRatio          float64 `json:"max_fast_quota_ratio"`
	DefaultSlowDelayMinSeconds int     `json:"default_slow_delay_min_seconds"`
	DefaultSlowDelayMaxSeconds int     `json:"default_slow_delay_max_seconds"`
	MaxSlowDelaySeconds        int     `json:"max_slow_delay_seconds"`
	DefaultSlowRejectRate      float64 `json:"default_slow_reject_rate"`
	MaxSlowRejectRate          float64 `json:"max_slow_reject_rate"`
	SpeedSlowRejectMessage     string  `json:"speed_slow_reject_message,omitempty"`
	SuisuEnabled               bool    `json:"suisu_enabled"`
	SuisuFallbackGroupID       *int64  `json:"suisu_fallback_group_id,omitempty"`
	SuisuSlowRouteRatio        float64 `json:"suisu_slow_route_ratio"`
	SuisuBusyRouteRatio        float64 `json:"suisu_busy_route_ratio"`

	// 高峰时段倍率：PeakRateEnabled 为 true 且请求时刻处于 [PeakStart, PeakEnd) 时，
	// token 计费倍率额外乘以 PeakRateMultiplier（详见 Group.PeakMultiplierAt）。
	// 必须随快照缓存，否则扣费路径拿到的 apiKey.Group 缺字段、高峰倍率失效。
	PeakRateEnabled    bool    `json:"peak_rate_enabled"`
	PeakStart          string  `json:"peak_start"`
	PeakEnd            string  `json:"peak_end"`
	PeakRateMultiplier float64 `json:"peak_rate_multiplier"`
}

// APIKeyAuthCacheEntry 缓存条目，支持负缓存
type APIKeyAuthCacheEntry struct {
	NotFound bool                `json:"not_found"`
	Snapshot *APIKeyAuthSnapshot `json:"snapshot,omitempty"`
}
