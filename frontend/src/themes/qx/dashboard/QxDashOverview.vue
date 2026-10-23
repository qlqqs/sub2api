<template>
  <!-- 极简线条：一个细边框区块，表头、指标、汇总之间只用 1px 细线分隔，没有底色和阴影 -->
  <section class="qd-today" data-testid="qd-overview">
    <header class="qd-today-head">
      <div>
        <span class="qd-kicker">{{ t('qxTheme.dashboard.today.kicker') }}</span>
        <h3>{{ t('qxTheme.dashboard.today.title') }}</h3>
      </div>
      <div class="qd-live">
        <i class="qd-live-dot" aria-hidden="true" />
        <span>{{ t('qxTheme.dashboard.today.last5m') }}</span>
        <span class="qd-live-val">RPM <b>{{ formatCompact(stats.rpm) }}</b></span>
        <span class="qd-live-val">TPM <b>{{ formatCompact(stats.tpm) }}</b></span>
      </div>
    </header>

    <div class="qd-kpis" :class="{ 'qd-kpis-3': isSimple }">
      <div v-if="!isSimple" class="qd-kpi qd-kpi-balance" data-testid="qd-balance">
        <div class="qd-kpi-label">{{ t('dashboard.balance') }}</div>
        <div class="qd-kpi-val">{{ formatBalance(balance, { grouping: true }) }}</div>
        <router-link to="/purchase" class="qd-kpi-act">{{ t('qxTheme.dashboard.today.recharge') }} →</router-link>
      </div>

      <div class="qd-kpi">
        <div class="qd-kpi-label">{{ t('dashboard.todayCost') }}</div>
        <div class="qd-kpi-val" :title="t('dashboard.actualSpend')" data-testid="qd-today-cost">{{ formatActual(stats.today_actual_cost) }}</div>
        <div class="qd-kpi-sub">{{ t('qxTheme.dashboard.today.official') }} <b>{{ formatOfficial(stats.today_cost) }}</b></div>
        <svg v-if="costSpark" class="qd-spark" viewBox="0 0 84 28" aria-hidden="true">
          <polyline :points="costSpark" fill="none" stroke="var(--qx-blue)" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round" />
        </svg>
      </div>

      <div class="qd-kpi">
        <div class="qd-kpi-label">{{ t('dashboard.todayRequests') }}</div>
        <div class="qd-kpi-val">{{ formatCount(stats.today_requests) }}</div>
        <div class="qd-kpi-sub">{{ t('qxTheme.dashboard.today.total') }} <b>{{ formatCount(stats.total_requests) }}</b></div>
        <svg v-if="requestSpark" class="qd-spark" viewBox="0 0 84 28" aria-hidden="true">
          <polyline :points="requestSpark" fill="none" stroke="var(--qx-cyan)" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round" />
        </svg>
      </div>

      <div class="qd-kpi">
        <div class="qd-kpi-label">{{ t('dashboard.todayTokens') }}</div>
        <div class="qd-kpi-val">{{ formatCompact(stats.today_tokens) }}</div>
        <div class="qd-kpi-sub">
          {{ t('dashboard.input') }} <b>{{ formatCompact(stats.today_input_tokens) }}</b> ·
          {{ t('dashboard.output') }} <b>{{ formatCompact(stats.today_output_tokens) }}</b> ·
          {{ t('dashboard.cache') }} <b>{{ formatCompact(stats.today_cache_creation_tokens + stats.today_cache_read_tokens) }}</b>
        </div>
      </div>
    </div>

    <footer class="qd-stats">
      <span class="qd-stats-item">{{ t('qxTheme.dashboard.today.keys') }} <b>{{ stats.active_api_keys }} / {{ stats.total_api_keys }}</b> {{ t('qxTheme.dashboard.today.keysActive') }}</span>
      <span class="qd-stats-item" :title="t('dashboard.costSummary', { actual: formatActual(stats.total_actual_cost), official: formatOfficial(stats.total_cost) })">
        {{ t('qxTheme.dashboard.today.totalCost') }} <b>{{ formatActual(stats.total_actual_cost) }}</b>
      </span>
      <span class="qd-stats-item">{{ t('qxTheme.dashboard.today.totalTokens') }} <b>{{ formatCompact(stats.total_tokens) }}</b></span>
      <span class="qd-stats-item">{{ t('qxTheme.dashboard.today.avgResponse') }} <b>{{ formatDuration(stats.average_duration_ms) }}</b></span>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserDashboardStats } from '@/api/usage'
import type { TrendDataPoint } from '@/types'
import { formatBalance } from '@/utils/balanceCurrency'
import { formatActual, formatCompact, formatCount, formatDuration, formatOfficial, sparklinePoints } from './metrics'

const props = defineProps<{
  stats: UserDashboardStats
  balance: number
  isSimple: boolean
  trend: TrendDataPoint[]
}>()
const { t } = useI18n()

const costSpark = computed(() => sparklinePoints(props.trend.map((p) => p.actual_cost || 0)))
const requestSpark = computed(() => sparklinePoints(props.trend.map((p) => p.requests || 0)))
</script>
