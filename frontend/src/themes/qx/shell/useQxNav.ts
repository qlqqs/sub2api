/**
 * CUSTOM: QxAPI 外壳侧边栏的导航数据，按分区（CONSOLE / BILLING / ACCOUNT …）组织。
 *
 * 条目、功能开关、简单模式、自定义菜单、后端模式的规则都照搬上游
 * `components/layout/AppSidebar.vue`，只是把扁平列表拆成分区。上游新增菜单时，
 * `__tests__/useQxNav.drift.spec.ts` 会因为路径缺失而失败，提醒同步到这里。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAdminSettingsStore, useAppStore, useAuthStore } from '@/stores'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import type { QxNavIconName } from './navIcons'
import { useStudioEnabled } from '../studio/studioStatus'

export interface QxNavItem {
  path: string
  label: string
  /** 图标名（见 navIcons.ts）；自定义菜单用 iconSvg，二者都没有时显示占位图标。 */
  icon: QxNavIconName | null
  iconSvg?: string
  hideInSimpleMode?: boolean
  children?: QxNavItem[]
  /** 为 true 时父项只负责展开 / 收起，不跳转；path 仅作分组的稳定键。 */
  expandOnly?: boolean
  /** 返回 false 时隐藏（undefined 视为显示，避免设置未加载时菜单闪烁）。 */
  featureFlag?: () => boolean | undefined
}

export interface QxNavSection {
  key: string
  title: string
  items: QxNavItem[]
}

/** 递归去掉 featureFlag() === false 的条目（与上游 applyFeatureFlags 相同）。 */
export function applyFeatureFlags(items: QxNavItem[]): QxNavItem[] {
  const out: QxNavItem[] = []
  for (const item of items) {
    if (item.featureFlag && item.featureFlag() === false) continue
    out.push(item.children ? { ...item, children: applyFeatureFlags(item.children) } : item)
  }
  return out
}

/** 遍历分区里的全部路径（含分组子项），供测试与路由匹配使用。 */
export function collectNavPaths(sections: QxNavSection[]): string[] {
  const out: string[] = []
  const walk = (items: QxNavItem[]) => {
    for (const item of items) {
      out.push(item.path)
      if (item.children) walk(item.children)
    }
  }
  for (const section of sections) walk(section.items)
  return out
}

export function useQxNav() {
  const { t } = useI18n()
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

  const isAdmin = computed(() => authStore.isAdmin)

  const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
  const flagPayment = makeSidebarFlag(FeatureFlags.payment)
  const flagAvailableChannels = makeSidebarFlag(FeatureFlags.availableChannels)
  const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)
  const flagAffiliate = makeSidebarFlag(FeatureFlags.affiliate)
  const flagRiskControl = makeSidebarFlag(FeatureFlags.riskControl)
  const flagPluginManagement = makeSidebarFlag(FeatureFlags.pluginManagement)
  const flagOpsMonitoring = () => adminSettingsStore.opsMonitoringEnabled
  const flagAdminPayment = () => adminSettingsStore.paymentEnabled
  const flagBatchImageAccess = () => canUseBatchImage.value
  // CUSTOM: 创作工作台（部署时配置了 Nova Studio 才显示）
  const studioEnabled = useStudioEnabled()
  const flagStudio = () => studioEnabled.value === true

  // 购买入口文案随站点计费模式切换（与上游一致）。
  const purchaseLabel = computed(() => {
    switch (resolveSiteBillingMode(appStore.cachedPublicSettings)) {
      case 'recharge_only':
        return t('nav.recharge')
      case 'subscription_only':
        return t('nav.subscribe')
      default:
        return t('nav.buySubscription')
    }
  })

  const customMenuItemsForUser = computed(() =>
    (appStore.cachedPublicSettings?.custom_menu_items ?? [])
      .filter((item) => item.visibility === 'user')
      .sort((a, b) => a.sort_order - b.sort_order)
      .map((item): QxNavItem => ({ path: `/custom/${item.id}`, label: item.label, icon: null, iconSvg: item.icon_svg }))
  )

  const customMenuItemsForAdmin = computed(() =>
    adminSettingsStore.customMenuItems
      .filter((item) => item.visibility === 'admin')
      .sort((a, b) => a.sort_order - b.sort_order)
      .map((item): QxNavItem => ({ path: `/custom/${item.id}`, label: item.label, icon: null, iconSvg: item.icon_svg }))
  )

  const section = (key: string, items: QxNavItem[]): QxNavSection => ({
    key,
    title: t(`qxTheme.nav.sections.${key}`),
    items
  })

  /** 功能开关 + 简单模式过滤，并去掉过滤后为空的分区。 */
  function finalize(sections: QxNavSection[]): QxNavSection[] {
    return sections
      .map((s) => {
        const visible = applyFeatureFlags(s.items)
        return { ...s, items: authStore.isSimpleMode ? visible.filter((item) => !item.hideInSimpleMode) : visible }
      })
      .filter((s) => s.items.length > 0)
  }

  /** 用户自己的菜单（用户端主菜单与管理员的「我的账户」共用），分成三段。 */
  function buildSelfItems(withDashboard: boolean) {
    const console: QxNavItem[] = []
    if (withDashboard) console.push({ path: '/dashboard', label: t('nav.dashboard'), icon: 'dashboard' })
    console.push(
      { path: '/keys', label: t('nav.apiKeys'), icon: 'key' },
      { path: '/studio', label: t('qxTheme.studio.title'), icon: 'studio', featureFlag: flagStudio },
      { path: '/batch-image', label: t('nav.batchImage'), icon: 'batchImage', hideInSimpleMode: true, featureFlag: flagBatchImageAccess },
      { path: '/usage', label: t('nav.usage'), icon: 'usage', hideInSimpleMode: true },
      { path: '/available-channels', label: t('nav.availableChannels'), icon: 'channel', hideInSimpleMode: true, featureFlag: flagAvailableChannels },
      { path: '/monitor', label: t('nav.channelStatus'), icon: 'signal', featureFlag: flagChannelMonitor }
    )
    const billing: QxNavItem[] = [
      { path: '/subscriptions', label: t('nav.mySubscriptions'), icon: 'creditCard', hideInSimpleMode: true, featureFlag: flagSubscription },
      { path: '/purchase', label: purchaseLabel.value, icon: 'recharge', hideInSimpleMode: true, featureFlag: flagPayment },
      { path: '/orders', label: t('nav.myOrders'), icon: 'orders', hideInSimpleMode: true, featureFlag: flagPayment },
      { path: '/redeem', label: t('nav.redeem'), icon: 'gift', hideInSimpleMode: true }
    ]
    const account: QxNavItem[] = [
      { path: '/affiliate', label: t('nav.affiliate'), icon: 'users', hideInSimpleMode: true, featureFlag: flagAffiliate },
      { path: '/profile', label: t('nav.profile'), icon: 'user' },
      ...customMenuItemsForUser.value
    ]
    return { console, billing, account }
  }

  const userSections = computed((): QxNavSection[] => {
    const { console, billing, account } = buildSelfItems(true)
    return finalize([section('console', console), section('billing', billing), section('account', account)])
  })

  const adminSections = computed((): QxNavSection[] => {
    const simple = authStore.isSimpleMode
    const sections = finalize([
      section('overview', [
        { path: '/admin/dashboard', label: t('nav.dashboard'), icon: 'dashboard' },
        { path: '/admin/ops', label: t('nav.ops'), icon: 'ops', featureFlag: flagOpsMonitoring },
        { path: '/admin/usage', label: t('nav.usage'), icon: 'usage' },
        { path: '/admin/audit-logs', label: t('nav.auditLogs'), icon: 'auditLog', hideInSimpleMode: true }
      ]),
      section('resources', [
        { path: '/admin/groups', label: t('nav.groups'), icon: 'folder' },
        {
          path: '/admin/channels',
          label: t('nav.channelManagement'),
          icon: 'channel',
          hideInSimpleMode: true,
          expandOnly: true,
          children: [
            { path: '/admin/channels/pricing', label: t('nav.channelPricing'), icon: 'priceTag' },
            { path: '/admin/channels/monitor', label: t('nav.channelMonitor'), icon: 'signal', featureFlag: flagChannelMonitor }
          ]
        },
        { path: '/admin/accounts', label: t('nav.accounts'), icon: 'globe' },
        { path: '/admin/proxies', label: t('nav.proxies'), icon: 'server' },
        { path: '/admin/plugins', label: t('nav.plugins'), icon: 'plugin', featureFlag: flagPluginManagement }
      ]),
      section('business', [
        { path: '/admin/users', label: t('nav.users'), icon: 'users', hideInSimpleMode: true },
        // 「仅充值」站点连管理端的「订阅管理」入口也一并收起（与上游一致）。
        { path: '/admin/subscriptions', label: t('nav.subscriptions'), icon: 'creditCard', hideInSimpleMode: true, featureFlag: flagSubscription },
        { path: '/admin/redeem', label: t('nav.redeemCodes'), icon: 'ticket', hideInSimpleMode: true },
        { path: '/admin/promo-codes', label: t('nav.promoCodes'), icon: 'gift', hideInSimpleMode: true },
        {
          path: '/admin/affiliates',
          label: t('nav.affiliateManagement'),
          icon: 'users',
          hideInSimpleMode: true,
          expandOnly: true,
          featureFlag: flagAffiliate,
          children: [
            { path: '/admin/affiliates/agents', label: t('nav.affiliateAgents'), icon: 'user' },
            { path: '/admin/affiliates/team', label: t('nav.affiliateTeam'), icon: 'usage' },
            { path: '/admin/affiliates/invites', label: t('nav.affiliateInviteRecords'), icon: 'userPlus' },
            { path: '/admin/affiliates/rebates', label: t('nav.affiliateRebateRecords'), icon: 'orders' },
            { path: '/admin/affiliates/transfers', label: t('nav.affiliateTransferRecords'), icon: 'swap' }
          ]
        },
        {
          path: '/admin/orders',
          label: t('nav.orderManagement'),
          icon: 'clipboard',
          hideInSimpleMode: true,
          expandOnly: true,
          featureFlag: flagAdminPayment,
          children: [
            { path: '/admin/orders/dashboard', label: t('nav.paymentDashboard'), icon: 'usage' },
            { path: '/admin/orders', label: t('nav.orderManagement'), icon: 'clipboard' },
            { path: '/admin/orders/plans', label: t('nav.paymentPlans'), icon: 'creditCard' }
          ]
        }
      ]),
      section('contentSecurity', [
        { path: '/admin/announcements', label: t('nav.announcements'), icon: 'bell' },
        {
          path: '/admin/security-audit',
          label: t('nav.securityAudit'),
          icon: 'shield',
          expandOnly: true,
          featureFlag: flagRiskControl,
          children: [
            { path: '/admin/risk-control', label: t('nav.contentModeration'), icon: 'shield' },
            { path: '/admin/prompt-audit', label: t('nav.promptAudit'), icon: 'eye' }
          ]
        }
      ]),
      section('system', [
        { path: '/admin/settings', label: t('nav.settings'), icon: 'cog' },
        ...customMenuItemsForAdmin.value
      ])
    ])

    if (simple) {
      // 简单模式：上游隐藏「我的账户」，把 API 密钥插到管理菜单里；这里单独成一段。
      sections.push(section('myAccount', [{ path: '/keys', label: t('nav.apiKeys'), icon: 'key' }]))
      return sections
    }
    const { console, billing, account } = buildSelfItems(false)
    const personal = finalize([section('myAccount', [...console, ...billing, ...account])])
    return [...sections, ...personal]
  })

  const sections = computed((): QxNavSection[] => {
    if (isAdmin.value) return adminSections.value
    // 后端模式下普通用户没有任何菜单（与上游一致）。
    if (appStore.backendModeEnabled) return []
    return userSections.value
  })

  return { sections, isAdmin, refreshBatchImageAccess }
}
