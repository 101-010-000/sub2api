import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAdminComplianceStore } from '@/stores/adminCompliance'

const mockAccept = vi.fn()

vi.mock('@/api/admin/compliance', () => ({
  default: {
    getStatus: vi.fn(),
    accept: (...args: any[]) => mockAccept(...args),
  },
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh',
}))

describe('useAdminComplianceStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('一次性消费合规确认后的待跳转路径', () => {
    const store = useAdminComplianceStore()

    store.setPendingRedirectPath('/admin/ops')

    expect(store.pendingRedirectPath).toBe('/admin/ops')
    expect(store.consumePendingRedirectPath()).toBe('/admin/ops')
    expect(store.consumePendingRedirectPath()).toBe('')
  })

  it('确认成功后标记为已初始化并隐藏弹窗', async () => {
    mockAccept.mockResolvedValue({
      required: false,
      version: 'v2026.06.10',
      document_path_zh: 'docs/legal/admin-compliance.zh.md',
      document_path_en: 'docs/legal/admin-compliance.en.md',
      document_url_zh: 'https://github.com/Wei-Shaw/sub2api/blob/main/docs/legal/admin-compliance.zh.md',
      document_url_en: 'https://github.com/Wei-Shaw/sub2api/blob/main/docs/legal/admin-compliance.en.md',
      ack_phrase_zh: '我已阅读、理解并同意 Sub2API 部署与运营合规承诺',
      ack_phrase_en: 'I have read, understood, and agree to the Sub2API Deployment and Operation Compliance Commitment',
    })
    const store = useAdminComplianceStore()

    await store.accept('我已阅读、理解并同意 Sub2API 部署与运营合规承诺')

    expect(store.initialized).toBe(true)
    expect(store.shouldShow).toBe(false)
  })
})
