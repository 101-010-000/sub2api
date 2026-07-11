import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { updateUser, updateUserAttributeValues, showError, showSuccess, authState } = vi.hoisted(() => ({
  updateUser: vi.fn(),
  updateUserAttributeValues: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  authState: { isSuperAdmin: false },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      update: updateUser,
    },
    userAttributes: {
      updateUserAttributeValues,
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

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true),
  }),
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

vi.mock('@/components/user/UserAttributeForm.vue', () => ({
  default: {
    name: 'UserAttributeForm',
    template: '<div />',
  },
}))

import UserEditModal from '../UserEditModal.vue'

const user = {
  id: 42,
  email: 'user@example.com',
  username: 'user',
  notes: '',
  role: 'user',
  status: 'active',
  concurrency: 1,
  rpm_limit: 0,
  api_key_max_active_ips: 0,
  api_key_max_active_ips_visible: false,
  admin_permissions: [],
}

describe('UserEditModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authState.isSuperAdmin = false
    updateUser.mockResolvedValue({ id: user.id })
  })

  it('非超级管理员编辑普通用户时不展示也不提交角色字段', async () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user: user as any,
      },
    })

    expect(wrapper.find('select').exists()).toBe(false)
    await wrapper.get('#edit-user-form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledTimes(1)
    expect(updateUser).toHaveBeenCalledWith(user.id, expect.not.objectContaining({
      role: expect.anything(),
      admin_permissions: expect.anything(),
    }))
    expect(showSuccess).toHaveBeenCalledWith('admin.users.userUpdated')
  })
})
