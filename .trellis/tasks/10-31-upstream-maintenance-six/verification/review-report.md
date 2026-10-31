# Trellis check — upstream maintenance six

## Verdict

**PASS for the reviewed code scope; final local acceptance remains pending.** No confirmed, in-scope executable defect found in the 21 candidate product files. This is not a claim that every delivery gate passed: local e2e/browser acceptance is still owned by the parent, backend security tooling is unavailable, and the baseline frontend audit remains nonzero.

Reviewed candidate: `/home/qlqq/workspace/sub2api-upstream-six`, baseline `27d51f9b594d430efcf40570a6ce04063e09701e`, upstream selection ending at `d851a38477597157e78450d30af2d8504764d085`. All **21/21 SHA-256 fingerprints** matched `candidate-manifest.json` at review start and completion. Baseline files were also checked against the manifest before ordinary filesystem diffs were read; the six new files were read in full. No product files, dependencies, Git state, Docker services, or other agents' outputs were modified by this reviewer.

Context read: task `check.jsonl`, PRD/design/implementation checklist, backend/frontend spec indexes, backend/frontend quality guidelines, error handling, frontend component/type safety, and upstream-sync guide. Source patches and shared verification logs were read, not treated as substitutes for inspecting the executable paths.

## Findings (fixed)

None. Product bytes remained frozen throughout this review.

## Findings (not fixed)

No confirmed in-scope executable defects requiring a product fix.

The known `follow-redirects` and `xlsx` audit findings remain outside this batch, as instructed. Their baseline/candidate comparison and the nonzero exception-check result are recorded in `frontend-final-summary.log`; this review neither expands their scope nor changes exceptions.

## Scope and contract review

| Requirement | Files reviewed | Result |
|---|---|---|
| R1 | `backend/go.mod`, `backend/go.sum` | The selected x/* version/checksum changes match source `0a2a29b32`. x/net is 0.62.0; crypto/sync/term/sys/text/tools move together. Go toolchain, unrelated direct dependencies, CI, and container build entrypoints are not changed. |
| R2 | `frontend/package.json`, `frontend/pnpm-lock.yaml`, `frontend/vite.config.ts`, `frontend/src/components/payment/__tests__/SubscriptionPlanCard.spec.ts`, `frontend/src/i18n/__tests__/runtime.spec.ts` | Package/lock/Vite files exactly match the corresponding frozen upstream mirror files. Only the selected dependency graph changes are present. Runtime-only alias remains; the obsolete JIT define is removed. Existing custom currency, no-group-ratio/no-peak-price, and model-plaza assertions remain. New runtime tests exercise real lazy-loaded locales, language changes, interpolation, title/storage/lang updates, and absence of Function/eval calls. |
| R3 | `backend/internal/service/openai_gateway_passthrough.go`, `openai_gateway_response_handling.go`, `openai_oauth_passthrough_non_stream_test.go`, `openai_nonstream_custom_contract_test.go` | Source `a3298b46e` is retained with the added custom-contract test. Original downstream stream intent survives OAuth request normalization; aggregation explicitly replaces the copied SSE Content-Type; incomplete/cancelled/canceled terminal response objects enter the existing output/usage processing. |
| R4 | `backend/internal/service/channel_service.go`, `channel_websearch_merge_test.go`, `frontend/src/views/admin/ChannelsView.vue`, `frontend/src/views/admin/__tests__/ChannelsView.webSearchEmulation.spec.ts`, `frontend/src/i18n/locales/en/admin/channels.ts`, `frontend/src/i18n/locales/zh/admin/channels.ts` | Source `8b3f33f28` is retained, with helper/comment placement corrected and an explicit-null test added. Merge is limited to web_search_emulation platform dimensions. Rendering and serialization use the same selected six-platform set; disabled sections send false. |
| R5 | `backend/internal/service/openai_codex_models_service.go`, `openai_models_cache_recheck_test.go` | Exact production hunk from `dece26153`; real cache recheck occurs inside the singleflight closure before creating a timeout/fetching. New tests distinguish refreshed content from old content and assert fetch count/ETag/freshness. |
| R6 | `backend/internal/service/admin_user.go`, `admin_service_role_test.go` | Only the three selected guard error constructions change. Source `b2e7c6094` test is retained and delete coverage is added. Guard conditions, update fields, financial operations, deletion cleanup/transactions, and cache invalidation remain unchanged. |

### R3: request/response and custom paths

- Traced `Forward` request-view stream extraction into `forwardOpenAIPassthrough`, normalization, streaming/nonstreaming dispatch, and the returned `OpenAIForwardResult.Stream`. The new code no longer overwrites the client's false flag with upstream-required true. Compact still forces the internal nonstreaming path and retains `writeOpenAICompactSSEBridge` handling for the custom client bridge.
- Both aggregators inspect failed/error terminals before success extraction; the new fallback does not turn those failures into successful JSON. Noncompleted extraction is limited to an object-valued `response` for the three terminal spellings.
- Missing upstream Content-Type uses physical SSE line framing for OAuth. Ordinary JSON containing the words `data:`/`event:` is not classified by a substring search.
- Once a terminal response is extracted, existing empty-output reconstruction, tool/namespace restoration, compaction supplementation, and usage extraction remain in place. The added custom test verifies citations, model replacement, image payload/size/count, and input/output usage for completed/incomplete/cancelled in both aggregators.
- That image test intentionally supplies the completed `response.output_item.done` event before the enclosing terminal; it proves the existing completed-image accounting contract, not every malformed/truncated upstream event sequence.
- Upstream model observation still happens before response rewriting; the separate HTTP client-model middleware is unchanged and now sees the correct JSON Content-Type. Existing middleware/route tests are included in the shared backend full unit result. The direct aggregator test alone is not being presented as an end-to-end middleware test.

### R4: exact merge and platform semantics

- `ChannelService.Update` reads the persisted channel, applies the narrow merge, validates, writes, invalidates channel/auth caches, and reloads. The API still uses a map-valued `features_config`; no request/schema/repository migration is needed.
- Omitted outer `features_config` remains a no-op. For a supplied outer map, omitted `web_search_emulation` inherits existing dimensions; explicit key `{}` or `null` clears that key. Explicit platform false wins. Other feature keys retain incoming whole-map replacement semantics, not a generic deep merge. The empty/null clearing rule applies to the **nested web_search_emulation value**, not to an omitted outer field.
- The frontend preserves unmanaged outer fields, reads each section's existing dimension, and sends explicit false for a disabled supported section. Platforms absent from the submitted map survive through the backend merge.
- The selected frontend set is `anthropic/openai/command_code/opencode_go/minimax/zhipu`, matching the upstream patch. Actual channel runtime support was traced through `shouldEmulateWebSearchFor` and `Channel.IsWebSearchEmulationEnabled(account.Platform)` in the Messages forwarding paths. This must not be confused with account-level force-on/off: the current `GetWebSearchEmulationMode` recognizes Anthropic/OpenAI API keys only; the other selected platforms follow their channel configuration. The comment referring to `webSearchEmulationPlatformEnabled` is not evidence of such a helper in this fork.
- The frontend whitelist test is a source-shape guard, not an executable cross-language parity or UI-save test. Parent-owned browser acceptance must supply the latter evidence; the prepared browser script covers partial API preservation and an OpenAI toggle save/reload. No expansion of account-level search policy is required by this patch's channel-saving contract.

### R5: deterministic regression evidence

The fresh-cache test seeds stale content, observes stale state, installs a different fresh response, then enters the real refresh function. It asserts the fresh object and zero fetches. Removing the production recheck causes the callback to return the old object, so the test is not tautological. Parent's `backend-cache-negative-control.log` confirms exactly that expected failure (exit 1); the normal focused suite and cache race run pass. The second test verifies stale upstream ETag transmission, one refresh, NotModified reuse, and renewed freshness. Existing expiry/stale-window/cache-key logic and client conditional-response processing are unchanged.

### R6: public errors

Traced user handler update/delete calls to `response.ErrorFrom`, which consumes these `ApplicationError` status/reason values. Disable/delete admin and last-admin demotion now yield 400 with stable distinct reasons; their checks still execute before the corresponding persistence operations. No broader admin-role or financial-policy change is introduced.

### Spec/template touch points

No new migration, platform identifier, config template, public field, or module boundary is introduced. Node 24.22.1 pins already satisfy the selected frontend dependency engines; workflow/Dockerfile pins remain unchanged. The task design documents the narrow R3/R4 exceptions; existing error/model-boundary/quality specs remain accurate. No mandatory `.trellis/spec/` edit was identified, and the unrelated dirty embedded-workbench guide was left alone.

## Verification

Evidence below is from the same frozen candidate. Shared checks were not duplicated.

| Gate | Result | Evidence / ownership |
|---|---|---|
| Frontend lint | **PASS** | `frontend-node24-lint-check.log`, exit 0, Node 24.22.1 + pnpm 9 |
| Frontend TypeCheck | **PASS** | `frontend-node24-typecheck.log`, exit 0 |
| Frontend tests | **PASS** | `frontend-node24-test-run.log`: 443 files / 3524 tests, no skips, exit 0 |
| Backend type/compile check | **PASS** | `backend-build.exit` = 0 |
| Backend vet | **PASS** | `backend-vet.exit` = 0 |
| Backend full unit suite | **PASS** | `backend-unit.exit` = 0; package results inspected |
| Backend focused tests | **PASS** | `backend-focused.exit` = 0; earlier failed attempts are retained separately, not counted as passing |
| Cache race check | **PASS** | `backend-race.exit` = 0 |
| Cache negative control | **EXPECTED FAILURE** | `backend-cache-negative-control.exit` = 1; fresh-object assertion fails with old implementation |
| Go module verification | **PASS** | `backend-mod-verify.exit` = 0 |
| Production/local image build | **PASS** | `local-build.exit` = 0; frontend production + embedded backend + image build completed |
| Changed Go formatting | **PASS** | Reviewer ran `gofmt -l` on every changed/new Go file; empty output |
| Whitespace | **PASS** | Reviewer ran permitted `git diff --check`; exit 0 |
| Candidate fingerprint stability | **PASS** | Reviewer recalculated all 21 SHA-256 hashes; all match manifest |
| Backend golangci-lint | **NOT RUN** | Tool absent locally; vet/formatting are not represented as a substitute for the complete CI lint set |
| govulncheck | **NOT RUN** | Tool absent locally |
| Frontend production audit / exception checker | **FAIL, baseline-known** | Both have nonzero exits; retained follow-redirects/xlsx findings explicitly out of scope |
| Complete local e2e | **PENDING** | Parent owns running `local-e2e.log`; no completion exit artifact at review cutoff |
| Browser i18n/CSP/channel acceptance | **PENDING** | Parent owns execution; prepared script is not proof of passing |
| Main-worktree transfer/fingerprint reconciliation | **PENDING** | Parent owns transfer and ledger; candidate review is not proof of transfer or integration |

No commit, push, production deployment, or final delivery authorization is inferred from this verdict.
