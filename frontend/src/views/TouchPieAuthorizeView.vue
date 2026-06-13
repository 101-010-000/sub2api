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
            <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
              已创建 {{ createdAPIKey?.provider_name || providerName }} 渠道，可以回到 Touch Pie 继续 setup。
            </p>
          </div>
        </div>

        <div v-else-if="!authStore.isAuthenticated" class="space-y-4">
          <div class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200">
            请先登录 Sub2API，再确认授权 Touch Pie。
          </div>
          <router-link :to="loginTarget" class="btn btn-primary w-full">登录后继续</router-link>
        </div>

        <div v-else class="space-y-5">
          <div>
            <label class="input-label">授权码</label>
            <div class="mt-1 rounded-lg border border-gray-200 bg-gray-100 px-4 py-3 font-mono text-lg tracking-wider text-gray-900 dark:border-dark-700 dark:bg-dark-900 dark:text-white">
              {{ userCode }}
            </div>
          </div>

          <div v-if="loadingBootstrap" class="rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300">
            正在读取可用分组...
          </div>

          <div v-else class="space-y-4">
            <div>
              <label class="input-label">TouchX 渠道分组</label>
              <select
                v-model.number="selectedGroupID"
                class="input mt-1"
                :disabled="submitting || groups.length === 0"
              >
                <option v-for="group in groups" :key="group.id" :value="group.id">
                  {{ group.name }}
                </option>
              </select>
              <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
                将创建一个 {{ providerName }} API Key，Touch Pie 会用它自动添加 Sub2API 渠道。
              </p>
            </div>

            <div
              v-if="apiKeyCandidates.length > 0"
              class="rounded-lg border px-4 py-3 text-sm"
              :style="{ borderColor: providerAccentColor, color: providerAccentColor }"
            >
              已检测到 {{ apiKeyCandidates.length }} 个 {{ providerName }} Key，本次确认会按所选分组创建新的渠道 Key。
            </div>

            <div v-if="groups.length === 0" class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200">
              当前账号没有可绑定分组，暂时无法为 Touch Pie 创建 API Key。
            </div>
          </div>

          <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300">
            {{ errorMessage }}
          </div>

          <div class="flex gap-3">
            <router-link to="/dashboard" class="btn btn-secondary flex-1">取消</router-link>
            <button class="btn btn-primary flex-1" :disabled="submitDisabled" @click="approve">
              {{ submitting ? '创建并授权中...' : '创建 Key 并授权' }}
            </button>
          </div>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { approveDevice, bootstrap, createAPIKey } from '@/api/touchPie'
import { useAuthStore } from '@/stores/auth'
import type { Group } from '@/types'
import type { TouchPieAPIKeyCandidate, TouchPieExportAPIKeyResponse } from '@/api/touchPie'

const route = useRoute()
const authStore = useAuthStore()
const loadingBootstrap = ref(false)
const submitting = ref(false)
const approved = ref(false)
const errorMessage = ref('')
const groups = ref<Group[]>([])
const apiKeyCandidates = ref<TouchPieAPIKeyCandidate[]>([])
const selectedGroupID = ref<number | null>(null)
const createdAPIKey = ref<TouchPieExportAPIKeyResponse | null>(null)
const providerName = ref('TouchX')
const providerAccentColor = ref('#8B5CF6')

const userCode = computed(() => {
  const raw = route.query.user_code
  return typeof raw === 'string' ? raw.trim() : ''
})

const loginTarget = computed(() => ({
  path: '/login',
  query: {
    redirect: route.fullPath
  }
}))

const submitDisabled = computed(() => {
  return submitting.value || loadingBootstrap.value || groups.value.length === 0 || selectedGroupID.value == null
})

function resolveErrorMessage(error: unknown): string {
  const err = error as { reason?: string; code?: string | number; message?: string } | null
  const code = String(err?.reason || err?.code || '')
  switch (code) {
    case 'TOUCH_PIE_DEVICE_EXPIRED':
      return '授权码已过期，请回到 Touch Pie 重新执行 /sub2api setup。'
    case 'TOUCH_PIE_DEVICE_CONSUMED':
      return '这次设备授权已经完成，请回到 Touch Pie 继续 setup。'
    case 'TOUCH_PIE_DEVICE_NOT_FOUND':
      return '授权码无效，请回到 Touch Pie 重新执行 /sub2api setup。'
    case 'TOUCH_PIE_AUTHORIZATION_PENDING':
      return 'Touch Pie 仍在等待确认，请点击确认授权。'
    case 'GROUP_NOT_ALLOWED':
      return '当前账号无权绑定所选分组，请重新选择。'
    default:
      return err?.message || '授权失败，请重新执行 /sub2api setup。'
  }
}

async function loadBootstrap(): Promise<void> {
  if (!userCode.value || !authStore.isAuthenticated || loadingBootstrap.value || approved.value) return
  loadingBootstrap.value = true
  errorMessage.value = ''
  try {
    const data = await bootstrap()
    groups.value = data.groups || []
    apiKeyCandidates.value = data.api_keys || []
    providerName.value = data.provider_name || providerName.value
    providerAccentColor.value = data.provider_accent_color || providerAccentColor.value
    if (selectedGroupID.value == null && groups.value.length > 0) {
      selectedGroupID.value = groups.value[0].id
    }
  } catch (error: unknown) {
    errorMessage.value = resolveErrorMessage(error)
  } finally {
    loadingBootstrap.value = false
  }
}

async function approve(): Promise<void> {
  if (!userCode.value || submitDisabled.value) return
  submitting.value = true
  errorMessage.value = ''
  try {
    createdAPIKey.value = await createAPIKey({
      name: providerName.value,
      group_id: selectedGroupID.value
    })
    await approveDevice(userCode.value)
    approved.value = true
  } catch (error: unknown) {
    errorMessage.value = resolveErrorMessage(error)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void loadBootstrap()
})

watch(
  () => authStore.isAuthenticated,
  () => {
    void loadBootstrap()
  }
)
</script>
