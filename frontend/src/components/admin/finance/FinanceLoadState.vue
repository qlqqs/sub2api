<template>
  <!-- CUSTOM: 财务标签页的加载 / 失败状态；有旧数据时加载中仍显示旧数据，避免布局跳动 -->
  <div
    v-if="error"
    class="flex flex-wrap items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
    role="alert"
    data-testid="finance-load-error"
  >
    <span>{{ t('admin.finance.loadFailed', { error }) }}</span>
    <button type="button" class="font-medium underline" @click="emit('retry')">{{ t('admin.finance.retry') }}</button>
  </div>
  <div v-else-if="loading && !hasData" class="flex items-center justify-center py-12">
    <LoadingSpinner />
  </div>
  <div v-else-if="hasData" :class="{ 'pointer-events-none opacity-60 transition-opacity': loading }">
    <slot />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

defineProps<{ loading: boolean; error: string; hasData: boolean }>()
const emit = defineEmits<{ retry: [] }>()

const { t } = useI18n()
</script>
