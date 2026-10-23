<template>
  <span class="relative block min-w-[7.5rem] overflow-hidden rounded px-2 py-0.5 text-right">
    <span
      v-if="share >= MIN_VISIBLE_SHARE"
      class="absolute inset-y-0 left-0 rounded bg-primary-500/10 dark:bg-primary-400/15"
      :style="{ width: teamShareWidth(share) }"
      aria-hidden="true"
    ></span>
    <span :class="['relative tabular-nums', muted ? 'text-gray-400 dark:text-dark-500' : 'font-medium']">{{ value }}</span>
  </span>
</template>

<script setup lang="ts">
import { teamShareWidth } from '@/utils/affiliateTeam'

// 不足 1% 的底条只有一两个像素宽，看起来像渲染瑕疵，直接不画。
const MIN_VISIBLE_SHARE = 0.01

/** 金额 + 单元格内淡色底条（占当前范围团队消费的比例），不另起一列。 */
defineProps<{
  value: string
  share: number
  muted?: boolean
}>()
</script>
