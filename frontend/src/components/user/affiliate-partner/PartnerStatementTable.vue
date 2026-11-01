<template>
  <!-- CUSTOM: 历次结算单：从设立当月到当月，月份倒序 -->
  <div class="card overflow-hidden" data-test="partner-statements">
    <h3 class="px-4 pb-3 pt-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('partner.statements.title') }}</h3>
    <div class="overflow-x-auto">
      <table class="table min-w-max whitespace-nowrap">
        <thead>
          <tr>
            <th scope="col">{{ t('partner.statements.month') }}</th>
            <th scope="col">{{ t('partner.statements.status') }}</th>
            <th scope="col" class="!text-right">{{ t('partner.summary.profit') }}</th>
            <th scope="col" class="!text-right">{{ t('partner.summary.rate') }}</th>
            <th scope="col" class="!text-right">{{ t('partner.summary.share') }}</th>
            <th scope="col">{{ t('partner.statements.paidOn') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="items.length === 0">
            <td colspan="6" class="py-6 text-center text-gray-500 dark:text-dark-400">{{ t('partner.statements.empty') }}</td>
          </tr>
          <tr
            v-for="item in items"
            :key="item.month"
            :class="[active === item.month && 'bg-primary-50/60 dark:bg-primary-900/10']"
            :data-test="`partner-statement-${item.month}`"
          >
            <td>
              <RouterLink
                v-if="monthRoute"
                :to="monthRoute(item.month)"
                class="font-mono font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
              >
                {{ item.month }}
              </RouterLink>
              <button
                v-else
                type="button"
                class="font-mono font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                data-test="partner-statement-open"
                @click="emit('select', item.month)"
              >
                {{ item.month }}
              </button>
            </td>
            <td><PartnerStatusBadge :status="item.status" /></td>
            <td class="text-right"><FinanceMoney :value="item.profit" /></td>
            <td class="text-right tabular-nums">{{ formatPartnerRate(item.share_rate) }}</td>
            <td class="text-right font-medium"><FinanceMoney :value="item.share_amount" :color-negative="false" /></td>
            <td class="text-xs text-gray-500 dark:text-dark-400">{{ item.paid_on ?? '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import type { PartnerMonthTotals } from '@/api/affiliatePartner'
import { formatPartnerRate } from '@/utils/affiliatePartner'
import PartnerStatusBadge from './PartnerStatusBadge.vue'

defineProps<{
  items: PartnerMonthTotals[]
  /** 用户端：月份链接到明细页；不传时点击发出 select（管理端打开明细对话框）。 */
  monthRoute?: (month: string) => RouteLocationRaw
  /** 当前打开的月份。 */
  active?: string
}>()

const emit = defineEmits<{ select: [month: string] }>()

const { t } = useI18n()
</script>
