/**
 * CUSTOM: QxAPI 主题（fork 自有）。
 *
 * 首页与登录相关页面（AuthLayout）换成本主题，官方组件文件保持原样，方便合并上游：
 * - `router/index.ts` 的首页路由指向 `QxHomeView.vue`；
 * - `components/layout/index.ts` 的 `AuthLayout` 指向 `QxAuthLayout.vue`；
 * - `main.ts` 调用 `app/install.ts` 给 <html> 加 qx-app，全站（控制台、管理后台）换成同一套配色；
 * - `router/index.ts` 的 `/dashboard` 指向 `dashboard/QxDashboardView.vue`。
 * `enabled = false` 时各入口直接渲染上游原版组件，全站样式不生效。
 */

export type QxProtocol = 'openai' | 'anthropic'

export const qxThemeConfig: {
  enabled: boolean
  /**
   * 本站对外提供的接口协议，决定首页的调用示例和接入地址。
   * 模型广场接口不暴露 OpenAI 分组的 messages 调度开关，所以在这里声明。
   */
  protocols: readonly QxProtocol[]
} = {
  enabled: true,
  protocols: ['openai', 'anthropic']
}

export const QX_PROTOCOL_LABELS: Record<QxProtocol, string> = {
  openai: 'OpenAI',
  anthropic: 'Anthropic'
}

/** 各协议的代表客户端，用于首页文案。 */
export const QX_PROTOCOL_TOOLS: Record<QxProtocol, string> = {
  openai: 'Codex',
  anthropic: 'Claude Code'
}

/**
 * 各协议填入客户端的接口地址：OpenAI 客户端（含 Codex）需要带 /v1，
 * Anthropic 客户端会自行拼接 /v1/messages（与 UseKeyModal 一致）。
 */
export function qxProtocolEndpoint(protocol: QxProtocol, origin: string): string {
  const root = origin.replace(/\/+$/, '').replace(/\/v1$/, '')
  return protocol === 'openai' ? `${root}/v1` : root
}

/** 各协议的主要接口路径，显示在首页接入流程图底部。 */
export const QX_PROTOCOL_PATHS: Record<QxProtocol, readonly string[]> = {
  openai: ['/v1/responses', '/v1/chat/completions'],
  anthropic: ['/v1/messages']
}
