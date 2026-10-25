# 阶段 3：Open WebUI fork 镜像与文档报告

日期：2026-10-24（UTC）  
仓库：`/home/qlqq/workspace/open-webui`，分支 `sub2api-custom`  
构建时 HEAD：`91ea4e3421d1d8069f13d4a463fcb5986d94ea60`；**构建包含当前未提交的阶段 1/2 工作区补丁，不等同于该 HEAD 的干净快照**。

## 结论

**阶段 3 的本地构建和 internal 网络运行验证门通过。**

- 完整 Docker 多阶段构建两次成功：首次 **241 秒**，纳入阶段 2 后续改动后再构建 **195 秒**。首次解包后镜像 **991 MB**；第二次 image store 内容尺寸约 **220 MB**（不同口径，见 §4.6）。
- 两次均新建/清理独立 internal 网络，UID/GID 1000，容器启动后约 **16 秒达到 healthy**；两轮分别观察 **153 秒 / 约112秒**，结束仍 healthy，无端口映射、只接一个 internal 网络。
- 容器内 favicon 200、index 200、外置 `/_app/` 脚本 200；index **0 内联脚本 / 0 内联事件属性**，前端 **0 sourcemap**。
- 启动日志没有外网请求失败、连接超时、模型/编码下载记录；数据/用户目录没有模型缓存文件。
- codemod 幂等通过，embed Vitest **55 passed**；后端 pytest 两次均 **301 passed**，第二次模拟 CI 不存在预构建 build 目录的静态资源来源。
- 已删除本次 `owui-embed-verify` 容器、`owui-embed-verify-net` 网络及 `owui-embed-verify:local` 镜像标签；没有全局清理，没有操作任何 `sub2api-*` Docker 资源。
- 没有提交、推送、创建远端仓库、执行远端 workflow 或部署生产；未读取禁止访问的凭据文件。

这不是阶段 6 的 sub2api 全链路验收，也未声称前端浏览器/CSP/Artifacts、真实计费或生产代理白名单全部通过。

## 1. 文件清单

本阶段只写入以下五个 fork 文件，加本报告：

| 文件 | 内容 |
|---|---|
| `docker/sub2api-embed.Dockerfile` | Node 前端、Python slim 依赖、非 root slim 运行时三阶段 |
| `.dockerignore` | 补排除 build、递归依赖/venv/cache、环境文件、`.webui_secret_key` 和 backend/data |
| `.github/workflows/sub2api-embed-image.yml` | 自定义分支测试门及 GHCR amd64 镜像发布 |
| `docs/sub2api-embed.md` | contract A–G、后端约束、实际 Artifact 协议、CSP、环境、网络/代理、compose、品牌许可与维护 |
| `scripts/rebase-upstream.md` | tag → 重放手写补丁 → 重生成 codemod → 测试/构建/e2e，补丁提交划分及 grep 核对 |

没有修改 backend/src/static、embed 脚本、package 文件或上游 workflow。工作区原有大量其他阶段改动保持不动。
初次实现时阶段 2 报告及协议文档均不存在，先按 `package.json`、`svelte.config.js`、`scripts/embed-*.mjs`、`src/lib/embed/artifact.ts` / `ArtifactFrame.svelte`、`static/embed/artifact-frame.html` 落实。收尾时 `docs/sub2api-embed-frontend.md` 新增，已完整读取并对齐引用/合并：Artifact 协议一致，补充续期 8 秒/失败 403 的处理、未使用 IndexedDB 且不清其他应用数据库的边界、codemod 重放的依赖顺序。未修改该前端文档。阶段 2 的后续源码改动触发本阶段第二次镜像构建和独立网络验证，详见下文。

## 2. Dockerfile 要点与默认值来源

### 构建链

1. `node:22-bookworm-slim`：`ONNXRUNTIME_NODE_INSTALL_CUDA=skip CYPRESS_INSTALL_BINARY=0 npm ci --no-audit --no-fund`。
2. `OWUI_BASE_PATH=/_chat`、Node heap 8192 MB；`npm run crypto:verify` → `npm exec -- vite build` → `node scripts/embed-postbuild.mjs` → 删除所有 `build/**/*.map`。不运行普通 `npm run build` / `pyodide:fetch`。
3. `python:3.12-slim-bookworm` 依赖阶段：独立 `/opt/venv`，安装原版 `backend/requirements-slim.txt` 与 `pypdf==6.7.5 docx2txt==0.9`；两个 loader 版本直接取本仓库 `backend/requirements.txt`。Python 3.12 与本机已验证 slim venv/薄镜像一致。
4. 独立 Python slim 运行时，复制 venv、后端、构建产物、package/CHANGELOG 和 LICENSE/HISTORY/NOTICE。不带 Node、CUDA、Ollama；不调用 embedding/whisper/tiktoken 预下载函数。
5. `USER 1000:1000`、`HOME=/home/app`；运行时代码/静态及数据目录有相应所有权。`CMD ["bash","start.sh"]`，由阶段 1 的脚本强制 `--no-proxy-headers`、拒绝 `--root-path`。
6. Python urllib HEALTHCHECK，15 秒间隔、5 秒超时、60 秒启动宽限、5 次重试；只查 `http://127.0.0.1:$PORT/static/favicon.png`，显式关闭代理，**不用 `/health`**。

### 默认值及来源

完整逐项清单可直接审计 Dockerfile `ENV` 与 `docs/sub2api-embed.md` §7；来源划分如下：

| 默认值组 | 值 / 来源 |
|---|---|
| S2A 契约 | `S2A_EMBED_MODE=1`；`S2A_GATEWAY_URL=http://sub2api:8080`；`S2A_TRUSTED_PROXY_CIDRS=''`（部署必填、空值 fail closed）；`S2A_HIDE_BRANDING=0`；`S2A_NATIVE_WEB_SEARCH=true`；`S2A_BUILTIN_TOOL_CATEGORIES=time,user_input,files,chats,notes,tasks,calendar,image_generation`；`S2A_SLIM_DOC_LOADERS=true`；`S2A_IMAGE_MAX_N=4`。依据 **contract G 最新版本**，不是 stage1 初稿中缺少 image_generation 的旧列表。 |
| 强制安全开关 | 全部取 `backend/open_webui/embed/settings.py::configure_environment()` 与 stage1 的“强制开关”章节：OFFLINE_MODE/WEBUI_AUTH/ENABLE_PASSWORD_AUTH/OPENAI_API/全文附件/插件/生图/编辑/标题为 true；认证独立入口、更新、直连、自建搜索、音频相关权限、记忆/代码/频道/自动化/子代理/分享/管理数据旁路/遥测等强制 false；workspace 用户权限逐项 false。Dockerfile 全量明示，文档列出每个变量名。 |
| 固定非布尔值 | 三个可信头、`DEFAULT_USER_ROLE=user`、`TERMINAL_SERVER_CONNECTIONS=[]`、`CONTENT_EXTRACTION_ENGINE=''`、`PDF_EXTRACT_IMAGES=false`、`AIOHTTP_CLIENT_ALLOW_REDIRECTS=false`、`OPENAI_API_CONFIGS={}`、生图/编辑 engine=openai，均来自 settings.py。OPENAI URL(S)=默认网关 `/v1`，OLLAMA_BASE_URL=默认网关以避免原版启动探测；代码启动时会根据实际 S2A_GATEWAY_URL 重新派生，不能靠 UI 修改。 |
| 可覆盖业务默认 | `IMAGE_GENERATION_MODEL=IMAGE_EDIT_MODEL=gpt-image-1`；工具循环上限 20；`RAG_FILE_MAX_SIZE=25`；完整扩展名列表同 settings.py 的 setdefault。 |
| 容器运行 | `USE_SLIM_DOCKER=true`、CUDA/Ollama false、单 worker、zh-CN，来源 design §8 / stage1；`ENV=prod DOCKER=true PORT=8080 HOST=0.0.0.0` 为上游容器约定；`DATA_DIR=/app/backend/data FRONTEND_BUILD_DIR=/app/build HOME=/home/app PATH=/opt/venv/bin:$PATH` 为本镜像路径。 |
| 日志、遥测及离线防御 | `PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1`；上游镜像已有的 `SCARF_NO_ANALYTICS=true DO_NOT_TRACK=true ANONYMIZED_TELEMETRY=false`；额外 `HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_HUB_DISABLE_TELEMETRY=1`，不替代 internal 网络。 |
| 构建元数据 | `BUILD_HASH` → 前端 APP_BUILD_HASH / 后端 WEBUI_BUILD_HASH / OCI revision；APP_BUILD_CHANNEL=unknown。无 .git 上下文依赖。 |
| 部署必须另给 | `WEBUI_SECRET_KEY`（稳定随机秘密）、`WEBUI_URL`、实际 `CORS_ALLOW_ORIGIN`、可信 proxy CIDR；可选白名单 HTTP(S)_PROXY 与 NO_PROXY。**这些秘密/代理值不写进镜像。** |

通过 image inspect 的 ENV 与在空进程环境运行 configure_environment 的结果比对：**90 个强制/默认值一致**（不计故意不写镜像的空 OPENAI_API_KEY(S)）。代码会强制清空配置中的 Key；镜像不设 WEBUI_SECRET_KEY、OPENAI_API_KEY(S)、HTTP_PROXY/HTTPS_PROXY。

公开镜像默认保留 Open WebUI 品牌；`LICENSE`、`LICENSE_HISTORY`、`LICENSE_NOTICE` 在 `/app/`，实测三个文件 SHA-256 与仓库一致。OCI license 标识为 `LicenseRef-Open-WebUI`，未误标纯 BSD/MIT。文档复述滚动 30 天 ≤50 自然人的品牌例外、书面/企业授权、分发同样受约束与超过门槛恢复品牌要求；设置页版权仍由前端负责保留。

## 3. GitHub Actions

参照 Nova 的 Custom Image 流程实现：
- `push.branches: [sub2api-custom]`，另支持 `workflow_dispatch`，job 限定 `qlqqs/open-webui` 及该分支，避免其他分支手动覆盖 latest。
- 后端 pytest + 前端 crypto 校验/codemod --check/embed Vitest，全部通过才发布。
- 后端测试无需完整前端编译；显式 `FRONTEND_BUILD_DIR=${{ github.workspace }}`，使测试从已跟踪的 `static/` 复制临时探针资源。**否则 clean checkout 没有 build/，stage1 smoke 的 favicon 断言可能失败。** 已用同等环境复跑通过。
- linux/amd64，Buildx + GHA cache；使用 GITHUB_TOKEN 登录 GHCR。
- `ghcr.io/qlqqs/open-webui:custom-latest` 和 `custom-<7位SHA>`。
- 默认 contents:read，仅 image job packages:write；不需要新增 DockerHub/Helm/PyPI secrets。
- YAML 结构解析与关键字段断言通过。实际 GitHub runner/仓库权限、GHCR 可见性尚未远端验证，首次发布需仓库管理员确认。

## 4. 本机构建与运行验证

### 4.1 首次构建

构建前 `df -h /`：剩余 **30G >10G**。Docker Engine 29.1.3；未调整 daemon 配置。

```bash
cd /home/qlqq/workspace/open-webui
df -h /
docker build -f docker/sub2api-embed.Dockerfile -t owui-embed-verify:local \
  --build-arg HTTP_PROXY="${HTTP_PROXY:-${http_proxy:-}}" \
  --build-arg HTTPS_PROXY="${HTTPS_PROXY:-${https_proxy:-}}" \
  --build-arg BUILD_HASH="$(git rev-parse HEAD)" .
```

- 2026-10-24 **18:48:54–18:52:55 UTC**，退出码 0，241 秒。
- 基础镜像解析/拉取、npm/pip 下载均成功，没有 daemon 拉取失败阻塞。
- build context **53.99 MB**；宿主 `node_modules` 约 1.3G、`build` 约 180M，均未送入上下文。
- postbuild：外置 3 段 inline script，remaining inline scripts=0 / handlers=0。
- `docker image ls` **991MB**；本机 containerd image store 下 `image inspect .Size=220018643 bytes`（约 220MB，为该 store 的另一尺寸口径，不能当成解包运行体积）。
- 本地镜像 ID / manifest list digest：`sha256:c08be2dcb717b0a41285be733ae7714f2ded2e617b4f3cf2e13f9cb7380576e6`，非 GHCR 推送 digest。
- 首次 build 日志现保存在 `/tmp/owui-embed-first-pass/owui-embed-build.log`；常规 `/tmp/owui-embed-build.log` 已更新为第二次构建，见 §4.6。

### 4.2 首次新 internal 网络运行

执行前检查两个名字都不存在，拒绝复用任何同名既有资源。随机秘密通过临时 shell 环境传入，无日志明文。

```bash
docker network create --internal owui-embed-verify-net
export WEBUI_SECRET_KEY="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
docker run -d --name owui-embed-verify --network owui-embed-verify-net \
  -e WEBUI_SECRET_KEY \
  -e S2A_GATEWAY_URL=http://127.0.0.1:9 \
  -e S2A_TRUSTED_PROXY_CIDRS=127.0.0.1/32 \
  -e WEBUI_URL=http://localhost/_chat \
  -e CORS_ALLOW_ORIGIN=http://localhost \
  owui-embed-verify:local
unset WEBUI_SECRET_KEY
docker inspect --format '{{.State.Health.Status}}' owui-embed-verify
docker logs --timestamps owui-embed-verify
```

结果：
- started `18:54:02.219 UTC`，约 16 秒 healthy；观察至 **18:56:35 UTC**，153 秒持续 healthy。
- 无启动异常；首次空库迁移有一条上游 SQLAlchemy `SAWarning`（tag 表历史迁移主键定义），**不是外网错误**。
- 日志文字“Installing external dependencies…”之后立即明确“ENABLE_PIP_INSTALL_FRONTMATTER_REQUIREMENTS is disabled, skipping…”，未实际 pip 安装。
- 唯一 `https://` 日志为 Open WebUI 启动 banner 中的项目地址，不是 HTTP 请求日志。
- 无外网请求失败/超时、embedding/whisper/tiktoken 下载或版本检查失败。数据目录仅 SQLite DB/WAL/SHM，home 中仅默认 shell 配置；没有模型缓存。
- 网络 inspect：`internal=true containers=1`；容器 ports={}、networks=1；无卷挂载到现有项目，无宿主端口发布。

### 4.3 容器内产物检查

执行 `docker exec -i owui-embed-verify python -`，Python urllib 禁用代理请求 favicon、带 `Accept: text/html` 请求 `/`，HTMLParser 计数 script/event 属性，再从 index 提取一个 `/_chat/_app/...js`，剥 `/_chat` 请求后端根路径。等价探针：

```bash
docker exec owui-embed-verify python -c \
  "import urllib.request; r=urllib.request.build_opener(urllib.request.ProxyHandler({})).open('http://127.0.0.1:8080/static/favicon.png', timeout=4); print(r.status)"
```

| 检查 | 结果 |
|---|---|
| UID/GID | 1000:1000 |
| `/static/favicon.png` | 200，21666 bytes，PNG magic 正确 |
| `/` HTML | 200 |
| inline script / inline event handler | 0 / 0 |
| 外链脚本 | loader.js + 3 个 `/_chat/_app/immutable/inline/*.js` |
| 取实际外置脚本（后端根路径） | 200，372 bytes |
| `/app/build/_app` | 存在，921 个文件 |
| `.map` | 0 |
| `/app/build/embed/artifact-frame.html` | 存在（其自身有独立 CSP，故不外置沙箱脚本） |
| 匿名 `/health` | 401，未放宽鉴权 |
| pypdf / docx2txt | 已安装 |
| torch / sentence_transformers / faster_whisper / chromadb | 未安装 |
| `/app/node_modules` / 后端 `.webui_secret_key` | 不存在 |
| LICENSE/HISTORY/NOTICE | 与仓库 SHA-256 一致 |

详细证据：`/tmp/owui-embed-runtime-check.json`、`/tmp/owui-embed-runtime.log`、`/tmp/owui-embed-health.json`、`/tmp/owui-embed-data-check.txt`、`/tmp/owui-embed-final-status.txt`。
日志观察不等同于完整抓包或对恶意插件的出站审计；本阶段未接入真实网关、出站代理或任何用户流量。

### 4.4 额外测试

```bash
node scripts/embed-codemod.mjs --check
npm run test:embed
env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy \
  -u OPENAI_API_KEY -u OPENAI_API_KEYS \
  FRONTEND_BUILD_DIR="$PWD" PYTHONDONTWRITEBYTECODE=1 \
  /home/qlqq/workspace/owui-poc/venv/bin/python -m pytest \
  -p no:cacheprovider -q backend/open_webui/test/embed
git diff --check -- .dockerignore docker .github/workflows/sub2api-embed-image.yml \
  docs/sub2api-embed.md scripts/rebase-upstream.md
```

- codemod：files changed=0。
- Vitest：8 文件 / **55 tests passed**，初轮1.71秒；前端后续改动后二次复跑 **55 passed / 1.84秒**。
- pytest：首次默认前端产物路径 **301 passed / 56.10 秒**；第二次 clean-CI 静态来源路径 **301 passed / 59.43 秒**。均有 2 条已知 Typer/Click 弃用 warning。
- 日志 `/tmp/owui-embed-{codemod,vitest,pytest,pytest-clean-static}.log`。
- 新 workflow YAML 使用 Python YAML BaseLoader 解析并断言 trigger/needs/platform，通过；未安装/声称运行 actionlint。
- standalone compose 示例经 `docker compose config --quiet` 校验通过（仅解析，没有启动服务）；两条 CSP 字符串与 contract B.6 逐字一致。
- 本阶段没有运行 sub2api e2e、真实浏览器验收或全仓库类型检查。

### 4.5 清理

先保存证据并核对资源 ID 与本次创建记录一致，再执行：

```bash
docker stop -t 20 owui-embed-verify
docker rm owui-embed-verify
docker network rm owui-embed-verify-net
docker image rm owui-embed-verify:local
```

随后 inspect 三种资源均确认不存在。日志 `/tmp/owui-embed-cleanup.log`。
保留 Docker 基础层和构建缓存（不运行任何全局 prune），根分区最终剩余约 **25G**。
没有操作已有 `sub2api-*` 或其他项目的容器、网络、卷、镜像。

### 4.6 阶段 2 并发改动后的第二次验证（最终镜像快照）

初次验证后发现阶段 2 新增前端协议文档，并修改 `src/lib/components/chat/Chat.svelte`、`src/lib/embed/app.ts`、`static/embed/model-avatar.svg`。已经读取前端文档并再次执行同一完整 Dockerfile 构建（依赖层命中缓存，前端从源码重新构建），不是直接沿用旧镜像：

- 构建：**19:00:44–19:03:59 UTC，195秒，退出码0**；日志 `/tmp/owui-embed-build.log`。
- 第二次镜像：`sha256:25d0e8b0092bd146381ef17bf9f0667756e79cb60b3af6c5709720dd656883fc`。
- image inspect `.Size=220020818 bytes`；监测脚本在 BuildKit 标签出现、解包仍进行时读到 `image ls=220MB`。这是内容尺寸/解包时机差异，**不能将它描述为把第一次991MB优化成220MB**。第一次完整解包后的991MB仍是可用的本机占用参考。
- 第二次新建同名但全新的 internal 网络/容器，未复用首次资源；相同随机-secret/不可达网关/受限CIDR配置。
- 验证脚本从 docker run 前 `19:03:57` 开始计时（包含等待 BuildKit 解包），到 healthy **25秒**；容器实际 StartedAt=`19:04:05.886 UTC`，即真正启动后约 **16秒** healthy。
- 持续检查至 **19:05:58 UTC**，从容器实际启动算约 **112秒**；healthy、internal=true、ports={}、networks=1。
- §4.3 全部断言再次通过：favicon/index/_app脚本200、UID1000、内联脚本/事件0、sourcemap0、921个_app文件、Artifact页和文档加载器存在、匿名health401、许可证哈希不变。
- 与第一次一样只有历史迁移 SAWarning；无外网失败/超时/模型下载。默认启动脚本未经替换，没有注入测试 monkeypatch。
- 第二次的探针结果/日志覆盖 `/tmp/owui-embed-runtime-check.json`、`runtime.log`、`health.json` 等；首次证据保存在 `/tmp/owui-embed-first-pass/`。可复查本次临时验证脚本 `/tmp/owui-stage3-verify.sh`。
- 最终再次删除并确认容器、网络、镜像标签不存在，`/tmp/owui-embed-second-verify.log` 与 `/tmp/owui-embed-cleanup.log` 留有结果。
- 最后检查未发现第二次构建开始后 src/static/embed脚本的新修改；`reports/stage2.md` 此时仍未生成，未声称读过不存在的最终报告。

## 5. 上游工作流审计与建议

**准确结论：目前上游有效 YAML 中，没有任何工作流直接匹配“推送 sub2api-custom 分支”事件。** 不应为了迎合“都会失败”的假设而删文件。
若该分支同时有指向 main/dev 的开放 PR，后续推送会另外产生 `pull_request:synchronize`，可能触发下表的 PR 测试，这是不同的事件。

| 上游文件 | 触发范围 | sub2api-custom 直接 push | 建议 |
|---|---|---|---|
| `backend.yaml` | main/dev push、PR，限定后端路径 | 不触发 | 可保留给变基 PR；若完全不用可单独禁用 |
| `frontend.yaml` | main/dev push、PR，排除纯后端变动 | 不触发 | 同上；会格式化/i18n 检查、运行含 pyodide 的原版 build，不适合作为自定义镜像门 |
| `docker.yaml` | main/dev push、v* tag push、手动 | 不触发 | **建议 fork 单独禁用**。5 变体×2 架构、CUDA 等构建昂贵；Helm 更新需要 HELM_CHARTS_APP_ID/PRIVATE_KEY，DockerHub 发布需要 DOCKERHUB_USERNAME/TOKEN |
| `release.yml` | main push | 不触发 | **建议禁用**，避免创建上游式 release 并触发 docker.yaml |
| `release-pypi.yml` | main/pypi-release push | 不触发 | **建议禁用**，fork 不应发布上游 open-webui PyPI 包，且缺对应 trusted publisher 配置 |
| `regression.yaml` | 目标 main/dev 的 PR，另有 dev/版本标题条件 | 不触发 | 按是否需要上游外部测试复用决定；可能因 PR 事件耗额度 |
| `issue-label.yaml` | issues opened/edited | 不触发 | 按需保留，不依赖自定义镜像秘密 |
| `codespell.disabled` / `lint-backend.disabled` / `lint-frontend.disabled` | 非 .yml/.yaml | 不触发 | 已非有效 Actions 工作流，无需改动 |

推荐由仓库管理员在 GitHub **Actions → 指定工作流 → 菜单 → Disable workflow** 逐个关闭不用的上游发布流；不要全局禁用 Actions，否则 Custom Image 也不能运行。
亦可由管理员另行执行（本阶段未执行任何远端命令）：

```bash
gh workflow disable docker.yaml --repo qlqqs/open-webui
gh workflow disable release.yml --repo qlqqs/open-webui
gh workflow disable release-pypi.yml --repo qlqqs/open-webui
# 需要恢复时使用 gh workflow enable <文件名> --repo qlqqs/open-webui
```

上述建议依据仓库中的实际触发器和 secrets 引用；没有读取或推断远端 secrets 值，也未确认远端当前启用/禁用状态。**所有上游工作流文件原样保留。**

## 6. 已知问题与后续

1. 本地完整构建及隔离启动已通过，不存在 daemon 拉镜像阻塞。构建仍有原版 Svelte a11y/大 chunk/重复动态导入等 warning；Docker 的 6 条 SecretsUsedInArgOrEnv 告警命中 WEBUI_AUTH、PASSWORD 开关及可信头名称，值都是布尔或头名，并无秘密写入。
2. 首次空库启动有历史 Alembic/SQLAlchemy 主键 SAWarning，服务正常健康；不能在报告中把“无启动报错”夸大为“零 warning”。
3. 基础镜像采用版本标签、Python 部分传递依赖由上游 requirements 解析，因此不是逐字节可复现供应链锁定；后续可另行评估 digest/锁文件策略。本阶段没有擅自改 package 或 requirements。
4. 阶段 2 前端文档在本阶段收尾时到达，已读取并合并契约。第一轮构建后 Chat.svelte、embed/app.ts、model-avatar.svg 有并发修改，已据此重新构建验证，而不是沿用第一次镜像的结论。后续再有源码变化仍需最终集成重建；本阶段未修改这些文件。
5. 公共镜像默认品牌保留；是否可在具体部署隐藏必须按实际人数及许可条件判断，开关本身不提供额外授权。
6. 出站白名单代理只写部署要求，没有复制/修改本机测试代理。特别提醒上游代理链的 DNS/私网限制需在最终出口落实；同进程插件可信，不能把 internal 网络当执行沙箱。
7. 本次只观察空库冷启动及静态探针，不证明所有业务操作无外网。Artifact CSP/导航滥用边界、账号切换、真实网关费用及生产代理白名单仍属阶段 6 联调门。
8. CI 没有远端运行过，GHCR package 可见性和 GitHub Token 授权需首次推送后由维护者核实。推送与一号机部署仍需用户批准。

## 约定变更提议

**无。** 按更新后的 contract 实施，包括原生 image_generation 类别、admin 工具服务器配置例外以及静态健康探针；未修改 contract.md。
