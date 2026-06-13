<template>
  <main class="min-h-screen bg-gray-50 px-4 py-10 dark:bg-dark-950">
    <section class="mx-auto max-w-lg">
      <div class="mb-6">
        <p class="text-sm font-medium text-primary-600 dark:text-primary-400">Touch Pie</p>
        <h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">授权 Sub2API 访问</h1>
        <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">
          确认后，本机 Touch Pie 会获取你的 Sub2API 登录凭证，用于加载 API Keys 和模型列表。
        </p>
      </div>

      <div class="card p-6">
        <div v-if="!userCode" class="space-y-4">
          <div class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300">
            缺少 user_code，请回到 Touch Pie 重新执行 /sub2api setup。
          </div>
          <router-link to="/dashboard" class="btn btn-secondary w-full">返回控制台</router-link>
        </div>

        <div v-else-if="approved" class="space-y-4 text-center">
          <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
            <span class="text-xl">✓</span>
          </div>
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">已授权</h2>
            <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">可以回到 Touch Pie 继续 setup。</p>
          </div>
        </div>

        <div v-else class="space-y-5">
          <div>
            <label class="input-label">授权码</label>
            <div class="mt-1 rounded-lg border border-gray-200 bg-gray-100 px-4 py-3 font-mono text-lg tracking-wider text-gray-900 dark:border-dark-700 dark:bg-dark-900 dark:text-white">
              {{ userCode }}
            </div>
          </div>

          <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300">
            {{ errorMessage }}
          </div>

          <div class="flex gap-3">
            <router-link to="/dashboard" class="btn btn-secondary flex-1">取消</router-link>
            <button class="btn btn-primary flex-1" :disabled="submitting" @click="approve">
              {{ submitting ? '授权中...' : '确认授权' }}
            </button>
          </div>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { approveDevice } from '@/api/touchPie'

const route = useRoute()
const submitting = ref(false)
const approved = ref(false)
const errorMessage = ref('')

const userCode = computed(() => {
  const raw = route.query.user_code
  return typeof raw === 'string' ? raw.trim() : ''
})

async function approve(): Promise<void> {
  if (!userCode.value || submitting.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    await approveDevice(userCode.value)
    approved.value = true
  } catch (error: any) {
    errorMessage.value = error?.message || '授权失败，请重新执行 /sub2api setup。'
  } finally {
    submitting.value = false
  }
}
</script>
