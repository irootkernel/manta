# Manta Implementation Note

Status: v0.1 baseline, `HARDE-001` through `HARDE-007`, `TAGS-001`, and `RELRV-001` through `RELRV-004` complete
Scope: Maintainer guidance for the standalone Manta v0.1 implementation, schema-v2 tags, release-readiness follow-up, and future changes

This document explains implementation constraints and verification expectations for contributors. It is not the parent-project adoption contract; integrators should start with the [integration guide](integration-guide.md).

## Implementation posture

Build Manta as a small deterministic Go CLI first. Do not add orchestration, session, or acceptance-authority concepts to the core package. Treat the optional run-scoped artifact layout as output path compatibility only.

The post-baseline HARDE sequence is complete. Preserve the contracts in `roadmap.md#harde-post-baseline-hardening-and-contract-closure` and `requirements-specs.md#rqhar-post-baseline-hardening-and-contract-closure`, and rerun affected roadmap verification for future changes.

## Suggested package boundaries

Names are illustrative; adapt them to the selected language and layout.

```text
cmd/manta/
  entrypoint and argument parsing
internal/config/
  config discovery, schema validation, redaction/noise config
internal/runner/
  process execution, timeout, stdout/stderr capture, raw-log writer
internal/artifacts/
  path planner, summary JSON, summary Markdown, status JSON, excerpts
internal/extract/
  generic parser, parser registry, parser-specific modules, span utilities
internal/rules/
  rule model, YAML load/save, CRUD, validation, test/propose
internal/safety/
  identifier validation, rooted path containment, redaction, noise filtering, regex/size bounds
```

## Runner guidance

- Preserve raw logs before summary filtering.
- Store and pass command configuration as argv arrays, not shell strings.
- In Go, prefer direct process execution over shell invocation for configured commands.
- Capture stdout and stderr ordering when the selected approach makes that practical. If perfect ordering is not possible, document the limitation and preserve both streams clearly.
- Record command argv, command ID, canonical tags, parser, start time, end time, duration, exit code, and execution status in derived artifacts; the raw log itself contains only original stdout/stderr evidence.
- Timeout must not become pass. Emit `timed_out` and preserve partial logs.
- Open the contained raw-log artifact before starting the command and stream stdout/stderr into it during execution.
- On Unix, run the command in its own process group, forward SIGINT/SIGTERM to that group, allow a two-second grace period, then force-kill remaining group members.
- Record operator interruption as `killed` with the process-compatible `128 + signal` exit code (`130` for SIGINT and `143` for SIGTERM).
- Prefer explicit internal errors for config/artifact failures instead of silently falling back to a different output path.

## Artifact writer guidance

- Plan artifact paths before execution starts.
- Ensure parent directories exist before writing.
- Write raw logs first, then bounded excerpts, summary JSON, summary Markdown, and status JSON.
- Include SHA-256 for raw logs and summary JSON.
- Use relative paths in JSON where practical so artifacts remain movable within a repository.
- Validate run IDs, configured command IDs, rule IDs, and generated failure IDs with `[A-Za-z0-9][A-Za-z0-9_-]*` before using them in artifact paths.
- Resolve every artifact read, write, directory creation, stat, and discovery operation against its allowed boundary; reject traversal, dangling links, and symlinks that resolve outside that boundary.
- Treat excerpt IDs like `F001` as summary-local. Store excerpt references as summary-directory-relative paths such as `excerpts/F001.log`, and resolve them through the summary path plus failure ID.
- If any required artifact cannot be written, report an internal error and do not claim success.

## Extraction guidance

Current supported parser labels:

- `generic`
- `vitest`
- `pytest`
- `go-test`
- `playwright`

Generic extraction is used only for the `generic` parser label. Fixture-backed Vitest, Pytest, Go test, and Playwright parsers fail closed when their own patterns do not match; they do not retry generic extraction.

Recommended generic patterns still matter for unknown output shapes:

- Lines containing `Error:`, `TypeError:`, `ReferenceError:`, `AssertionError:`, `panic:`, `Traceback`, `FAIL`, `FAILED`, or `✗`.
- File-line references such as `path/to/file.ts:42:13`, `path/to/file.py:42`, and Go test package lines.
- Test names near failure markers.
- Stack lines immediately following an error marker.

Span bounds:

- Include small context before and after the matched marker.
- Treat `max_block_lines` as the matched block size including its start line, and stop at known summary or blank-line boundaries when they occur earlier.
- Limit the entire extracted span, including before/after context, to 160 lines.
- Always enforce maximum lines and bytes per excerpt.

Extractor status guidance:

- `precise`: every accepted failure span has a file or test name.
- `partial`: at least one accepted failure span has neither a file nor a test name.
- `degraded`: a failed, timed-out, or killed command has no accepted failure span, extraction failed internally, extraction inspected only a bounded tail of an oversized raw log, or surfaced failure/warning records were truncated.
- `no_match`: a passing command has no accepted failure span and extraction completed without an internal error; warnings may still be present.

Extraction internal errors follow the artifact/CLI matrix in `architecture.md`. When artifact writes remain safe, Manta preserves raw evidence and materializes empty degraded evidence; bounded, redacted diagnostics go to stderr rather than the JSON schemas.

For execution and summarize logs larger than 256 KiB, extraction uses the final 256 KiB beginning at the first complete line. It preserves absolute line and byte offsets into the full raw log and always reports `degraded`, including when the retained tail contains a precise match. An oversized unbroken line has no complete tail line to inspect. Rule-only `rules test` extraction remains fail closed above 256 KiB so overmatch validation is never based on a partial fixture.

After redaction and noise filtering, retain deterministic prefixes of at most 50 failures and 50 warnings. Assign excerpt references before measuring the rendered formats, then retain the largest failure prefix that fits within 64 KiB in both summary JSON and Markdown before using the remaining budget for the largest warning prefix. Counts always equal retained array lengths, truncation degrades evidence quality, and excerpt files are written only for retained failures. Keep the writer size checks as fail-closed guards for non-evidence metadata overflow.

## Fixture-backed parser examples

Current fixture logs live under `internal/extract/testdata/`:

- `vitest.raw.log`
- `pytest.raw.log`
- `go-test.raw.log`
- `playwright.raw.log`

These fixtures back automated extraction tests and should remain the source of truth for parser-specific documentation.

## Rule implementation guidance

Rules are data, not code. A safe project-local rule now requires provenance and can be tested directly against fixture logs.

Fixture-backed example using the Vitest log under `internal/extract/testdata/vitest.raw.log` lines `7:9`:

```yaml
id: vitest-empty-state-v1
tags: [unit, vitest]
parser: vitest
status: active
provenance:
  created_by: operator
  source_run: local-vitest
  source_command: vitest
  source_log_sha256: sha256:...
  source_span:
    start_line: 7
    end_line: 9
  reason: "Capture the Vitest FAIL block for renders empty state"
match:
  start:
    regex: "^FAIL  src/foo\\.test\\.ts > renders empty state$"
  end:
    any_of:
      - regex: "^$"
    max_block_lines: 16
  include_context:
    before: 1
    after: 1
extract:
  file_line:
    regex: "(?P<file>[^\\s:]+\\.[A-Za-z0-9]+):(?P<line>\\d+)"
  test_name:
    regex: "^\\s*[×✗-]\\s+(?P<test>.+)$"
confidence: medium
```

Validation rejects unknown YAML fields, extra YAML documents, missing IDs or provenance, duplicate IDs, negative or oversized context, a combined matched-block/context budget above 160 lines, excessive `max_block_lines`, invalid capture groups, invalid or unsupported regex, inconsistent active/disabled deletion reasons, and rule overmatch during rule-only `rules test` extraction.

## Regex safety guidance

- Use Go `regexp` with RE2 semantics only.
- Do not support PCRE-only features or backtracking-dependent behavior.
- Bound regex input size before matching; use the bounded complete-line tail for runtime and summarize extraction, and reject oversized rule-test fixtures.
- Bound extracted block lines, excerpt bytes, and summary bytes independently of regex success.
- Fail closed on invalid or unsupported regex.

## Redaction and noise filtering guidance

Apply in this order for surfaced artifacts:

1. Extract bounded spans from raw log.
2. Copy execution metadata and extracted evidence into a surface-only summary.
3. Assign literal excerpt references, then redact summary metadata and evidence and apply noise filtering.
4. Apply the per-kind record caps and actual JSON/Markdown byte budget to the surfaced summary.
5. Redact, noise-filter, bound, and write excerpts only for retained failures, then write both summary artifacts.
6. Derive status hashes and console metadata from the final retained redacted summary, retaining literal artifact references.

Raw-log policy is fixed: raw logs remain original local evidence and are not redacted by default. Artifact-reference fields remain literal and usable, so operators must not place secrets in artifact-bearing IDs or paths. Docs and CLI output should warn that raw logs may contain unredacted values.

## Testing guidance

Tests should cover:

- Passing command.
- Failing command with obvious error span.
- Failing command with no parser match, producing degraded extraction.
- Timeout with partial log.
- Built-binary SIGINT and SIGTERM handling on Unix across standalone and `--run-id` layouts, including process-group forwarding, partial raw evidence, `killed` status, and exit codes `130` and `143`.
- Redaction of summary/status/console command metadata, failure/warning fields, and excerpts, with hashes calculated from final redacted values.
- Literal artifact references remaining resolvable even when command metadata is redacted.
- Noise filtering in summary while raw log remains unchanged.
- Rule test with expected span.
- Rule overmatch rejection.
- Extreme rule context values failing closed before command execution, plus defensive extraction bounds that prevent overflow or panic for unvalidated in-memory rules.
- Artifact path generation for `.manta/`, caller-selected `--output-dir`, and `.manta/runs/scoped/<run_id>/...` layouts, plus built-binary rejection of external `.manta/runs/standalone` and `.manta/runs/scoped` symlinks before command execution.
- Sequential, goroutine-concurrent, and cross-process standalone directory allocation within one UTC-second interval, including configured, ad-hoc, and summarize evidence preservation.
- Invalid run, command, rule, and failure IDs failing before command execution or artifact writes.
- Traversal, cross-run excerpt access, dangling links, and external symlink escape failing closed across artifact and rule operations.
- Internal symlinks whose canonical targets remain inside the applicable boundary continuing to work.
- Specialized parser fixtures for `vitest`, `pytest`, `go-test`, and `playwright`.
- Specialized parser misses with generic-looking markers, covering `no_match` for pass and `degraded` for failed, timed-out, and killed states without generic fallback, including a built-binary E2E probe.
- Extraction internal errors after pass, failure, timeout, kill, and standalone summarize at the artifact-materialization boundary.
- Oversized passing, failing, and summarize logs using bounded-tail extraction, including built-binary probes for preserved raw evidence, summary/status hashes, Markdown output, absolute spans, and CLI exit behavior.
- Noisy passing, failing, and summarize logs that exceed failure/warning record caps, including authoritative or inferred exits, truncation fields, rendered size bounds, terminal status artifacts, retained signature hashes, and excerpt counts; also cover noise filtering and redaction expansion before bounding.
- Exact generated Markdown shape for a fixed summary, plus a built-binary fresh-fixture workflow covering version, configured/ad-hoc run, summarize, excerpt, JSON output, and the complete rule lifecycle.
- Unsupported historical `--verbose` and `--no-color` placeholders failing closed with config exit code `2`.
- Actual `make install` and `make install-toolchain` execution in isolated temporary roots, including installed-version and resolver checks.
- Toolchain resolver selection from `MANTA_BIN`, absolute `manta.binary_path`, and versioned `manta.cli_version`, including argument forwarding and fail-closed missing, unsafe, or mismatched selections.

## Release-readiness checklist

Before the next release tag, verify all of the following:

- `go build ./cmd/manta`
- `make test`
- `make install` and `make install-toolchain` in isolated temporary roots, including installed-version and resolver checks
- configured run smoke test
- ad-hoc run smoke test
- built-binary SIGINT/SIGTERM interruption smoke across standalone and `--run-id` layouts, including partial raw evidence, `killed` status, and exit codes `130` and `143`
- summarize smoke test from an existing raw log
- parser fixture coverage for `generic`, `vitest`, `pytest`, `go-test`, and `playwright`
- rule lifecycle coverage for `list/search/show/create/update/delete/test/propose`
- fresh-fixture execution of every documented Manta CLI command with generated Markdown compared to the documented shape
- toolchain resolver status and forwarding checks for environment, absolute-path metadata, and versioned metadata selection
- artifact path and containment verification for `.manta/`, `--output-dir`, and `.manta/runs/scoped/<run_id>/...`, including external `.manta/runs/standalone` and `.manta/runs/scoped` symlink rejection
- collision checks confirming repeated standalone operations retain distinct raw, summary, Markdown, status, and excerpt artifacts with unchanged raw-log checksums
- watcher status JSON compatibility, including status-hash inputs
- release notes mention known limitations, especially raw-log redaction policy, rule proposals remaining run-local until promoted, and the current platform-verification boundary

## Implementation guardrails

- Do not introduce a dependency on an external orchestration runtime.
- Do not introduce broad fallback behavior.
- Do not silently ignore artifact-write failures.
- Do not allow rules to alter pass/fail status.
- Do not dump full raw logs to console by default.
- Do not mark documentation or roadmap tasks done without executable evidence once implementation begins.
