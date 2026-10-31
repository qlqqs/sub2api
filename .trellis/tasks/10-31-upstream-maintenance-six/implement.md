# 实施清单

## Phase 1 — 规划
- [x] 用户选定六项；建立父任务下独立批次。
- [x] 读取上游差异、当前实现与既有工作流；完成PRD收敛和设计。
- [x] 固定六项源SHA，保存规划基线。
- [x] 最终摘要获用户“开始”确认，已执行task.py start。

## Phase 2 — 候选实施
- [x] 重读状态、refs/worktrees、spec和台账，记录实施前指纹；核对无进行中Git操作。
- [x] 为本批建立独立候选；满足镜像工作区门禁时只ff本地main，不推送。
- [x] 按源补丁应用R1、R3–R6；审查依赖，补singleflight确定性回归及必要协议回归。
- [x] 验证首组后适配R2，保留自定义Node pin、CI和套餐断言；只用pnpm9。
- [x] 核对全体文件范围、无迁移/版本号/其他补丁混入。

## Phase 2 — 验证
后端命令去除 http_proxy/https_proxy/HTTP_PROXY/HTTPS_PROXY/ALL_PROXY/all_proxy/OPENAI_API_KEY；以下均从候选对应目录运行并记录exit code。
- [x] go mod verify；定向单测覆盖管理员、渠道合并、缓存、OAuth非流式、compact、响应名/citations/图片与usage。
- [x] go test -tags unit ./...；相关缓存并发测试-race；go build ./...；go vet ./...。
- [x] govulncheck / golangci-lint 本机未安装，未执行；Go模块校验、unit、race、build、vet通过。
- [x] CI=true npx -y pnpm@9 install --frozen-lockfile（若需重新生成锁仅用pnpm9，先审查差异）。
- [x] npx -y pnpm@9 run lint:check / typecheck / test:run；pnpm9 audit --prod --audit-level=high，逐项核对本批漏洞与存量问题，不扩大或延长例外。
- [x] 确认本机共享构建/e2e未被其他任务使用，SUB2API_REPO=<候选> /home/qlqq/workspace/sub2api-local/build.sh；该目录docker compose up -d。
- [x] 本机完整e2e，Nova/Chat按实际运行状态记录执行或跳过；既有FIN-24若仍失败核对基线与证据。
- [x] 浏览器验收Qx首页/登录/用户/后台、套餐文字、中英文切换、渠道搜索保存，捕获console和CSP违规；复用本机测试账号及模拟上游。
- [x] Trellis check：检查spec一致性、定制跨层契约、锁文件和构建产物范围。

## Phase 3 — 本地交付与记录
- [x] 核对custom HEAD和目标文件未变化，仅移入本批文件；变化则重新整合复验；保护已有embedded-workbench-guide.md。
- [x] 更新父任务state/追加decisions，记录六项状态、11单元范围处置、指纹和测试结果，不将未提交标integrated。
- [x] 更新本批验证报告、implement/task状态及父任务批次入口；journal使用--no-commit且--commit -。
- [x] 检查需更新的规范；生产记录不变。交付实际通过/失败/跳过和待办，等待提交/推送/部署授权。

## 当前状态
候选21个产品文件已完成，模块校验、后端定向/全量unit/race/build/vet、前端3524测试/lint/typecheck及本机生产构建通过。原始前端audit及例外策略均exit1；剩余follow-redirects和xlsx为基线已存在，未扩大范围。完整e2e442/442通过，浏览器功能通过；正常缓存CSP检查失败，禁用缓存对照36/36通过。21文件已回灌并核对指纹。未提交、推送或部署。

## 执行补充
- 前端worker在独立依赖安装下先通过R4/套餐15项基线，再实施R2，与后端检查并行；最终统一验证六项，未额外创建提交。
- 首次后端定向测试在下载依赖期间中止，先按宿主网络配置预取依赖后重跑；真实测试全部去除代理及OPENAI_API_KEY。
- 新增图片计数回归夹具最初缺output_item.done，导致incomplete/cancelled计数断言失败；补齐“图片已完成后整轮终止”的事件序列，未改生产计数语义。原始失败日志保留。
- 缓存负向对照通过go overlay使用HEAD旧实现；新增测试如期失败，候选实现通过，未改动正在验证的文件。

