# KAT Architecture Decision Records

Status: Complete
Scope: Accepted baseline decisions

## ADR status legend

- Proposed: under discussion.
- Accepted: current project baseline.
- Superseded: replaced by a later ADR.
- Rejected: recorded but not adopted.

## ADR-0001: KAT remains standalone for v0.1

Status: Accepted
Date: 2026-06-24

### Context

KAT must be usable as an independent test-evidence tool in arbitrary repositories. Its first milestone should not require an external orchestration runtime.

### Decision

KAT v0.1 will be implemented as a standalone deterministic CLI. It may optionally write a fixed run-scoped artifact layout when a run ID is supplied, but it must not require an external orchestration runtime.

### Consequences

- KAT can be developed and tested independently.
- Integrations remain optional artifact consumers.
- Documentation must not assume that an orchestration runtime is available.

## ADR-0002: Command exit status is authoritative

Status: Accepted
Date: 2026-06-24

### Context

KAT extracts summaries from raw logs. Extraction quality may be precise, partial, degraded, or missing. A parser must not affect the truth of the executed command.

### Decision

The executed command's exit code and timeout/killed state determine pass/fail status. Rules and parsers only locate and summarize evidence. They must never convert a failing command into pass. Extraction quality is tracked separately by `extractor_status`.

`internal_error` is reserved for a KAT evidence-pipeline failure when no authoritative non-pass command result must be retained. If extraction fails after a command exited `0`, summary and status artifacts keep `exit_code: 0`, use `status: internal_error` and `extractor_status: degraded`, and the KAT process exits `4`. If the command already failed, timed out, or was killed, that state and exit code remain authoritative. Standalone summarize has no authoritative execution result, so an extraction internal error uses `status: internal_error` and exit code `4` in its artifacts and exits `4`.

### Consequences

- Failed commands with no matched failure span still fail.
- Specialized parser misses do not retry generic extraction.
- Extraction internal errors preserve failed, timed-out, and killed command truth while still materializing degraded artifacts when possible.
- `extractor_status: degraded` becomes a rule-mining signal, not a fallback pass path.
- CLI exit behavior remains useful in CI and scripts.

## ADR-0003: Preserve raw logs and write compact summaries

Status: Accepted
Date: 2026-06-24

### Context

Large raw logs are expensive for human and LLM review, but auditability requires preserving original evidence.

### Decision

KAT always preserves raw logs and writes compact summary JSON, summary Markdown, status JSON, and bounded excerpts. Noise filters affect summaries, not raw logs. Redaction applies to surfaced command metadata and extracted evidence. Raw logs remain original evidence and are not redacted by default; operators must be warned that raw logs may contain unredacted values. Stable artifact-reference fields remain literal locators so deterministic consumers can resolve them.

### Consequences

- Operators can review compact summaries first.
- Raw evidence remains available for audit and rule improvement.
- Summary artifacts can be consumed by automation, no-agent watchers, or humans.
- Raw-log sharing must be treated as a deliberate operator action.
- Run IDs, command IDs, output directories, and other artifact-bearing path components must not contain secrets because usable artifact references are not rewritten by redaction.

## ADR-0004: YAML project config with explicit argv command entries

Status: Accepted
Date: 2026-06-24

### Context

Different repositories use different test commands and log formats. KAT needs predictable command definitions without hard-coding project policy.

### Decision

KAT reads `.kkachi/tester.yaml` by default. Command entries define argv arrays, lane, parser, and timeout. Rule files may live under `.kkachi/tester/rules/*.yaml`.

### Consequences

- Project-local setup is explicit and reviewable.
- Configured commands avoid shell-quoting ambiguity.
- Invalid config can fail closed before command execution.
- Specialized parsers and rules can be introduced incrementally.

## ADR-0005: Status JSON is the watcher boundary

Status: Accepted
Date: 2026-06-24

### Context

Long-running test execution should not require an active agent to wait for completion. Watchers need a deterministic surface.

### Decision

KAT writes compact status JSON for polling. Configured redaction is applied to surfaced command ID, lane, and failure/warning signatures before their hashes are calculated. Watcher compatibility is defined by hashing exactly these ordered fields: `command_id`, `status`, `exit_code`, `extractor_status`, `raw_log_sha256`, `failure_signatures`, `warning_signatures`, `summary_path`, and `raw_log_path`. Path fields remain literal references.

### Consequences

- KAT supports no-agent polling without embedding watcher logic.
- Status fields and path references must stay stable.
- `status_hash` is calculated after redaction from the final surfaced values.
- Full review remains outside KAT.

## ADR-0006: Go single-binary implementation baseline

Status: Accepted
Date: 2026-06-24

### Context

KAT needs a boring implementation baseline with straightforward process execution, deterministic file IO, YAML support, and simple binary distribution.

### Decision

KAT v0.1 is implemented in Go and packaged as a standalone single binary named `kkachi-agent-tester`.

### Consequences

- Local development centers on Go tooling and module layout.
- Distribution can target a single compiled binary per platform.
- Runner, artifact, and parser behavior can be implemented without a runtime dependency on Node or Python.

## ADR-0007: Regex safety uses Go RE2 plus explicit size bounds

Status: Accepted
Date: 2026-06-24

### Context

Rules, redaction, and extraction all depend on regex, but regex safety must not rely on best-effort timeouts or broad fallback behavior.

### Decision

KAT uses Go `regexp` with RE2 semantics only. Unsupported or invalid regex fails closed. Safety is reinforced with explicit bounds on regex input size, extracted block size, excerpt size, and summary size.

### Consequences

- Catastrophic-backtracking-style regex behavior is avoided by construction.
- PCRE-only features are out of scope for project-local rules.
- Validation and documentation can state a crisp supported-regex surface.

## ADR-0008: First runnable slice requires only the generic parser

Status: Accepted
Date: 2026-06-24

### Context

The CLI, runner, and artifact pipeline are useful before fixture-backed specialized parsers exist, and implementing parser labels without evidence creates busywork.

### Decision

The first runnable KAT implementation requires only the `generic` parser. Specialized parser labels may exist in config and CLI contracts, but unsupported labels fail closed until they are implemented from real fixture evidence.

### Consequences

- MVP scope stays focused on execution and artifact correctness.
- Specialized parsers are added against real repository evidence instead of invented formats.
- Rule proposal remains useful even before runner-specific parsers exist.

## Future ADR candidates

- Built-in parser module boundary after specialized parsers exist.
- CI integration surface.
