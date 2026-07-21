# Manta Parent-Project Integration Guide

Status: Current for `manta v0.1.4`
Audience: Projects that invoke Manta or consume Manta evidence

Manta is a standalone deterministic test runner and evidence producer. A parent project owns when and why tests run; Manta owns command execution, raw-log preservation, bounded extraction, and factual artifacts for that one invocation.

## Integration boundary

```text
Parent project / CI / operator
  | chooses command, version, repository, run ID, and retention policy
  v
manta
  | executes command and records factual evidence
  v
status.json + summary.json + summary.md + excerpts + raw.log
  |
  v
Watcher / evidence consumer / human reviewer
```

The command exit code is authoritative. Parsers and rules describe evidence quality; they cannot convert failure to pass. Manta artifacts do not grant review acceptance, waiver, final acceptance, release, or runtime-activation authority.

## Supported capability matrix

| Area | Supported in v0.1.4 | Integration note |
|---|---|---|
| Configured execution | Yes | `run <command-id>` reads `.manta/tester.yaml`. |
| Ad-hoc execution | Yes | `run --tag <tag> [--tag <tag> ...] -- <argv...>` can run without configured commands. |
| Existing-log processing | Yes | `summarize <raw-log>` copies and summarizes a log without rerunning the command. Its inferred result is not authoritative execution metadata. |
| Failure excerpt lookup | Yes | `excerpt --summary <path> <failure-id>` validates contained references before reading. |
| Parsers | Yes | `generic`, `vitest`, `pytest`, `go-test`, and `playwright`. |
| Project extraction rules | Yes | Strict YAML CRUD, provenance, fixture testing, bounded spans, and run-local proposals. |
| Standalone artifacts | Yes | Collision-free `.manta/runs/standalone/<UTC-timestamp>[-NNN]/` or `<output-dir>/runs/...`. |
| Parent run artifacts | Yes | `--run-id <id>` writes only under `.manta/runs/scoped/<id>/artifacts/test/`. |
| Human output | Yes | Compact console output, Markdown summary, and bounded excerpts. |
| Machine output | Yes | `--json`, summary JSON, and deterministic status JSON. |
| Redacted derived evidence | Yes | Configured redaction covers surfaced metadata, summaries, status, warnings, failures, and excerpts. |
| Original raw evidence | Yes | Raw logs are preserved and intentionally not redacted. |
| Timeout | Yes | A timed-out command retains partial evidence and uses status `timed_out` with exit code `124`. |
| Operator interruption | Yes | Unix SIGINT/SIGTERM process-group behavior is covered by built-binary tests; non-Unix builds signal the direct child and have a narrower guarantee. |
| Deterministic binary selection | Yes | The bundled Python 3 resolver selects an explicit environment, metadata, or versioned toolchain binary and never falls back to `PATH`. |

## Not provided by Manta v0.1

These are current boundaries, not hidden partial features:

- Test planning, test generation, code review, or acceptance decisions.
- External orchestration, workflow/session management, or acceptance-state management.
- A resident watcher daemon, running-state heartbeat, or progress events. Manta writes final `status.json`; the parent project owns in-flight state, polling, and notification.
- Automatic issue creation, release, push, install, update, or runtime activation.
- Automatic generic-parser fallback after a specialized parser misses.
- Redaction of the original raw log or of literal artifact-reference paths.
- Automatic promotion of `.manta/rule-proposals/` into active project rules.
- Consumer-specific evidence snapshots. Consumers should use or normalize the existing status, summary, and raw-log references.
- A bundled CI-provider workflow or a cross-platform release matrix. The repository tests platform-neutral behavior plus additional Unix-only install, process-group, and signal behavior.
- A successful built-in `--help` surface. The current CLI returns config exit code `2` for `--help`; use the [CLI reference](user-interface.md) for command syntax.

Open v0.1.4 release-readiness review items are recorded in `todo.md`. The boundaries above are not future commitments; a new requirement and roadmap item should be approved before broadening them.

## Local-only Manta state

Ignore the entire `.manta/` directory. Manta config, reviewed local rules, toolchain metadata, proposals, and evidence are machine-local development state rather than portable source inputs:

```gitignore
.manta/
```

Projects that need shared automation must generate or provision their local Manta state through their own bootstrap process. Manta does not distribute `.manta/` content. Never commit an absolute `manta.binary_path` that is meaningful only on one machine.

## 1. Select and verify the binary

For ordinary local use, install and verify the pinned release:

```bash
go install github.com/irootkernel/manta@v0.1.4
manta --version
```

For deterministic automation, prefer the bundled resolver and one explicit source:

1. `MANTA_BIN=/absolute/path/to/manta`
2. `.manta/toolchain.yaml` `manta.binary_path`
3. `.manta/toolchain.yaml` `manta.cli_version`, resolved under `${MANTA_TOOLCHAIN_ROOT:-$HOME/.local/manta/toolchains}`

Example portable version selection:

```yaml
schema_version: "manta.toolchain.v1"
manta:
  cli_version: "0.1.4"
```

Validate selection before invoking Manta:

```bash
scripts/manta-toolchain --toolchain-status
scripts/manta-toolchain --version
```

The resolver requires Python 3, absolute executable overrides, and an exact semantic-version match for metadata-selected binaries. It deliberately does not search `PATH`.

## 2. Define project commands

Create `.manta/tester.yaml`:

```yaml
version: 2
commands:
  unit:
    command: ["go", "test", "./..."]
    tags: [go, unit]
    parser: go-test
    timeout_sec: 600
  e2e:
    command: ["pnpm", "playwright", "test"]
    tags: [e2e, web]
    parser: playwright
    timeout_sec: 1800
noise_filters:
  - "Browserslist: caniuse-lite is outdated"
redaction:
  patterns:
    - name: credential
      regex: '(?i)(token|api[_-]?key)=\S+'
      replace: '$1=<redacted>'
```

Integration rules:

- Use argv arrays, not a shell command string. Add `sh -c` explicitly only when shell behavior is required.
- Choose the specialized parser only when the command emits that runner's output. Use `generic` for other output.
- Set `timeout_sec` from `1` to `86400`; invalid config fails before execution.
- Command IDs, run IDs, rule IDs, and tags must match `[A-Za-z0-9][A-Za-z0-9_-]*`.
- Config and rule files accept one YAML document, reject unknown fields, and use Go RE2 regex syntax.
- Tags are sorted and deduplicated. A rule applies when its parser matches and all of its tags are present on the run; multiple active rules may apply to one raw log.

## 3. Choose an invocation layout

Use standalone mode for local or independent automation:

```bash
manta run unit
```

Use a parent-owned run ID when evidence must attach to an existing parent run:

```bash
run_id=parent-run-001
manta --run-id "$run_id" run unit
```

The parent must create a safe identifier before invocation; Manta validates it and creates the artifact directory. Do not derive run IDs from secrets or untrusted path fragments. Standalone mode allocates a new directory for every operation, but `--run-id` uses fixed filenames: invoking the same command ID again under the same run ID replaces that command's prior artifacts. The parent owns run/command uniqueness and retry retention.

Use compact JSON on stdout when the caller needs returned artifact paths:

```bash
run_id=parent-run-001
manta --json --run-id "$run_id" run unit
```

The Manta process exits with the test command's non-zero code when available. Callers must capture output and artifact paths without treating every non-zero Manta process as an infrastructure failure.

## 4. Consume artifacts by purpose

With `--run-id`, artifacts are written under:

```text
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.status.json
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.json
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.md
.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.raw.log
.manta/runs/scoped/<run_id>/artifacts/test/excerpts/<failure-id>.log
```

Consume them in this order:

1. Use the process exit code as the immediate execution result.
2. Poll `status.json` for compact, deterministic state and deduplication.
3. Read `summary.json` for structured evidence or `summary.md` for human review.
4. Read bounded excerpts for individual failures.
5. Open or transmit the raw log only under the parent project's sensitive-data policy.

Do not rewrite artifact-reference fields before resolving them. Redaction intentionally leaves those paths literal while redacting surfaced command metadata and extracted content.

## 5. Interpret result and evidence quality separately

The parent project should model two dimensions:

| Dimension | Fields | Meaning |
|---|---|---|
| Command result | `status`, `exit_code` | Authoritative execution outcome |
| Evidence quality | `extractor_status` | `precise`, `partial`, `degraded`, or `no_match` |

Important cases:

- A failed, timed-out, or killed command stays non-pass even if extraction is `degraded`.
- A specialized-parser miss after a passing command is `passed` plus `no_match`.
- An extraction internal error after a passing command leaves artifact `exit_code: 0`, sets artifact `status: internal_error`, and makes Manta exit `4`.
- `summarize` has no authoritative process result; inferred status is evidence interpretation only.

See the [architecture extraction policy](architecture.md#failure-and-degraded-extraction-policy) for the full state table.

## 6. Poll without an agent

`status.json` is the stable watcher boundary. Manta materializes it after command execution and extraction finish; it does not write a `running` state or heartbeat. Until the file appears, the parent must distinguish “still running” from “invocation failed before artifact materialization” using its own process state.

A watcher that suppresses duplicate notifications must hash exactly this ordered input set:

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

Manta also writes `status_hash` from these final, redacted surfaced values. A parent watcher owns polling frequency, notification policy, retries, retention, and any transition into an external state store.

## 7. Integrate another evidence consumer

Treat Manta output as factual test evidence only. A consumer should:

- preserve the command result and `extractor_status` independently;
- preserve resolvable status, summary, and raw-log references;
- normalize references on the consumer side when its schema differs;
- not infer review, waiver, final, or acceptance state from Manta artifacts.

Manta remains standalone and imposes no evidence-consumer runtime dependency.

## Rollout checklist

- [ ] Pin and verify one Manta version.
- [ ] Ignore the entire `.manta/` directory.
- [ ] Provision local `.manta/tester.yaml` and any reviewed active rules on each development machine that needs them.
- [ ] Exercise one passing and one failing command through Manta.
- [ ] Exercise timeout handling for at least one long-running command.
- [ ] Confirm the selected parser recognizes the parent project's real logs; treat `degraded` as a rule/parser improvement signal.
- [ ] Confirm redaction in summary, status, excerpts, and console output using representative secrets.
- [ ] Confirm raw-log storage and sharing follow the parent project's sensitive-data policy.
- [ ] Verify the caller preserves underlying exit codes and does not treat parser quality as pass/fail.
- [ ] If using `--run-id`, verify all paths stay inside the matching run and the consumer resolves literal references unchanged.
- [ ] If polling, contract-test the status fields and ordered watcher hash inputs.
- [ ] Record which component owns retries, notifications, acceptance, retention, and cleanup.

## Compatibility and upgrades

Pin Manta by semantic version and run the parent project's passing/failing integration fixtures before upgrading. Changes to config, status/summary fields, exit semantics, parser behavior, redaction boundaries, or artifact layouts require synchronized updates to requirements, architecture, user documentation, and executable contract tests in this repository.

### Schema v2 tag migration

Schema v2 is a breaking replacement for the single-lane contract:

| Schema v1 surface | Schema v2 replacement |
|---|---|
| `version: 1` | `version: 2` |
| command or rule `lane: unit` | `tags: [unit]`, optionally with additional selector dimensions |
| `run --lane unit -- <command...>` | `run --tag unit [--tag <tag> ...] -- <command...>` |
| `rules propose --lane unit ...` | `rules propose --tag unit [--tag <tag> ...] ...` |
| summary/status `"lane": "unit"` | canonical JSON array `"tags": ["unit"]` |
| lane-derived ad-hoc command IDs | `adhoc-<UTC timestamp>` command IDs independent of tags |

There is no v1 decoder or `--lane` compatibility alias. Old config, rule fields, and CLI flags fail closed with config exit code `2` before command execution. Consumers that hash status fields must insert comma-joined canonical tags immediately after `command_id` in the ordered watcher input.

For exact CLI syntax and a complete tested rule fixture, see the [CLI reference](user-interface.md). For JSON shapes and path-safety semantics, see the [architecture](architecture.md).
