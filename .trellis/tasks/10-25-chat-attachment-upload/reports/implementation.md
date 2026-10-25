# 对话附件修复与本机验收（2026-10-25）

## 实现
- Open WebUI fork `src/lib/embed/image-upload.ts`：FileReader promise、base64 本地解码、canvas.toBlob；无压缩/无尺寸变化复用原 File，不重新编码；转换后的 MIME 使用实际编码类型。
- `MessageInput.svelte`：图片选择/拖拽/粘贴共同入口及截图不再 fetch(data:)；读取/压缩失败明确 toast、可以重试。
- sub2api `limitChatBody`：files 路由请求体 26 MiB = 25 MiB 文件 + 1 MiB multipart 余量，其他请求 5 MiB 不变；fork 仍强制文件本身 25 MiB。
- 严格 CSP、HEIC 限制、模型能力校验、压缩设置和临时聊天内联语义均不改变。

## 检查
- `go test -tags unit ./...`（去掉代理与 OPENAI_API_KEY）：通过；`go build ./...` 通过。
- `vitest run src/lib/embed`：11 文件、76 测试通过，含新增 11 个附件转换用例。
- 新工具/测试的严格 TypeScript 检查通过；`embed:check` 无改动、通过。
- fork 后端嵌入测试：438 passed，2 个依赖弃用 warning。
- 两套本机镜像构建成功；`sub2api-local` 与 `sub2api-local-chat` healthy。
- 全量 `svelte-check` **未通过**：5694 errors / 188 warnings；离线基线（同工作树、仅 MessageInput 回到 HEAD）5696 errors / 188 warnings。按相对路径+诊断文本对照，新增诊断 0，移除旧诊断 2。不是宣称全量类型检查通过。
- 新工具/测试格式合规；MessageInput 全文件 Prettier 检查有一个原有 GatewayErrorHint 属性换行差异，与本改动无关，未为格式整理扩大改动。
- 两仓库 `git diff --check` 通过。未改锁文件。

## 浏览器测试说明
脚本：`scripts/upload-ui.cjs`，硬编码 loopback；使用本机 p9 的 CA，凭据仅取 sub2api-local/.env；不访问一号机，不保存明文凭据。
真实 Chromium → sub2api 控制台 → iframe → Open WebUI 文件服务；发送经过真实网关/本机模拟上游，不代表真实供应商模型的识图准确率测试。
截图用例通过 canvas MediaStream 模拟 OS 选屏，只验证截图按钮后的编码/上传链路；未自动操作操作系统选屏窗口。
早期测试脚本存在 Frame.reload 不存在、Chromium 对 25 MiB POST 的 DevTools response 缓存淘汰问题；已修脚本为 frame.goto 与浏览器 response.clone 观测，未绕过真实网络上传。
完整结果见后续 `browser-result.json`；截图保存在 `/home/qlqq/workspace/sub2api-local/e2e/runs/chat-upload-20261025-final/`。

## 复盘
- 根因：跨层 CSP 契约冲突 + 隐含「本地 data URL fetch 不受 connect-src 约束」假设 + 浏览器图片选择覆盖缺口。
- 之前 API 上传测试无法发现文件选择后、请求发出前的失败；本次用真实页面 CSP 覆盖入口和提交/回显。
- 同类截图转换一并修复；频道也存在同类上游实现，但嵌入模式禁用频道，本次不改禁用功能。
- 规范已更新：backend/quality-guidelines.md 的「嵌入对话附件请求体」，fork docs/sub2api-embed.md。

## 交付边界
用户在本机验收后明确批准「提交不推送」。本轮仅选择性本地提交两个仓库的附件修复；Open WebUI 提交 `e3c8a95`，sub2api 代码、测试、规范及本报告随本次提交保存。未推送、未部署一号机、未进行线上复测；任务保留以便后续继续。
当前工作树有其他任务的支付/文案修改；fork settings.py 也有预先存在的修改，均保留、不纳入本次提交范围。本机 build 按约定从完整工作树构建，线上提交须仅选本次文件。

## 最终浏览器结果
2026-10-25 完整一轮 **14/14 PASS**，CSP 违规 0，未捕获页面异常 0。覆盖 PNG/JPEG/WebP 原字节上传、拖拽、粘贴、压缩尺寸、读取失败重试、截图、PDF 文本提取、TXT+图片发送与刷新回显、临时聊天、25 MiB 边界、前后端超限拒绝。最终结果文件 `browser-result.json`。
