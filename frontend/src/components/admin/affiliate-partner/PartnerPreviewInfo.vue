<template>
  <!-- CUSTOM: 设为合伙人 / 挂靠前的预览：档位、团队规模、当前所属区域、不能操作的原因 -->
  <div class="space-y-2 rounded-lg bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800/60" data-test="partner-preview">
    <p class="text-gray-700 dark:text-gray-300">
      {{ levelLabel }}
      <span class="mx-1 text-gray-300 dark:text-dark-600">·</span>
      {{ t('admin.partners.preview.team', { owned: team.owned, viaUpline: team.via_upline }) }}
    </p>
    <p v-if="currentRegion" class="text-amber-600 dark:text-amber-400" data-test="partner-preview-region">
      {{ regionHint }}
    </p>
    <p v-if="reason" class="text-red-600 dark:text-red-400" role="alert" data-test="partner-preview-reason">{{ reason }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PartnerRegionRef, PartnerTeamSize } from '@/api/admin/affiliatePartners'

const props = defineProps<{
  agentLevel: number
  team: PartnerTeamSize
  currentRegion: PartnerRegionRef | null
  reason: string | null
  /** 当前所属区域的提示文案 key（设为合伙人 / 挂靠两种说法）。 */
  regionHintKey: string
}>()

const { t } = useI18n()

const levelLabel = computed(() => t(`admin.affiliates.levels.${props.agentLevel === 1 || props.agentLevel === 2 ? props.agentLevel : 3}`))
const regionHint = computed(() =>
  props.currentRegion
    ? t(props.regionHintKey, {
        region: props.currentRegion.region_name,
        partner: props.currentRegion.partner.email || props.currentRegion.partner.username
      })
    : ''
)
</script>
