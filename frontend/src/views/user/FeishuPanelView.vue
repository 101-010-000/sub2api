<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-4">
      <div class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
              {{ localText('飞书应用面板', 'Feishu app panel') }}
            </p>
            <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ user?.username || user?.email || localText('我的面板', 'My panel') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ localText('查看账户状态、订阅与通知渠道。', 'View account status, subscriptions, and notification channels.') }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="refreshPanel">
            <Icon name="refresh" size="sm" class="mr-1" :class="loading ? 'animate-spin' : ''" />
            {{ localText('刷新', 'Refresh') }}
          </button>
        </div>
      </div>

      <div v-if="loading && !loaded" class="flex justify-center py-10">
        <LoadingSpinner />
      </div>

      <template v-else>
        <div class="grid grid-cols-2 gap-3">
          <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ localText('余额', 'Balance') }}</p>
            <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
              ${{ formatMoney(user?.balance || 0) }}
            </p>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ localText('并发', 'Concurrency') }}</p>
            <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ user?.concurrency || 0 }}
            </p>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ localText('活跃订阅', 'Active subscriptions') }}</p>
            <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ activeSubscriptions.length }}
            </p>
          </div>
          <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ localText('API Key', 'API keys') }}</p>
            <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ apiKeyCount }}
            </p>
          </div>
        </div>

        <div class="rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="font-medium text-gray-900 dark:text-white">{{ localText('飞书通知', 'Feishu notifications') }}</h2>
          </div>
          <div class="space-y-4 p-5">
            <div class="flex items-start justify-between gap-4">
              <div>
                <p class="font-medium text-gray-900 dark:text-white">{{ notificationTitle }}</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ notificationDescription }}</p>
              </div>
              <span class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium" :class="notificationBadgeClass">
                {{ notificationBadge }}
              </span>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="!notificationStatus?.app_id || binding"
                @click="bindFeishu"
              >
                <Icon v-if="binding" name="refresh" size="sm" class="mr-1 animate-spin" />
                <Icon v-else name="link" size="sm" class="mr-1" />
                {{ notificationStatus?.bound ? localText('重新绑定', 'Rebind') : localText('绑定飞书', 'Bind Feishu') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="!notificationStatus?.bound || savingNotification"
                @click="toggleNotification"
              >
                <Icon name="bell" size="sm" class="mr-1" />
                {{ notificationEnabled ? localText('关闭通知', 'Disable') : localText('开启通知', 'Enable') }}
              </button>
            </div>
          </div>
        </div>

        <div class="rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <h2 class="font-medium text-gray-900 dark:text-white">{{ localText('订阅', 'Subscriptions') }}</h2>
          </div>
          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-if="activeSubscriptions.length === 0" class="px-5 py-6 text-sm text-gray-500 dark:text-gray-400">
              {{ localText('当前没有活跃订阅。', 'No active subscriptions.') }}
            </div>
            <div
              v-for="subscription in activeSubscriptions.slice(0, 3)"
              :key="subscription.id"
              class="flex items-center justify-between gap-4 px-5 py-4"
            >
              <div class="min-w-0">
                <p class="truncate font-medium text-gray-900 dark:text-white">
                  {{ subscription.group?.name || `#${subscription.group_id}` }}
                </p>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ formatSubscriptionExpiry(subscription.expires_at) }}
                </p>
              </div>
              <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">
                {{ subscription.status }}
              </span>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <router-link to="/dashboard" class="btn btn-primary justify-center">
            <Icon name="grid" size="sm" class="mr-1" />
            {{ localText('仪表盘', 'Dashboard') }}
          </router-link>
          <router-link to="/profile" class="btn btn-secondary justify-center">
            <Icon name="cog" size="sm" class="mr-1" />
            {{ localText('通知设置', 'Settings') }}
          </router-link>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'
import { keysAPI, userAPI } from '@/api'
import subscriptionsAPI from '@/api/subscriptions'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { FeishuNotificationStatus, UserSubscription } from '@/types'

const { locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(false)
const loaded = ref(false)
const binding = ref(false)
const savingNotification = ref(false)
const notificationStatus = ref<FeishuNotificationStatus | null>(null)
const activeSubscriptions = ref<UserSubscription[]>([])
const apiKeyCount = ref(0)

const user = computed(() => authStore.user)
const localText = (zh: string, en: string) =>
  String(locale.value || '').startsWith('zh') ? zh : en

const notificationEnabled = computed(() =>
  Boolean(notificationStatus.value?.notification_enabled ?? notificationStatus.value?.enabled)
)

const notificationBadge = computed(() => {
  if (!notificationStatus.value?.app_id) return localText('未配置', 'Not configured')
  if (!notificationStatus.value?.bound) return localText('未绑定', 'Unbound')
  return notificationEnabled.value ? localText('已开启', 'Enabled') : localText('已关闭', 'Disabled')
})

const notificationBadgeClass = computed(() => {
  if (!notificationStatus.value?.app_id) return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
  if (!notificationStatus.value?.bound) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return notificationEnabled.value
    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
})

const notificationTitle = computed(() => {
  if (!notificationStatus.value?.app_id) return localText('系统未启用飞书通知', 'Feishu notifications are not configured')
  if (!notificationStatus.value?.bound) return localText('绑定后接收普通通知', 'Bind Feishu to receive regular notifications')
  return localText('当前通知身份', 'Current notification identity')
})

const notificationDescription = computed(() => {
  const status = notificationStatus.value
  if (!status?.app_id) return localText('请联系管理员配置飞书通知 App。', 'Ask an admin to configure the Feishu notification app.')
  if (!status.bound) return localText('open_id 按通知 App 单独保存，不会复用登录 App 身份。', 'open_id is stored per notification app and is not reused from the login app.')
  const identity = [status.open_id_hint, status.union_id_hint].filter(Boolean).join(' / ')
  return identity || localText('已绑定飞书通知身份。', 'Feishu notification identity is bound.')
})

function formatMoney(value: number): string {
  return Number(value || 0).toFixed(2)
}

function formatSubscriptionExpiry(expiresAt?: string | null): string {
  if (!expiresAt) return localText('长期有效', 'No expiration')
  const expiresTime = new Date(expiresAt).getTime()
  if (!Number.isFinite(expiresTime)) return expiresAt
  const days = Math.ceil((expiresTime - Date.now()) / 86400000)
  if (days < 0) return localText('已到期', 'Expired')
  return localText(`${days} 天后到期`, `Expires in ${days} days`)
}

async function refreshPanel() {
  loading.value = true
  try {
    const [profile, notificationSettings, subscriptions, keys] = await Promise.all([
      authStore.refreshUser(),
      userAPI.getNotificationSettings(),
      subscriptionsAPI.getActiveSubscriptions().catch(() => [] as UserSubscription[]),
      keysAPI.list(1, 1).catch(() => ({ total: 0 })),
    ])
    authStore.user = profile
    notificationStatus.value = notificationSettings.feishu
    activeSubscriptions.value = subscriptions
    apiKeyCount.value = Number(keys.total || 0)
    loaded.value = true
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, localText('加载飞书面板失败', 'Failed to load Feishu panel')))
  } finally {
    loading.value = false
  }
}

async function bindFeishu() {
  if (!notificationStatus.value?.app_id) return
  binding.value = true
  try {
    await userAPI.startFeishuNotifyBinding('/feishu/panel', notificationStatus.value.bind_start_path)
  } catch (err: unknown) {
    binding.value = false
    appStore.showError(extractApiErrorMessage(err, localText('发起飞书绑定失败', 'Failed to start Feishu binding')))
  }
}

async function toggleNotification() {
  const status = notificationStatus.value
  if (!status?.bound) return
  savingNotification.value = true
  try {
    const updated = await userAPI.updateNotificationSettings({
      feishu_notification_enabled: !notificationEnabled.value,
    })
    notificationStatus.value = updated.feishu
    appStore.showSuccess(localText('通知设置已保存', 'Notification setting saved'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, localText('保存通知设置失败', 'Failed to save notification setting')))
  } finally {
    savingNotification.value = false
  }
}

onMounted(() => {
  refreshPanel()
})
</script>
