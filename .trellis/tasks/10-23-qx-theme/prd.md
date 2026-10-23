# QxAPI 独立主题：首页与登录页

## 目标

一号机（站名 QxAPI）的首页与登录相关页面不再与其他 sub2api 站撞脸，同时保持可以顺畅合并上游。

## 已确认的设计（v3 demo）

- 简洁、带一点科技感：白底、中性石墨灰文字、logo 亮蓝 `#1D4EFF` 作主色，logo 青色只用于细线点缀；首屏淡点阵底纹、代码窗口、等宽数字、1px 细线；无光斑、无渐变背景、无入场动画。
- 支持深色模式。
- demo 与截图：`/tmp/qxapi-demo/home-v3.html`、`/tmp/qxapi-shots/v3-*.png`（临时目录）。

## 内容规则

- 价格只来自模型广场接口 `/api/v1/model-plaza`，与模型广场显示一致（单价 × 生效倍率，¥，分档），页面不出现倍率或折扣字样。
- 可能有多个分组：多于一个分组时显示分组标签切换。
- 兼容协议：OpenAI 与 Anthropic（OpenAI 分组开启了 messages 调度）。模型广场接口不暴露该开关，因此协议列表是主题配置项。
- 后台设置了自定义首页内容（home_content）或简洁首页（compact_home_enabled）时，仍渲染上游原版首页。

## 上游同步约束

- 新代码全部放在 fork 自有目录 `frontend/src/themes/qx/`。
- 上游文件只改接入点：
  - `router/index.ts`：首页路由组件指向主题入口；
  - `components/layout/index.ts`：`AuthLayout` 导出指向主题外壳；
  - `tailwind.config.js`：primary 改为 CSS 变量，默认值仍为原青色；
  - `i18n/locales/{zh,en}/index.ts`：引入主题文案。
- 主题开关在 `themes/qx/config.ts`，关闭后两个入口直接渲染上游原版组件。

## 验收

- 前端 lint / typecheck / vitest 通过，`git status` 无锁文件改动。
- 本机环境 `./build.sh` 后用 Playwright 截图首页、登录页（浅色、深色、手机）。
- 其他页面（控制台、后台）外观不变。
