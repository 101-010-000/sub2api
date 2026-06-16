import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { resolveCompletedSetupRedirectPath } from '@/router/setupRedirect'
import { firstAccessibleAdminPath, resolveAdminRoutePermissions } from '@/utils/adminPermissions'

const ADMIN_COMPLIANCE_HOLD_PATH = '/home'

// Mock 导航加载状态
vi.mock('@/composables/useNavigationLoading', () => {
  const mockStart = vi.fn()
  const mockEnd = vi.fn()
  return {
    useNavigationLoadingState: () => ({
      startNavigation: mockStart,
      endNavigation: mockEnd,
      isLoading: { value: false },
    }),
    useNavigationLoading: () => ({
      startNavigation: mockStart,
      endNavigation: mockEnd,
      isLoading: { value: false },
    }),
  }
})

// Mock 路由预加载
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

// Mock API 相关模块
vi.mock('@/api', () => ({
  authAPI: {
    getCurrentUser: vi.fn().mockResolvedValue({ data: {} }),
    logout: vi.fn(),
  },
  isTotp2FARequired: () => false,
}))

vi.mock('@/api/admin/system', () => ({
  checkUpdates: vi.fn(),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn(),
}))


// 用于测试的 auth 状态
interface MockAuthState {
  isAuthenticated: boolean
  isAdmin: boolean
  adminPermissions?: string[]
  isSimpleMode: boolean
  backendModeEnabled: boolean
  hasPendingAuthSession: boolean
  setupNeedsSetup?: boolean
  adminComplianceRequired?: boolean
}

/**
 * 将 router/index.ts 中 beforeEach 守卫的核心逻辑提取为可测试的函数
 */
function simulateGuard(
  toPath: string,
  toMeta: Record<string, any>,
  authState: MockAuthState
): string | false | null {
  return simulateGuardResult(toPath, toMeta, authState).redirect
}

function simulateGuardResult(
  toPath: string,
  toMeta: Record<string, any>,
  authState: MockAuthState
): { redirect: string | false | null; shouldFetchAdminCompliance: boolean; pendingComplianceRedirect?: string } {
  const allow = { redirect: null, shouldFetchAdminCompliance: false }
  const redirect = (path: string) => ({ redirect: path, shouldFetchAdminCompliance: false })
  const requiresAuth = toMeta.requiresAuth !== false
  const requiresAdmin = toMeta.requiresAdmin === true
  const isSuperAdmin = authState.isAdmin
  const hasAdminPermission = (permission: string) => {
    return isSuperAdmin || (authState.adminPermissions ?? []).includes(permission)
  }
  const canAccessAdmin = isSuperAdmin || (authState.adminPermissions ?? []).length > 0
  const adminHomePath = () => firstAccessibleAdminPath(hasAdminPermission)

  if (toPath === '/setup' && authState.setupNeedsSetup === false) {
    return redirect(resolveCompletedSetupRedirectPath(authState.isAuthenticated, canAccessAdmin))
  }

  // 不需要认证的路由
  if (!requiresAuth) {
    if (
      authState.isAuthenticated &&
      (toPath === '/login' || toPath === '/register')
    ) {
      if (authState.backendModeEnabled && !canAccessAdmin) {
        return allow
      }
      return redirect(canAccessAdmin ? adminHomePath() : '/dashboard')
    }
    if (authState.backendModeEnabled && !authState.isAuthenticated) {
      const allowed = ['/login', '/key-usage', '/setup', '/payment/result']
      const callbackPaths = [
        '/auth/callback',
        '/auth/linuxdo/callback',
        '/auth/oidc/callback',
        '/auth/wechat/callback',
        '/auth/wechat/payment/callback',
      ]
      const pendingAuthPaths = ['/register', '/email-verify']
      const isAllowed =
        allowed.some((path) => toPath === path || toPath.startsWith(path)) ||
        callbackPaths.includes(toPath) ||
        (authState.hasPendingAuthSession && pendingAuthPaths.includes(toPath))
      if (!isAllowed) {
        return redirect('/login')
      }
    }
    return allow // 允许通过
  }

  // 需要认证但未登录
  if (!authState.isAuthenticated) {
    return redirect('/login')
  }

  // 需要管理员但不是管理员
  if (requiresAdmin && !canAccessAdmin) {
    return redirect('/dashboard')
  }

  let shouldFetchAdminCompliance = false
  if (requiresAdmin) {
    if (toMeta.requiresSuperAdmin && !isSuperAdmin) {
      return redirect(adminHomePath())
    }
    const permissions = toMeta.adminPermission ? [toMeta.adminPermission] : resolveAdminRoutePermissions(toPath)
    if (permissions.length > 0 && !permissions.some(hasAdminPermission)) {
      return redirect(adminHomePath())
    }
    shouldFetchAdminCompliance = canAccessAdmin
    if (authState.adminComplianceRequired) {
      return {
        redirect: toPath === ADMIN_COMPLIANCE_HOLD_PATH ? false : ADMIN_COMPLIANCE_HOLD_PATH,
        shouldFetchAdminCompliance,
        pendingComplianceRedirect: toPath,
      }
    }
  }

  // 简易模式限制
  if (authState.isSimpleMode) {
    const restrictedPaths = [
      '/admin/groups',
      '/admin/subscriptions',
      '/admin/redeem',
      '/subscriptions',
      '/redeem',
    ]
    if (restrictedPaths.some((path) => toPath.startsWith(path))) {
      return redirect(canAccessAdmin ? adminHomePath() : '/dashboard')
    }
  }

  // Backend mode: admin gets full access, non-admin blocked
  if (authState.backendModeEnabled) {
    if (authState.isAuthenticated && canAccessAdmin) {
      return { redirect: null, shouldFetchAdminCompliance }
    }
    const allowed = ['/login', '/key-usage', '/setup', '/payment/result']
    const callbackPaths = [
      '/auth/callback',
      '/auth/linuxdo/callback',
      '/auth/oidc/callback',
      '/auth/wechat/callback',
      '/auth/wechat/payment/callback',
    ]
    const pendingAuthPaths = ['/register', '/email-verify']
    const isAllowed =
      allowed.some((path) => toPath === path || toPath.startsWith(path)) ||
      callbackPaths.includes(toPath) ||
      (authState.hasPendingAuthSession && pendingAuthPaths.includes(toPath))
    if (!isAllowed) {
      return redirect('/login')
    }
  }

  return { redirect: null, shouldFetchAdminCompliance } // 允许通过
}

describe('路由守卫逻辑', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  // --- 未认证用户 ---

  describe('未认证用户', () => {
    const authState: MockAuthState = {
      isAuthenticated: false,
      isAdmin: false,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问需要认证的页面重定向到 /login', () => {
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBe('/login')
    })

    it('访问管理页面重定向到 /login', () => {
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/login')
    })

    it('访问公开页面允许通过', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('访问 /home 公开页面允许通过', () => {
      const redirect = simulateGuard('/home', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })
  })

  // --- 已认证普通用户 ---

  describe('已认证普通用户', () => {
    const authState: MockAuthState = {
      isAuthenticated: true,
      isAdmin: false,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问 /login 重定向到 /dashboard', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('访问 /register 重定向到 /dashboard', () => {
      const redirect = simulateGuard('/register', { requiresAuth: false }, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('访问 /dashboard 允许通过', () => {
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBeNull()
    })

    it('访问管理页面被拒绝，重定向到 /dashboard', () => {
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('访问 /admin/users 被拒绝', () => {
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/dashboard')
    })
  })

  // --- 已认证管理员 ---

  describe('已认证管理员', () => {
    const authState: MockAuthState = {
      isAuthenticated: true,
      isAdmin: true,
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问 /login 重定向到 /admin/dashboard', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('访问管理页面允许通过', () => {
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBeNull()
    })

    it('访问用户页面允许通过', () => {
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBeNull()
    })
  })

  // --- 已认证委派后台用户 ---

  describe('已认证委派后台用户', () => {
    const authState: MockAuthState = {
      isAuthenticated: true,
      isAdmin: false,
      adminPermissions: ['admin.users.read'],
      isSimpleMode: false,
      backendModeEnabled: false,
      hasPendingAuthSession: false,
    }

    it('访问有 read 权限的后台页面允许通过', () => {
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
      expect(redirect).toBeNull()
    })

    it('访问有权限的后台页面会触发合规预检查', () => {
      const result = simulateGuardResult('/admin/users', { requiresAdmin: true }, authState)
      expect(result).toEqual({
        redirect: null,
        shouldFetchAdminCompliance: true,
      })
    })

    it('合规未确认时暂存目标后台页并跳转到安全承载页', () => {
      const result = simulateGuardResult(
        '/admin/users',
        { requiresAdmin: true },
        { ...authState, adminComplianceRequired: true },
      )

      expect(result).toEqual({
        redirect: '/home',
        shouldFetchAdminCompliance: true,
        pendingComplianceRedirect: '/admin/users',
      })
    })

    it('访问缺少权限的后台页面重定向到首个可访问后台页', () => {
      const redirect = simulateGuard('/admin/accounts', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/admin/users')
    })

    it('访问超级管理员页面重定向到首个可访问后台页', () => {
      const redirect = simulateGuard('/admin/settings', { requiresAdmin: true, requiresSuperAdmin: true }, authState)
      expect(redirect).toBe('/admin/users')
    })

    it('访问 /login 重定向到首个可访问后台页', () => {
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/users')
    })

    it('只有用户属性权限也可以进入用户父页面', () => {
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, {
        ...authState,
        adminPermissions: ['admin.user_attributes.read'],
      })
      expect(redirect).toBeNull()
    })

    it('只有账号子模块权限也可以进入账号父页面', () => {
      const redirect = simulateGuard('/admin/accounts', { requiresAdmin: true }, {
        ...authState,
        adminPermissions: ['admin.scheduled_tests.read'],
      })
      expect(redirect).toBeNull()
    })

    it('只有备份或支付权限也可以进入设置父页面', () => {
      expect(simulateGuard('/admin/settings', { requiresAdmin: true }, {
        ...authState,
        adminPermissions: ['admin.backup.read'],
      })).toBeNull()
      expect(simulateGuard('/admin/settings', { requiresAdmin: true }, {
        ...authState,
        adminPermissions: ['admin.payment.read'],
      })).toBeNull()
    })
  })

  // --- 简易模式 ---

  describe('简易模式受限路由', () => {
    it('普通用户简易模式访问 /subscriptions 重定向到 /dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/subscriptions', {}, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('普通用户简易模式访问 /redeem 重定向到 /dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/redeem', {}, authState)
      expect(redirect).toBe('/dashboard')
    })

    it('管理员简易模式访问 /admin/groups 重定向到 /admin/dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/admin/groups', { requiresAdmin: true }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('管理员简易模式访问 /admin/subscriptions 重定向', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard(
        '/admin/subscriptions',
        { requiresAdmin: true },
        authState
      )
      expect(redirect).toBe('/admin/dashboard')
    })

    it('简易模式下非受限页面正常访问', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBeNull()
    })

    it('简易模式下 /keys 正常访问', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        isSimpleMode: true,
        backendModeEnabled: false,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/keys', {}, authState)
      expect(redirect).toBeNull()
    })
  })

  describe('Backend Mode', () => {
    it('unauthenticated: /home redirects to /login', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/home', { requiresAuth: false }, authState)
      expect(redirect).toBe('/login')
    })

    it('unauthenticated: /login is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /key-usage is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/key-usage', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /setup is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/setup', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: initialized /setup redirects to /login', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
        setupNeedsSetup: false,
      }
      const redirect = simulateGuard('/setup', { requiresAuth: false }, authState)
      expect(redirect).toBe('/login')
    })

    it('admin: initialized /setup redirects to /admin/dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
        setupNeedsSetup: false,
      }
      const redirect = simulateGuard('/setup', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('admin: /admin/dashboard is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/admin/dashboard', { requiresAdmin: true }, authState)
      expect(redirect).toBeNull()
    })

    it('admin: /login redirects to /admin/dashboard', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: true,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBe('/admin/dashboard')
    })

    it('non-admin authenticated: /dashboard redirects to /login', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/dashboard', {}, authState)
      expect(redirect).toBe('/login')
    })

    it('non-admin authenticated: /login is allowed (no redirect loop)', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/login', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('non-admin authenticated: /key-usage is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/key-usage', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('delegated admin authenticated: /admin/users is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: true,
        isAdmin: false,
        adminPermissions: ['admin.users.read'],
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/admin/users', { requiresAdmin: true }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: callback routes are allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/auth/wechat/callback', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: WeChat payment callback route is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/auth/wechat/payment/callback', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /payment/result is allowed', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/payment/result', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /register is allowed when a pending auth session exists', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: true,
      }
      const redirect = simulateGuard('/register', { requiresAuth: false }, authState)
      expect(redirect).toBeNull()
    })

    it('unauthenticated: /email-verify is blocked without a pending auth session', () => {
      const authState: MockAuthState = {
        isAuthenticated: false,
        isAdmin: false,
        isSimpleMode: false,
        backendModeEnabled: true,
        hasPendingAuthSession: false,
      }
      const redirect = simulateGuard('/email-verify', { requiresAuth: false }, authState)
      expect(redirect).toBe('/login')
    })
  })
})
