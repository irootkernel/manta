# KAT Architecture

Status: Complete
Scope: Standalone KAT v0.1 architecture

## Architecture goals

KAT is a deterministic test and log-evidence tool. It should run test commands, preserve raw output, extract bounded failure evidence, and produce compact artifacts that humans, GJC, KAS, KAH, or simple no-agent watchers can consume. For this repository setup, KAT must remain independent from KAS and KAH.

## Implementation baseline

- Implementation language: Go.
- Packaging target: standalone single binary named `kkachi-agent-tester`.
- Regex engine baseline: Go `regexp` (RE2 semantics) only.
- Current supported parser labels: `generic`, `vitest`, `pytest`, `go-test`, and `playwright`.
- Unknown parser labels fail closed.

## Non-goals

- KAT is not an autonomous test-writing agent.
- KAT is not a KAS authority layer.
- KAT is not a KAH state ledger.
- For GAJAE-009, KAH normalizes existing KAT v0.1.0 status/summary/raw-log artifacts for attachment, so KAT does not emit a separate bindable evidence snapshot. KAT output remains factual test evidence, not KAS/KAH/GJC authority.
- KAT does not decide that a failed command passed.
- KAT does not rely on terminal/tmux log streaming as a control plane.

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
1. User runs `kkachi-agent-tester run unit`.
2. CLI resolves repository root and config path.
3. Config loader validates `.kkachi/tester.yaml`.
4. Command registry resolves `unit` to command argv / lane / parser / timeout.
5. Artifact writer opens the contained raw log before command execution.
6. Runner executes the command in the selected working directory and streams stdout/stderr into the raw log.
7. CLI closes and validates the contained raw log, then the extraction engine processes the captured raw bytes with the selected parser plus project rules.
8. Redactor and noise filters shape surfaced artifacts.
9. Artifact writer writes summary JSON, summary Markdown, excerpts, and status JSON.
10. CLI exits with the underlying test command status or a documented KAT internal error code.
```

On Unix, the runner starts the command in its own process group. SIGINT and SIGTERM are forwarded to the group, with a two-second grace period before remaining members are force-killed. Interrupted runs retain partial raw evidence, produce `status: killed`, and use the process-compatible exit codes `130` and `143` respectively.

## Data flow: summarize existing raw log

```text
1. User runs `kkachi-agent-tester summarize fixtures/unit.raw.log`.
2. CLI resolves repository root, config path, and raw-log path.
3. Config loader validates optional redaction/noise config and project rules.
4. KAT infers `command_id` and `lane` from the raw-log basename when no execution metadata exists.
5. Artifact writer reserves a standalone run directory, or uses the fixed `--run-id` layout, and copies the original raw bytes into it.
6. Extraction engine applies the `generic` parser plus matching project rules to the copied evidence.
7. Redactor and noise filters shape surfaced artifacts.
8. Artifact writer writes summary JSON, summary Markdown, excerpts, and status JSON in the same artifact layout.
9. CLI exits `0` when summarization succeeds because no test command was executed in this mode.
```

## Artifact layout

When a run ID is supplied:

```text
.kkachi/runs/<run_id>/artifacts/test/<command-id>.raw.log
.kkachi/runs/<run_id>/artifacts/test/<command-id>.summary.json
.kkachi/runs/<run_id>/artifacts/test/<command-id>.summary.md
.kkachi/runs/<run_id>/artifacts/test/<command-id>.status.json
.kkachi/runs/<run_id>/artifacts/test/excerpts/<failure-id>.log
```

Standalone mode may write to:

```text
.kat/runs/<UTC-timestamp>[-NNN]/<command-id>.raw.log
.kat/runs/<UTC-timestamp>[-NNN]/<command-id>.summary.json
.kat/runs/<UTC-timestamp>[-NNN]/<command-id>.summary.md
.kat/runs/<UTC-timestamp>[-NNN]/<command-id>.status.json
.kat/runs/<UTC-timestamp>[-NNN]/excerpts/<failure-id>.log
```

Standalone run directories are reserved atomically. The timestamp-only name is tried first, followed by zero-padded numeric suffixes on collision, so configured, ad-hoc, and summarize operations cannot reuse an existing standalone directory. Explicit `--run-id` paths are fixed compatibility paths and do not use this suffix allocator.

Failure IDs such as `F001` are summary-local identifiers. Excerpt lookup is deterministic through summary context, not by assuming failure IDs are globally unique.
The summary stores each excerpt as a summary-directory-relative reference such as `excerpts/F001.log`. Artifact-bearing identifiers use `[A-Za-z0-9][A-Za-z0-9_-]*`; canonical containment checks reject absolute paths, traversal, cross-run references, dangling links, and symlinks that resolve outside the selected boundary.

The containment boundary depends on the operation:

- Default standalone and `--run-id` artifact writes are contained by the repository root.
- `--output-dir` artifact writes are contained by the caller-selected output directory, whether that directory is relative or absolute.
- Default `summarize` writes are contained by the repository root and copy the input raw evidence into a newly reserved `.kat/runs/` directory before materializing derived artifacts.
- Excerpt reads are contained by the canonical `<summary-dir>/excerpts/` directory.
- Project rules and rule proposals are contained by the repository root.

Absolute `--output-dir` and `--summary` inputs remain valid where documented; the absolute-path rejection applies to artifact-bearing identifiers and embedded excerpt references. Symlinks whose canonical targets remain inside the applicable boundary are allowed, while dangling links and links that resolve outside it fail closed.

## Config model

Default config path:

```text
.kkachi/tester.yaml
```

Minimal shape:

```yaml
version: 1
commands:
  unit:
    command:
      - pnpm
      - vitest
      - run
    lane: unit
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

## Summary JSON contract

The summary JSON should be stable enough for downstream tools while remaining implementation-friendly:

```yaml
status: failed | passed | timed_out | killed | internal_error
command_id: unit
lane: unit
parser: generic
command_argv:
  - pnpm
  - vitest
  - run
exit_code: 1
duration_ms: 18422
raw_log: .kat/runs/20260624T010203/unit.raw.log
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
lane: unit
exit_code: 1
extractor_status: precise | partial | degraded | no_match
summary_path: .kat/runs/20260624T010203/unit.summary.json
summary_sha256: sha256:...
raw_log_path: .kat/runs/20260624T010203/unit.raw.log
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
2. `status`
3. `exit_code`
4. `extractor_status`
5. `raw_log_sha256`
6. `failure_signatures`
7. `warning_signatures`
8. `summary_path`
9. `raw_log_path`

Other fields may be present for convenience, but watcher compatibility is defined by the field set above.

## Failure and degraded extraction policy

Command execution status is authoritative. Parser quality only affects evidence quality.

- `exit_code == 0` and no internal error: command passed.
- `exit_code != 0`: command failed even if no parser matched a span.
- Non-zero exit with no useful span should emit `status: failed` and `extractor_status: degraded`.
- Parser or rule failure should never convert a failed command into pass.

## Raw-log handling policy

- Raw logs are preserved as original source evidence.
- Raw logs are not redacted by default.
- Summaries, excerpts, status JSON, and console-safe surfaced text do apply redaction.
- Documentation and CLI output should warn that raw logs may contain unredacted secrets or sensitive values and should be shared cautiously.

## Extension points

- Project-local YAML extraction rules.
- Parser registry entries for specialized runners.
- Rule proposal from raw-log spans.
- Optional Kkachi-compatible output layout when a run ID is supplied.
- Future shared parser promotion after repeated cross-project evidence.
