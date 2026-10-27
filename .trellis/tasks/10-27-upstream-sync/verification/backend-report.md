# M2–M4 backend candidate verification — 2026-10-27

## Scope and exact candidate

- Worktree: `/home/qlqq/workspace/sub2api-upstream-candidate` only.
- Base HEAD: `a18a7a3a4d7d5f81284f8165a2f9cd464a0bf132`.
- Applied with `git show --format= --binary <source> | git apply` after `git apply --check`; no index writes, commits, push, deploy, original-worktree edits, Docker operations or system dependency installs.
- M1 files belong to the security implementer. Their upgraded `backend/go.mod` was re-read before testing: Go **1.27.3**. `go.sum` unchanged.
- This is an **uncommitted candidate**, not integration into custom. Dirty source identities: `backend-tested-files.sha256`; complete seven-file owned diff: `backend-candidate.patch` and `backend-patch.sha256`. Original upstream diffs: `backend-source-*.patch`.

## Source mapping / files changed

| Unit / source SHA | Owned files (`backend/internal/service/`) | Adaptation |
|---|---|---|
| M2 / `38b72d734e0aa92ce7547a5ea8618abb7290cf98` | `model_rate_limit.go`, new `model_rate_limit_snapshot_cow_test.go` | Exact upstream patch; maps are copied before replacing this account's Extra. No later scheduler-cache optimization. |
| M3 / `e0fa4b94f9b962949da039c09e63968358324e26` | `ratelimit_service.go`, `ratelimit_service_403_html_test.go` | Exact upstream patch; Cloudflare 1010 edge blocks bypass account penalties, genuine 403 and platform-specific handling remain. |
| M4 / `e068c3d02ff8664c1142d5f7c51a05a82f75ad3d` | `openai_gateway_passthrough.go`, `openai_capacity_shed_test.go`, `openai_ws_http_bridge_test.go` | Exact upstream patch plus **four test-only rows** asserting tool-call arguments, argument delta, image partial and citation are still client output. |

No model catalog/capability/pricing changes, c591 identity/Retry-After/empty-delta changes, or Anthropic streaming/usage changes. The M2 upstream regression uses existing model identifiers only as fixtures. Custom TTFT helpers, response model rewriting, citations, image backfill, financial logic and existing provider policies are not modified.

## Environment and commands

All test/build commands run from the candidate `backend/`, using:

```sh
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY \
  -u ALL_PROXY -u all_proxy -u OPENAI_API_KEY GOMAXPROCS=4 <command>
```

The first unproxied Go 1.27.3 auto-download timed out. A bounded `timeout 180s env -u OPENAI_API_KEY GOMAXPROCS=4 go version` with the existing proxy then downloaded the toolchain into the **user Go module cache**, successfully. Actual tests subsequently ran without proxies/API key and with `go version go1.27.3 linux/amd64`. Evidence: `backend-environment.log`, `backend-toolchain-download.log`.

Commands executed serially (never overlapping test/build jobs):

```sh
go test -p 4 -count=1 -tags unit ./internal/service -run 'Test(SetAccountModelRateLimitSnapshot|ApplyAccountSchedulingThreshold|HandleUpstreamError|IsCloudflareBotBlockResponse|IsHTMLResponse|TempUnscheduleRetryableErrorSkipsRequestScopedTransient|StreamFailedEventCapacity|OpenAI.*(Capacity|Stream|WSHTTPBridge|Images|Cline)|ProxyOpenAIWSHTTPBridge|SanitizeOpenAICapacity|ChatGateway.*Citations|BackfillOpenAIImages|ImagesURLToB64|IsBackfillImage)' -v

go test -race -p 4 -count=1 -tags unit ./internal/service -run 'Test(SetAccountModelRateLimitSnapshot|ApplyAccountSchedulingThreshold_FableLimitStaysOnOwnSnapshotCopy|HandleUpstreamError_.*403|HandleUpstreamError_Cloudflare|IsCloudflareBotBlockResponse|OpenAIStream(ErrorFrame|MetadataAndKeepalive)|ProxyOpenAIWSHTTPBridgeTurn(StagesMetadata|DoesNotReplayCapacity))' -v

go test -p 4 -count=1 -tags unit ./...

go build -p 4 ./...
```

## Results

| Check | Result | Evidence |
|---|---|---|
| Scoped apply checks, gofmt, git diff --check; no staged changes; tested file hashes unchanged | PASS | `backend-final-checks.log`, owned patch |
| Focused service regression | PASS, 283 top-level tests, 0 top-level skips; test execution 8.836s | `backend-focused.log`, exit 0 |
| Focused race regression | PASS, 23 top-level tests, no race reported; test execution 1.231s | `backend-race.log`, exit 0 |
| Full backend unit | PASS, 58 packages reported `ok` (other packages have no tests); service package 216.533s | `backend-unit.log`, exit 0 |
| Backend build | PASS | `backend-build.log`, exit 0 |

All checks completed by 2026-10-27T16:03:30Z. No failing test or unrelated failure was observed. Full unit success does not imply all individual tests executed: environment-dependent tests may skip (notably the intentionally disabled real OpenAI comparison).

The focused suite covers COW shared-master isolation, per-copy Fable scheduling, cross-platform Cloudflare HTML/JSON 1010, unrelated/genuine 403 behavior, overload before/after semantic output through native/passthrough/WS bridge, buffered/streaming citations, and image URL-to-base64 behavior. Race coverage includes the new COW concurrent-copy tests and selected 403/stream/bridge boundaries.

## Limits / follow-ups

- COW isolates shallow **copies** sharing old maps; it does not make concurrent mutation of one identical `*Account` object safe.
- `golangci-lint` is not installed locally; CI lint remains pending.
- No integration-tag database/container tests, shared local Docker build/e2e or real upstream API requests were run by this implementer. Local environment acceptance and final combined-worktree verification remain main-session responsibilities.
- No source commit is claimed as integrated or shipped. Main session owns task/ledger/spec/journal updates and further authorization.
