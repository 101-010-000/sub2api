export type AdminPermissionAction = 'read' | 'write'

export interface AdminPermissionModule {
  key: string
  label: string
  read: string
  write: string
  routePath: string
}

export const adminPermissionModules: AdminPermissionModule[] = [
  { key: 'dashboard', label: '仪表盘', read: 'admin.dashboard.read', write: 'admin.dashboard.write', routePath: '/admin/dashboard' },
  { key: 'ops', label: '运维监控', read: 'admin.ops.read', write: 'admin.ops.write', routePath: '/admin/ops' },
  { key: 'users', label: '用户管理', read: 'admin.users.read', write: 'admin.users.write', routePath: '/admin/users' },
  { key: 'groups', label: '分组管理', read: 'admin.groups.read', write: 'admin.groups.write', routePath: '/admin/groups' },
  { key: 'accounts', label: '账号管理', read: 'admin.accounts.read', write: 'admin.accounts.write', routePath: '/admin/accounts' },
  { key: 'channels', label: '渠道管理', read: 'admin.channels.read', write: 'admin.channels.write', routePath: '/admin/channels/pricing' },
  { key: 'channel_monitors', label: '渠道监控', read: 'admin.channel_monitors.read', write: 'admin.channel_monitors.write', routePath: '/admin/channels/monitor' },
  { key: 'subscriptions', label: '订阅管理', read: 'admin.subscriptions.read', write: 'admin.subscriptions.write', routePath: '/admin/subscriptions' },
  { key: 'announcements', label: '公告管理', read: 'admin.announcements.read', write: 'admin.announcements.write', routePath: '/admin/announcements' },
  { key: 'proxies', label: '代理管理', read: 'admin.proxies.read', write: 'admin.proxies.write', routePath: '/admin/proxies' },
  { key: 'risk_control', label: '风控中心', read: 'admin.risk_control.read', write: 'admin.risk_control.write', routePath: '/admin/risk-control' },
  { key: 'redeem_codes', label: '卡密管理', read: 'admin.redeem_codes.read', write: 'admin.redeem_codes.write', routePath: '/admin/redeem' },
  { key: 'promo_codes', label: '优惠码管理', read: 'admin.promo_codes.read', write: 'admin.promo_codes.write', routePath: '/admin/promo-codes' },
  { key: 'settings', label: '系统设置', read: 'admin.settings.read', write: 'admin.settings.write', routePath: '/admin/settings' },
  { key: 'data_management', label: '数据管理', read: 'admin.data_management.read', write: 'admin.data_management.write', routePath: '/admin/settings' },
  { key: 'backup', label: '备份管理', read: 'admin.backup.read', write: 'admin.backup.write', routePath: '/admin/settings' },
  { key: 'system', label: '系统管理', read: 'admin.system.read', write: 'admin.system.write', routePath: '/admin/settings' },
  { key: 'usage', label: '用量记录', read: 'admin.usage.read', write: 'admin.usage.write', routePath: '/admin/usage' },
  { key: 'user_attributes', label: '用户属性', read: 'admin.user_attributes.read', write: 'admin.user_attributes.write', routePath: '/admin/users' },
  { key: 'error_passthrough', label: '错误透传', read: 'admin.error_passthrough.read', write: 'admin.error_passthrough.write', routePath: '/admin/accounts' },
  { key: 'tls_fingerprint_profiles', label: 'TLS 指纹', read: 'admin.tls_fingerprint_profiles.read', write: 'admin.tls_fingerprint_profiles.write', routePath: '/admin/accounts' },
  { key: 'scheduled_tests', label: '定时测试', read: 'admin.scheduled_tests.read', write: 'admin.scheduled_tests.write', routePath: '/admin/accounts' },
  { key: 'affiliates', label: '邀请返利', read: 'admin.affiliates.read', write: 'admin.affiliates.write', routePath: '/admin/affiliates/invites' },
  { key: 'payment', label: '支付管理', read: 'admin.payment.read', write: 'admin.payment.write', routePath: '/admin/orders/dashboard' },
]

export const allAdminPermissionKeys = adminPermissionModules.flatMap((item) => [item.read, item.write])

export function normalizeAdminPermissions(permissions: unknown): string[] {
  if (!Array.isArray(permissions)) return []
  const known = new Set(allAdminPermissionKeys)
  return [...new Set(permissions.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter((item) => known.has(item)))].sort()
}

export function resolveAdminRoutePermission(path: string): string | undefined {
  return resolveAdminRoutePermissions(path)[0]
}

export function resolveAdminRoutePermissions(path: string): string[] {
  const normalized = path.trim()
  if (normalized === '/admin' || normalized.startsWith('/admin/dashboard')) return ['admin.dashboard.read']
  if (normalized.startsWith('/admin/ops')) return ['admin.ops.read']
  if (normalized.startsWith('/admin/users')) return ['admin.users.read']
  if (normalized.startsWith('/admin/groups')) return ['admin.groups.read']
  if (normalized.startsWith('/admin/accounts')) return ['admin.accounts.read']
  if (normalized.startsWith('/admin/channels/monitor')) return ['admin.channel_monitors.read']
  if (normalized.startsWith('/admin/channels')) return ['admin.channels.read']
  if (normalized.startsWith('/admin/subscriptions')) return ['admin.subscriptions.read']
  if (normalized.startsWith('/admin/announcements')) return ['admin.announcements.read']
  if (normalized.startsWith('/admin/proxies')) return ['admin.proxies.read']
  if (normalized.startsWith('/admin/risk-control')) return ['admin.risk_control.read']
  if (normalized.startsWith('/admin/redeem')) return ['admin.redeem_codes.read']
  if (normalized.startsWith('/admin/promo-codes')) return ['admin.promo_codes.read']
  if (normalized.startsWith('/admin/settings')) return ['admin.settings.read', 'admin.data_management.read', 'admin.backup.read', 'admin.system.read']
  if (normalized.startsWith('/admin/usage')) return ['admin.usage.read']
  if (normalized.startsWith('/admin/affiliates')) return ['admin.affiliates.read']
  if (normalized.startsWith('/admin/orders')) return ['admin.payment.read']
  return []
}

export function firstAccessibleAdminPath(hasPermission: (permission: string) => boolean): string {
  for (const module of adminPermissionModules) {
    if (hasPermission(module.read)) return module.routePath
  }
  return '/dashboard'
}
