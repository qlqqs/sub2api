<template>
  <!-- 团队消费根页：左侧是「邀请 | 团队消费」分区 Tab，日期与刷新放进它的右侧插槽；下钻页左侧是面包屑；
       没有面包屑（管理端汇总页，标题已在顶部栏）时日期与刷新靠右。 -->
  <component
    :is="sectionTabs ? AffiliateSectionTabs : 'div'"
    :class="sectionTabs ? undefined : ['flex flex-wrap items-center gap-3', crumbs.length > 0 ? 'justify-between' : 'justify-end']"
  >
    <nav v-if="!sectionTabs && crumbs.length > 0" :aria-label="t('affiliate.team.breadcrumb')" class="min-w-0" data-test="team-breadcrumb">
      <ol class="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-lg font-semibold">
        <li
          v-for="(crumb, index) in crumbs"
          :key="index"
          class="flex min-w-0 items-center gap-x-1.5"
        >
          <span v-if="index > 0" class="text-gray-300 dark:text-dark-600" aria-hidden="true">/</span>
          <span
            v-if="crumb.loading"
            class="inline-block h-5 w-20 animate-pulse rounded bg-gray-200 dark:bg-dark-700"
            aria-hidden="true"
          ></span>
          <RouterLink
            v-else-if="crumb.to"
            :to="crumb.to"
            class="truncate text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400"
          >
            {{ crumb.label }}
          </RouterLink>
          <span v-else class="truncate text-gray-900 dark:text-white" aria-current="page">{{ crumb.label }}</span>
        </li>
      </ol>
    </nav>

    <div class="flex items-center gap-2">
      <DateRangePicker
        :start-date="startDate"
        :end-date="endDate"
        @change="(range) => emit('rangeChange', { start: range.startDate, end: range.endDate })"
      />
      <button
        type="button"
        class="btn btn-secondary rounded-lg p-2.5"
        :aria-label="t('common.refresh')"
        :title="t('common.refresh')"
        :disabled="refreshing"
        data-test="team-refresh"
        @click="emit('refresh')"
      >
        <Icon name="refresh" size="sm" :class="refreshing && 'animate-spin'" />
      </button>
    </div>
  </component>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Icon from '@/components/icons/Icon.vue'
import AffiliateSectionTabs from '@/components/user/AffiliateSectionTabs.vue'

export interface TeamCrumb {
  label: string
  /** 有 to 的段可点击；最后一段（当前页）没有 to。 */
  to?: RouteLocationRaw
  /** 名称尚未加载时显示占位条。 */
  loading?: boolean
}

defineProps<{
  crumbs: TeamCrumb[]
  startDate: string
  endDate: string
  refreshing?: boolean
  /** 团队消费根页显示分区 Tab，代替面包屑。 */
  sectionTabs?: boolean
}>()

const emit = defineEmits<{
  rangeChange: [range: { start: string; end: string }]
  refresh: []
}>()

const { t } = useI18n()
</script>
