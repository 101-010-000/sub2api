import { apiClient } from './client'

export interface TouchPieApproveResponse {
  approved: boolean
}

export async function approveDevice(userCode: string): Promise<TouchPieApproveResponse> {
  const { data } = await apiClient.post<TouchPieApproveResponse>('/touch-pie/device/approve', {
    user_code: userCode
  })
  return data
}

export const touchPieAPI = {
  approveDevice
}
