<template>
  <TeamConsumptionPage :view="view" :subject-id="subjectId" />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listTeamLines } from '@/api/affiliatePartner'
import { defaultAffiliateTeamRange, type AffiliateTeamView } from '@/composables/useAffiliateTeamQuery'
import { teamUserName } from '@/utils/affiliateTeam'
import TeamConsumptionPage from './TeamConsumptionPage.vue'
import { provideAffiliateTeamContext } from './teamContext'
import { createPartnerAgentContext, createPartnerDirectContext, createPartnerRegionContext } from './partnerTeamContext'

/** CUSTOM: 合伙人团队消费的一个视角（区域 / 直属团队 / 某渠道经理）。换视角时由父级用 :key 整个重建。 */
export type PartnerTeamScopeKind = 'region' | 'direct' | 'agent'

const props = defineProps<{
  scope: PartnerTeamScopeKind
  /** scope = agent 时为挂靠的渠道经理 ID；非法为 -1。 */
  agentId: number
  view: AffiliateTeamView
  subjectId: number
}>()

const { t } = useI18n()

function agentContext() {
  // 面包屑上的渠道经理名称取自区域的渠道经理线（代理端接口不返回被查看者本人）。
  const name = ref('')
  if (props.agentId > 0) {
    const range = defaultAffiliateTeamRange()
    listTeamLines({ start_date: range.start, end_date: range.end }, {})
      .then((lines) => {
        const line = lines.find((item) => item.line_agent?.user_id === props.agentId)?.line_agent
        name.value = line ? teamUserName(line, t('affiliate.team.deleted')) : t('partner.region.lineAgent')
      })
      .catch(() => {
        name.value = t('partner.region.lineAgent')
      })
  }
  return createPartnerAgentContext(() => props.agentId, () => name.value)
}

provideAffiliateTeamContext(
  props.scope === 'region'
    ? createPartnerRegionContext(() => t('partner.region.title'))
    : props.scope === 'direct'
      ? createPartnerDirectContext(() => t('partner.region.directTeam'))
      : agentContext()
)
</script>
