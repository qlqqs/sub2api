<template>
  <!-- CUSTOM: 结算单明细（用户端明细页与管理端对话框共用）：合计、各线、成本拆分、调整项、提示 -->
  <div class="space-y-4">
    <div
      v-if="detail.warnings.length > 0"
      class="flex gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
      role="status"
      data-test="partner-warnings"
    >
      <Icon name="exclamationTriangle" size="md" class="mt-0.5 shrink-0" />
      <ul class="list-disc space-y-0.5 pl-4">
        <li v-for="(warning, index) in detail.warnings" :key="index">{{ warning }}</li>
      </ul>
    </div>

    <PartnerMonthSummary :totals="detail.totals" />
    <PartnerLinesTable :lines="detail.lines" :line-route="lineRoute" />
    <PartnerCostTables :by-group="detail.cost_by_group" :by-model="detail.cost_by_model" />

    <section class="card overflow-hidden" data-test="partner-adjustments">
      <h3 class="px-4 pb-3 pt-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('partner.adjustments.title') }}</h3>
      <div class="overflow-x-auto">
        <table class="table min-w-max whitespace-nowrap">
          <thead>
            <tr>
              <th scope="col">{{ t('partner.adjustments.note') }}</th>
              <th scope="col" class="!text-right">{{ t('partner.adjustments.amount') }}</th>
              <th scope="col">{{ t('partner.adjustments.createdAt') }}</th>
              <th v-if="$slots.actions" scope="col" class="!text-right"><span class="sr-only">{{ t('common.actions') }}</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="detail.adjustments.length === 0">
              <td :colspan="$slots.actions ? 4 : 3" class="py-6 text-center text-gray-500 dark:text-dark-400">{{ t('partner.adjustments.empty') }}</td>
            </tr>
            <tr v-for="item in detail.adjustments" :key="item.id" data-test="partner-adjustment-row">
              <td class="max-w-md whitespace-normal">{{ item.note }}</td>
              <td class="text-right"><FinanceMoney :value="item.amount" /></td>
              <td class="text-xs text-gray-500 dark:text-dark-400">
                {{ formatDateTime(item.created_at) }}<template v-if="item.created_by_email"> · {{ item.created_by_email }}</template>
              </td>
              <td v-if="$slots.actions" class="text-right"><slot name="actions" :item="item" /></td>
            </tr>
          </tbody>
        </table>
      </div>
      <slot name="footer" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import Icon from '@/components/icons/Icon.vue'
import type { PartnerAdjustment, PartnerLine, PartnerStatementDetail } from '@/api/affiliatePartner'
import { formatDateTime } from '@/utils/format'
import PartnerCostTables from './PartnerCostTables.vue'
import PartnerLinesTable from './PartnerLinesTable.vue'
import PartnerMonthSummary from './PartnerMonthSummary.vue'

defineProps<{
  detail: PartnerStatementDetail
  lineRoute?: (line: PartnerLine) => RouteLocationRaw | undefined
}>()

defineSlots<{
  /** 管理端：调整项的删除按钮。 */
  actions?: (props: { item: PartnerAdjustment }) => unknown
  /** 管理端：新增调整项表单。 */
  footer?: () => unknown
}>()

const { t } = useI18n()
</script>
