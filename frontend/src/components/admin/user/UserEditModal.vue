<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.editUser')"
    width="normal"
    @close="$emit('close')"
  >
    <form v-if="user" id="edit-user-form" @submit.prevent="handleUpdateUser" class="space-y-5">
      <div>
        <label class="input-label">{{ t('admin.users.email') }}</label>
        <input v-model="form.email" type="email" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.password') }}</label>
        <div class="flex gap-2">
          <div class="relative flex-1">
            <input v-model="form.password" type="text" class="input pr-10" :placeholder="t('admin.users.enterNewPassword')" />
            <button v-if="form.password" type="button" @click="copyPassword" class="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1 transition-colors hover:bg-gray-100 dark:hover:bg-dark-700" :class="passwordCopied ? 'text-green-500' : 'text-gray-400'">
              <svg v-if="passwordCopied" class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
              <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" /></svg>
            </button>
          </div>
          <button type="button" @click="generatePassword" class="btn btn-secondary px-3">
            <Icon name="refresh" size="md" />
          </button>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.username') }}</label>
        <input v-model="form.username" type="text" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.notes') }}</label>
        <textarea v-model="form.notes" rows="3" class="input"></textarea>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.columns.concurrency') }}</label>
        <input v-model.number="form.concurrency" type="number" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.form.rpmLimit') }}</label>
        <input
          v-model.number="form.rpm_limit"
          type="number"
          min="0"
          step="1"
          class="input"
          :placeholder="t('admin.users.form.rpmLimitPlaceholder')"
        />
        <p class="input-hint">{{ t('admin.users.form.rpmLimitHint') }}</p>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.form.apiKeyMaxActiveIPs') }}</label>
        <input
          v-model.number="form.api_key_max_active_ips"
          type="number"
          min="0"
          step="1"
          class="input"
          :placeholder="t('admin.users.form.apiKeyMaxActiveIPsPlaceholder')"
        />
        <p class="input-hint">{{ t('admin.users.form.apiKeyMaxActiveIPsHint') }}</p>
        <label class="mt-3 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="form.api_key_max_active_ips_visible" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          <span>{{ t('admin.users.form.apiKeyMaxActiveIPsVisible') }}</span>
        </label>
      </div>
      <div v-if="isSuperAdmin">
        <label class="input-label">后台权限</label>
        <div class="grid gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
          <div
            v-for="module in adminPermissionModules"
            :key="module.key"
            class="grid grid-cols-[1fr_auto_auto] items-center gap-3 text-sm"
          >
            <span class="min-w-0 truncate text-gray-700 dark:text-gray-300">{{ module.label }}</span>
            <label class="inline-flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="hasPermission(module.read)"
                @change="togglePermission(module.read, eventChecked($event))"
              />
              <span>查看</span>
            </label>
            <label class="inline-flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="hasPermission(module.write)"
                @change="togglePermission(module.write, eventChecked($event))"
              />
              <span>编辑</span>
            </label>
          </div>
        </div>
      </div>
      <UserAttributeForm v-model="form.customAttributes" :user-id="user?.id" />
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" type="button" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="edit-user-form" :disabled="submitting" class="btn btn-primary">
          {{ submitting ? t('admin.users.updating') : t('common.update') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserAttributeValuesMap } from '@/types'
import { adminPermissionModules, normalizeAdminPermissions } from '@/utils/adminPermissions'
import BaseDialog from '@/components/common/BaseDialog.vue'
import UserAttributeForm from '@/components/user/UserAttributeForm.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean, user: AdminUser | null }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n(); const appStore = useAppStore(); const authStore = useAuthStore(); const { copyToClipboard } = useClipboard()

const submitting = ref(false); const passwordCopied = ref(false)
const form = reactive({ email: '', password: '', username: '', notes: '', concurrency: 1, rpm_limit: 0, api_key_max_active_ips: 0, api_key_max_active_ips_visible: false, admin_permissions: [] as string[], customAttributes: {} as UserAttributeValuesMap })
const isSuperAdmin = computed(() => authStore.isSuperAdmin)

watch(() => props.user, (u) => {
  if (u) {
    Object.assign(form, { email: u.email, password: '', username: u.username || '', notes: u.notes || '', concurrency: u.concurrency, rpm_limit: u.rpm_limit ?? 0, api_key_max_active_ips: u.api_key_max_active_ips ?? 0, api_key_max_active_ips_visible: u.api_key_max_active_ips_visible ?? false, admin_permissions: normalizeAdminPermissions(u.admin_permissions), customAttributes: {} })
    passwordCopied.value = false
  }
}, { immediate: true })

const hasPermission = (permission: string) => form.admin_permissions.includes(permission)

const eventChecked = (event: Event) => {
  return (event.target as HTMLInputElement | null)?.checked === true
}

const togglePermission = (permission: string, enabled: boolean) => {
  const next = new Set(form.admin_permissions)
  const module = adminPermissionModules.find((item) => item.read === permission || item.write === permission)
  if (enabled) {
    next.add(permission)
    if (module && permission === module.write) next.add(module.read)
  } else {
    next.delete(permission)
    if (module && permission === module.read) next.delete(module.write)
  }
  form.admin_permissions = normalizeAdminPermissions([...next])
}

const generatePassword = () => {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%^&*'
  let p = ''; for (let i = 0; i < 16; i++) p += chars.charAt(Math.floor(Math.random() * chars.length))
  form.password = p
}
const copyPassword = async () => {
  if (form.password && await copyToClipboard(form.password, t('admin.users.passwordCopied'))) {
    passwordCopied.value = true; setTimeout(() => passwordCopied.value = false, 2000)
  }
}
const handleUpdateUser = async () => {
  if (!props.user) return
  if (!form.email.trim()) {
    appStore.showError(t('admin.users.emailRequired'))
    return
  }
  if (form.concurrency < 1) {
    appStore.showError(t('admin.users.concurrencyMin'))
    return
  }
  if (form.api_key_max_active_ips < 0) {
    appStore.showError(t('admin.users.form.apiKeyMaxActiveIPsInvalid'))
    return
  }
  submitting.value = true
  try {
    const data: any = { email: form.email, username: form.username, notes: form.notes, concurrency: form.concurrency, rpm_limit: form.rpm_limit, api_key_max_active_ips: Math.floor(form.api_key_max_active_ips || 0), api_key_max_active_ips_visible: form.api_key_max_active_ips_visible }
    if (isSuperAdmin.value) data.admin_permissions = normalizeAdminPermissions(form.admin_permissions)
    if (form.password.trim()) data.password = form.password.trim()
    await adminAPI.users.update(props.user.id, data)
    if (Object.keys(form.customAttributes).length > 0) await adminAPI.userAttributes.updateUserAttributeValues(props.user.id, form.customAttributes)
    appStore.showSuccess(t('admin.users.userUpdated'))
    emit('success'); emit('close')
  } catch (e: any) {
    appStore.showError(e.response?.data?.detail || t('admin.users.failedToUpdate'))
  } finally { submitting.value = false }
}
</script>
