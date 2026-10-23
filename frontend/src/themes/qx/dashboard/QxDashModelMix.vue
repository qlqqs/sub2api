<template>
  <section class="qd-panel" data-testid="qd-mix">
    <header class="qd-panel-head">
      <div>
        <span class="qd-kicker">{{ t('qxTheme.dashboard.mix.kicker') }}</span>
        <h3>{{ t('qxTheme.dashboard.mix.title') }}</h3>
      </div>
      <span class="qd-panel-note">{{ t('qxTheme.dashboard.mix.range') }}</span>
    </header>
    <div v-if="loading && mix.rows.length === 0" class="qd-empty"><LoadingSpinner size="md" /></div>
    <div v-else-if="mix.rows.length === 0" class="qd-empty">{{ t('dashboard.noDataAvailable') }}</div>
    <ol v-else class="qd-mix">
      <li v-for="(row, i) in mix.rows" :key="row.model" class="qd-mix-row" :class="[TONES[i] ?? 'qd-tone-muted', { 'is-zero': row.share === 0 }]" data-testid="qd-mix-row">
        <span class="qd-mix-rank">{{ String(i + 1).padStart(2, '0') }}</span>
        <div class="qd-mix-body">
          <div class="qd-mix-top">
            <span class="qd-mix-name" :title="row.model">{{ row.model }}</span>
            <span class="qd-mix-share">{{ row.share }}%</span>
          </div>
          <div class="qd-bar"><i :style="{ width: `${Math.max(row.share, row.share > 0 ? 1 : 0)}%` }" /></div>
          <div class="qd-mix-meta">
            <span>{{ formatCompact(row.total_tokens) }} tokens · {{ t('qxTheme.dashboard.mix.requests', { count: formatCount(row.requests) }) }}</span>
            <span class="qd-mix-cost" :title="`${t('dashboard.standard')} ${formatOfficial(row.cost)}`">{{ formatActual(row.actual_cost) }}</span>
          </div>
        </div>
      </li>
    </ol>
    <footer v-if="mix.rest > 0" class="qd-panel-foot">{{ t('qxTheme.dashboard.mix.more', { count: mix.rest }) }}</footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelStat } from '@/types'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { buildModelMix, formatActual, formatCompact, formatCount, formatOfficial } from './metrics'

/** CUSTOM: QxAPI 主题仪表盘的模型分布：近 7 天按实际花费排前 6 个模型，细线进度条。 */
const props = defineProps<{
  models: ModelStat[]
  loading: boolean
}>()
const { t } = useI18n()

const TONES = ['qd-tone-blue', 'qd-tone-cyan', 'qd-tone-amber', 'qd-tone-green']
const mix = computed(() => buildModelMix(props.models))
</script>
