import { apiClient } from './client'
import type { Group } from '@/types'

export interface TouchPieStartRequest {
  base_url?: string
}

export interface TouchPieStartResponse {
  device_code: string
  user_code: string
  verification_uri: string
  verification_uri_complete: string
  expires_at: string
  interval_seconds: number
}

export interface TouchPieApproveResponse {
  approved: boolean
}

export interface TouchPieTokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: 'Bearer' | string
  user_id: number
  api_key_id?: number
}

export interface TouchPieExportAPIKeyResponse {
  id: number
  name: string
  key: string
  status: string
  provider_name: string
  provider_source: string
  provider_accent_color: string
  default_model: string
}

export interface TouchPieAPIKeyCandidate {
  id: number
  name: string
  status: string
  group_id?: number | null
}

export interface TouchPieBootstrapResponse {
  groups: Group[]
  api_keys: TouchPieAPIKeyCandidate[]
  provider_name: string
  provider_source: string
  provider_accent_color: string
  default_model: string
}

export interface TouchPieCreateAPIKeyRequest {
  name?: string
  group_id?: number | null
}

export async function startDevice(baseURL?: string): Promise<TouchPieStartResponse> {
  const payload: TouchPieStartRequest = {}
  if (baseURL?.trim()) {
    payload.base_url = baseURL.trim()
  }
  const { data } = await apiClient.post<TouchPieStartResponse>('/touch-pie/device/start', payload)
  return data
}

export async function bootstrap(): Promise<TouchPieBootstrapResponse> {
  const { data } = await apiClient.get<TouchPieBootstrapResponse>('/touch-pie/bootstrap')
  return data
}

export async function createAPIKey(req: TouchPieCreateAPIKeyRequest): Promise<TouchPieExportAPIKeyResponse> {
  const { data } = await apiClient.post<TouchPieExportAPIKeyResponse>('/touch-pie/api-keys', req)
  return data
}

export async function approveDevice(userCode: string, apiKeyID?: number | null): Promise<TouchPieApproveResponse> {
  const payload: { user_code: string; api_key_id?: number } = { user_code: userCode }
  if (apiKeyID != null) {
    payload.api_key_id = apiKeyID
  }
  const { data } = await apiClient.post<TouchPieApproveResponse>('/touch-pie/device/approve', payload)
  return data
}

export async function requestToken(deviceCode: string): Promise<TouchPieTokenResponse> {
  const { data } = await apiClient.post<TouchPieTokenResponse>('/touch-pie/device/token', {
    device_code: deviceCode
  })
  return data
}

export async function exportAPIKey(keyID: number): Promise<TouchPieExportAPIKeyResponse> {
  const { data } = await apiClient.post<TouchPieExportAPIKeyResponse>(`/touch-pie/api-keys/${keyID}/export`)
  return data
}

export const touchPieAPI = {
  startDevice,
  bootstrap,
  createAPIKey,
  approveDevice,
  requestToken,
  exportAPIKey
}
