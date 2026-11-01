<template>
  <!-- CUSTOM: 合伙人「利润分成」：本月预估、按渠道经理拆分、历次结算单 -->
  <AppLayout>
    <div class="space-y-4">
      <AffiliateSectionTabs partner />

      <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
        <div v-if="data" class="space-y-4">
          <section class="card p-5" data-test="partner-overview">
            <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ data.overview.region_name }}</h2>
              <span class="text-sm text-gray-600 dark:text-gray-300" data-test="partner-rate">
                {{ t('partner.rate', { rate: formatPartnerRate(data.overview.share_rate) }) }}
                <template v-if="data.overview.next_share_rate">
                  · {{ t('partner.nextRate', { month: data.overview.next_rate_month, rate: formatPartnerRate(data.overview.next_share_rate) }) }}
                </template>
              </span>
            </div>
            <dl class="mt-3 flex flex-wrap gap-x-6 gap-y-2 text-sm">
              <div>
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.lossCarried') }}</dt>
                <dd class="font-medium text-gray-900 dark:text-white" data-test="partner-loss-carried"><FinanceMoney :value="data.overview.loss_carried" /></dd>
              </div>
              <div>
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.agentCount') }}</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ data.overview.agent_count }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.memberCount') }}</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ data.overview.member_count }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.since') }}</dt>
                <dd class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(data.overview.since) }}</dd>
              </div>
            </dl>
          </section>

          <PartnerMonthSummary
            :totals="data.overview.current_month"
            :title="t('partner.currentMonth', { month: data.overview.current_month.month })"
          />
          <PartnerLinesTable :lines="data.overview.lines" :line-route="partnerLineRoute" />
          <PartnerStatementTable :items="data.statements" :month-route="statementRoute" />
        </div>
      </FinanceLoadState>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import FinanceLoadState from '@/components/admin/finance/FinanceLoadState.vue'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import AffiliateSectionTabs from '@/components/user/AffiliateSectionTabs.vue'
import PartnerLinesTable from '@/components/user/affiliate-partner/PartnerLinesTable.vue'
import PartnerMonthSummary from '@/components/user/affiliate-partner/PartnerMonthSummary.vue'
import PartnerStatementTable from '@/components/user/affiliate-partner/PartnerStatementTable.vue'
import affiliatePartnerAPI from '@/api/affiliatePartner'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { formatPartnerRate, partnerLineRoute } from '@/utils/affiliatePartner'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()

const { data, loading, error, load } = useFinanceLoad(
  async (signal) => {
    const [overview, statements] = await Promise.all([
      affiliatePartnerAPI.getOverview({ signal }),
      affiliatePartnerAPI.listStatements({ signal })
    ])
    return { overview, statements }
  },
  () => t('partner.loadFailed')
)


function statementRoute(month: string): RouteLocationRaw {
  return { name: 'AffiliatePartnerStatement', params: { month } }
}

onMounted(load)
</script>
