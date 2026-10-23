<template>
  <AppLayout>
    <!-- 换到另一位代理（上级链接）时整页重建，各层之间切换沿用同一实例 -->
    <TeamConsumptionPage :key="viewerId" :view="view" :subject-id="subjectId" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { createAdminAffiliateTeamContext } from '@/components/admin/affiliate-team/adminTeamContext'
import TeamConsumptionPage from '@/components/user/affiliate-team/TeamConsumptionPage.vue'
import { provideAffiliateTeamContext } from '@/components/user/affiliate-team/teamContext'
import { parseAffiliateTeamRouteId, type AffiliateTeamView } from '@/composables/useAffiliateTeamQuery'

// 管理员查看某代理的团队消费页：与该代理自己看到的页面相同（同一套接口与组件，以被查看的代理为根）。
const route = useRoute()

const viewerId = computed(() => parseAffiliateTeamRouteId(route.params.viewerId))
const view = computed<AffiliateTeamView>(() => {
  if (route.name === 'AdminAffiliateTeamViewerAgent') return 'agent'
  if (route.name === 'AdminAffiliateTeamViewerCustomer') return 'customer'
  return 'team'
})
/** 代理 ID 非法时整页按不存在处理。 */
const subjectId = computed(() => {
  if (viewerId.value < 0) return -1
  return view.value === 'team' ? 0 : parseAffiliateTeamRouteId(route.params.id)
})

provideAffiliateTeamContext(createAdminAffiliateTeamContext(() => viewerId.value))
</script>
