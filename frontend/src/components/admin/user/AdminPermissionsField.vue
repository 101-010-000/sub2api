<template>
  <div>
    <label class="input-label">{{ label }}</label>
    <div class="grid gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
      <div
        v-for="module in adminPermissionModules"
        :key="module.key"
        :data-test="`admin-permission-row-${module.key}`"
        class="grid grid-cols-[1fr_auto_auto] items-center gap-3 text-sm"
      >
        <span class="min-w-0 truncate text-gray-700 dark:text-gray-300">{{ module.label }}</span>
        <label class="inline-flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
          <input
            type="checkbox"
            :data-test="`admin-permission-${module.key}-read`"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :checked="hasPermission(module.read)"
            @change="togglePermission(module.read, eventChecked($event))"
          />
          <span>{{ readLabel }}</span>
        </label>
        <label class="inline-flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
          <input
            type="checkbox"
            :data-test="`admin-permission-${module.key}-write`"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :checked="hasPermission(module.write)"
            @change="togglePermission(module.write, eventChecked($event))"
          />
          <span>{{ writeLabel }}</span>
        </label>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { adminPermissionModules, normalizeAdminPermissions } from '@/utils/adminPermissions'

withDefaults(defineProps<{
  label?: string
  readLabel?: string
  writeLabel?: string
}>(), {
  label: '后台权限',
  readLabel: '查看',
  writeLabel: '编辑',
})

const model = defineModel<string[]>({ required: true })

const hasPermission = (permission: string) => model.value.includes(permission)

const eventChecked = (event: Event) => {
  return (event.target as HTMLInputElement | null)?.checked === true
}

const togglePermission = (permission: string, enabled: boolean) => {
  const next = new Set(model.value)
  const module = adminPermissionModules.find((item) => item.read === permission || item.write === permission)
  if (enabled) {
    next.add(permission)
    if (module && permission === module.write) next.add(module.read)
  } else {
    next.delete(permission)
    if (module && permission === module.read) next.delete(module.write)
  }
  model.value = normalizeAdminPermissions([...next])
}
</script>
