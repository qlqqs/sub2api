<template>
  <span :class="['badge whitespace-nowrap', PARTNER_STATUS_BADGE[status]]" :data-test="`partner-status-${status}`">
    {{ t(`partner.status.${status}`) }}
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PartnerMonthStatus } from '@/api/affiliatePartner'
import { PARTNER_STATUS_BADGE } from '@/utils/affiliatePartner'

defineProps<{ status: PartnerMonthStatus }>()

const { t } = useI18n()
</script>
