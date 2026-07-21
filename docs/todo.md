# Manta Todo

Status: 8 open items from the v0.1.4 release-readiness review
Scope: Documentation and implementation follow-up notes

## Todo status legend

- `Open`: not started.
- `Active`: currently being worked.
- `Blocked`: waiting on a decision or dependency.

## Release gate

Tag and publish `v0.1.4` only after the items below are closed and the release-readiness review passes again. Until that tag exists, the documented `go install github.com/irootkernel/manta@v0.1.4` command does not resolve: local tags currently stop at `v0.1.3`, and the remote has no tags or releases.

## Open items

Items are listed in recommended fix order. Line references were verified against the current tree and may drift as fixes land.

### RELRV-002 `Open` — Cap failure/warning counts so summaries stay writable

- Severity: high (empirically reproduced).
- Problem: extracted failure/warning records are unbounded, while `WriteSummaryJSON` hard-fails above `safety.MaxSummaryBytes` (64 KiB). A passing command that prints 5,000 `warning:` lines aborts with artifact exit `3` and writes no summary, Markdown, or status artifact at all, so watchers never observe a terminal state.
- Evidence: `internal/extract/extract.go:166-175` (unbounded generic warnings; parser failure loops are also uncapped); `internal/artifacts/artifacts.go:126-129`.
- Done when: failure and warning counts have explicit caps with a truncation indicator in summary artifacts, summary JSON stays under its size bound by construction, the run keeps its authoritative exit code, and tests cover noisy passing and failing logs.

### RELRV-003 `Open` — Playwright parser misses failure headers without trailing padding

- Severity: medium.
- Problem: `playwrightFailRE` ends in `\s+─*$`, so a realistic failure header with no trailing decoration (plausible for non-TTY or CI-piped output) produces zero matches; because specialized parsers never fall back to generic extraction, a real failure surfaces only as `degraded` with no failure span.
- Evidence: `internal/extract/parsers.go:16`.
- Done when: the regex accepts unpadded headers, the fixture gains an unpadded failure line, and parser tests assert file/line/test capture for both shapes.

### RELRV-004 `Open` — Pytest parser cannot capture line numbers; fixture is unrealistic

- Severity: medium.
- Problem: pytest extraction only parses short-summary `FAILED path::test - reason` lines, which carry no line number, so `Failure.Line` is never populated; `internal/extract/testdata/pytest.raw.log` repeats the same short-summary line instead of modeling the `FAILURES` detail block (`path.py:LINE: ExceptionType`) that real pytest emits.
- Evidence: `internal/extract/parsers.go:14,58-72`; `internal/extract/testdata/pytest.raw.log`.
- Done when: the fixture reflects genuine pytest output including the detail block, the parser captures file and line (and test name) from it, and extraction tests assert `Line` population.

### RELRV-005 `Open` — Bound rule/config input sizes (YAML amplification, propose raw-log read)

- Severity: medium.
- Problem: (a) strict YAML decoding has no Manta-side bound against anchor/alias scalar-reuse amplification — a ~1 MiB rule or config file can decode into tens of MiB of strings on every invocation that loads config or rules; only yaml.v3's internal excessive-aliasing guard applies, and it does not cover flat large-scalar reuse. (b) `rules propose` reads the entire `--raw-log` with `os.ReadFile` and splits it with no size cap, unlike every other rule-facing entry point, which is bounded at 256 KiB.
- Evidence: `internal/safety/yaml.go:12` and its callers `internal/config/config.go:43`, `internal/rules/storage.go:316`, `internal/cli/rules.go:272`; `internal/rules/storage.go:196-207`.
- Done when: explicit size caps on config, rule, and proposal source files and on the propose raw-log input fail closed with config exit `2`, consistent with `MANTA-REQ-RQSEC-004`, with tests for each entry point.

### RELRV-006 `Open` — Interrupted-run hash integrity; defensive rule-regex handling

- Severity: medium (low-probability windows).
- Problem: (a) on the interrupt and timeout paths the runner discards the copier/`cmd.Wait` error, and `raw_log_sha256` is computed from the in-memory mirror, so a mid-run raw-log write failure (ENOSPC, EIO) combined with SIGINT/SIGTERM/timeout publishes a hash that does not match the shorter on-disk raw log; the normal completion path already fails closed with exit `4`. (b) `extract` compiles rule start/end regexes with discarded errors and dereferences the resulting nil regex; unreachable through the CLI because stored rules are pre-validated, but it panics for unvalidated in-memory rules, contradicting the defensive-bounds intent that already covers integer bounds.
- Evidence: `internal/runner/runner.go:27-36` and the interrupt/timeout drains around `internal/runner/runner.go:77-130`; `internal/cli/cli.go:280`; `internal/extract/extract.go:83-96`.
- Done when: raw-log write errors on interrupted and timed-out paths surface as explicit artifact/internal errors, or the recorded hash reflects persisted bytes, covered by a fault-injection test; rule regex compile errors are checked and reported instead of dereferenced.

### RELRV-007 `Open` — Fix the fixture-backed vitest rule example in the implementation note

- Severity: docs (empirically reproduced).
- Problem: the documented rule `vitest-empty-state-v1` claims to match `internal/extract/testdata/vitest.raw.log` lines `7:9`, but that fixture's FAIL header starts with a leading space and the example's `^FAIL  ...$` start regex tolerates none, so the documented `rules test` invocation reports "produced no failures" and exits `4`.
- Evidence: `docs/implementation-note.md:110-142`; `internal/extract/testdata/vitest.raw.log:7`.
- Done when: the example matches its cited fixture (for example `^\s*FAIL\s+...`) and the documented `rules test` command has been re-run successfully against it.

### RELRV-008 `Open` — Architecture JSON contract examples omit real fields

- Severity: docs.
- Problem: the Summary and Status JSON contract examples omit fields the binary always writes: `started_at` and `ended_at` in summary JSON, and `status_hash` in status JSON (`status_hash` is discussed in ADR-0005 and the integration guide but missing from the architecture contract example).
- Evidence: `docs/architecture.md:155-239`; `internal/model/types.go:169-170,194-195`.
- Done when: both contract examples list every field a fresh run actually produces, cross-checked against generated artifacts.

### RELRV-009 `Open` — Stale AGENTS.md project root; traceability audit does not resolve cited tests

- Severity: docs/test.
- Problem: (a) `AGENTS.md` records project root `/Users/draccoon/Workspace/SeventeenthEarth/manta`, but the repository lives at `/Users/draccoon/Workspace/SeventeenthEarth/kkachi/kkachi-agent-tester`. (b) The audit regression test only requires each matrix evidence cell to be non-empty text; it never verifies that the cited `Test*` names exist (all 45 currently do, by discipline rather than enforcement), and the `MANTA-REQ-RQWAT-002` row cites a function (`ComputeStatusHash`) instead of a test.
- Evidence: `AGENTS.md:10`; `e2e/audit_regression_e2e_test.go:39-81`; `docs/requirements-test-matrix.md:56`.
- Done when: the recorded project root matches the real checkout, and the audit test resolves every cited `Test*` identifier against the codebase, with an explicit rule for non-test evidence rows.

## Out-of-scope reminder

Unsupported and out-of-scope v0.1 capabilities are listed in the [integration guide](integration-guide.md#not-provided-by-manta-v01). They are not implicitly planned; add an approved requirement and roadmap item before treating one as future work.
