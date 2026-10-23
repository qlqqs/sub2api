<template>
  <AppLayout>
    <div class="qx-dash">
      <AnnouncementBanner variant="qx" />
      <div v-if="loading && !stats" class="qd-loading"><LoadingSpinner /></div>
      <QxDashOverview
        v-else-if="stats"
        :stats="stats"
        :balance="user?.balance || 0"
        :is-simple="authStore.isSimpleMode"
        :trend="trendData"
      />
      <div class="qd-row">
        <QxDashModelMix :models="modelStats" :loading="loadingModels" />
        <QxDashQuickStart />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import './dashboard.css'
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats } from '@/api/usage'
import type { ModelStat, TrendDataPoint } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AnnouncementBanner from '@/components/user/dashboard/AnnouncementBanner.vue'
import QxDashOverview from './QxDashOverview.vue'
import QxDashModelMix from './QxDashModelMix.vue'
import QxDashQuickStart from './QxDashQuickStart.vue'
import { recentDaysRange } from './metrics'

/**
 * CUSTOM: QxAPI 主题的用户仪表盘：公告横幅 + 今日概览 + 模型分布 / 快捷操作，极简线条样式。
 * 统计与上游 views/user/DashboardView.vue 同一个接口；近 7 天按天的趋势只用来画迷你折线，模型分布取同一时间范围。
 */
const RECENT_DAYS = 7

const authStore = useAuthStore()
const user = computed(() => authStore.user)

const stats = ref<UserDashboardStats | null>(null)
const loading = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const loadingModels = ref(false)

async function loadStats() {
  loading.value = true
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

async function loadTrend() {
  const range = recentDaysRange(RECENT_DAYS)
  try {
    const res = await usageAPI.getDashboardTrend({ start_date: range.startDate, end_date: range.endDate, granularity: 'day' })
    trendData.value = res.trend || []
  } catch (error) {
    console.error('Failed to load trend:', error)
  }
}

async function loadModels() {
  const range = recentDaysRange(RECENT_DAYS)
  loadingModels.value = true
  try {
    const res = await usageAPI.getDashboardModels({ start_date: range.startDate, end_date: range.endDate })
    modelStats.value = res.models || []
  } catch (error) {
    console.error('Failed to load model stats:', error)
  } finally {
    loadingModels.value = false
  }
}

onMounted(() => {
  loadStats()
  loadTrend()
  loadModels()
})
</script>
