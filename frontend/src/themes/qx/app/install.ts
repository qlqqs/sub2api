/**
 * CUSTOM: QxAPI 全站主题开关。`main.ts` 在挂载前调用一次。
 * 主题开启时给 <html> 加 qx-app 类：tokens.css 替换 Tailwind 色阶变量，app.css 改官方公共类的样式，
 * pages/*.css 是非管理页面的极简线条样式（另需 QxShell 加的 qx-line 类）。
 * 关闭时两份样式表仍会加载，但所有规则都限定在 html.qx-app 下，不产生任何效果。
 */
import { qxThemeConfig } from '../config'
import './tokens.css'
import './app.css'
import '../pages/line.css'
import '../pages/pages-console.css'
import '../pages/pages-account.css'

export const QX_APP_CLASS = 'qx-app'

export function applyQxAppTheme(root: HTMLElement = document.documentElement): void {
  root.classList.toggle(QX_APP_CLASS, qxThemeConfig.enabled)
}
