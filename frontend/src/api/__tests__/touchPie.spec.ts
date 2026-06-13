import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import { touchPieAPI } from '@/api/touchPie'

describe('touch pie api', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockReset()
    post.mockResolvedValue({ data: {} })
  })

  it('starts device authorization with optional base url', async () => {
    await touchPieAPI.startDevice(' https://sub2api.test/ ')

    expect(post).toHaveBeenCalledWith('/touch-pie/device/start', {
      base_url: 'https://sub2api.test/'
    })
  })

  it('starts device authorization without sending empty base url', async () => {
    await touchPieAPI.startDevice('   ')

    expect(post).toHaveBeenCalledWith('/touch-pie/device/start', {})
  })

  it('approves device with user code and selected api key', async () => {
    await touchPieAPI.approveDevice('ABCD1234', 7)

    expect(post).toHaveBeenCalledWith('/touch-pie/device/approve', {
      user_code: 'ABCD1234',
      api_key_id: 7
    })
  })

  it('approves device without api key when not selected', async () => {
    await touchPieAPI.approveDevice('ABCD1234')

    expect(post).toHaveBeenCalledWith('/touch-pie/device/approve', {
      user_code: 'ABCD1234'
    })
  })

  it('loads bootstrap data for authenticated touch pie setup', async () => {
    await touchPieAPI.bootstrap()

    expect(get).toHaveBeenCalledWith('/touch-pie/bootstrap')
  })

  it('creates touch pie api key with selected group', async () => {
    await touchPieAPI.createAPIKey({ group_id: 7 })

    expect(post).toHaveBeenCalledWith('/touch-pie/api-keys', {
      group_id: 7
    })
  })

  it('requests token with device code', async () => {
    await touchPieAPI.requestToken('device-code')

    expect(post).toHaveBeenCalledWith('/touch-pie/device/token', {
      device_code: 'device-code'
    })
  })

  it('exports api key through touch pie endpoint', async () => {
    await touchPieAPI.exportAPIKey(7)

    expect(post).toHaveBeenCalledWith('/touch-pie/api-keys/7/export')
  })
})
