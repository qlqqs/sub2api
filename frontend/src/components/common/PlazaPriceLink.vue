<template>
  <!-- CUSTOM: 用户端「查看价格」链接,跳到模型广场对应分组;广场关闭或当前用户无权访问时不渲染 -->
  <RouterLink
    v-if="visible"
    :to="target"
    class="inline-flex items-center gap-1 whitespace-nowrap text-xs font-medium text-primary-600 transition-colors hover:text-primary-700 hover:underline dark:text-primary-400 dark:hover:text-primary-300"
    :title="t('modelPlaza.priceLink.title')"
    data-testid="plaza-price-link"
    @click.stop
  >
    <Icon name="grid" size="xs" class="h-3.5 w-3.5" />
    {{ label || t('modelPlaza.priceLink.view') }}
  </RouterLink>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { canViewModelPlaza, modelPlazaPriceRoute } from '@/utils/modelPlazaAccess'

const props = withDefaults(
  defineProps<{
    /** 目标分组;缺省时打开模型广场首页。 */
    groupId?: number | null
    /** 链接文字;缺省为「查看价格」。 */
    label?: string
  }>(),
  { groupId: null, label: '' }
)

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const isAuthenticated = computed(() => authStore.isAuthenticated === true)
const visible = computed(() => canViewModelPlaza(appStore.cachedPublicSettings, isAuthenticated.value))
const target = computed(() => modelPlazaPriceRoute(props.groupId, isAuthenticated.value))
</script>
