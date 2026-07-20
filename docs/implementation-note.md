# KAT Implementation Note

Status: v0.1 baseline complete; HARDE follow-up planned
Scope: Guidance for implementing complex KAT v0.1 and post-baseline hardening areas without KAS/KAH dependency

## Implementation posture

Build KAT as a small deterministic Go CLI first. Do not add KAS, KAH, GJC session, or authority concepts to the core package. Treat optional Kkachi artifact layout as output path compatibility only.

The current implementation baseline is not the end of hardening work. Use `roadmap.md#harde-post-baseline-hardening-and-contract-closure` and `requirements-specs.md#rqhar-post-baseline-hardening-and-contract-closure` as the implementation sequence and contract for the next PRs. Do not mark a `HARDE` task complete from code or proxy test success alone; run and observe every verification condition recorded in its roadmap row.

## Suggested package boundaries

Names are illustrative; adapt them to the selected language and layout.

```text
cmd/kkachi-agent-tester/
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
  redaction, noise filtering, bounded regex helpers, size limits
```

## Runner guidance

- Preserve raw logs before summary filtering.
- Store and pass command configuration as argv arrays, not shell strings.
- In Go, prefer direct process execution over shell invocation for configured commands.
- Capture stdout and stderr ordering when the selected approach makes that practical. If perfect ordering is not possible, document the limitation and preserve both streams clearly.
- Store command metadata with the raw log: command argv, working directory, start time, end time, duration, exit code, timeout, and interruption state.
- Timeout must not become pass. Emit `timed_out` and preserve partial logs.
- Prefer explicit internal errors for config/artifact failures instead of silently falling back to a different output path.

## Artifact writer guidance

- Plan artifact paths before execution starts.
- Ensure parent directories exist before writing.
- Write raw logs first, then summary JSON, summary Markdown, excerpts, and status JSON.
- Include SHA-256 for raw logs and summary JSON.
- Use relative paths in JSON where practical so artifacts remain movable within a repository.
- Treat excerpt IDs like `F001` as summary-local. Deterministic excerpt lookup is via summary path plus failure ID.
- If any required artifact cannot be written, report an internal error and do not claim success.

## Extraction guidance

Current supported parser labels:

- `generic`
- `vitest`
- `pytest`
- `go-test`
- `playwright`

Generic extraction remains the fallback, but parser-specific extraction is now implemented for fixture-backed Vitest, Pytest, Go test, and Playwright logs.

Recommended generic patterns still matter for unknown output shapes:

- Lines containing `Error:`, `TypeError:`, `ReferenceError:`, `AssertionError:`, `panic:`, `Traceback`, `FAIL`, `FAILED`, or `✗`.
- File-line references such as `path/to/file.ts:42:13`, `path/to/file.py:42`, and Go test package lines.
- Test names near failure markers.
- Stack lines immediately following an error marker.

Span bounds:

- Include small context before and after the matched marker.
- Stop at known summary boundaries, blank-line boundaries, or `max_block_lines`.
- Always enforce maximum lines and bytes per excerpt.

Extractor status guidance:

- `precise`: one or more bounded spans with useful signature and likely location.
- `partial`: spans exist but key metadata is missing.
- `degraded`: command failed but spans are missing or too broad.
- `no_match`: command passed or no relevant failure/warning evidence exists.

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
lane: unit
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

Validation rejects missing IDs, missing provenance, duplicate IDs, excessive `max_block_lines`, invalid capture groups, invalid or unsupported regex, and rule overmatch during `rules test`.

## Regex safety guidance

- Use Go `regexp` with RE2 semantics only.
- Do not support PCRE-only features or backtracking-dependent behavior.
- Bound regex input size before matching.
- Bound extracted block lines, excerpt bytes, and summary bytes independently of regex success.
- Fail closed on invalid or unsupported regex.

## Redaction and noise filtering guidance

Apply in this order for surfaced artifacts:

1. Extract bounded spans from raw log.
2. Apply redaction to surfaced text.
3. Apply noise filtering where appropriate for summaries.
4. Write summary/excerpt/status artifacts.

Raw-log policy is fixed: raw logs remain original local evidence and are not redacted by default. Docs and CLI output should warn that raw logs may contain unredacted values.

## Testing guidance for the future implementation

Test fixtures should cover:

- Passing command.
- Failing command with obvious error span.
- Failing command with no parser match, producing degraded extraction.
- Timeout with partial log.
- Redaction in summary and excerpts.
- Noise filtering in summary while raw log remains unchanged.
- Rule test with expected span.
- Rule overmatch rejection.
- Artifact path generation for `.kat/` and `.kkachi/runs/<run_id>/...`.
- Specialized parser fixtures for `vitest`, `pytest`, `go-test`, and `playwright`.

## Release-readiness checklist

Before tagging `v0.1.3`, verify all of the following:

- `go build ./cmd/kkachi-agent-tester`
- `make test`
- configured run smoke test
- ad-hoc run smoke test
- summarize smoke test from an existing raw log
- parser fixture coverage for `generic`, `vitest`, `pytest`, `go-test`, and `playwright`
- rule lifecycle coverage for `list/search/show/create/update/delete/test/propose`
- artifact path verification for both `.kat/` and `.kkachi/runs/<run_id>/...`
- watcher status JSON compatibility, including status-hash inputs
- release notes mention known limitations, especially raw-log redaction policy and rule proposals remaining run-local until promoted

## Implementation guardrails

- Do not import KAS or KAH internals.
- Do not introduce broad fallback behavior.
- Do not silently ignore artifact-write failures.
- Do not allow rules to alter pass/fail status.
- Do not dump full raw logs to console by default.
- Do not mark documentation or roadmap tasks done without executable evidence once implementation begins.
