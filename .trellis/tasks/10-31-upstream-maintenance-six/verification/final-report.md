# 六项上游维护更新：本地交付报告

## 当前结果
六项更新已选择性移入 `sub2api-custom` 工作区，21 个产品文件与已验证候选逐字节一致。HEAD仍为 `27d51f9b594d430efcf40570a6ce04063e09701e`，未暂存、提交、推送或部署。原有 `embedded-workbench-guide.md` 改动指纹保持不变。

本地纯上游 main 已从 f178e5ae3 快进至 d851a3847；origin/main 未推送。候选保留在 `/home/qlqq/workspace/sub2api-upstream-six`，不是新的产品提交。

## 选入与适配
| 上游单元 | 内容 | 适配 |
|---|---|---|
| 0a2a29b32 | x/net 0.62.0及配套x/*依赖 | go.mod/go.sum采用源补丁，不改工具链 |
| d851a3847 | vue-i18n/Intlify 11.4.16、decode-uri-component修补 | package/lock/vite与源逐字节一致；保留Node24.22.1/自定义CI及套餐定制断言，新增4项真实语言包回归 |
| a3298b46e | OAuth非流式JSON及Content-Type、终止事件 | 原样保留源生产补丁，增加模型别名/citations/图片/usage六组组合断言 |
| 8b3f33f28 | 渠道搜索平台维度保留及开关 | 修正上游注释位置，增加显式null清空测试；真实浏览器保存OpenAI开关并刷新核对 |
| dece26153 | singleflight内缓存重检 | 增加fresh竞态窗口和stale ETag回归；go overlay旧实现负向对照如期失败 |
| b2e7c6094 | 管理员保护400业务错误 | 保留赠送/线下充值行为，补管理员删除reason回归 |

没有合入Gemini、管理MCP、模型上新、VERSION或迁移。11个新增主线单元已逐项记录范围决定；旧延期项未自动重开。

## 验证
| 检查 | 实际结果 |
|---|---|
| Go模块校验、定向测试、全量unit、缓存race、build、vet | 全部exit0，工具链Go1.27.3 |
| 前端升级前定向基线 / 升级后定向 | 15/15、19/19通过 |
| 前端lint/typecheck/full tests | Node24.22.1 + pnpm9.15.9；443文件、3524测试全部通过，无跳过 |
| 本机生产构建 | 前端+embed后端+Docker均通过；镜像76127e20e006… |
| 完整e2e | **442/442通过，exit0**；Nova与Chat均执行，无阶段跳过；原始444条记录按用例ID取最后一次得到442项 |
| 正常缓存浏览器 | 35项功能/页面检查通过，0未捕获JS错误；**CSP检查失败，exit1**（3次既有nonce/304违规） |
| 禁用HTTP缓存浏览器对照 | 36/36通过，0 CSP违规、0未捕获JS错误，exit0；不代替正常缓存场景 |
| Trellis全范围审查 | 21文件无确认的范围内缺陷，格式/diff检查通过；review-report为浏览器验收前的检查快照 |
| govulncheck / golangci-lint | 本机未安装，未执行 |
| 原始前端audit / 例外策略 | **均exit1**；不作为通过 |

当前工作区另做pnpm9冻结安装与typecheck，结果见custom-frontend-*.exit；产品哈希再次核对，不沿用旧依赖冒充升级验证。

E2E证据：`local-e2e-report.md`、本机 `/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-072240/`。FIN-24本轮通过，不据此宣称修复了历史缺陷。
浏览器覆盖Qx首页/登录/控制台、双语切换、实际月套餐文案、管理员用户页、渠道部分维度API保存与OpenAI开关保存刷新、移动端。截图 `/tmp/sub2api-six-browser/`；临时套餐/分组/渠道已清理，搜索配置已恢复，见browser-cleanup.log。

## 仍未解决的质量项
1. **存量依赖告警**：high从7降至3，无新增；剩余follow-redirects `GHSA-mrwp-f3q8-3g9p` 与xlsx两项。xlsx已有例外，follow-redirects没有例外，所以策略审计同样失败。本批目标vue-i18n/Intlify和decode-uri-component告警已从报告消失。未更新额外依赖或扩展例外。
2. **既有CSP/304 nonce不匹配**：正常缓存下语言切换reload收到304，新CSP头与旧HTML nonce不一致。HTTP探针及三个相关文件与基线一致证明其不属于本批代码变化。保留失败结果，未放宽CSP或修改缓存机制；详见csp-followup.md。

因此：本批范围内功能验证通过，但完整质量门禁未全绿。任务保持in_progress；应独立决定上述修复范围，再取得提交/推送/部署授权，不自动发布七号机。

## 记录与规范
- 已追加 `.trellis/upstream-sync/decisions.jsonl`、更新state，保留旧批次交付事实；本批仅记applied_in_custom_worktree，target_commits为空。
- 更新本任务prd/design/implement/task及父任务批次入口；开发journal/index用--no-commit追加。
- backend/frontend质量规范补充缓存、非流式/搜索/管理员错误及i18n可执行回归契约；上游guide补实际Node工具链核对。
- 生产部署文档无需修改：本轮没有生产操作。无数据库迁移、API字段新增或索引新增；其余spec索引无需改动。
