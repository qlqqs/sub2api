<template>
  <AppLayout>
    <PartnerTeamScope :key="`${scope}:${agentId}`" :scope="scope" :agent-id="agentId" :view="view" :subject-id="subjectId" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PartnerTeamScope, { type PartnerTeamScopeKind } from '@/components/user/affiliate-team/PartnerTeamScope.vue'
import { parseAffiliateTeamRouteId, type AffiliateTeamView } from '@/composables/useAffiliateTeamQuery'

// CUSTOM: 合伙人的区域团队消费：区域 → 渠道经理 / 直属团队 → 分销代理 → 客户，八个路由名共用这个视图。
const route = useRoute()

const name = computed(() => String(route.name ?? ''))
const scope = computed<PartnerTeamScopeKind>(() => {
  if (name.value.startsWith('AffiliateRegionDirect')) return 'direct'
  if (name.value.startsWith('AffiliateRegionAgent')) return 'agent'
  return 'region'
})
const agentId = computed(() => (scope.value === 'agent' ? parseAffiliateTeamRouteId(route.params.agentId) : 0))
const view = computed<AffiliateTeamView>(() => {
  if (name.value.endsWith('Customer')) return 'customer'
  if (name.value === 'AffiliateRegionDirectAgent' || name.value === 'AffiliateRegionAgentAgent') return 'agent'
  return 'team'
})
/** 渠道经理 ID 非法时整页按不存在处理。 */
const subjectId = computed(() => {
  if (agentId.value < 0) return -1
  return view.value === 'team' ? 0 : parseAffiliateTeamRouteId(route.params.id)
})
</script>
