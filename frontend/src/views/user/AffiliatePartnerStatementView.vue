<template>
  <!-- CUSTOM: 合伙人某月结算单明细 -->
  <AppLayout>
    <div class="space-y-4">
      <nav :aria-label="t('affiliate.team.breadcrumb')" data-test="partner-breadcrumb">
        <ol class="flex flex-wrap items-center gap-x-1.5 text-lg font-semibold">
          <li>
            <RouterLink
              :to="{ name: 'AffiliatePartner' }"
              class="text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400"
            >
              {{ t('partner.tab') }}
            </RouterLink>
          </li>
          <li class="flex items-center gap-x-1.5">
            <span class="text-gray-300 dark:text-dark-600" aria-hidden="true">/</span>
            <span class="font-mono text-gray-900 dark:text-white" aria-current="page">{{ month }}</span>
          </li>
        </ol>
      </nav>

      <FinanceLoadState :loading="loading" :error="error" :has-data="!!data" @retry="load">
        <template v-if="data">
          <PartnerStatementDetailBody :detail="data" :line-route="partnerLineRoute" />
        </template>
      </FinanceLoadState>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import FinanceLoadState from '@/components/admin/finance/FinanceLoadState.vue'
import PartnerStatementDetailBody from '@/components/user/affiliate-partner/PartnerStatementDetailBody.vue'
import affiliatePartnerAPI from '@/api/affiliatePartner'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { partnerLineRoute } from '@/utils/affiliatePartner'

const { t } = useI18n()
const route = useRoute()

const month = computed(() => String(route.params.month ?? ''))

const { data, loading, error, load } = useFinanceLoad(
  (signal) => affiliatePartnerAPI.getStatement(month.value, { signal }),
  () => t('partner.loadFailed')
)


watch(month, load, { immediate: true })
</script>
