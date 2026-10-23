<template>
  <div class="card flex flex-col p-4" data-test="team-models">
    <h3 class="mb-3 text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('affiliate.team.models.title') }}</h3>
    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="h-4 animate-pulse rounded bg-gray-100 dark:bg-dark-700/60"></div>
    </div>
    <div
      v-else-if="models.length === 0"
      class="flex flex-1 items-center justify-center py-8 text-sm text-gray-500 dark:text-dark-400"
    >
      {{ t('affiliate.team.chart.empty') }}
    </div>
    <ul v-else class="space-y-0.5">
      <li v-for="item in models" :key="item.model || '__other__'">
        <component
          :is="isClickable(item) ? 'button' : 'div'"
          :type="isClickable(item) ? 'button' : undefined"
          :aria-pressed="isClickable(item) ? item.model === activeModel : undefined"
          :title="`${item.model || t('affiliate.team.models.other')} · ${formatTeamMoney(item.cost)}`"
          :class="[
            'grid w-full grid-cols-[minmax(0,1fr)_2.75rem_4.5rem] items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm',
            isClickable(item) && 'transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/50',
            item.model !== '' && item.model === activeModel && 'bg-primary-50 dark:bg-primary-900/20'
          ]"
          data-test="team-model-row"
          @click="isClickable(item) && emit('select', item.model)"
        >
          <span
            :class="[
              'truncate',
              item.model ? 'text-gray-800 dark:text-gray-100' : 'text-gray-500 dark:text-dark-400',
              item.model !== '' && item.model === activeModel && 'font-medium text-primary-700 dark:text-primary-300'
            ]"
          >
            {{ item.model || t('affiliate.team.models.other') }}
          </span>
          <span class="text-right text-xs tabular-nums text-gray-500 dark:text-dark-400">{{ formatTeamShare(item.share) }}</span>
          <span class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700" aria-hidden="true">
            <span
              class="block h-full rounded-full bg-primary-500 dark:bg-primary-400"
              :style="{ width: teamShareWidth(item.share) }"
            ></span>
          </span>
        </component>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { AffiliateTeamModelShare } from '@/types'
import { formatTeamMoney, formatTeamShare, teamShareWidth } from '@/utils/affiliateTeam'

const props = withDefaults(defineProps<{
  models: AffiliateTeamModelShare[]
  loading?: boolean
  /** 只在客户详情开启：点击设置请求明细的模型筛选。 */
  clickable?: boolean
  activeModel?: string
}>(), {
  loading: false,
  clickable: false,
  activeModel: ''
})

const emit = defineEmits<{
  select: [model: string]
}>()

const { t } = useI18n()

// 「其他」（model 为空串）不能作为筛选条件。
function isClickable(item: AffiliateTeamModelShare): boolean {
  return props.clickable && item.model !== ''
}
</script>
