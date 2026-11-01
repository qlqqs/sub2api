<template>
  <!-- CUSTOM: 合伙人详情：概览（改区域名和比例）/ 渠道经理（挂靠）/ 结算单 -->
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
        <RouterLink
          :to="{ name: 'AdminAffiliatePartners' }"
          class="text-sm text-gray-500 hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400"
        >
          ← {{ t('nav.affiliatePartners') }}
        </RouterLink>
        <template v-if="detail">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white" data-test="partner-detail-title">
            {{ detail.region_name }}
          </h2>
          <span class="text-sm text-gray-600 dark:text-gray-300">
            {{ detail.user.email }} <span class="text-xs text-gray-500">#{{ detail.user.id }}</span>
          </span>
        </template>
      </div>

      <nav class="tabs w-max" role="tablist" :aria-label="t('nav.affiliatePartners')">
        <button
          v-for="key in TABS"
          :key="key"
          type="button"
          role="tab"
          class="tab whitespace-nowrap"
          :class="{ 'tab-active': tab === key }"
          :aria-selected="tab === key"
          :data-test="`partner-tab-${key}`"
          @click="selectTab(key)"
        >
          {{ t(`admin.partners.tabs.${key}`) }}
        </button>
      </nav>

      <FinanceLoadState :loading="loading" :error="error" :has-data="!!detail" @retry="load">
        <div v-if="detail">
          <!-- 概览 -->
          <div v-if="tab === 'overview'" class="space-y-4" role="tabpanel">
            <PartnerMonthSummary
              :totals="detail.current_month"
              :title="t('partner.currentMonth', { month: detail.current_month.month })"
            />
            <section class="card p-5">
              <dl class="flex flex-wrap gap-x-8 gap-y-2 text-sm">
                <div>
                  <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('partner.since') }}</dt>
                  <dd class="text-gray-900 dark:text-white">{{ formatDateTime(detail.since) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.partners.columns.agentCount') }}</dt>
                  <dd class="text-gray-900 dark:text-white">{{ detail.agent_count }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.partners.columns.memberCount') }}</dt>
                  <dd class="text-gray-900 dark:text-white">{{ detail.member_count }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.partners.columns.lossCarried') }}</dt>
                  <dd class="text-gray-900 dark:text-white"><FinanceMoney :value="detail.loss_carried" /></dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.partners.columns.unconfirmedMonths') }}</dt>
                  <dd class="text-gray-900 dark:text-white">{{ detail.unconfirmed_months }}</dd>
                </div>
              </dl>
            </section>
            <form class="card space-y-4 p-5" data-test="partner-edit-form" @submit.prevent="saveEdit">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.partners.edit.title') }}</h3>
              <div class="grid gap-4 sm:grid-cols-2">
                <div>
                  <label for="partner-edit-region" class="input-label">{{ t('admin.partners.fields.regionName') }}</label>
                  <input id="partner-edit-region" v-model="edit.region" type="text" class="input" maxlength="64" data-test="partner-edit-region" />
                </div>
                <div>
                  <label for="partner-edit-rate" class="input-label">{{ t('admin.partners.fields.shareRate') }}</label>
                  <div class="relative">
                    <input id="partner-edit-rate" v-model="edit.rate" type="text" inputmode="decimal" class="input pr-8" data-test="partner-edit-rate" />
                    <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">%</span>
                  </div>
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                    {{ t('admin.partners.edit.rateHint', { rate: formatPartnerRate(detail.share_rate) }) }}
                    <template v-if="detail.next_share_rate">
                      · {{ t('partner.nextRate', { month: detail.next_rate_month, rate: formatPartnerRate(detail.next_share_rate) }) }}
                    </template>
                  </p>
                  <p v-if="'error' in editRate" class="mt-1 text-xs text-red-500">{{ t('admin.partners.errors.rate') }}</p>
                </div>
              </div>
              <div class="flex justify-end">
                <button type="submit" class="btn btn-primary" :disabled="!editPayload || edit.saving" data-test="partner-edit-submit">
                  {{ edit.saving ? t('common.saving') : t('common.save') }}
                </button>
              </div>
            </form>
          </div>

          <!-- 渠道经理 -->
          <div v-else-if="tab === 'agents'" class="space-y-3" role="tabpanel">
            <div class="flex justify-end">
              <button type="button" class="btn btn-primary" data-test="partner-attach-open" @click="attachOpen = true">
                + {{ t('admin.partners.attach.title') }}
              </button>
            </div>
            <div class="card overflow-hidden">
              <div class="overflow-x-auto">
                <table class="table min-w-max whitespace-nowrap" data-test="partner-agents-table">
                  <thead>
                    <tr>
                      <th scope="col">{{ t('admin.partners.columns.user') }}</th>
                      <th scope="col">{{ t('admin.partners.agents.attachedAt') }}</th>
                      <th scope="col">{{ t('admin.partners.agents.source') }}</th>
                      <th scope="col" class="!text-right">{{ t('admin.partners.agents.team') }}</th>
                      <th scope="col" class="!text-right">{{ t('partner.lines.consumption') }}</th>
                      <th scope="col" class="!text-right">{{ t('partner.lines.revenue') }}</th>
                      <th scope="col" class="!text-right">{{ t('partner.lines.account_cost') }}</th>
                      <th scope="col" class="!text-right">{{ t('partner.lines.commission') }}</th>
                      <th scope="col" class="!text-right">{{ t('partner.lines.profit') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-if="detail.agents.length === 0">
                      <td colspan="9" class="py-8 text-center text-gray-500 dark:text-dark-400">{{ t('admin.partners.agents.empty') }}</td>
                    </tr>
                    <tr v-for="agent in detail.agents" :key="agent.user.id" data-test="partner-agent-row">
                      <td>
                        <RouterLink
                          :to="{ name: 'AdminAffiliateTeamViewer', params: { viewerId: String(agent.user.id) } }"
                          class="text-sm font-medium text-primary-600 hover:underline dark:text-primary-400"
                        >
                          {{ agent.user.email || '-' }}
                        </RouterLink>
                        <div class="text-xs text-gray-500 dark:text-dark-400">#{{ agent.user.id }}<template v-if="agent.user.username"> · {{ agent.user.username }}</template></div>
                      </td>
                      <td class="text-xs">{{ formatDateTime(agent.attached_at) }}</td>
                      <td class="text-xs">
                        <template v-if="agent.source === 'promote'">{{ t('admin.partners.agents.sourcePromote') }}</template>
                        <template v-else>{{ t('admin.partners.agents.sourceManual') }}</template>
                        <span v-if="agent.attached_by_email" class="block text-gray-400">{{ agent.attached_by_email }}</span>
                      </td>
                      <td class="text-right text-xs tabular-nums">
                        {{ t('admin.partners.preview.team', { owned: agent.team.owned, viaUpline: agent.team.via_upline }) }}
                      </td>
                      <td class="text-right"><FinanceMoney :value="agent.current_month.consumption" /></td>
                      <td class="text-right"><FinanceMoney :value="agent.current_month.revenue" /></td>
                      <td class="text-right"><FinanceMoney :value="agent.current_month.account_cost" /></td>
                      <td class="text-right"><FinanceMoney :value="agent.current_month.commission" /></td>
                      <td class="text-right"><FinanceMoney :value="agent.current_month.profit" /></td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          <!-- 结算单 -->
          <div v-else role="tabpanel">
            <FinanceLoadState :loading="statementsLoading" :error="statementsError" :has-data="!!statements" @retry="loadStatements">
              <PartnerStatementTable v-if="statements" :items="statements" :active="openMonth" @select="openMonth = $event" />
            </FinanceLoadState>
          </div>
        </div>
      </FinanceLoadState>
    </div>

    <PartnerAttachDialog
      :show="attachOpen"
      :partner-id="partnerId"
      :region-name="detail?.region_name ?? ''"
      @close="attachOpen = false"
      @attached="onAttached"
    />
    <PartnerStatementDialog
      :show="openMonth !== ''"
      :partner-id="partnerId"
      :month="openMonth"
      :statements="statements ?? []"
      @close="openMonth = ''"
      @changed="onStatementChanged"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import FinanceLoadState from '@/components/admin/finance/FinanceLoadState.vue'
import FinanceMoney from '@/components/admin/finance/FinanceMoney.vue'
import PartnerAttachDialog from '@/components/admin/affiliate-partner/PartnerAttachDialog.vue'
import PartnerStatementDialog from '@/components/admin/affiliate-partner/PartnerStatementDialog.vue'
import PartnerMonthSummary from '@/components/user/affiliate-partner/PartnerMonthSummary.vue'
import PartnerStatementTable from '@/components/user/affiliate-partner/PartnerStatementTable.vue'
import affiliatePartnersAPI, { type UpdatePartnerRequest } from '@/api/admin/affiliatePartners'
import { useFinanceLoad } from '@/composables/useFinanceLoad'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatPartnerRate, partnerRateFromPercent } from '@/utils/affiliatePartner'
import { decimalRateToPercent } from '@/utils/financeFormat'
import { formatDateTime } from '@/utils/format'

type Tab = 'overview' | 'agents' | 'statements'
const TABS: readonly Tab[] = ['overview', 'agents', 'statements']

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const partnerId = computed(() => Number(route.params.userId))

// 页签记在 query 里，刷新和分享链接能回到同一页签。
const tab = computed<Tab>(() => TABS.find((key) => key === route.query.tab) ?? 'overview')
function selectTab(key: Tab) {
  void router.replace({ query: key === 'overview' ? {} : { tab: key } })
}

const { data: detail, loading, error, load } = useFinanceLoad(
  (signal) => affiliatePartnersAPI.getPartner(partnerId.value, { signal }),
  () => t('common.unknownError')
)

const {
  data: statements,
  loading: statementsLoading,
  error: statementsError,
  load: loadStatements
} = useFinanceLoad((signal) => affiliatePartnersAPI.listStatements(partnerId.value, { signal }), () => t('common.unknownError'))

watch(partnerId, () => void load(), { immediate: true })
watch(
  () => [partnerId.value, tab.value] as const,
  ([, current]) => {
    if (current === 'statements') void loadStatements()
  },
  { immediate: true }
)

// ---------- 修改区域名和比例 ----------
const edit = reactive({ region: '', rate: '', saving: false })

/** 比例框预填下月比例（有的话），否则为当前比例。 */
function editBaseRate(): string {
  return decimalRateToPercent(detail.value?.next_share_rate ?? detail.value?.share_rate)
}

watch(detail, (value) => {
  if (!value) return
  edit.region = value.region_name
  edit.rate = editBaseRate()
})

const editRate = computed(() => partnerRateFromPercent(edit.rate))

/** 有改动且合法时的请求体；否则 null。 */
const editPayload = computed<UpdatePartnerRequest | null>(() => {
  const current = detail.value
  if (!current) return null
  const payload: UpdatePartnerRequest = {}
  const region = edit.region.trim()
  if (!region) return null
  if (region !== current.region_name) payload.region_name = region
  const rate = editRate.value
  if (!('value' in rate)) return null
  if (edit.rate.trim() !== editBaseRate()) payload.share_rate = rate.value
  return Object.keys(payload).length > 0 ? payload : null
})

async function saveEdit() {
  const payload = editPayload.value
  if (!payload || edit.saving) return
  edit.saving = true
  try {
    await affiliatePartnersAPI.updatePartner(partnerId.value, payload)
    appStore.showSuccess(payload.share_rate ? t('admin.partners.edit.savedRate') : t('common.saved'))
    await load()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    edit.saving = false
  }
}

// ---------- 挂靠 ----------
const attachOpen = ref(false)

function onAttached() {
  attachOpen.value = false
  void load()
}

// ---------- 结算单 ----------
const openMonth = ref('')

function onStatementChanged() {
  void loadStatements()
  void load()
}
</script>
