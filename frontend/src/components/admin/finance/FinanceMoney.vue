<template>
  <span
    class="tabular-nums"
    :class="negative && colorNegative ? 'text-red-600 dark:text-red-400' : undefined"
    :data-negative="negative ? 'true' : undefined"
  >{{ text }}</span>
</template>

<script setup lang="ts">
// CUSTOM: 财务金额展示：字符串只格式化不运算，负数标红（不只靠颜色：保留负号）。
// 模板根节点前不放注释，保持单根节点，父组件传入的 class 能落到 span 上。
import { computed } from 'vue'
import { formatAmount, formatCny, isNegativeAmount } from '@/utils/financeFormat'

const props = withDefaults(
  defineProps<{
    value: string | number | null | undefined
    /** 小数位：金额 2 位，单价 4 位。 */
    digits?: number
    /** 是否带 ¥ 前缀。 */
    currency?: boolean
    /** 负数是否标红（利润、差额等）。 */
    colorNegative?: boolean
  }>(),
  { digits: 2, currency: true, colorNegative: true }
)

const negative = computed(() => isNegativeAmount(props.value, props.digits))
const text = computed(() =>
  props.currency ? formatCny(props.value, props.digits) : formatAmount(props.value, props.digits)
)
</script>
