import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TouchPieAuthorizeView from '@/views/TouchPieAuthorizeView.vue'

const { routeState, authState, bootstrapMock, createAPIKeyMock, approveDeviceMock } = vi.hoisted(() => ({
  routeState: {
    fullPath: '/touch-pie/authorize?user_code=ABCD1234',
    query: {
      user_code: 'ABCD1234',
    } as Record<string, unknown>,
  },
  authState: {
    isAuthenticated: true,
  },
  bootstrapMock: vi.fn(),
  createAPIKeyMock: vi.fn(),
  approveDeviceMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('@/api/touchPie', () => ({
  bootstrap: (...args: any[]) => bootstrapMock(...args),
  createAPIKey: (...args: any[]) => createAPIKeyMock(...args),
  approveDevice: (...args: any[]) => approveDeviceMock(...args),
}))

function mountView() {
  return mount(TouchPieAuthorizeView, {
    global: {
      stubs: {
        RouterLink: {
          name: 'RouterLink',
          props: ['to'],
          template: '<a :href="typeof to === `string` ? to : to.path"><slot /></a>',
        },
      },
    },
  })
}

describe('TouchPieAuthorizeView', () => {
  beforeEach(() => {
    routeState.fullPath = '/touch-pie/authorize?user_code=ABCD1234'
    routeState.query = { user_code: 'ABCD1234' }
    authState.isAuthenticated = true
    bootstrapMock.mockReset()
    bootstrapMock.mockResolvedValue({
      groups: [{ id: 3, name: 'OpenAI', status: 'active' }],
      api_keys: [],
      provider_name: 'TouchX',
      provider_source: 'touchx',
      provider_accent_color: '#8B5CF6',
      default_model: 'gpt-5.5',
    })
    createAPIKeyMock.mockReset()
    createAPIKeyMock.mockResolvedValue({
      id: 9,
      name: 'TouchX',
      key: 'sk-touchx',
      status: 'active',
      provider_name: 'TouchX',
      provider_source: 'touchx',
      provider_accent_color: '#8B5CF6',
      default_model: 'gpt-5.5',
    })
    approveDeviceMock.mockReset()
  })

  it('shows missing code state', () => {
    routeState.fullPath = '/touch-pie/authorize'
    routeState.query = {}

    const wrapper = mountView()

    expect(wrapper.text()).toContain('缺少 user_code')
    expect(wrapper.text()).toContain('返回控制台')
  })

  it('asks unauthenticated user to login and preserves redirect', () => {
    authState.isAuthenticated = false

    const wrapper = mountView()

    expect(wrapper.text()).toContain('请先登录 Sub2API')
    expect(wrapper.findComponent({ name: 'RouterLink' }).props('to')).toEqual({
      path: '/login',
      query: {
        redirect: '/touch-pie/authorize?user_code=ABCD1234',
      },
    })
  })

  it('creates touchx api key, approves device and shows success state', async () => {
    approveDeviceMock.mockResolvedValue({ approved: true })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(createAPIKeyMock).toHaveBeenCalledWith({
      name: 'TouchX',
      group_id: 3,
    })
    expect(approveDeviceMock).toHaveBeenCalledWith('ABCD1234')
    expect(wrapper.text()).toContain('已授权')
    expect(wrapper.text()).toContain('已创建 TouchX 渠道')
  })

  it('maps expired device error to setup retry copy', async () => {
    approveDeviceMock.mockRejectedValue({ reason: 'TOUCH_PIE_DEVICE_EXPIRED' })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('授权码已过期')
    expect(wrapper.text()).toContain('重新执行 /sub2api setup')
  })

  it('loads groups and asks user to choose touchx channel group', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(bootstrapMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('TouchX 渠道分组')
    expect(wrapper.find('select').element.value).toBe('3')
  })
})
