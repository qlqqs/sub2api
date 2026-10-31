# 浏览器 CSP 基线问题（本批未修）

## 现象与证据
正常缓存下切换语言会整页 reload。`/dashboard` 与 `/purchase?tab=subscription` 的内联 `window.__APP_CONFIG__` 脚本出现 `script-src-elem` 拒绝；未出现 unsafe-eval 需求或消息编译异常。

`csp-304-baseline-probe.json` 记录同一路径：首次200的HTML nonce与随后304的CSP头nonce不同。浏览器沿用缓存HTML，故旧nonce不能通过新CSP头。

## 根因与范围
`backend/internal/server/middleware/security_headers.go` 每请求生成nonce；`backend/internal/web/embed_on.go:serveIndexHTML` 命中ETag直接返回304，而不发送本次nonce对应HTML。`LocaleSwitcher.vue` 的语言切换reload触发此路径。

三个文件在候选与custom基线HEAD完全相同，均不属于六项补丁。问题是既有HTTP缓存与响应头契约不一致，非vue-i18n v11新增的动态执行。

## 对照与后续
- 普通缓存浏览器验收保留原始CSP失败，不忽略或加入例外。
- 另用CDP禁用HTTP缓存运行同一脚本，仅用于区分依赖升级和304路径；此对照不替代真实缓存场景验收。
- 本批不修改服务端缓存、CSP或语言切换机制。独立修复时应保证HTML与CSP nonce配套，并覆盖首次200、ETag重验证和真实浏览器reload；不能通过开放unsafe-inline/unsafe-eval解决。

## 脚本修正记录
前三次探索运行依次被新手引导遮罩、未进入订阅页签、空搜索provider导致开关按既有规则隐藏所中断。修正验收脚本的环境准备：按已有流程Esc关闭引导、进入`?tab=subscription`、添加不会调用外网的临时占位provider。均未修改产品代码；临时套餐、分组、渠道和配置在finally清理/恢复。原始日志保留为browser-attempt1/2/3，最终两轮见browser-normal与browser-nocache。
