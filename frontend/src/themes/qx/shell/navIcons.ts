/**
 * CUSTOM: QxAPI 侧边栏图标表。能用 `@/components/icons/Icon.vue` 的就用它的图标名；
 * Icon.vue 里没有的（相机、文件夹、渠道、信号、价签、票券、充值）从上游 AppSidebar.vue 抄路径，
 * 充值换成与演示稿一致的线框货币符号。
 */
import type Icon from '@/components/icons/Icon.vue'

export type IconName = InstanceType<typeof Icon>['$props']['name']

export type QxNavIconDef = { icon: IconName } | { paths: readonly string[] }

export const QX_NAV_ICONS = {
  dashboard: { icon: 'grid' },
  key: { icon: 'key' },
  chat: { icon: 'chat' },
  studio: { icon: 'sparkles' },
  usage: { icon: 'chart' },
  ops: { icon: 'trendingUp' },
  auditLog: { icon: 'document' },
  globe: { icon: 'globe' },
  server: { icon: 'server' },
  plugin: { icon: 'cube' },
  users: { icon: 'users' },
  user: { icon: 'user' },
  userPlus: { icon: 'userPlus' },
  creditCard: { icon: 'creditCard' },
  gift: { icon: 'gift' },
  orders: { icon: 'document' },
  clipboard: { icon: 'clipboard' },
  ledger: { icon: 'dollar' }, // CUSTOM: 财务明细
  swap: { icon: 'swap' },
  bell: { icon: 'bell' },
  shield: { icon: 'shield' },
  eye: { icon: 'eye' },
  cog: { icon: 'cog' },
  link: { icon: 'link' },
  download: { icon: 'download' },
  batchImage: {
    paths: [
      'M6.827 6.175A2.31 2.31 0 015.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 00-1.134-.175 2.31 2.31 0 01-1.64-1.055l-.822-1.316a2.25 2.25 0 00-1.906-1.059H9.554a2.25 2.25 0 00-1.906 1.059l-.821 1.316z',
      'M16.5 12.75a4.5 4.5 0 11-9 0 4.5 4.5 0 019 0zM18.75 10.5h.008v.008h-.008V10.5z'
    ]
  },
  folder: {
    paths: [
      'M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z'
    ]
  },
  channel: {
    paths: [
      'M6.429 9.75L2.25 12l4.179 2.25m0-4.5l5.571 3 5.571-3m-11.142 0L2.25 7.5 12 2.25l9.75 5.25-4.179 2.25m0 0l4.179 2.25L12 17.25 2.25 12m15.321-2.25l4.179 2.25L12 17.25l-9.75-5.25'
    ]
  },
  signal: {
    paths: [
      'M9.348 14.651a3.75 3.75 0 010-5.303m5.304 0a3.75 3.75 0 010 5.303m-7.425 2.122a6.75 6.75 0 010-9.546m9.546 0a6.75 6.75 0 010 9.546M5.106 18.894c-3.808-3.807-3.808-9.98 0-13.788m13.788 0c3.808 3.807 3.808 9.98 0 13.788M12 12h.008v.008H12V12zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z'
    ]
  },
  priceTag: {
    paths: [
      'M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z',
      'M6 6h.008v.008H6V6z'
    ]
  },
  ticket: {
    paths: [
      'M16.5 6v.75m0 3v.75m0 3v.75m0 3V18m-9-5.25h5.25M7.5 15h3M3.375 5.25c-.621 0-1.125.504-1.125 1.125v3.026a2.999 2.999 0 010 5.198v3.026c0 .621.504 1.125 1.125 1.125h17.25c.621 0 1.125-.504 1.125-1.125v-3.026a2.999 2.999 0 010-5.198V6.375c0-.621-.504-1.125-1.125-1.125H3.375z'
    ]
  },
  recharge: {
    paths: ['M21 12a9 9 0 11-18 0 9 9 0 0118 0z', 'M9 7.5l3 4 3-4M9 12.5h6M9 15.5h6M12 11.5v6']
  }
} as const satisfies Record<string, QxNavIconDef>

export type QxNavIconName = keyof typeof QX_NAV_ICONS
