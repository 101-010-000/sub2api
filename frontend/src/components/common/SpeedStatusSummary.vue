<template>
  <div
    v-if="status?.enabled"
    :class="[
      'rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700/50',
      compact ? 'space-y-1.5 p-2' : 'space-y-3 p-3'
    ]"
  >
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <span
          :class="[
            'rounded-full px-2 py-0.5 text-[11px] font-medium',
            stateClass
          ]"
        >
          {{ stateLabel }}
        </span>
        <span class="text-xs text-gray-500 dark:text-gray-400">
          fast {{ formatPercent(status.config.fast_quota_ratio) }}
        </span>
      </div>
      <span v-if="!compact" class="text-[11px] text-gray-500 dark:text-gray-400">
        {{ billingModeLabel }}
      </span>
    </div>

    <div v-if="window" class="space-y-1">
      <div class="flex items-center gap-2">
        <span class="w-8 flex-shrink-0 text-xs font-medium text-gray-500 dark:text-gray-400">
          {{ windowLabel }}
        </span>
        <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
          <div
            class="h-full rounded-full bg-emerald-500 transition-all"
            :style="{ width: progressWidth(window.fast_used_usd, window.fast_limit_usd) }"
          ></div>
        </div>
        <span class="whitespace-nowrap text-xs tabular-nums text-gray-600 dark:text-gray-300">
          ${{ window.fast_used_usd.toFixed(2) }} / ${{ window.fast_limit_usd.toFixed(2) }}
        </span>
      </div>
      <div v-if="!compact" class="pl-10 text-[11px] text-gray-500 dark:text-gray-400">
        slow ${{ window.slow_used_usd.toFixed(2) }} / ${{ window.slow_limit_usd.toFixed(2) }}
      </div>
    </div>

    <div v-else class="text-[11px] text-amber-600 dark:text-amber-300">
      需配置分组日/周/月额度
    </div>

    <div class="text-[11px] text-gray-500 dark:text-gray-400">
      slow {{ status.config.slow_delay_min_seconds }}-{{ status.config.slow_delay_max_seconds }}s
      · {{ formatPercent(status.config.slow_reject_rate) }}
      · {{ status.slow_reject_count }}/{{ status.slow_request_count }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SubscriptionSpeedWindowStatus, UserSpeedStatus } from '@/types'

const props = withDefaults(defineProps<{
  status?: UserSpeedStatus | null
  compact?: boolean
}>(), {
  status: null,
  compact: false
})

const formatPercent = (value: number | null | undefined): string => {
  const numeric = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return `${Math.round(numeric * 100)}%`
}

const progressWidth = (used: number, limit: number): string => {
  if (!limit) return '0%'
  return `${Math.min((used / limit) * 100, 100)}%`
}

const window = computed<SubscriptionSpeedWindowStatus | null>(() => {
  const status = props.status
  if (!status) return null
  return status.daily || status.weekly || status.monthly || null
})

const windowLabel = computed(() => {
  const status = props.status
  if (!status) return ''
  if (status.daily) return '每日'
  if (status.weekly) return '每周'
  if (status.monthly) return '每月'
  return ''
})

const stateLabel = computed(() => {
  switch (props.status?.state) {
    case 'fast':
      return 'fast'
    case 'slow':
      return 'slow'
    case 'exhausted':
      return '已耗尽'
    default:
      if (props.status?.enabled && !window.value) {
        return '未配置额度'
      }
      return '未开启'
  }
})

const stateClass = computed(() => {
  switch (props.status?.state) {
    case 'fast':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'slow':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'exhausted':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
    default:
      if (props.status?.enabled && !window.value) {
        return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
      }
      return 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-300'
  }
})

const billingModeLabel = computed(() => {
  if (props.status?.billing_mode === 'subscription') return '订阅额度'
  if (props.status?.billing_mode === 'balance') return '余额额度'
  return ''
})
</script>
