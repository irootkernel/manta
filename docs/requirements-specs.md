# Manta Requirement Specs

Status: v0.1 baseline complete; hardening requirements complete (`HARDE-001` through `HARDE-007` complete)
Scope: Manta v0.1 standalone baseline and post-baseline hardening
Source context: standalone deterministic Manta v0.1 CLI behavior and evidence contracts.

## Requirement status legend

- `[ ]` Not started
- `[~]` In progress; track details in `todo.md`
- `[x]` Complete
- `Blocked` means external decision or missing dependency prevents implementation.

Implementation note: the original v0.1 roadmap and the recorded `RQHAR` hardening requirements are implemented. A checked requirement means that specific behavior is implemented and mapped to evidence; it does not imply support for capabilities outside its wording. See the [integration guide](integration-guide.md) for the current capability matrix and explicit v0.1 boundaries. No open implementation items are currently recorded in `todo.md`.

## RQCLI: Command-line interface

- [x] `MANTA-REQ-RQCLI-001` Provide a standalone CLI binary named `manta`.
- [x] `MANTA-REQ-RQCLI-002` Support configured command execution with `manta run <command-id>`.
- [x] `MANTA-REQ-RQCLI-003` Support ad-hoc command execution with `manta run --lane <lane> -- <command...>`.
- [x] `MANTA-REQ-RQCLI-004` Support raw-log summarization with `manta summarize <raw-log>`.
- [x] `MANTA-REQ-RQCLI-005` Support excerpt retrieval with `manta excerpt --summary <summary-path> <failure-id>`.
- [x] `MANTA-REQ-RQCLI-006` Return process-compatible exit codes: successful test commands exit `0`, failed test commands return the underlying non-zero exit code when possible, and Manta internal errors use distinct documented codes.

## RQCFG: Project configuration

- [x] `MANTA-REQ-RQCFG-001` Read default project config from `.manta/tester.yaml`.
- [x] `MANTA-REQ-RQCFG-002` Allow explicit config override with a CLI flag.
- [x] `MANTA-REQ-RQCFG-003` Define command entries with `command` argv arrays, `lane`, `parser`, and `timeout_sec`.
- [x] `MANTA-REQ-RQCFG-004` Define noise filters that remove low-value lines from summaries without removing raw-log content.
- [x] `MANTA-REQ-RQCFG-005` Define redaction rules that apply to summary, status, and excerpt outputs.
- [x] `MANTA-REQ-RQCFG-006` Validate config before execution and fail closed on invalid command IDs, unsafe timeout values, malformed redaction rules, unsupported parser labels, or invalid rule files.

## RQRUN: Deterministic command runner

- [x] `MANTA-REQ-RQRUN-001` Execute configured and ad-hoc commands in the target repository working directory.
- [x] `MANTA-REQ-RQRUN-002` Capture stdout and stderr while preserving ordering when possible.
- [x] `MANTA-REQ-RQRUN-003` Preserve raw logs exactly as observed before summary noise filtering.
- [x] `MANTA-REQ-RQRUN-004` Record exit code, start time, end time, duration, command argv, command ID, lane, and parser.
- [x] `MANTA-REQ-RQRUN-005` Enforce per-command timeout and report `timed_out` status without claiming pass.
- [x] `MANTA-REQ-RQRUN-006` Handle interrupted/killed runs with explicit status and partial raw-log preservation.

## RQART: Artifact outputs

- [x] `MANTA-REQ-RQART-001` Write raw log artifacts to `.manta/runs/scoped/<run_id>/artifacts/test/<command-id>.raw.log` when a run ID is supplied.
- [x] `MANTA-REQ-RQART-002` Support standalone artifact output under `.manta/` or a caller-specified output directory when `--run-id` is not supplied.
- [x] `MANTA-REQ-RQART-003` Write summary JSON with execution status, command metadata, raw-log path, raw-log SHA-256, extractor status, failure count, warning count, failure spans, warning spans, and excerpt references.
- [x] `MANTA-REQ-RQART-004` Write summary Markdown for human review.
- [x] `MANTA-REQ-RQART-005` Write status JSON suitable for no-agent watchers.
- [x] `MANTA-REQ-RQART-006` Write failure excerpt files for bounded review without replaying full raw logs.
- [x] `MANTA-REQ-RQART-007` Keep all generated artifact paths stable and relative to the repository root where practical.

## RQEXT: Extraction and parser behavior

- [x] `MANTA-REQ-RQEXT-001` Provide a generic parser that can identify common failure and warning patterns.
- [x] `MANTA-REQ-RQEXT-002` Support parser labels such as `generic`, `vitest`, `pytest`, `go-test`, and `playwright`, while requiring only `generic` in the first runnable implementation slice.
- [x] `MANTA-REQ-RQEXT-003` Extract bounded failure spans with start/end line and byte offsets.
- [x] `MANTA-REQ-RQEXT-004` Extract signature, file, line, test name, stack-top entries, and excerpt path when available.
- [x] `MANTA-REQ-RQEXT-005` Report `extractor_status` as `precise`, `partial`, `degraded`, or `no_match`.
- [x] `MANTA-REQ-RQEXT-006` Report degraded extraction when a failed, timed-out, or killed command has no useful failure span.
- [x] `MANTA-REQ-RQEXT-007` Never use extraction rules, parser matches, or parser misses to override the executed command's exit code or authoritative non-pass status.

## RQRUL: Rule lifecycle and CRUD

- [x] `MANTA-REQ-RQRUL-001` Provide `rules list`, `rules search`, `rules show`, `rules create`, `rules update`, `rules delete`, `rules test`, and `rules propose` command surfaces.
- [x] `MANTA-REQ-RQRUL-002` Store project-local rules in `.manta/tester/rules/*.yaml`.
- [x] `MANTA-REQ-RQRUL-003` Preserve rule provenance: source run, command, raw-log checksum, source span, reason, creator, and status.
- [x] `MANTA-REQ-RQRUL-004` Support disabled rules and deletion reasons.
- [x] `MANTA-REQ-RQRUL-005` Test rules against raw-log fixtures and expected spans.
- [x] `MANTA-REQ-RQRUL-006` Detect overmatch, unsupported or invalid regex, excessive block length, and invalid capture groups.
- [x] `MANTA-REQ-RQRUL-007` Keep run-local proposed rules separate from project-local active rules.

## RQSEC: Safety, redaction, and fail-closed behavior

- [x] `MANTA-REQ-RQSEC-001` Redact configured secrets and sensitive values from summaries, excerpts, and status files while retaining literal artifact-reference fields required for deterministic lookup.
- [x] `MANTA-REQ-RQSEC-002` Preserve raw logs as original evidence, clearly mark that they may contain unredacted data, and avoid treating them as share-safe artifacts.
- [x] `MANTA-REQ-RQSEC-003` Fail closed on malformed config, missing command definitions, invalid or unsupported regex, artifact-write failure, or unsupported parser configuration.
- [x] `MANTA-REQ-RQSEC-004` Bound extracted block size, excerpt size, summary size, and regex input size.
- [x] `MANTA-REQ-RQSEC-005` Avoid broad fallback behavior; a specialized-parser miss reports `no_match` after a pass and `degraded` after a non-pass result, while an accepted span with missing key metadata remains `partial`.

## RQWAT: Watcher status compatibility

- [x] `MANTA-REQ-RQWAT-001` Produce deterministic status JSON that no-agent watchers can poll without invoking an LLM.
- [x] `MANTA-REQ-RQWAT-002` Define watcher compatibility around exactly these status-hash inputs: command ID, status, exit code, extractor status, raw-log checksum, failure signatures, warning signatures, summary path, and raw-log path.
- [x] `MANTA-REQ-RQWAT-003` Keep watcher-facing output compact and action-oriented.

## RQDOC: Documentation and operator guidance

- [x] `MANTA-REQ-RQDOC-001` Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes.
- [x] `MANTA-REQ-RQDOC-002` Add CLI examples after the first executable implementation exists. See roadmap task `DOCUM-002`.
- [x] `MANTA-REQ-RQDOC-003` Add parser/rule examples based on real fixture logs. See roadmap task `RULES-003`.
- [x] `MANTA-REQ-RQDOC-004` Add release-readiness checklist before tagging Manta v0.1.0. See roadmap task `DOCUM-003`.

## RQHAR: Post-baseline hardening and contract closure

- [x] `MANTA-REQ-RQHAR-001` Validate every artifact-bearing identifier and reference, reject path syntax in identifiers, and fail closed when a resolved path or symlink would escape its allowed artifact boundary.
- [x] `MANTA-REQ-RQHAR-002` Plan and open raw-log artifacts before command execution, handle operator interruption signals explicitly, forward termination to the child process, and preserve bounded partial raw/status evidence with an explicit non-pass state.
- [x] `MANTA-REQ-RQHAR-003` Allocate collision-free standalone run directories so repeated executions in the same timestamp interval never overwrite earlier raw, summary, status, or excerpt artifacts.
- [x] `MANTA-REQ-RQHAR-004` Apply configured redaction consistently to surfaced summary, status, excerpt, and console-safe command metadata while preserving original raw logs and usable literal artifact references unchanged.
- [x] `MANTA-REQ-RQHAR-005` Define one fail-closed contract for specialized-parser misses and internal errors, align implementation and documentation to that contract, and test `precise`, `partial`, `degraded`, `no_match`, and any retained `internal_error` behavior.
- [x] `MANTA-REQ-RQHAR-006` Make every documented CLI option and example match executable behavior, including the disposition of `--verbose` and `--no-color`, self-contained rule examples, generated Markdown shape, and toolchain resolver/operator guidance.
- [x] `MANTA-REQ-RQHAR-007` Add end-to-end regression coverage for artifact containment, symlink escape, interruption, collision resistance, redaction boundaries, parser/error-state behavior, CLI examples, and both standalone and `--run-id` layouts before declaring hardening complete.

## Out of scope for v0.1 standalone setup

These are intentional current boundaries, not incomplete checked requirements or implicit roadmap commitments. Integration owners should also review [Not provided by Manta v0.1](integration-guide.md#not-provided-by-manta-v01).

- External workflow orchestration, session management, or acceptance-state management.
- Automatic issue tracker creation.
- Any rule that changes command pass/fail status.
- Live runtime state changes, credentials, secrets, or provider configuration.
