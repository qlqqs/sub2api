<template>
  <AppLayout>
    <TeamConsumptionPage :view="view" :subject-id="subjectId" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import TeamConsumptionPage from '@/components/user/affiliate-team/TeamConsumptionPage.vue'
import { parseAffiliateTeamRouteId, type AffiliateTeamView } from '@/composables/useAffiliateTeamQuery'

// 代理端团队消费页：三个路由名共用这个视图，页面主体与管理端代理视角共用（未提供上下文即代理端）。
const route = useRoute()

const view = computed<AffiliateTeamView>(() => {
  if (route.name === 'AffiliateTeamAgent') return 'agent'
  if (route.name === 'AffiliateTeamCustomer') return 'customer'
  return 'team'
})
const subjectId = computed(() => (view.value === 'team' ? 0 : parseAffiliateTeamRouteId(route.params.id)))
</script>
