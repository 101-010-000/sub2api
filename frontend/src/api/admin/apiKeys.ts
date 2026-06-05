/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey, ApiKeyRuntimeStatus, FetchOptions } from '@/types'

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

/**
 * Update an API key's group binding
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(id: number, groupId: number | null): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, {
    group_id: groupId === null ? 0 : groupId
  })
  return data
}

export async function updateApiKeyRuntimeLimits(
  id: number,
  limits: {
    max_active_ips?: number
    ip_idle_timeout_seconds?: number
    max_concurrency?: number
  }
): Promise<UpdateApiKeyGroupResult> {
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, limits)
  return data
}

export async function getRuntime(
  id: number,
  options?: FetchOptions
): Promise<ApiKeyRuntimeStatus> {
  const { data } = await apiClient.get<ApiKeyRuntimeStatus>(`/admin/api-keys/${id}/runtime`, {
    signal: options?.signal
  })
  return data
}

export async function removeRuntimeIP(id: number, ip: string): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    `/admin/api-keys/${id}/runtime/ips/remove`,
    { ip }
  )
  return data
}

export async function clearRuntimeIPs(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    `/admin/api-keys/${id}/runtime/ips/clear`
  )
  return data
}

export const apiKeysAPI = {
  updateApiKeyGroup,
  updateApiKeyRuntimeLimits,
  getRuntime,
  removeRuntimeIP,
  clearRuntimeIPs
}

export default apiKeysAPI
