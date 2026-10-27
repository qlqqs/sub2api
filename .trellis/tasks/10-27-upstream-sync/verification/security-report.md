# 安全链候选验证摘要

来源：已停止的 security implementer 日志，主代理已核对结果及 diff。

- Go 1.27.3；根镜像及三个 workflow 版本断言同步。保留 Node 24.22.1 定制 pin；未动 backend/Dockerfile/deploy/Dockerfile。
- follow-redirects 1.16.1；DOMPurify 3.4.17（直接/传递依赖与 override）。
- pnpm 9 冻结安装、lint、typecheck 均 exit 0；Vitest 437 文件/3463 测试通过。
- 原始生产依赖审计 exit 1：2 high（xlsx）、13 moderate、2 low；现有 audit-exceptions 检查 exit 0。原始审计不是通过，未改/延长例外。
- 主代理已核对根 Dockerfile 只改 Go 镜像、依赖补丁符合源范围，candidate/source 文件指纹有记录。
- 生产前端+嵌入后端+本机 Docker 构建 exit 0；日志 local-build.log。镜像 sha256:f55b02e38debbadcfb8c4958885c7dec081ac066349a6e75b1d10db0c2ad14a7。
- 尚未创建提交、推送或部署。产品文件移入 custom 工作区的指纹见 custom-worktree-transfer.json。
