# Seedance 视频接入（mmgate）+ 分组分润系数 + Nova 内置视频插件

日期：2026-10-31。主代理负责规划和协调。实现分给 4 个子代理，测试由第 5 个子代理负责。全部在本机完成验证。
**不提交、不推送、不部署、不连接七号机**：这几步由主代理在用户同意后执行。

## 背景与已定决策

- 上游是 mmgate，走火山方舟（Ark）协议，基地址 `https://mmgate.cn/ark/api/v3`。
  - 文档：
    - https://mmgate.cn/docs/billing
    - https://mmgate.cn/docs/seedance/create
  - 网关的 Seedance 透传已在本地提交 `90bff42b5`（未推送）。相关代码：
    - `backend/internal/service/seedance.go`
    - `backend/internal/handler/seedance.go`
    - 路由 `/api/v3/contents/generations/tasks`
  - 透传规则：`content` 和 `omni_reference_task_type` 原样转发，所以 6 种生成方式在 API 层都已支持：
    - 文生视频
    - 图生视频
    - 首尾帧
    - 参考生成
    - 视频编辑
    - 视频延长
- 余额单位是 ¥，充值 1:1。
  - 建一个专用分组「Seedance 视频」，倍率 1，渠道「视频」按秒定价，单价直接填人民币：
    - SD2.0：¥0.75/s
    - SD2.5：¥1.125/s
  - 所有生成方式、有声无声、各分辨率都是同一单价。
- 计费按 mmgate 文档规则：
  - 请求里有参考视频时（多模态参考、视频编辑、视频延长）：
    计费秒数 = 成片秒数 + max(floor(参考视频总秒数), ceil(成片秒数 × 2/3))
  - 只有参考图或参考音频、没有参考视频时，按成片秒数计费。
  - 自动时长（`duration=-1` 或不传，仅 2.5 支持）：建任务时按模型最长时长预估，成功后按实际时长扣费。
  - 只有成功才扣费。
- 时长范围：
  - 2.0：4–15 秒
  - 2.5：4–30 秒，或自动
  - 计费秒数上限：成片 30 + 参考 30 = 60。
- 参考视频上限：
  - 2.0：最多 3 个，总时长 15 秒
  - 2.5：最多 10 个，总时长 30 秒
  - 参考视频必须是公网 URL（不支持 base64）。参考图可以用 base64（≤10MB），但本项目统一用 URL。
- 分润：
  - 生产配置：按消费结算返利，利润池 11%，二级代理 7%。
  - Seedance 毛利很薄（扣掉 3.2% 支付手续费后约 4.8%），所以新增**按分组设置的分润系数**：消费先乘以系数，再按原来的两级规则分润。
  - 未设置的分组按 100% 计算，系数 0 表示该分组不分润。
  - Seedance 分组计划设为 30%。
  - 只影响之后的结算，不回溯已结算的部分。
- Nova（创作工作台）：
  - 删除内置插件 `ccode-h3`，新增内置插件 `qx-seedance`，经 sub2api 网关调用。
  - 参考素材支持上传和公网链接两种方式。
  - 上传的素材通过签名的 24 小时公开链接给 mmgate 拉取。链接由 sub2api 在 `https://www.qlqqx.com` 上转发。
  - 生成结果下载到 Nova，视频任务保留 3 天，图片仍保留 7 天。

## 分工

| 代理 | 模型 | 仓库 | 任务书 |
|---|---|---|---|
| BE-S（后端 sub2api） | codex gpt-6.1-sol high | `/home/qlqq/workspace/sub2api` 的 `backend/` | `task-be-sub2api.md` |
| BE-N（后端 Nova） | codex gpt-6.1-sol high | `/home/qlqq/workspace/nova-image-studio` 的 `backend/`、`docker/`、`docs/` | `task-be-nova.md` |
| FE-S（前端 sub2api） | claude opus-5.5 high | `/home/qlqq/workspace/sub2api` 的 `frontend/` | `task-fe-sub2api.md` |
| FE-N（前端 Nova） | claude opus-5.5 high | `/home/qlqq/workspace/nova-image-studio` 的 `frontend/` | `task-fe-nova.md` |
| QA（全链路测试） | codex gpt-6.1-sol high | `/home/qlqq/workspace/sub2api-local*` 环境与 e2e | `task-test.md`（实现完成后启动） |

前后端之间的接口约定见 `contract.md`，所有代理以它为准。需要改约定时，先在任务书末尾的「约定变更」里写明，再通知主代理。

## 共同规则（每个代理必须遵守）

1. 开始前读取：
   - `/home/qlqq/workspace/AGENTS.md`
   - `/home/qlqq/workspace/sub2api/AGENTS.md`
   - 本目录下的 `prd.md`、`contract.md` 和自己的任务书
   - 对应层的 `.trellis/spec/`（sub2api）
   - Nova 的 `docs/sub2api-embed.md`、`docs/plugins/*.md`
2. 不 `git commit`、不 `git push`、不 `git stash`、不 `git reset`、不切换分支。工作区里已有别人的未提交改动，不要回滚。
3. 只改自己负责的目录。确实需要改对方目录时，先停下，在任务书末尾写明需求并通知主代理。
4. 不连接七号机或任何远端，不使用 `~/workspace/work/credentials.md`。
5. 禁止过度防御性编程：不为假设场景加校验、兜底、兼容分支或抽象。注释和文案要简洁，写当前事实，不堆砌警告。代码风格跟周边一致（sub2api 后端注释以中文为主，Seedance 文件里是英文，跟随所在文件）。
6. 工具约束：
   - 前端只用 `npx -y pnpm@9`，禁止用全局 pnpm。
   - Go 单测命令：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`
   - 禁止 `docker system prune` / `docker image prune`。端口只监听 127.0.0.1。
7. 完成时在自己的任务书末尾追加「完成报告」，写清楚：
   - 改了哪些文件、做了什么；
   - 跑了哪些测试，结果如何（失败要贴输出）；
   - 遗留问题和需要别的代理配合的地方。
