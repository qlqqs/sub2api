<template>
  <DashboardView v-if="!qxThemeConfig.enabled" />
  <QxDashboard v-else />
</template>

<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { qxThemeConfig } from '../config'
import QxDashboard from './QxDashboard.vue'

/** CUSTOM: 用户仪表盘路由入口。主题关闭时渲染上游原版 DashboardView。 */
defineOptions({ name: 'DashboardView' })

const DashboardView = defineAsyncComponent(() => import('@/views/user/DashboardView.vue'))
</script>
