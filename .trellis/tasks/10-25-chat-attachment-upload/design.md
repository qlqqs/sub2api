# 变更边界

行为差距在 Open WebUI 的 MessageInput.svelte 图片预处理：fetch(data:) 不兼容 connect-src self。
- fork 新增 src/lib/embed/image-upload.ts 与单测：FileReader Promise、base64 data URL 本地解码、canvas.toBlob；原文件未压缩时不重新编码。
- MessageInput.svelte：两个 fetch(data:) 替换为上述工具；图片流程捕获失败并使用现有翻译提示。不修改压缩尺寸策略或禁用的频道。
- sub2api chat_proxy.go/test：文件路由请求上限 26 MiB（25 MiB 文件 + 1 MiB 表单余量），其他请求维持 5 MiB；后端文件校验保持 25 MiB。
- 文档同步新契约，本机 e2e 增加回归脚本与报告。
- 不放宽 CSP、不引入依赖、不改变 HEIC/模型支持范围、不触碰其他任务未提交内容、不自行提交推送部署。
