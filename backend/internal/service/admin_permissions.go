package service

import (
	"fmt"
	"sort"
	"strings"
)

const (
	AdminPermissionDashboardRead               = "admin.dashboard.read"
	AdminPermissionDashboardWrite              = "admin.dashboard.write"
	AdminPermissionOpsRead                     = "admin.ops.read"
	AdminPermissionOpsWrite                    = "admin.ops.write"
	AdminPermissionUsersRead                   = "admin.users.read"
	AdminPermissionUsersWrite                  = "admin.users.write"
	AdminPermissionGroupsRead                  = "admin.groups.read"
	AdminPermissionGroupsWrite                 = "admin.groups.write"
	AdminPermissionAccountsRead                = "admin.accounts.read"
	AdminPermissionAccountsWrite               = "admin.accounts.write"
	AdminPermissionChannelsRead                = "admin.channels.read"
	AdminPermissionChannelsWrite               = "admin.channels.write"
	AdminPermissionChannelMonitorsRead         = "admin.channel_monitors.read"
	AdminPermissionChannelMonitorsWrite        = "admin.channel_monitors.write"
	AdminPermissionSubscriptionsRead           = "admin.subscriptions.read"
	AdminPermissionSubscriptionsWrite          = "admin.subscriptions.write"
	AdminPermissionAnnouncementsRead           = "admin.announcements.read"
	AdminPermissionAnnouncementsWrite          = "admin.announcements.write"
	AdminPermissionProxiesRead                 = "admin.proxies.read"
	AdminPermissionProxiesWrite                = "admin.proxies.write"
	AdminPermissionRiskControlRead             = "admin.risk_control.read"
	AdminPermissionRiskControlWrite            = "admin.risk_control.write"
	AdminPermissionRedeemCodesRead             = "admin.redeem_codes.read"
	AdminPermissionRedeemCodesWrite            = "admin.redeem_codes.write"
	AdminPermissionPromoCodesRead              = "admin.promo_codes.read"
	AdminPermissionPromoCodesWrite             = "admin.promo_codes.write"
	AdminPermissionSettingsRead                = "admin.settings.read"
	AdminPermissionSettingsWrite               = "admin.settings.write"
	AdminPermissionDataManagementRead          = "admin.data_management.read"
	AdminPermissionDataManagementWrite         = "admin.data_management.write"
	AdminPermissionBackupRead                  = "admin.backup.read"
	AdminPermissionBackupWrite                 = "admin.backup.write"
	AdminPermissionSystemRead                  = "admin.system.read"
	AdminPermissionSystemWrite                 = "admin.system.write"
	AdminPermissionUsageRead                   = "admin.usage.read"
	AdminPermissionUsageWrite                  = "admin.usage.write"
	AdminPermissionUserAttributesRead          = "admin.user_attributes.read"
	AdminPermissionUserAttributesWrite         = "admin.user_attributes.write"
	AdminPermissionErrorPassthroughRead        = "admin.error_passthrough.read"
	AdminPermissionErrorPassthroughWrite       = "admin.error_passthrough.write"
	AdminPermissionTLSFingerprintProfilesRead  = "admin.tls_fingerprint_profiles.read"
	AdminPermissionTLSFingerprintProfilesWrite = "admin.tls_fingerprint_profiles.write"
	AdminPermissionScheduledTestsRead          = "admin.scheduled_tests.read"
	AdminPermissionScheduledTestsWrite         = "admin.scheduled_tests.write"
	AdminPermissionAffiliatesRead              = "admin.affiliates.read"
	AdminPermissionAffiliatesWrite             = "admin.affiliates.write"
	AdminPermissionPaymentRead                 = "admin.payment.read"
	AdminPermissionPaymentWrite                = "admin.payment.write"
)

type AdminPermissionDefinition struct {
	Key    string `json:"key"`
	Module string `json:"module"`
	Action string `json:"action"`
	Label  string `json:"label"`
}

var adminPermissionDefinitions = []AdminPermissionDefinition{
	{Key: AdminPermissionDashboardRead, Module: "dashboard", Action: "read", Label: "仪表盘查看"},
	{Key: AdminPermissionDashboardWrite, Module: "dashboard", Action: "write", Label: "仪表盘操作"},
	{Key: AdminPermissionOpsRead, Module: "ops", Action: "read", Label: "运维监控查看"},
	{Key: AdminPermissionOpsWrite, Module: "ops", Action: "write", Label: "运维监控编辑"},
	{Key: AdminPermissionUsersRead, Module: "users", Action: "read", Label: "用户查看"},
	{Key: AdminPermissionUsersWrite, Module: "users", Action: "write", Label: "用户编辑"},
	{Key: AdminPermissionGroupsRead, Module: "groups", Action: "read", Label: "分组查看"},
	{Key: AdminPermissionGroupsWrite, Module: "groups", Action: "write", Label: "分组编辑"},
	{Key: AdminPermissionAccountsRead, Module: "accounts", Action: "read", Label: "账号查看"},
	{Key: AdminPermissionAccountsWrite, Module: "accounts", Action: "write", Label: "账号编辑"},
	{Key: AdminPermissionChannelsRead, Module: "channels", Action: "read", Label: "渠道查看"},
	{Key: AdminPermissionChannelsWrite, Module: "channels", Action: "write", Label: "渠道编辑"},
	{Key: AdminPermissionChannelMonitorsRead, Module: "channel_monitors", Action: "read", Label: "渠道监控查看"},
	{Key: AdminPermissionChannelMonitorsWrite, Module: "channel_monitors", Action: "write", Label: "渠道监控编辑"},
	{Key: AdminPermissionSubscriptionsRead, Module: "subscriptions", Action: "read", Label: "订阅查看"},
	{Key: AdminPermissionSubscriptionsWrite, Module: "subscriptions", Action: "write", Label: "订阅编辑"},
	{Key: AdminPermissionAnnouncementsRead, Module: "announcements", Action: "read", Label: "公告查看"},
	{Key: AdminPermissionAnnouncementsWrite, Module: "announcements", Action: "write", Label: "公告编辑"},
	{Key: AdminPermissionProxiesRead, Module: "proxies", Action: "read", Label: "代理查看"},
	{Key: AdminPermissionProxiesWrite, Module: "proxies", Action: "write", Label: "代理编辑"},
	{Key: AdminPermissionRiskControlRead, Module: "risk_control", Action: "read", Label: "风控查看"},
	{Key: AdminPermissionRiskControlWrite, Module: "risk_control", Action: "write", Label: "风控编辑"},
	{Key: AdminPermissionRedeemCodesRead, Module: "redeem_codes", Action: "read", Label: "卡密查看"},
	{Key: AdminPermissionRedeemCodesWrite, Module: "redeem_codes", Action: "write", Label: "卡密编辑"},
	{Key: AdminPermissionPromoCodesRead, Module: "promo_codes", Action: "read", Label: "优惠码查看"},
	{Key: AdminPermissionPromoCodesWrite, Module: "promo_codes", Action: "write", Label: "优惠码编辑"},
	{Key: AdminPermissionSettingsRead, Module: "settings", Action: "read", Label: "系统设置查看"},
	{Key: AdminPermissionSettingsWrite, Module: "settings", Action: "write", Label: "系统设置编辑"},
	{Key: AdminPermissionDataManagementRead, Module: "data_management", Action: "read", Label: "数据管理查看"},
	{Key: AdminPermissionDataManagementWrite, Module: "data_management", Action: "write", Label: "数据管理编辑"},
	{Key: AdminPermissionBackupRead, Module: "backup", Action: "read", Label: "备份查看"},
	{Key: AdminPermissionBackupWrite, Module: "backup", Action: "write", Label: "备份创建"},
	{Key: AdminPermissionSystemRead, Module: "system", Action: "read", Label: "系统信息查看"},
	{Key: AdminPermissionSystemWrite, Module: "system", Action: "write", Label: "系统操作"},
	{Key: AdminPermissionUsageRead, Module: "usage", Action: "read", Label: "用量查看"},
	{Key: AdminPermissionUsageWrite, Module: "usage", Action: "write", Label: "用量任务编辑"},
	{Key: AdminPermissionUserAttributesRead, Module: "user_attributes", Action: "read", Label: "用户属性查看"},
	{Key: AdminPermissionUserAttributesWrite, Module: "user_attributes", Action: "write", Label: "用户属性编辑"},
	{Key: AdminPermissionErrorPassthroughRead, Module: "error_passthrough", Action: "read", Label: "错误透传查看"},
	{Key: AdminPermissionErrorPassthroughWrite, Module: "error_passthrough", Action: "write", Label: "错误透传编辑"},
	{Key: AdminPermissionTLSFingerprintProfilesRead, Module: "tls_fingerprint_profiles", Action: "read", Label: "TLS 指纹查看"},
	{Key: AdminPermissionTLSFingerprintProfilesWrite, Module: "tls_fingerprint_profiles", Action: "write", Label: "TLS 指纹编辑"},
	{Key: AdminPermissionScheduledTestsRead, Module: "scheduled_tests", Action: "read", Label: "定时测试查看"},
	{Key: AdminPermissionScheduledTestsWrite, Module: "scheduled_tests", Action: "write", Label: "定时测试编辑"},
	{Key: AdminPermissionAffiliatesRead, Module: "affiliates", Action: "read", Label: "邀请返利查看"},
	{Key: AdminPermissionAffiliatesWrite, Module: "affiliates", Action: "write", Label: "邀请返利编辑"},
	{Key: AdminPermissionPaymentRead, Module: "payment", Action: "read", Label: "支付查看"},
	{Key: AdminPermissionPaymentWrite, Module: "payment", Action: "write", Label: "支付编辑"},
}

var adminPermissionSet = func() map[string]struct{} {
	out := make(map[string]struct{}, len(adminPermissionDefinitions))
	for _, def := range adminPermissionDefinitions {
		out[def.Key] = struct{}{}
	}
	return out
}()

var adminPermissionWriteReadDependencies = func() map[string]string {
	reads := make(map[string]string)
	out := make(map[string]string)
	for _, def := range adminPermissionDefinitions {
		if def.Action == "read" {
			reads[def.Module] = def.Key
		}
	}
	for _, def := range adminPermissionDefinitions {
		if def.Action == "write" {
			if readKey := reads[def.Module]; readKey != "" {
				out[def.Key] = readKey
			}
		}
	}
	return out
}()

func AdminPermissionDefinitions() []AdminPermissionDefinition {
	out := make([]AdminPermissionDefinition, len(adminPermissionDefinitions))
	copy(out, adminPermissionDefinitions)
	return out
}

func IsKnownAdminPermission(permission string) bool {
	_, ok := adminPermissionSet[strings.TrimSpace(permission)]
	return ok
}

func NormalizeAdminPermissions(permissions []string) ([]string, error) {
	if len(permissions) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(permissions))
	out := make([]string, 0, len(permissions))
	for _, raw := range permissions {
		permission := strings.TrimSpace(raw)
		if permission == "" {
			continue
		}
		if !IsKnownAdminPermission(permission) {
			return nil, fmt.Errorf("unknown admin permission %q", permission)
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		out = append(out, permission)
		if readPermission := adminPermissionWriteReadDependencies[permission]; readPermission != "" {
			if _, ok := seen[readPermission]; !ok {
				seen[readPermission] = struct{}{}
				out = append(out, readPermission)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func HasAdminPermission(permissions []string, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	for _, item := range permissions {
		if strings.TrimSpace(item) == permission {
			return true
		}
	}
	return false
}
