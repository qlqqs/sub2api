<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <!-- 分区：邀请 | 团队消费（只对能进入团队消费页的用户显示） -->
        <AffiliateSectionTabs v-if="detail.team_access" />

        <!-- 代理：档位与比例 -->
        <div v-if="isAgent" class="card p-5" data-test="affiliate-agent-card">
          <p class="text-lg font-semibold text-gray-900 dark:text-white" data-test="affiliate-agent-level">
            {{ levelLabel(detail.agent_level) }}
          </p>
          <p
            v-if="detail.agent_rate_percent != null"
            class="mt-1 text-2xl font-semibold text-primary-600 dark:text-primary-400"
            data-test="affiliate-agent-rate"
          >
            {{ agentRateText }}
          </p>
          <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
            <span data-test="affiliate-owned-customers">{{ t('affiliate.agent.customers', { n: detail.owned_customer_count ?? 0 }) }}</span>
            <template v-if="upgradeProgressText">
              <span class="mx-1 text-gray-300 dark:text-dark-600">·</span>
              <span data-test="affiliate-upgrade-progress">{{ upgradeProgressText }}</span>
            </template>
          </p>
        </div>

        <!-- 邀请好友 -->
        <div class="card p-6" data-test="affiliate-invite-card">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.invite.title') }}</h3>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-300" data-test="affiliate-invite-text">{{ inviteText }}</p>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
              <div class="flex flex-col items-stretch gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                <code class="min-w-0 break-all text-sm font-semibold text-gray-900 dark:text-white sm:flex-1 sm:truncate">{{ detail.aff_code }}</code>
                <button type="button" class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
              <div class="flex flex-col items-stretch gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center">
                <code class="min-w-0 break-all text-sm text-gray-700 dark:text-gray-300 sm:flex-1 sm:truncate">{{ inviteLink }}</code>
                <button type="button" class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyInviteLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <ul class="mt-4 space-y-1 text-xs text-gray-500 dark:text-dark-400">
            <li>{{ t('affiliate.tips.line1') }}</li>
            <li>{{ t('affiliate.tips.line2') }}</li>
            <li v-if="detail.aff_frozen_quota > 0">{{ t('affiliate.tips.line3') }}</li>
          </ul>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <!-- 返利余额 -->
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400" data-test="affiliate-available-quota">
              {{ formatCurrency(detail.aff_quota) }}
            </p>
            <button
              type="button"
              class="btn btn-primary btn-sm mt-3"
              :disabled="transferring || detail.aff_quota <= 0"
              data-test="affiliate-transfer"
              @click="transferQuota"
            >
              <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
              <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
            </button>
          </div>

          <!-- 累计收益 -->
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(detail.aff_history_quota) }}
            </p>
            <p v-if="isAgent" class="mt-1 text-xs text-gray-500 dark:text-dark-400" data-test="affiliate-total-breakdown">
              {{ t('affiliate.stats.totalBreakdown', { agent: formatCurrency(detail.reward_totals?.agent ?? 0), invite: formatCurrency(detail.reward_totals?.invite ?? 0) }) }}
            </p>
            <p class="mt-1 text-xs text-amber-600 dark:text-amber-400">
              {{ t('affiliate.stats.frozenQuota') }} {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <div v-if="(detail.reward_totals?.invitee_signup_bonus ?? 0) > 0" class="card p-5" data-test="affiliate-signup-bonus-received">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.signupBonusReceived') }}</p>
          <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ formatCurrency(detail.reward_totals?.invitee_signup_bonus) }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.signupBonusReceivedHint') }}</p>
        </div>

        <!-- 邀请列表：分页 + 搜索（昵称模糊 / 完整邮箱） -->
        <div class="card overflow-hidden p-6" data-test="affiliate-invitees">
          <div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.invitees.title') }}</h3>
            <div class="flex w-full flex-wrap items-center justify-end gap-x-4 gap-y-2 sm:w-auto">
              <p v-if="appoint" class="text-sm text-gray-600 dark:text-gray-300" data-test="affiliate-appoint-summary">
                {{ t('affiliate.invitees.appointSummary', { used: appoint.used, max: appoint.max > 0 ? appoint.max : t('affiliate.invitees.unlimited') }) }}
              </p>
              <div v-if="inviteeTotal > 0 || inviteeSearch" class="w-full sm:w-64" data-test="affiliate-invitee-search">
                <SearchInput
                  v-model="inviteeSearchDraft"
                  :placeholder="t('affiliate.team.searchPlaceholder')"
                  :debounce-ms="300"
                  @search="onInviteeSearch"
                />
              </div>
            </div>
          </div>
          <p v-if="appointLimitReached" class="mt-1 text-xs text-amber-600 dark:text-amber-400" data-test="affiliate-appoint-limit">
            {{ t('affiliate.invitees.limitReached') }}
          </p>
          <div v-if="!inviteesLoaded" class="mt-4 space-y-3" aria-hidden="true">
            <div v-for="i in 3" :key="i" class="h-5 animate-pulse rounded bg-gray-100 dark:bg-dark-700/60"></div>
          </div>
          <div
            v-else-if="invitees.length === 0"
            class="mt-4 rounded-xl border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
            data-test="affiliate-invitees-empty"
          >
            {{ inviteeSearch ? t('affiliate.invitees.noMatches') : t('affiliate.invitees.empty') }}
          </div>
          <div v-else :class="['mt-4 overflow-x-auto transition-opacity', inviteesLoading && 'opacity-60']">
            <table class="w-full min-w-[640px] whitespace-nowrap text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th scope="col" class="px-3 py-2 font-medium" data-test="affiliate-col-nickname">{{ t('affiliate.invitees.columns.nickname') }}</th>
                  <th scope="col" class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.email') }}</th>
                  <th scope="col" class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                  <th v-if="isAgent" scope="col" class="px-3 py-2 font-medium" data-test="affiliate-col-ownership">{{ t('affiliate.invitees.columns.ownership') }}</th>
                  <th scope="col" class="px-3 py-2 text-right font-medium">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th v-if="appoint" scope="col" class="px-3 py-2 font-medium" data-test="affiliate-col-agent-role">{{ t('affiliate.invitees.columns.agentRole') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in invitees"
                  :key="item.user_id"
                  class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                  :data-test="'affiliate-invitee-' + item.user_id"
                >
                  <td class="px-3 py-3 font-medium text-gray-900 dark:text-white" data-test="affiliate-invitee-nickname">{{ item.username || '-' }}</td>
                  <td class="px-3 py-3 text-gray-500 dark:text-dark-400">{{ item.email || '-' }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</td>
                  <td v-if="isAgent" class="px-3 py-3 text-gray-700 dark:text-gray-300" data-test="affiliate-invitee-ownership">
                    {{ item.owned_by_me ? t('affiliate.invitees.ownedByMe') : t('affiliate.invitees.ownedByOther') }}
                  </td>
                  <td class="px-3 py-3 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(item.total_rebate) }}</td>
                  <td v-if="appoint" class="px-3 py-3" data-test="affiliate-invitee-role">
                    <div v-if="item.appointed_by_me" class="flex items-center gap-2">
                      <span class="text-gray-900 dark:text-white">{{ levelShort(item.agent_level ?? 2) }}</span>
                      <button
                        type="button"
                        class="btn btn-secondary btn-sm"
                        :disabled="appointingId !== null"
                        data-test="affiliate-revoke"
                        @click="askRevoke(item)"
                      >
                        {{ t('affiliate.invitees.revoke') }}
                      </button>
                    </div>
                    <div v-else-if="item.appointable" class="flex items-center gap-2">
                      <span class="text-gray-700 dark:text-gray-300">{{ levelShort(item.agent_level ?? 3) }}</span>
                      <button
                        type="button"
                        class="btn btn-primary btn-sm"
                        :disabled="appointLimitReached || appointingId !== null"
                        :title="appointLimitReached ? t('affiliate.invitees.limitReached') : undefined"
                        data-test="affiliate-appoint"
                        @click="askAppoint(item)"
                      >
                        {{ t('affiliate.invitees.appoint') }}
                      </button>
                    </div>
                    <span v-else class="text-gray-400 dark:text-dark-500" data-test="affiliate-not-appointable">
                      — <span class="text-xs">{{ t('affiliate.invitees.notAppointable') }}</span>
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination
            v-if="inviteeTotal > INVITEE_PAGE_SIZE"
            class="-mx-6 -mb-6 mt-4"
            :total="inviteeTotal"
            :page="inviteePage"
            :page-size="INVITEE_PAGE_SIZE"
            :show-page-size-selector="false"
            @update:page="onInviteePage"
          />
        </div>
      </template>
    </div>

    <ConfirmDialog
      :show="confirmState.show"
      :title="confirmState.title"
      :message="confirmState.message"
      @confirm="runConfirm"
      @cancel="cancelConfirm"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import AffiliateSectionTabs from '@/components/user/AffiliateSectionTabs.vue'
import userAPI from '@/api/user'
import type { AffiliateInvitee, UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTime } from '@/utils/format'
import { formatBalance } from '@/utils/balanceCurrency'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import {
  formatRatePercent,
  formatUsdShort,
  isInviteeBonusActive,
  normalizeInviteeSignupBonus,
} from '@/utils/affiliateRewards'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

// CUSTOM: affiliate rebates are balance units -> ¥ (same decimals/grouping as utils/format formatCurrency)
function formatCurrency(amount: number | null | undefined): string {
  const n = amount ?? 0
  return formatBalance(n, { decimals: n > 0 && n < 0.01 ? 6 : 2, grouping: true })
}

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)
const appointingId = ref<number | null>(null)

const isAgent = computed(() => detail.value?.agent_level === 1 || detail.value?.agent_level === 2)
const appoint = computed(() => (detail.value?.agent_level === 1 ? detail.value.appoint ?? null : null))
const appointLimitReached = computed(() => {
  const a = appoint.value
  return !!a && a.max > 0 && a.used >= a.max
})

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

function levelLabel(level: number | null | undefined): string {
  return t(`affiliate.levels.${level === 1 || level === 2 ? level : 3}`)
}

function levelShort(level: number | null | undefined): string {
  return t(`affiliate.levelShort.${level === 1 || level === 2 ? level : 3}`)
}

const agentRateText = computed(() => {
  const d = detail.value
  if (!d || d.agent_rate_percent == null) return ''
  const rate = formatRatePercent(d.agent_rate_percent)
  const recharge = d.rebate_base === 'recharge'
  // 一级代理同时从直属客户和下级二级代理团队拿利润，分开写清楚两个比例。
  if (d.agent_level === 1 && d.team_rate_percent != null && d.team_rate_percent > 0) {
    const team = formatRatePercent(d.team_rate_percent)
    return recharge
      ? t('affiliate.agent.rateTopRecharge', { rate, team })
      : t('affiliate.agent.rateTopConsumption', { rate, team })
  }
  return recharge
    ? t('affiliate.agent.rateRecharge', { rate })
    : t('affiliate.agent.rateConsumption', { rate })
})

// Upgrade progress only exists in auto mode for unlocked agents with a next level.
const upgradeProgressText = computed(() => {
  const d = detail.value
  if (!d || !isAgent.value || d.agent_upgrade_mode !== 'auto' || d.agent_level_locked) return ''
  if (d.next_agent_level == null || d.next_level_threshold == null) return ''
  const params = {
    remaining: Math.max(0, d.next_level_threshold - d.current_invite_count),
    level: levelLabel(d.next_agent_level),
    count: d.current_invite_count,
    threshold: d.next_level_threshold,
  }
  return t('affiliate.agent.progress', params)
})

// 「好友每笔消费，你得 5%；好友首次充值满 ¥10，你得 ¥2，好友得 10%，封顶 ¥20」— generated from the
// enabled rewards. Both first-recharge rewards share one clause; a threshold common to both goes
// up front, otherwise each side carries its own. Agents only earn profit share, so their card
// never shows the inviter reward.
const inviteText = computed(() => {
  const d = detail.value
  if (!d) return ''
  const parts: string[] = []
  const signupBonus = normalizeInviteeSignupBonus(d.invitee_signup_bonus)
  if (signupBonus > 0) parts.push(t('affiliate.invite.friendSignupBonus', { amount: formatUsdShort(signupBonus) }))
  const reward = isAgent.value ? null : d.invite_reward
  if (reward?.mode === 'percent' && reward.rate_percent > 0) {
    const base = d.rebate_base === 'recharge' ? t('affiliate.invite.baseRecharge') : t('affiliate.invite.baseConsumption')
    let text = t('affiliate.invite.rewardPercent', { base, rate: formatRatePercent(reward.rate_percent) })
    if (reward.duration_days > 0) text += t('affiliate.invite.rewardPercentDays', { days: reward.duration_days })
    if (reward.per_invitee_cap > 0) text += t('affiliate.invite.rewardPercentCap', { cap: formatUsdShort(reward.per_invitee_cap) })
    parts.push(text)
  }
  const mine = reward?.mode === 'first_recharge' && reward.fixed_amount > 0 ? reward : null
  const bonus = isInviteeBonusActive(d.invitee_bonus) ? d.invitee_bonus : null
  if (mine || bonus) {
    const mins = [mine?.min_recharge ?? null, bonus?.min_recharge ?? null].filter((m): m is number => m !== null)
    const sharedMin = mins.every((m) => m === mins[0]) ? mins[0] : null
    const clauses = [sharedMin
      ? t('affiliate.invite.firstRechargeMin', { min: formatUsdShort(sharedMin) })
      : t('affiliate.invite.firstRecharge')]
    if (mine) {
      const amount = formatUsdShort(mine.fixed_amount)
      clauses.push(sharedMin === null && mine.min_recharge > 0
        ? t('affiliate.invite.youGetMin', { min: formatUsdShort(mine.min_recharge), amount })
        : t('affiliate.invite.youGet', { amount }))
    }
    if (bonus) {
      const value = bonus.mode === 'fixed' ? formatUsdShort(bonus.value) : `${formatRatePercent(bonus.value)}%`
      clauses.push(sharedMin === null && bonus.min_recharge > 0
        ? t('affiliate.invite.friendGetsMin', { min: formatUsdShort(bonus.min_recharge), bonus: value })
        : t('affiliate.invite.friendGets', { bonus: value }))
      if (bonus.mode === 'percent' && bonus.max > 0) {
        clauses.push(t('affiliate.invite.friendCap', { max: formatUsdShort(bonus.max) }))
      }
    }
    parts.push(clauses.join(t('affiliate.invite.clauseSeparator')))
  }
  return parts.length > 0 ? parts.join(t('affiliate.invite.separator')) : t('affiliate.invite.noReward')
})

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

// ---------- 邀请列表（分页 + 搜索） ----------
const INVITEE_PAGE_SIZE = 20
const invitees = ref<AffiliateInvitee[]>([])
const inviteeTotal = ref(0)
const inviteePage = ref(1)
/** 已生效的搜索词；inviteeSearchDraft 是输入框里的草稿（防抖后才生效）。 */
const inviteeSearch = ref('')
const inviteeSearchDraft = ref('')
const inviteesLoaded = ref(false)
const inviteesLoading = ref(false)
let inviteeController: AbortController | null = null

function isCancel(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const e = error as { name?: unknown; code?: unknown }
  return e.name === 'AbortError' || e.name === 'CanceledError' || e.code === 'ERR_CANCELED'
}

async function loadInvitees(): Promise<void> {
  inviteeController?.abort()
  const controller = new AbortController()
  inviteeController = controller
  inviteesLoading.value = true
  try {
    const res = await userAPI.listAffiliateInvitees(
      { search: inviteeSearch.value || undefined, page: inviteePage.value, page_size: INVITEE_PAGE_SIZE },
      { signal: controller.signal }
    )
    if (inviteeController !== controller) return
    invitees.value = res.items ?? []
    inviteeTotal.value = res.total ?? 0
    inviteesLoaded.value = true
    // 取消任命等操作后当前页可能已越界：回到最后一页。
    if (invitees.value.length === 0 && inviteeTotal.value > 0 && inviteePage.value > 1) {
      inviteePage.value = Math.max(1, res.pages)
      void loadInvitees()
    }
  } catch (error) {
    if (inviteeController !== controller || isCancel(error)) return
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (inviteeController === controller) {
      inviteeController = null
      inviteesLoading.value = false
    }
  }
}

function onInviteeSearch(value: string) {
  inviteeSearch.value = value.trim()
  inviteePage.value = 1
  void loadInvitees()
}

function onInviteePage(page: number) {
  inviteePage.value = page
  void loadInvitees()
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

// ---------- 一级代理任命二级 ----------
const confirmState = reactive<{ show: boolean; title: string; message: string; action: (() => Promise<void>) | null }>({
  show: false,
  title: '',
  message: '',
  action: null,
})

function openConfirm(title: string, message: string, action: () => Promise<void>) {
  confirmState.title = title
  confirmState.message = message
  confirmState.action = action
  confirmState.show = true
}

async function runConfirm() {
  const action = confirmState.action
  confirmState.show = false
  confirmState.action = null
  if (action) await action()
}

function cancelConfirm() {
  confirmState.show = false
  confirmState.action = null
}

async function setInviteeLevel(invitee: AffiliateInvitee, level: 2 | 3, successKey: string) {
  if (appointingId.value !== null) return
  appointingId.value = invitee.user_id
  try {
    await userAPI.setInviteeAgentLevel(invitee.user_id, level)
    appStore.showSuccess(t(successKey))
    // 任命计数在详情里，身份与按钮在当前页列表里，两者都刷新。
    await Promise.all([loadAffiliateDetail(true), loadInvitees()])
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'affiliate.errors', t('common.error')))
  } finally {
    appointingId.value = null
  }
}

function askAppoint(invitee: AffiliateInvitee) {
  if (appointLimitReached.value) return
  openConfirm(
    t('affiliate.invitees.appointTitle'),
    t('affiliate.invitees.appointConfirm'),
    () => setInviteeLevel(invitee, 2, 'affiliate.invitees.appointSuccess'),
  )
}

function askRevoke(invitee: AffiliateInvitee) {
  openConfirm(
    t('affiliate.invitees.revokeTitle'),
    t('affiliate.invitees.revokeConfirm'),
    () => setInviteeLevel(invitee, 3, 'affiliate.invitees.revokeSuccess'),
  )
}

onMounted(() => {
  void loadAffiliateDetail()
  void loadInvitees()
})

onBeforeUnmount(() => {
  inviteeController?.abort()
})
</script>
