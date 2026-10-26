<template>
  <!-- CUSTOM: 财务趋势：同一 ¥ 轴上的收入、账号成本、分润、其他支出、利润（明细见「利润明细」表） -->
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('admin.finance.overview.trendTitle') }}
    </h3>
    <div class="h-72">
      <Line v-if="chartData" :data="chartData" :options="chartOptions" />
      <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.finance.noData') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { isDocumentDark } from '@/composables/documentDark'
import type { FinanceSeriesRow } from '@/api/admin/finance'
import { formatCny } from '@/utils/financeFormat'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const props = defineProps<{ series: FinanceSeriesRow[] }>()

const { t } = useI18n()

type SeriesKey = 'revenue' | 'account_cost' | 'commission' | 'expenses' | 'profit'

/** 固定顺序的分类色（浅 / 深各一套，已过色盲与对比度校验）；颜色跟随指标，不随数据变化。 */
const SERIES: Array<{ key: SeriesKey; label: string; light: string; dark: string }> = [
  { key: 'revenue', label: 'admin.finance.overview.series.revenue', light: '#2a78d6', dark: '#3987e5' },
  { key: 'account_cost', label: 'admin.finance.overview.series.accountCost', light: '#eb6834', dark: '#d95926' },
  { key: 'commission', label: 'admin.finance.overview.series.commission', light: '#1baf7a', dark: '#199e70' },
  { key: 'expenses', label: 'admin.finance.overview.series.expenses', light: '#eda100', dark: '#c98500' },
  { key: 'profit', label: 'admin.finance.overview.series.profit', light: '#e87ba4', dark: '#d55181' }
]

const isDark = computed(() => isDocumentDark())

const chartData = computed(() => {
  if (!props.series.length) return null
  return {
    labels: props.series.map((row) => row.period),
    datasets: SERIES.map((s) => {
      const color = isDark.value ? s.dark : s.light
      return {
        label: t(s.label),
        // 只用于绘图位置；提示框里的数字取接口原始字符串
        data: props.series.map((row) => Number(row[s.key]) || 0),
        borderColor: color,
        backgroundColor: color,
        borderWidth: s.key === 'profit' ? 3 : 2,
        pointRadius: props.series.length > 45 ? 0 : 3,
        pointHoverRadius: 5,
        pointHitRadius: 10,
        tension: 0.25,
        fill: false
      }
    })
  }
})

/** 轴刻度：小额保留 2 位，避免 0.5 / 1 舍入成同一个刻度；toFixed 先把浮点刻度规整成十进制字符串。 */
function axisLabel(value: string | number): string {
  const n = Number(value)
  if (!Number.isFinite(n)) return ''
  const digits = Math.abs(n) < 10 ? 2 : 0
  return formatCny(n.toFixed(digits), digits)
}

const chartOptions = computed(() => {
  const text = isDark.value ? '#c3c2b7' : '#52514e'
  const grid = isDark.value ? '#2c2c2a' : '#e1e0d9'
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    scales: {
      x: { ticks: { color: text, maxRotation: 0, autoSkip: true }, grid: { display: false } },
      y: {
        ticks: { color: text, callback: axisLabel },
        grid: { color: grid }
      }
    },
    plugins: {
      legend: { position: 'top' as const, labels: { color: text, usePointStyle: true, boxHeight: 8 } },
      tooltip: {
        callbacks: {
          label: (ctx: { datasetIndex: number; dataIndex: number; dataset: { label?: string } }) => {
            const key = SERIES[ctx.datasetIndex]?.key
            const row = props.series[ctx.dataIndex]
            return `${ctx.dataset.label ?? ''}: ${key && row ? formatCny(row[key]) : ''}`
          }
        }
      }
    }
  }
})
</script>
