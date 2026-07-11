import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { createUser, showError, showSuccess, authState } = vi.hoisted(() => ({
  createUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  authState: { isSuperAdmin: true },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      create: createUser,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>',
  },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    name: 'Icon',
    template: '<span />',
  },
}))

import UserCreateModal from '../UserCreateModal.vue'

function mountModal() {
  return mount(UserCreateModal, {
    props: {
      show: true,
    },
  })
}

async function fillRequiredFields(wrapper: ReturnType<typeof mountModal>) {
  const inputs = wrapper.findAll('input')
  await inputs[0].setValue(' delegated@example.com ')
  await inputs[1].setValue(' strong-pass ')
}

describe('UserCreateModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authState.isSuperAdmin = true
    createUser.mockResolvedValue({ id: 100 })
  })

  it('超级管理员创建用户时可提交后台委派权限，并自动补齐写权限依赖的查看权限', async () => {
    const wrapper = mountModal()
    await fillRequiredFields(wrapper)

    expect(wrapper.get('[data-test="admin-permission-row-users"]').text()).toContain('用户管理')
    await wrapper.get('[data-test="admin-permission-users-write"]').setValue(true)
    await wrapper.get('#create-user-form').trigger('submit.prevent')
    await flushPromises()

    expect(createUser).toHaveBeenCalledTimes(1)
    expect(createUser).toHaveBeenCalledWith(expect.objectContaining({
      email: 'delegated@example.com',
      password: 'strong-pass',
      admin_permissions: ['admin.users.read', 'admin.users.write'],
    }))
    expect(showSuccess).toHaveBeenCalledWith('admin.users.userCreated')
  })

  it('非超级管理员创建用户时不展示也不提交后台委派权限', async () => {
    authState.isSuperAdmin = false
    const wrapper = mountModal()
    await fillRequiredFields(wrapper)

    expect(wrapper.text()).not.toContain('后台权限')
    expect(wrapper.find('select').exists()).toBe(false)
    await wrapper.get('#create-user-form').trigger('submit.prevent')
    await flushPromises()

    expect(createUser).toHaveBeenCalledTimes(1)
    expect(createUser.mock.calls[0][0]).not.toHaveProperty('admin_permissions')
    expect(createUser.mock.calls[0][0]).not.toHaveProperty('role')
  })

  it('表单校验失败时不提交创建请求', async () => {
    const wrapper = mountModal()
    await wrapper.get('#create-user-form').trigger('submit.prevent')
    await flushPromises()

    expect(createUser).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.users.emailRequired')
    expect(showSuccess).not.toHaveBeenCalled()
  })
})
