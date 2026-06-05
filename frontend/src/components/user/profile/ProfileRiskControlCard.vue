<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h2 class="text-lg font-medium text-gray-900 dark:text-white">
            {{ t('profile.riskControl.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('profile.riskControl.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="loading"
          @click="loadStatus"
        >
          {{ loading ? t('common.loading') : t('common.refresh') }}
        </button>
      </div>
    </div>

    <div class="space-y-4 px-6 py-6">
      <div
        v-if="loadError"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ loadError }}
      </div>

      <div
        v-else-if="loading && !status"
        class="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-300"
      >
        {{ t('common.loading') }}
      </div>

      <div
        v-else-if="status?.banned"
        class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-900/60 dark:bg-red-900/20"
      >
        <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div class="space-y-2">
            <div class="inline-flex items-center rounded-full bg-red-100 px-2.5 py-1 text-xs font-semibold text-red-700 dark:bg-red-900/40 dark:text-red-200">
              {{ t('profile.riskControl.banned') }}
            </div>
            <p class="text-sm text-red-700 dark:text-red-200">
              {{ t('profile.riskControl.remaining', { time: formatDuration(status.remaining_seconds) }) }}
            </p>
            <p v-if="status.reason" class="text-xs text-red-600 dark:text-red-300">
              {{ t('profile.riskControl.reason', { reason: status.reason }) }}
            </p>
            <p class="text-xs text-gray-600 dark:text-gray-300">
              {{ t('profile.riskControl.attempts', { used: status.self_unban_attempts_used, max: status.self_unban_max_attempts }) }}
            </p>
            <p v-if="status.self_unban_wait_seconds > 0" class="text-xs text-yellow-700 dark:text-yellow-300">
              {{ t('profile.riskControl.waitSecond', { time: formatDuration(status.self_unban_wait_seconds) }) }}
            </p>
            <p v-else-if="!status.self_unban_available" class="text-xs text-gray-600 dark:text-gray-300">
              {{ resetText }}
            </p>
          </div>

          <button
            type="button"
            class="btn btn-primary whitespace-nowrap"
            :disabled="unbanning || !status.self_unban_available"
            @click="handleSelfUnban"
          >
            {{ unbanning ? t('profile.riskControl.unbanning') : t('profile.riskControl.selfUnban') }}
          </button>
        </div>
      </div>

      <div
        v-else
        class="rounded-xl border border-green-200 bg-green-50 p-4 text-sm text-green-700 dark:border-green-900/60 dark:bg-green-900/20 dark:text-green-200"
      >
        {{ t('profile.riskControl.notBanned') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { userAPI, type UserRiskControlBanStatus } from '@/api/user'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const status = ref<UserRiskControlBanStatus | null>(null)
const loading = ref(true)
const unbanning = ref(false)
const loadError = ref('')

const resetText = computed(() => {
  if (!status.value?.self_unban_window_reset_at) {
    return t('profile.riskControl.noAttempts')
  }
  return t('profile.riskControl.windowReset', {
    time: new Date(status.value.self_unban_window_reset_at).toLocaleString()
  })
})

function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.ceil(seconds || 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  if (hours > 0) {
    return t('profile.riskControl.durationHours', { hours, minutes })
  }
  if (minutes > 0) {
    return t('profile.riskControl.durationMinutes', { minutes, seconds: secs })
  }
  return t('profile.riskControl.durationSeconds', { seconds: secs })
}

async function loadStatus() {
  loading.value = true
  loadError.value = ''
  try {
    status.value = await userAPI.getRiskControlBanStatus()
  } catch (error: unknown) {
    loadError.value = extractApiErrorMessage(error, t('profile.riskControl.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function handleSelfUnban() {
  if (!status.value?.self_unban_available) {
    return
  }
  unbanning.value = true
  try {
    const result = await userAPI.selfUnbanRiskControl()
    if (result.unbanned) {
      appStore.showSuccess(result.message || t('profile.riskControl.unbanSuccess'))
    } else if (result.wait_seconds > 0) {
      appStore.showError(t('profile.riskControl.waitSecond', { time: formatDuration(result.wait_seconds) }))
    } else {
      appStore.showError(result.message || t('profile.riskControl.unbanUnavailable'))
    }
    await loadStatus()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('profile.riskControl.unbanFailed')))
  } finally {
    unbanning.value = false
  }
}

onMounted(loadStatus)
</script>
