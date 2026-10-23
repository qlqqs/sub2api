<template>
  <div class="card flex flex-col p-4" data-test="team-trend">
    <div class="mb-3 flex items-center gap-4 text-xs text-gray-600 dark:text-gray-300">
      <span class="inline-flex items-center gap-1.5">
        <span class="h-2.5 w-2.5 rounded-sm" :style="{ backgroundColor: COST_COLOR }" aria-hidden="true"></span>
        {{ t('affiliate.team.chart.cost') }}
      </span>
      <span class="inline-flex items-center gap-1.5">
        <span class="h-0.5 w-3.5 rounded-full" :style="{ backgroundColor: EARNINGS_COLOR }" aria-hidden="true"></span>
        {{ label('affiliate.team.chart.earnings') }}
      </span>
    </div>
    <div class="relative h-56">
      <div v-if="loading" class="h-full w-full animate-pulse rounded-lg bg-gray-100 dark:bg-dark-700/60"></div>
      <div
        v-else-if="isEmpty"
        class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-dark-400"
        data-test="team-trend-empty"
      >
        {{ t('affiliate.team.chart.empty') }}
      </div>
      <Bar v-else :data="barData" :options="chartOptions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  BarController,
  BarElement,
  CategoryScale,
  LinearScale,
  LineController,
  LineElement,
  PointElement,
  Tooltip,
  type ChartData,
  type ChartOptions,
  type TooltipItem
} from 'chart.js'
import { Bar } from 'vue-chartjs'
import type { AffiliateTeamTrendPoint } from '@/types'
import { formatBalance } from '@/utils/balanceCurrency'
import { formatTeamMoney } from '@/utils/affiliateTeam'
import { useTeamLabel } from './teamContext'

// 柱状图叠加一条收益折线：只注册用到的部件。
ChartJS.register(BarController, BarElement, LineController, LineElement, PointElement, CategoryScale, LinearScale, Tooltip)

// teal-600 / amber-600：明暗两种卡片底色下都通过色觉与对比度校验。
const COST_COLOR = '#0d9488'
const EARNINGS_COLOR = '#d97706'

const props = defineProps<{
  trend: AffiliateTeamTrendPoint[]
  loading?: boolean
}>()

const { t } = useI18n()
const label = useTeamLabel()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const axisColors = computed(() => ({
  text: isDarkMode.value ? '#9ca3af' : '#6b7280',
  grid: isDarkMode.value ? '#334155' : '#f3f4f6'
}))

const isEmpty = computed(() => props.trend.every((point) => point.cost === 0 && point.earnings === 0))
const singleDay = computed(() => props.trend.length === 1)

const chartData = computed<ChartData<'bar' | 'line', number[], string>>(() => ({
  labels: props.trend.map((point) => point.date.slice(5)),
  datasets: [
    {
      type: 'bar' as const,
      label: t('affiliate.team.chart.cost'),
      data: props.trend.map((point) => point.cost),
      backgroundColor: COST_COLOR,
      borderRadius: 4,
      borderSkipped: 'start' as const,
      maxBarThickness: 24,
      order: 1
    },
    {
      type: 'line' as const,
      label: label('affiliate.team.chart.earnings'),
      data: props.trend.map((point) => point.earnings),
      borderColor: EARNINGS_COLOR,
      backgroundColor: EARNINGS_COLOR,
      borderWidth: 2,
      tension: 0.3,
      // 只有一天时折线没有线段，用圆点表示。
      pointRadius: singleDay.value ? 4 : 0,
      pointHoverRadius: 4,
      order: 0
    }
  ]
}))

// vue-chartjs 的 Bar 只接受 'bar' 数据集类型；混合图（含 type: 'line' 数据集）由 Chart.js 运行时支持。
const barData = computed(() => chartData.value as unknown as ChartData<'bar', number[], string>)

const chartOptions = computed<ChartOptions<'bar'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: false,
  interaction: { mode: 'index', intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      itemSort: (a: TooltipItem<'bar'>, b: TooltipItem<'bar'>) => a.datasetIndex - b.datasetIndex,
      callbacks: {
        title: (items: TooltipItem<'bar'>[]) => props.trend[items[0]?.dataIndex ?? 0]?.date ?? '',
        label: (item: TooltipItem<'bar'>) => `${item.dataset.label}: ${formatTeamMoney(Number(item.raw))}`
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: axisColors.value.text, font: { size: 10 }, maxRotation: 0, autoSkipPadding: 8 }
    },
    y: {
      beginAtZero: true,
      border: { display: false },
      grid: { color: axisColors.value.grid },
      ticks: {
        color: axisColors.value.text,
        font: { size: 10 },
        maxTicksLimit: 5,
        callback: (value: string | number) => formatBalance(Number(value), { minDecimals: 0, maxDecimals: 2, grouping: true })
      }
    }
  }
}))
</script>
