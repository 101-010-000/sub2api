<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ localText('通知渠道', 'Notification channels') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{
          localText(
            '普通通知优先走飞书，未绑定或发送失败时回落邮件；重要通知仍使用邮件。',
            'Regular notifications use Feishu first and fall back to email. Critical notifications still use email.'
          )
        }}
      </p>
    </div>

    <div class="space-y-4 px-6 py-6">
      <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <Icon name="refresh" size="sm" class="animate-spin" />
        {{ localText('正在加载通知渠道', 'Loading notification channels') }}
      </div>

      <div
        v-else
        class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div class="flex min-w-0 items-start gap-3">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-300">
              <Icon name="chat" size="md" />
            </div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="font-medium text-gray-900 dark:text-white">
                  {{ localText('飞书', 'Feishu') }}
                </h3>
                <span
                  class="rounded-full px-2 py-0.5 text-xs font-medium"
                  :class="statusBadgeClass"
                >
                  {{ statusLabel }}
                </span>
              </div>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ statusDescription }}
              </p>
              <div v-if="identitySummary.length > 0" class="mt-3 flex flex-wrap gap-2">
                <span
                  v-for="item in identitySummary"
                  :key="item.label"
                  class="rounded-md bg-gray-100 px-2 py-1 font-mono text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                >
                  {{ item.label }}: {{ item.value }}
                </span>
              </div>
            </div>
          </div>

          <div class="flex shrink-0 flex-wrap items-center gap-2 sm:justify-end">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="!canBind || binding"
              @click="bindFeishu"
            >
              <Icon v-if="binding" name="refresh" size="sm" class="mr-1 animate-spin" />
              <Icon v-else name="link" size="sm" class="mr-1" />
              {{ feishuStatus?.bound ? localText('重新绑定', 'Rebind') : localText('绑定', 'Bind') }}
            </button>

            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="!canOpenPanel"
              @click="openPanel"
            >
              <Icon name="externalLink" size="sm" class="mr-1" />
              {{ localText('打开面板', 'Open panel') }}
            </button>

            <label
              class="relative inline-flex items-center"
              :class="feishuStatus?.bound && !saving ? 'cursor-pointer' : 'cursor-not-allowed opacity-60'"
              :title="toggleTitle"
            >
              <input
                v-model="notifyEnabled"
                type="checkbox"
                class="sr-only peer"
                :disabled="!feishuStatus?.bound || saving"
                @change="toggleFeishu"
              />
              <span class="h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white dark:bg-gray-700 dark:after:border-gray-600"></span>
            </label>
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-900/40 dark:text-gray-300">
        <div class="flex items-start gap-2">
          <Icon name="mail" size="sm" class="mt-0.5 shrink-0 text-gray-400" />
          <p>
            {{
              localText(
                '邮件仍作为兜底渠道，收件人沿用余额通知邮箱配置。',
                'Email remains the fallback channel and uses the balance notification email settings.'
              )
            }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import { userAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { FeishuNotificationStatus } from '@/types'

const i18n = useI18n()
const locale = computed(() => {
  const current = (i18n as { locale?: { value?: unknown } }).locale?.value
  return typeof current === 'string' ? current : 'en'
})
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const binding = ref(false)
const notifyEnabled = ref(false)
const feishuStatus = ref<FeishuNotificationStatus | null>(null)

const localText = (zh: string, en: string) =>
  String(locale.value || '').startsWith('zh') ? zh : en

const canBind = computed(() => Boolean(feishuStatus.value?.app_id))
const canOpenPanel = computed(() => Boolean(feishuStatus.value?.can_open_panel && feishuStatus.value?.panel_url))

const statusLabel = computed(() => {
  if (!canBind.value) return localText('未配置', 'Not configured')
  if (!feishuStatus.value?.bound) return localText('未绑定', 'Unbound')
  return notifyEnabled.value ? localText('已开启', 'Enabled') : localText('已关闭', 'Disabled')
})

const statusBadgeClass = computed(() => {
  if (!canBind.value) return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (!feishuStatus.value?.bound) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return notifyEnabled.value
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
})

const statusDescription = computed(() => {
  if (!canBind.value) {
    return localText('系统尚未配置飞书通知 App。', 'The Feishu notification app is not configured.')
  }
  if (!feishuStatus.value?.bound) {
    return localText('绑定后会默认开启飞书通知，并保存当前通知 App 的 open_id。', 'Binding enables Feishu notifications by default and stores this app-specific open_id.')
  }
  return notifyEnabled.value
    ? localText('普通通知会优先发送到飞书。', 'Regular notifications are sent to Feishu first.')
    : localText('飞书通知已关闭，普通通知会直接走邮件。', 'Feishu notifications are off; regular notifications use email.')
})

const identitySummary = computed(() => {
  const status = feishuStatus.value
  if (!status?.bound) return []
  return [
    status.open_id_hint ? { label: 'open_id', value: status.open_id_hint } : null,
    status.union_id_hint ? { label: 'union_id', value: status.union_id_hint } : null,
    status.tenant_key ? { label: 'tenant', value: status.tenant_key } : null,
  ].filter((item): item is { label: string; value: string } => Boolean(item))
})

const toggleTitle = computed(() => {
  if (!feishuStatus.value?.bound) {
    return localText('请先绑定飞书', 'Bind Feishu first')
  }
  return notifyEnabled.value
    ? localText('关闭飞书通知', 'Disable Feishu notifications')
    : localText('开启飞书通知', 'Enable Feishu notifications')
})

async function loadNotificationSettings() {
  loading.value = true
  try {
    const settings = await userAPI.getNotificationSettings()
    feishuStatus.value = settings.feishu
    notifyEnabled.value = settings.feishu.notification_enabled ?? settings.feishu.enabled
  } catch (err: unknown) {
    appStore.showError?.(extractApiErrorMessage(err, localText('加载通知渠道失败', 'Failed to load notification channels')))
  } finally {
    loading.value = false
  }
}

async function toggleFeishu() {
  if (!feishuStatus.value?.bound) {
    notifyEnabled.value = false
    return
  }
  const nextEnabled = notifyEnabled.value
  saving.value = true
  try {
    const settings = await userAPI.updateNotificationSettings({
      feishu_notification_enabled: nextEnabled,
    })
    feishuStatus.value = settings.feishu
    notifyEnabled.value = settings.feishu.notification_enabled ?? settings.feishu.enabled
    appStore.showSuccess?.(localText('已保存通知渠道', 'Notification channel saved'))
  } catch (err: unknown) {
    notifyEnabled.value = !nextEnabled
    appStore.showError?.(extractApiErrorMessage(err, localText('保存通知渠道失败', 'Failed to save notification channel')))
  } finally {
    saving.value = false
  }
}

async function bindFeishu() {
  if (!canBind.value) return
  binding.value = true
  try {
    await userAPI.startFeishuNotifyBinding('/profile', feishuStatus.value?.bind_start_path)
  } catch (err: unknown) {
    binding.value = false
    appStore.showError?.(extractApiErrorMessage(err, localText('发起飞书绑定失败', 'Failed to start Feishu binding')))
  }
}

function openPanel() {
  const url = feishuStatus.value?.panel_url || '/feishu/panel'
  if (/^https?:\/\//i.test(url)) {
    window.open(url, '_blank', 'noopener')
    return
  }
  window.location.href = url.startsWith('/') ? url : `/${url}`
}

onMounted(() => {
  loadNotificationSettings()
})
</script>
