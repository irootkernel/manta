# KAT Requirement Specs

Status: v0.1 baseline complete; hardening requirements in progress (`HARDE-001` through `HARDE-004` complete)
Scope: KAT v0.1 standalone baseline and post-baseline hardening
Source context: KAS v0.2 / KAH v0.2 / KAT v0.1 / GJC delegated execution SOT, with KAT kept independent from KAS and KAH for this repository setup.

## Requirement status legend

- `[ ]` Not started
- `[~]` In progress; track details in `todo.md`
- `[x]` Complete
- `Blocked` means external decision or missing dependency prevents implementation.

Implementation note: the original v0.1 roadmap is implemented. Repository review identified post-baseline correctness and contract gaps that are now tracked as the `RQHAR` requirements below and the `HARDE` epic in `roadmap.md`. Existing baseline completion records do not imply that the hardening requirements are complete.

## RQCLI: Command-line interface

- [x] `KAT-REQ-RQCLI-001` Provide a standalone CLI binary named `kkachi-agent-tester`.
- [x] `KAT-REQ-RQCLI-002` Support configured command execution with `kkachi-agent-tester run <command-id>`.
- [x] `KAT-REQ-RQCLI-003` Support ad-hoc command execution with `kkachi-agent-tester run --lane <lane> -- <command...>`.
- [x] `KAT-REQ-RQCLI-004` Support raw-log summarization with `kkachi-agent-tester summarize <raw-log>`.
- [x] `KAT-REQ-RQCLI-005` Support excerpt retrieval with `kkachi-agent-tester excerpt --summary <summary-path> <failure-id>`.
- [x] `KAT-REQ-RQCLI-006` Return process-compatible exit codes: successful test commands exit `0`, failed test commands return the underlying non-zero exit code when possible, and KAT internal errors use distinct documented codes.

## RQCFG: Project configuration

- [x] `KAT-REQ-RQCFG-001` Read default project config from `.kkachi/tester.yaml`.
- [x] `KAT-REQ-RQCFG-002` Allow explicit config override with a CLI flag.
- [x] `KAT-REQ-RQCFG-003` Define command entries with `command` argv arrays, `lane`, `parser`, and `timeout_sec`.
- [x] `KAT-REQ-RQCFG-004` Define noise filters that remove low-value lines from summaries without removing raw-log content.
- [x] `KAT-REQ-RQCFG-005` Define redaction rules that apply to summary, status, and excerpt outputs.
- [x] `KAT-REQ-RQCFG-006` Validate config before execution and fail closed on invalid command IDs, unsafe timeout values, malformed redaction rules, unsupported parser labels, or invalid rule files.

## RQRUN: Deterministic command runner

- [x] `KAT-REQ-RQRUN-001` Execute configured and ad-hoc commands in the target repository working directory.
- [x] `KAT-REQ-RQRUN-002` Capture stdout and stderr while preserving ordering when possible.
- [x] `KAT-REQ-RQRUN-003` Preserve raw logs exactly as observed before summary noise filtering.
- [x] `KAT-REQ-RQRUN-004` Record exit code, start time, end time, duration, command argv, command ID, lane, and parser.
- [x] `KAT-REQ-RQRUN-005` Enforce per-command timeout and report `timed_out` status without claiming pass.
- [x] `KAT-REQ-RQRUN-006` Handle interrupted/killed runs with explicit status and partial raw-log preservation.

## RQART: Artifact outputs

- [x] `KAT-REQ-RQART-001` Write raw log artifacts to `.kkachi/runs/<run_id>/artifacts/test/<command-id>.raw.log` when a run ID is supplied.
- [x] `KAT-REQ-RQART-002` Support standalone artifact output under `.kat/` or a caller-specified output directory when no Kkachi run ID is supplied.
- [x] `KAT-REQ-RQART-003` Write summary JSON with execution status, command metadata, raw-log path, raw-log SHA-256, extractor status, failure count, warning count, failure spans, warning spans, and excerpt references.
- [x] `KAT-REQ-RQART-004` Write summary Markdown for human review.
- [x] `KAT-REQ-RQART-005` Write status JSON suitable for no-agent watchers.
- [x] `KAT-REQ-RQART-006` Write failure excerpt files for bounded review without replaying full raw logs.
- [x] `KAT-REQ-RQART-007` Keep all generated artifact paths stable and relative to the repository root where practical.

## RQEXT: Extraction and parser behavior

- [x] `KAT-REQ-RQEXT-001` Provide a generic parser that can identify common failure and warning patterns.
- [x] `KAT-REQ-RQEXT-002` Support parser labels such as `generic`, `vitest`, `pytest`, `go-test`, and `playwright`, while requiring only `generic` in the first runnable implementation slice.
- [x] `KAT-REQ-RQEXT-003` Extract bounded failure spans with start/end line and byte offsets.
- [x] `KAT-REQ-RQEXT-004` Extract signature, file, line, test name, stack-top entries, and excerpt path when available.
- [x] `KAT-REQ-RQEXT-005` Report `extractor_status` as `precise`, `partial`, `degraded`, or `no_match`.
- [x] `KAT-REQ-RQEXT-006` Report degraded extraction when a command fails but no useful failure span is found.
- [x] `KAT-REQ-RQEXT-007` Never use extraction rules to override command exit status.

## RQRUL: Rule lifecycle and CRUD

- [x] `KAT-REQ-RQRUL-001` Provide `rules list`, `rules search`, `rules show`, `rules create`, `rules update`, `rules delete`, `rules test`, and `rules propose` command surfaces.
- [x] `KAT-REQ-RQRUL-002` Store project-local rules in `.kkachi/tester.yaml` or `.kkachi/tester/rules/*.yaml`.
- [x] `KAT-REQ-RQRUL-003` Preserve rule provenance: source run, command, raw-log checksum, source span, reason, creator, and status.
- [x] `KAT-REQ-RQRUL-004` Support disabled rules and deletion reasons.
- [x] `KAT-REQ-RQRUL-005` Test rules against raw-log fixtures and expected spans.
- [x] `KAT-REQ-RQRUL-006` Detect overmatch, unsupported or invalid regex, excessive block length, and invalid capture groups.
- [x] `KAT-REQ-RQRUL-007` Keep run-local proposed rules separate from project-local active rules.

## RQSEC: Safety, redaction, and fail-closed behavior

- [x] `KAT-REQ-RQSEC-001` Redact configured secrets and sensitive values from summaries, excerpts, and status files while retaining literal artifact-reference fields required for deterministic lookup.
- [x] `KAT-REQ-RQSEC-002` Preserve raw logs as original evidence, clearly mark that they may contain unredacted data, and avoid treating them as share-safe artifacts.
- [x] `KAT-REQ-RQSEC-003` Fail closed on malformed config, missing command definitions, invalid or unsupported regex, artifact-write failure, or unsupported parser configuration.
- [x] `KAT-REQ-RQSEC-004` Bound extracted block size, excerpt size, summary size, and regex input size.
- [x] `KAT-REQ-RQSEC-005` Avoid broad fallback behavior; when precision is not available, mark extraction as degraded.

## RQWAT: Watcher status compatibility

- [x] `KAT-REQ-RQWAT-001` Produce deterministic status JSON that no-agent watchers can poll without invoking an LLM.
- [x] `KAT-REQ-RQWAT-002` Define watcher compatibility around exactly these status-hash inputs: command ID, status, exit code, extractor status, raw-log checksum, failure signatures, warning signatures, summary path, and raw-log path.
- [x] `KAT-REQ-RQWAT-003` Keep watcher-facing output compact and action-oriented.
- [x] `KAT-REQ-GAJAE-009` For GAJAE-009, confirm KAH-side normalization can consume existing KAT v0.1.0 status/summary/raw-log artifacts without KAT source changes. KAT does not emit an additional bindable GAJAE evidence snapshot, and KAT output remains factual evidence only with no review, MAR, waiver, final, or acceptance claims.

## RQDOC: Documentation and operator guidance

- [x] `KAT-REQ-RQDOC-001` Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes.
- [x] `KAT-REQ-RQDOC-002` Add CLI examples after the first executable implementation exists. See `todo.md#TD-DOC-001`.
- [x] `KAT-REQ-RQDOC-003` Add parser/rule examples based on real fixture logs. See `todo.md#TD-RULE-001`.
- [x] `KAT-REQ-RQDOC-004` Add release-readiness checklist before tagging KAT v0.1.0. See `todo.md#TD-REL-001`.

## RQHAR: Post-baseline hardening and contract closure

- [x] `KAT-REQ-RQHAR-001` Validate every artifact-bearing identifier and reference, reject path syntax in identifiers, and fail closed when a resolved path or symlink would escape its allowed artifact boundary.
- [x] `KAT-REQ-RQHAR-002` Plan and open raw-log artifacts before command execution, handle operator interruption signals explicitly, forward termination to the child process, and preserve bounded partial raw/status evidence with an explicit non-pass state.
- [x] `KAT-REQ-RQHAR-003` Allocate collision-free standalone run directories so repeated executions in the same timestamp interval never overwrite earlier raw, summary, status, or excerpt artifacts.
- [x] `KAT-REQ-RQHAR-004` Apply configured redaction consistently to surfaced summary, status, excerpt, and console-safe command metadata while preserving original raw logs and usable literal artifact references unchanged.
- [ ] `KAT-REQ-RQHAR-005` Define one fail-closed contract for specialized-parser misses and internal errors, align implementation and documentation to that contract, and test `precise`, `partial`, `degraded`, `no_match`, and any retained `internal_error` behavior.
- [ ] `KAT-REQ-RQHAR-006` Make every documented CLI option and example match executable behavior, including the disposition of `--verbose` and `--no-color`, self-contained rule examples, generated Markdown shape, and toolchain resolver/operator guidance.
- [ ] `KAT-REQ-RQHAR-007` Add end-to-end regression coverage for artifact containment, symlink escape, interruption, collision resistance, redaction boundaries, parser/error-state behavior, CLI examples, and both standalone and `--run-id` layouts before declaring hardening complete.

## Out of scope for v0.1 standalone setup

- KAS command semantics or skill packaging.
- KAH run-state ledger implementation; GAJAE-009 is satisfied by KAH-side normalization of existing KAT v0.1.0 factual status/summary/raw-log artifacts without KAT source changes.
- GJC session management.
- Automatic issue tracker creation.
- Any rule that changes command pass/fail status.
- Live runtime state changes, credentials, secrets, or provider configuration.
