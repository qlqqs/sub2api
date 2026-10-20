# Journal - qlqqs (Part 1)

> AI development session journal
> Started: 2026-09-04

---



## Session 5: 同步上游并合并定制分支
<!-- trellis-session: v=2 fp=f57486390b716d0a -->

**Date**: 2026-10-20
**Task**: 同步上游并合并定制分支
**Branch**: `sub2api-custom`

### Summary

保持 upstream/main、origin/main、本地 main 完全一致；仅将上游改动合并进 sub2api-custom。

### Main Changes

- 合并上游，保留补绑及提现功能与测试，统一 Gin user_id 动态参数并增加共存回归测试。

### Git Commits

| Hash | Message |
|------|---------|
| `8f2cbb53d` | Merge pull request #8177 from feeeei/fix/gemini-negative-input-tokens |

### Testing

- [OK] 前端 2873 项测试、类型检查、构建和相关 ESLint 通过；后端定向单元测试、25 项邀请返利集成测试和构建通过。
- [OK] 代理导致验证码本地请求超时，清除测试进程代理后 repository 全包通过；真实 OpenAI API 对比测试不作为离线验证通过项。

### Status

[OK] **Completed**
