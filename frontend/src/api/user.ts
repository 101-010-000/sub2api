/**
 * User API endpoints
 * Handles user profile management and password changes
 */

import { apiClient } from './client'
import {
  resolveWeChatOAuthStartStrict,
  prepareOAuthBindAccessTokenCookie,
  type WeChatOAuthPublicSettings,
} from './auth'
import type {
  User,
  ChangePasswordRequest,
  NotifyEmailEntry,
  UserAuthProvider,
  UserAffiliateDetail,
  AffiliateTransferResponse,
  PlatformQuotasResponse,
  UserNotificationSettings,
  UpdateUserNotificationSettingsRequest,
} from '@/types'

/**
 * Get current user profile
 * @returns User profile data
 */
export async function getProfile(): Promise<User> {
  const { data } = await apiClient.get<User>('/user/profile')
  return data
}

/**
 * Update current user profile
 * @param profile - Profile data to update
 * @returns Updated user profile data
 */
export async function updateProfile(profile: {
  username?: string
  avatar_url?: string | null
  balance_notify_enabled?: boolean
  balance_notify_threshold?: number | null
  balance_notify_extra_emails?: NotifyEmailEntry[]
}): Promise<User> {
  const { data } = await apiClient.put<User>('/user', profile)
  return data
}

/**
 * Change current user password
 * @param passwords - Old and new password
 * @returns Success message
 */
export async function changePassword(
  oldPassword: string,
  newPassword: string
): Promise<{ message: string }> {
  const payload: ChangePasswordRequest = {
    old_password: oldPassword,
    new_password: newPassword
  }

  const { data } = await apiClient.put<{ message: string }>('/user/password', payload)
  return data
}

/**
 * Send verification code for adding a notify email
 * @param email - Email address to verify
 */
export async function sendNotifyEmailCode(email: string): Promise<void> {
  await apiClient.post('/user/notify-email/send-code', { email })
}

/**
 * Verify and add a notify email
 * @param email - Email address to add
 * @param code - Verification code
 */
export async function verifyNotifyEmail(email: string, code: string): Promise<void> {
  await apiClient.post('/user/notify-email/verify', { email, code })
}

/**
 * Remove a notify email
 * @param email - Email address to remove
 */
export async function removeNotifyEmail(email: string): Promise<void> {
  await apiClient.delete('/user/notify-email', { data: { email } })
}

/**
 * Toggle a notify email's disabled state
 * @param email - Email address (empty string for primary email placeholder)
 * @param disabled - Whether to disable the email
 */
export async function toggleNotifyEmail(email: string, disabled: boolean): Promise<User> {
  const { data } = await apiClient.put<User>('/user/notify-email/toggle', { email, disabled })
  return data
}

export async function getNotificationSettings(): Promise<UserNotificationSettings> {
  const { data } = await apiClient.get<UserNotificationSettings>('/user/notification-settings')
  return data
}

export async function updateNotificationSettings(
  payload: UpdateUserNotificationSettingsRequest
): Promise<UserNotificationSettings> {
  const { data } = await apiClient.patch<UserNotificationSettings>('/user/notification-settings', payload)
  return data
}

export async function sendEmailBindingCode(email: string): Promise<void> {
  await apiClient.post('/user/account-bindings/email/send-code', { email })
}

export async function bindEmailIdentity(payload: {
  email: string
  verify_code: string
  password: string
}): Promise<User> {
  const { data } = await apiClient.post<User>('/user/account-bindings/email', payload)
  return data
}

export async function unbindAuthIdentity(provider: BindableOAuthProvider): Promise<User> {
  const { data } = await apiClient.delete<User>(`/user/account-bindings/${provider}`)
  return data
}

export type BindableOAuthProvider = Exclude<UserAuthProvider, 'email'>

export interface UserRiskControlBanStatus {
  user_id: number
  banned: boolean
  reason: string
  triggered_at?: string
  banned_until?: string
  remaining_seconds: number
  self_unban_available: boolean
  self_unban_attempts_used: number
  self_unban_max_attempts: number
  self_unban_wait_seconds: number
  self_unban_window_reset_at?: string
}

export interface UserRiskControlSelfUnbanResponse {
  user_id: number
  unbanned: boolean
  status: string
  attempts_used: number
  max_attempts: number
  wait_seconds: number
  window_reset_at?: string
  message: string
}

interface BuildOAuthBindingStartURLOptions {
  redirectTo?: string
  wechatOAuthSettings?: WeChatOAuthPublicSettings | null
}

export function resolveWeChatOAuthMode(): 'open' | 'mp' {
  if (typeof navigator === 'undefined') {
    return 'open'
  }
  return /MicroMessenger/i.test(navigator.userAgent) ? 'mp' : 'open'
}

function resolveWeChatOAuthBindingMode(
  settings?: WeChatOAuthPublicSettings | null
): 'open' | 'mp' | null {
  if (settings) {
    return resolveWeChatOAuthStartStrict(settings).mode
  }
  return resolveWeChatOAuthMode()
}

export function buildOAuthBindingStartURL(
  provider: BindableOAuthProvider,
  options: BuildOAuthBindingStartURLOptions = {}
): string | null {
  const redirectTo = options.redirectTo?.trim() || '/profile'
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalized = apiBase.replace(/\/$/, '')
  const params = new URLSearchParams({
    redirect: redirectTo,
    intent: 'bind_current_user'
  })

  if (provider === 'wechat') {
    const mode = resolveWeChatOAuthBindingMode(options.wechatOAuthSettings)
    if (!mode) {
      return null
    }
    params.set('mode', mode)
  }

  return `${normalized}/auth/oauth/${provider}/bind/start?${params.toString()}`
}

export function buildFeishuNotifyBindStartURL(
  redirectTo = '/profile',
  bindStartPath = '/api/v1/auth/oauth/feishu/notify/bind/start'
): string {
  const redirect = redirectTo.trim() || '/profile'
  const path = bindStartPath.trim() || '/api/v1/auth/oauth/feishu/notify/bind/start'
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalizedApiBase = apiBase.replace(/\/$/, '')
  let normalizedPath: string
  if (/^https?:\/\//i.test(path)) {
    normalizedPath = path
  } else if (path.startsWith('/api/') && /^https?:\/\//i.test(normalizedApiBase)) {
    normalizedPath = `${new URL(normalizedApiBase).origin}${path}`
  } else if (path.startsWith('/api/')) {
    normalizedPath = path
  } else {
    normalizedPath = `${normalizedApiBase}/${path.replace(/^\//, '')}`
  }
  const separator = normalizedPath.includes('?') ? '&' : '?'
  return `${normalizedPath}${separator}${new URLSearchParams({ redirect }).toString()}`
}

export async function startOAuthBinding(
  provider: BindableOAuthProvider,
  options: BuildOAuthBindingStartURLOptions = {}
): Promise<void> {
  if (typeof window === 'undefined') {
    return
  }
  const startURL = buildOAuthBindingStartURL(provider, options)
  if (!startURL) {
    return
  }
  await prepareOAuthBindAccessTokenCookie()
  window.location.href = startURL
}

export async function startFeishuNotifyBinding(
  redirectTo = '/profile',
  bindStartPath?: string
): Promise<void> {
  if (typeof window === 'undefined') {
    return
  }
  await prepareOAuthBindAccessTokenCookie()
  window.location.href = buildFeishuNotifyBindStartURL(redirectTo, bindStartPath)
}

export async function getAffiliateDetail(): Promise<UserAffiliateDetail> {
  const { data } = await apiClient.get<UserAffiliateDetail>('/user/aff')
  return data
}

export async function transferAffiliateQuota(): Promise<AffiliateTransferResponse> {
  const { data } = await apiClient.post<AffiliateTransferResponse>('/user/aff/transfer')
  return data
}

/**
 * 获取当前用户的平台限额 + 用量。
 */
export async function getMyPlatformQuotas(): Promise<PlatformQuotasResponse> {
  const { data } = await apiClient.get<PlatformQuotasResponse>('/user/platform-quotas')
  return data
}


export async function getRiskControlBanStatus(): Promise<UserRiskControlBanStatus> {
  const { data } = await apiClient.get<UserRiskControlBanStatus>('/user/risk-control/ban-status')
  return data
}

export async function selfUnbanRiskControl(): Promise<UserRiskControlSelfUnbanResponse> {
  const { data } = await apiClient.post<UserRiskControlSelfUnbanResponse>(
    '/user/risk-control/self-unban'
  )
  return data
}

export const userAPI = {
  getProfile,
  updateProfile,
  changePassword,
  sendNotifyEmailCode,
  verifyNotifyEmail,
  removeNotifyEmail,
  toggleNotifyEmail,
  getNotificationSettings,
  updateNotificationSettings,
  sendEmailBindingCode,
  bindEmailIdentity,
  unbindAuthIdentity,
  buildOAuthBindingStartURL,
  buildFeishuNotifyBindStartURL,
  startOAuthBinding,
  startFeishuNotifyBinding,
  getAffiliateDetail,
  transferAffiliateQuota,
  getMyPlatformQuotas,
  getRiskControlBanStatus,
  selfUnbanRiskControl,
}

export default userAPI
