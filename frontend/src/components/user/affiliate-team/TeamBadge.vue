<template>
  <HelpTooltip v-if="hint" :content="hint" class="!ml-0">
    <template #trigger>
      <span :class="[badgeClass, 'cursor-help']"><slot /></span>
    </template>
  </HelpTooltip>
  <span v-else :class="badgeClass"><slot /></span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

const props = withDefaults(defineProps<{
  /** primary = 二级代理；gray = 历史。 */
  tone?: 'primary' | 'gray'
  /** 悬停说明。 */
  hint?: string
}>(), {
  tone: 'primary',
  hint: ''
})

const badgeClass = computed(() => [
  'inline-flex shrink-0 items-center rounded px-1.5 py-px text-[11px] font-medium leading-4 ring-1 ring-inset',
  props.tone === 'gray'
    ? 'bg-gray-50 text-gray-500 ring-gray-200 dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-600'
    : 'bg-primary-50 text-primary-700 ring-primary-200 dark:bg-primary-900/30 dark:text-primary-300 dark:ring-primary-800'
])
</script>
