<template>
  <Icon v-if="'icon' in def" :name="def.icon" :stroke-width="1.6" class="qx-nav-icon" />
  <svg v-else class="qx-nav-icon h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.6">
    <path v-for="d in def.paths" :key="d" stroke-linecap="round" stroke-linejoin="round" :d="d" />
  </svg>
</template>

<script setup lang="ts">
// CUSTOM: QxAPI 侧边栏图标，名字见 navIcons.ts。
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { QX_NAV_ICONS, type QxNavIconDef, type QxNavIconName } from './navIcons'

const props = defineProps<{ name: QxNavIconName }>()
const def = computed((): QxNavIconDef => QX_NAV_ICONS[props.name])
</script>
