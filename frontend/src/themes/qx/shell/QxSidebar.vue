<template>
  <aside class="qx-side" :class="{ 'is-collapsed': sidebarCollapsed, 'is-open': mobileOpen }">
    <!-- 品牌：Logo + 站点名 + 版本号 -->
    <div class="qx-side-brand">
      <router-link :to="homePath" class="qx-side-logo" @click="handleMenuItemClick(homePath)">
        <img v-if="settingsLoaded" :src="siteLogo || '/logo.svg'" alt="Logo" />
      </router-link>
      <router-link :to="homePath" class="qx-side-name" @click="handleMenuItemClick(homePath)">{{ siteName }}</router-link>
      <div class="qx-side-version">
        <VersionBadge :version="siteVersion" />
      </div>
    </div>

    <nav ref="sidebarNavRef" class="qx-side-nav scrollbar-hide">
      <div v-for="section in sections" :key="section.key" class="qx-side-section" :data-section="section.key">
        <span class="qx-side-kicker" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">{{ section.title }}</span>

        <template v-for="item in section.items" :key="item.path">
          <!-- 可展开分组 -->
          <template v-if="item.children?.length">
            <button
              type="button"
              class="qx-side-link"
              :class="{ 'is-active': isGroupActive(item) && !isGroupExpanded(item) }"
              :title="sidebarCollapsed ? item.label : undefined"
              :aria-expanded="isGroupExpanded(item) ? 'true' : 'false'"
              @click="handleGroupClick(item)"
            >
              <QxNavIcon v-if="item.icon" :name="item.icon" />
              <span class="qx-side-label">{{ item.label }}</span>
              <svg
                class="qx-side-chevron"
                :class="{ 'is-open': isGroupExpanded(item) }"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="1.8"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
              </svg>
            </button>
            <div v-if="!sidebarCollapsed && isGroupExpanded(item)" class="qx-side-children">
              <router-link
                v-for="child in item.children"
                :key="child.path"
                :to="child.path"
                class="qx-side-link is-child"
                :class="{ 'is-active': activeChildPath(item) === child.path }"
                @click="handleMenuItemClick(child.path)"
              >
                <QxNavIcon v-if="child.icon" :name="child.icon" />
                <span class="qx-side-label">{{ child.label }}</span>
              </router-link>
            </div>
          </template>

          <!-- 普通条目 -->
          <router-link
            v-else
            :to="item.path"
            class="qx-side-link"
            :class="{ 'is-active': isActive(item.path) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :id="tourId(item.path)"
            :data-tour="item.path === '/keys' ? 'sidebar-my-keys' : undefined"
            @click="handleMenuItemClick(item.path)"
          >
            <span v-if="item.iconSvg" class="qx-nav-icon qx-side-svg" v-html="sanitizeSvg(item.iconSvg)"></span>
            <QxNavIcon v-else :name="item.icon ?? 'link'" />
            <span class="qx-side-label">{{ item.label }}</span>
          </router-link>
        </template>
      </div>
    </nav>

    <!-- 底部：深色模式 + 收起 -->
    <div class="qx-side-foot">
      <button
        type="button"
        class="qx-side-link is-muted"
        :title="sidebarCollapsed ? themeLabel : undefined"
        @click="toggleTheme"
      >
        <Icon :name="isDark ? 'sun' : 'moon'" :stroke-width="1.6" class="qx-nav-icon" />
        <span class="qx-side-label">{{ themeLabel }}</span>
      </button>
      <button
        type="button"
        class="qx-side-link is-muted qx-side-collapse"
        :title="sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
        @click="toggleSidebar"
      >
        <Icon :name="sidebarCollapsed ? 'chevronRight' : 'chevronLeft'" :stroke-width="1.6" class="qx-nav-icon" />
        <span class="qx-side-label">{{ t('nav.collapse') }}</span>
      </button>
    </div>
  </aside>

  <!-- 移动端遮罩 -->
  <transition name="fade">
    <div v-if="mobileOpen" class="qx-side-overlay" @click="closeMobile"></div>
  </transition>
</template>

<script setup lang="ts">
/**
 * CUSTOM: QxAPI 外壳的侧边栏（由 QxShell 渲染，替代上游 AppSidebar.vue）。
 * 菜单数据见 useQxNav.ts；分组展开、新手引导、深色模式、滚动位置恢复的逻辑照搬上游。
 * 新手引导依赖的选择器必须保留：#sidebar-channel-manage、#sidebar-group-manage、#sidebar-wallet、
 * [data-tour="sidebar-my-keys"]。
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAdminSettingsStore, useAppStore, useOnboardingStore } from '@/stores'
import VersionBadge from '@/components/common/VersionBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeSvg } from '@/utils/sanitize'
import { sanitizeUrl } from '@/utils/url'
import QxNavIcon from './QxNavIcon.vue'
import { useQxNav, type QxNavItem } from './useQxNav'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const onboardingStore = useOnboardingStore()
const adminSettingsStore = useAdminSettingsStore()
const { sections, isAdmin, refreshBatchImageAccess } = useQxNav()

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const mobileOpen = computed(() => appStore.mobileOpen)
const sidebarNavRef = ref<HTMLElement | null>(null)
const isDark = ref(document.documentElement.classList.contains('dark'))
const themeLabel = computed(() => (isDark.value ? t('nav.lightMode') : t('nav.darkMode')))

const homePath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteVersion = computed(() => appStore.siteVersion)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

// 分组的展开状态：没有记录时跟随路由（子项选中即展开），用户点击后以点击结果为准。
const groupExpandOverrides = ref<Map<string, boolean>>(new Map())

/** 新手引导用的 id（与上游 AppSidebar 相同）。 */
const TOUR_IDS: Record<string, string> = {
  '/admin/accounts': 'sidebar-channel-manage',
  '/admin/groups': 'sidebar-group-manage',
  '/admin/redeem': 'sidebar-wallet'
}
function tourId(path: string): string | undefined {
  return TOUR_IDS[path]
}

function toggleSidebar() {
  appStore.toggleSidebar()
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function closeMobile() {
  appStore.setMobileOpen(false)
}

function handleMenuItemClick(itemPath: string) {
  if (mobileOpen.value) {
    setTimeout(() => {
      appStore.setMobileOpen(false)
    }, 150)
  }

  const pathToSelector: Record<string, string> = {
    '/admin/groups': '#sidebar-group-manage',
    '/admin/accounts': '#sidebar-channel-manage',
    '/keys': '[data-tour="sidebar-my-keys"]'
  }
  const selector = pathToSelector[itemPath]
  if (selector && onboardingStore.isCurrentStep(selector)) {
    onboardingStore.nextStep(500)
  }
}

function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(path + '/')
}

/** 分组内对应当前路由的子项：完全相同或最长前缀（下钻页归到最具体的子项）。 */
function activeChildPath(item: QxNavItem): string | null {
  let best: string | null = null
  for (const child of item.children ?? []) {
    if (isActive(child.path) && (best === null || child.path.length > best.length)) best = child.path
  }
  return best
}

function isGroupActive(item: QxNavItem): boolean {
  return activeChildPath(item) !== null
}

function isGroupExpanded(item: QxNavItem): boolean {
  const override = groupExpandOverrides.value.get(item.path)
  if (override !== undefined) return override
  return isGroupActive(item)
}

function toggleGroup(item: QxNavItem) {
  groupExpandOverrides.value.set(item.path, !isGroupExpanded(item))
}

/**
 * 分组父项的点击：收起状态下不处理（子项不可见）；expandOnly 只切换展开；
 * 否则跳到父项路径并保持展开（与上游一致）。
 */
function handleGroupClick(item: QxNavItem) {
  if (sidebarCollapsed.value) return
  if (item.expandOnly) {
    toggleGroup(item)
    return
  }
  if (route.path !== item.path) {
    router.push(item.path)
  }
  groupExpandOverrides.value.set(item.path, true)
}

// 初始化主题（上游 AppSidebar 在这里读取 localStorage.theme）。
const savedTheme = localStorage.getItem('theme')
if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
  isDark.value = true
  document.documentElement.classList.add('dark')
}

// 管理员设置（运维监控、支付、自定义菜单等开关）。
watch(
  isAdmin,
  (v) => {
    if (v) adminSettingsStore.fetch()
  },
  { immediate: true }
)

onMounted(() => {
  void refreshBatchImageAccess()
  if (isAdmin.value) {
    adminSettingsStore.fetch()
  }
  // 路由切换会重新挂载布局，恢复侧边栏滚动位置
  if (appStore.sidebarScrollTop > 0 && sidebarNavRef.value) {
    void nextTick(() => {
      if (sidebarNavRef.value) {
        sidebarNavRef.value.scrollTop = appStore.sidebarScrollTop
      }
    })
  }
})

onBeforeUnmount(() => {
  if (sidebarNavRef.value) {
    appStore.sidebarScrollTop = sidebarNavRef.value.scrollTop
  }
})
</script>
