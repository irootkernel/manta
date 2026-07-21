# Manta Architecture

Status: Complete through `HARDE-007`, `TAGS-001`, and `RELRV-001`
Scope: Standalone Manta v0.1 architecture, including schema-v2 tag selectors and release-readiness follow-up

This document defines Manta's technical and artifact contracts. See the [integration guide](integration-guide.md) for parent-project ownership, supported capability status, and rollout guidance.

## Architecture goals

Manta is a deterministic test and log-evidence tool. It should run test commands, preserve raw output, extract bounded failure evidence, and produce compact artifacts that humans, automation, or simple no-agent watchers can consume. Manta remains independent from external orchestration runtimes.

## Implementation baseline

- Implementation language: Go.
- Packaging target: standalone single binary named `manta`.
- Regex engine baseline: Go `regexp` (RE2 semantics) only.
- Current supported parser labels: `generic`, `vitest`, `pytest`, `go-test`, and `playwright`.
- Unknown parser labels fail closed.

## Non-goals

- Manta is not an autonomous test-writing agent.
- Manta is not a workflow authority or state ledger.
- Manta does not emit consumer-specific evidence snapshots; downstream consumers may normalize the factual status, summary, and raw-log references.
- Manta does not decide that a failed command passed.
- Manta does not rely on terminal/tmux log streaming as a control plane.

## Component overview

```text
CLI
 ├─ Config Loader
 ├─ Command Registry
 ├─ Runner
 │   ├─ Process Executor
 │   ├─ Timeout Controller
 │   └─ Raw Log Writer
 ├─ Extraction Engine
 │   ├─ Generic Parser
 │   ├─ Parser Registry
 │   ├─ Project Rules
 │   ├─ Noise Filter
 │   └─ Redactor
 ├─ Artifact Writer
 │   ├─ summary.json
 │   ├─ summary.md
 │   ├─ status.json
 │   └─ excerpts/*.log
 └─ Rule Manager
     ├─ CRUD
     ├─ Rule Test
     └─ Rule Propose
```

## Data flow: configured command

```text
1. User runs `manta run unit`.
2. CLI resolves repository root and config path.
3. Config loader validates `.manta/tester.yaml`.
4. Command registry resolves `unit` to command argv / canonical tags / parser / timeout.
5. Artifact writer opens the contained raw log before command execution.
6. Runner executes the command in the selected working directory and streams stdout/stderr into the raw log.
7. CLI closes and validates the contained raw log, then the extraction engine processes the captured raw bytes with the selected parser plus project rules.
8. Redactor and noise filters shape surfaced artifacts.
9. Artifact writer writes bounded excerpts, summary JSON, summary Markdown, and status JSON.
10. CLI exits with the underlying test command status or a documented Manta internal error code.
```

On Unix, the runner starts the command in its own process group. SIGINT and SIGTERM are forwarded to the group, with a two-second grace period before remaining members are force-killed. Interrupted runs retain partial raw evidence, produce `status: killed`, and use the process-compatible exit codes `130` and `143` respectively.

## Data flow: summarize existing raw log

```text
1. User runs `manta summarize fixtures/unit.raw.log`.
2. CLI resolves repository root, config path, and raw-log path.
3. Config loader validates optional redaction/noise config and project rules.
4. Manta infers `command_id` from the raw-log basename and uses it as a single tag when the caller does not supply `--tag` values.
5. Artifact writer reserves a standalone run directory, or uses the fixed `--run-id` layout, and copies the original raw bytes into it.
6. Extraction engine applies the `generic` parser plus matching project rules to the copied evidence.
7. Redactor and noise filters shape surfaced artifacts.
8. Artifact writer writes bounded excerpts, summary JSON, summary Markdown, and status JSON in the same artifact layout.
9. CLI exits `0` when summarization succeeds because no test command was executed in this mode.
```

## Artifact layout

When a run ID is supplied:

```text
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.raw.log
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.json
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.md
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.status.json
.manta/runs/scoped/<run_id>/artifacts/test/excerpts/<failure-id>.log
```

Standalone mode may write to:

```text
.manta/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.raw.log
.manta/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.summary.json
.manta/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.summary.md
.manta/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.status.json
.manta/runs/standalone/<UTC-timestamp>[-NNN]/excerpts/<failure-id>.log
```

Standalone run directories are reserved atomically. The timestamp-only name is tried first, followed by zero-padded numeric suffixes on collision, so configured, ad-hoc, and summarize operations cannot reuse an existing standalone directory. Explicit `--run-id` paths are fixed compatibility paths and do not use this suffix allocator.

Failure IDs such as `F001` are summary-local identifiers. Excerpt lookup is deterministic through summary context, not by assuming failure IDs are globally unique.
The summary stores each excerpt as a summary-directory-relative reference such as `excerpts/F001.log`. Artifact-bearing identifiers use `[A-Za-z0-9][A-Za-z0-9_-]*`; canonical containment checks reject absolute paths, traversal, cross-run references, dangling links, and symlinks that resolve outside the selected boundary.

The containment boundary depends on the operation:

- Default standalone and `--run-id` artifact writes are contained by the repository root.
- `--output-dir` artifact writes are contained by the caller-selected output directory, whether that directory is relative or absolute.
- Default `summarize` writes are contained by the repository root and copy the input raw evidence into a newly reserved `.manta/runs/standalone/` directory before materializing derived artifacts.
- Excerpt reads are contained by the canonical `<summary-dir>/excerpts/` directory.
- Project rules and rule proposals are contained by the repository root.

Absolute `--output-dir` and `--summary` inputs remain valid where documented; the absolute-path rejection applies to artifact-bearing identifiers and embedded excerpt references. Symlinks whose canonical targets remain inside the applicable boundary are allowed, while dangling links and links that resolve outside it fail closed.

## Config model

Default config path:

```text
.manta/tester.yaml
```

Minimal shape:

```yaml
version: 2
commands:
  unit:
    command:
      - pnpm
      - vitest
      - run
    tags: [unit, web]
    parser: generic
    timeout_sec: 600
noise_filters:
  - "Browserslist: caniuse-lite is outdated"
redaction:
  patterns:
    - name: token
      regex: "(?i)(token|api[_-]?key)=\\S+"
      replace: "$1=<redacted>"
```

Tags are canonicalized by sorting and removing duplicates. They select project rules rather than commands: the parser must match exactly and every rule tag must be contained in the run tags. Multiple active rules can therefore apply to one raw log. Tags never alter the authoritative command exit code.

## Summary JSON contract

The summary JSON should be stable enough for downstream tools while remaining implementation-friendly:

```yaml
status: failed | passed | timed_out | killed | internal_error
command_id: unit
tags:
  - go
  - unit
parser: generic
command_argv:
  - pnpm
  - vitest
  - run
exit_code: 1
duration_ms: 18422
raw_log: .manta/runs/standalone/20260624T010203/unit.raw.log
raw_log_sha256: sha256:...
extractor_status: precise | partial | degraded | no_match
failure_count: 2
warning_count: 4
failures:
  - id: F001
    kind: test_failure
    signature: "TypeError: Cannot read properties of undefined"
    file: src/foo.test.ts
    line: 42
    test_name: "renders empty state"
    raw_span:
      start_line: 1842
      end_line: 1917
      start_byte: 88211
      end_byte: 92108
    excerpt: excerpts/F001.log
    stack_top:
      - src/foo.ts:42
      - src/foo.test.ts:19
warnings:
  - id: W001
    signature: "deprecated API"
    raw_span:
      start_line: 712
      end_line: 718
```

## Status JSON contract

Status JSON is for no-agent polling and should be compact:

```yaml
status: failed | passed | timed_out | killed | internal_error
command_id: unit
tags:
  - go
  - unit
exit_code: 1
extractor_status: precise | partial | degraded | no_match
summary_path: .manta/runs/standalone/20260624T010203/unit.summary.json
summary_sha256: sha256:...
raw_log_path: .manta/runs/standalone/20260624T010203/unit.raw.log
raw_log_sha256: sha256:...
failure_signatures:
  - sha256:...
warning_signatures:
  - sha256:...
updated_at: 2026-06-24T01:02:03Z
```

### Watcher hash input set

No-agent watchers should suppress duplicate notifications by hashing exactly this ordered field set:

1. `command_id`
2. comma-joined canonical `tags`
3. `status`
4. `exit_code`
5. `extractor_status`
6. `raw_log_sha256`
7. `failure_signatures`
8. `warning_signatures`
9. `summary_path`
10. `raw_log_path`

Other fields may be present for convenience, but watcher compatibility is defined by the field set above.

## Failure and degraded extraction policy

Command execution status is authoritative. Parser quality only affects evidence quality.

- `exit_code == 0` and no internal error: command passed.
- A failed command retains its non-zero exit code even if no parser matched a span.
- Timed-out and killed commands retain their original status and process-compatible exit code.
- Any authoritative non-pass result with no useful span retains its status and exit code with `extractor_status: degraded`.
- Execution and summarize logs larger than 256 KiB are extracted from at most the final 256 KiB, beginning at the first complete line in that window. Spans retain absolute line and byte offsets into the full raw log.
- A bounded-tail scan always reports `extractor_status: degraded`, even when it finds useful evidence, because earlier evidence may have been omitted.
- Rule fixture testing still rejects inputs larger than 256 KiB because an incomplete scan cannot prove that a rule did not miss or overmatch evidence.
- Parser or rule matches and misses never convert an authoritative non-pass result into pass.
- Project rules run before the selected parser. When no rule matches, a specialized parser uses only its own patterns and never retries generic extraction.

| Extraction outcome | Artifact status / exit code | Extractor status | Manta CLI exit code |
|---|---|---|---:|
| Specialized parser miss after command pass | `passed` / `0` | `no_match` | `0` |
| Specialized parser miss after command failure, timeout, or kill | original status / original exit code | `degraded` | original exit code |
| Bounded-tail extraction after command pass | `passed` / `0` | `degraded` | `0` |
| Bounded-tail extraction after command failure, timeout, or kill | original status / original exit code | `degraded` | original exit code |
| Bounded-tail extraction during standalone summarize | inferred status / inferred exit code | `degraded` | `0` |
| Extraction internal error after command pass | `internal_error` / `0` | `degraded` | `4` |
| Extraction internal error after command failure, timeout, or kill | original status / original exit code | `degraded` | original exit code |
| Extraction internal error during standalone summarize | `internal_error` / `4` | `degraded` | `4` |

When extraction fails internally, Manta preserves the raw log and writes empty failure/warning collections plus summary and status artifacts whenever those writes remain safe. The bounded, configured-redaction-aware diagnostic is emitted on stderr and is not added to the JSON schemas. Configuration, pre-execution, and artifact-write failures retain their existing fatal-error behavior.

## Raw-log handling policy

- Raw logs are preserved as original source evidence.
- Raw logs are not redacted by default.
- Summaries, excerpts, status JSON, and console-safe surfaced text apply configured redaction to command metadata and extracted evidence, including command argv, identifiers, tags, failure source paths, signatures, test names, stack entries, and warnings.
- Artifact-reference fields such as `raw_log`, `summary_path`, `raw_log_path`, and excerpt references remain literal locators so watchers, automation, and operators can resolve them. Operators must not place secrets in artifact-bearing identifiers or paths.
- Status signature hashes and `status_hash` are computed from the final redacted metadata and signatures. Canonical comma-joined tags follow `command_id` in the ordered watcher field set.
- Documentation and CLI output should warn that raw logs may contain unredacted secrets or sensitive values and should be shared cautiously.

## Extension points

- Project-local YAML extraction rules.
- Parser registry entries for specialized runners.
- Rule proposal from raw-log spans.
- Optional fixed run-scoped output layout when a run ID is supplied.
- Future shared parser promotion after repeated cross-project evidence.
