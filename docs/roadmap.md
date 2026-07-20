# KAT Roadmap

Status: v0.1 standalone MVP complete; GAJAE integration-contract work complete; HARDE hardening epic in progress
Scope: Implementation tracking for the KAT v0.1 standalone baseline and post-baseline hardening

Task status values: `Planned`, `In Progress`, `Blocked`, `Done`, `Deferred`.

Existing `Done` entries record completion of the original v0.1 implementation slices. They do not supersede or satisfy the later `HARDE` tasks, which close correctness, safety, verification, and documentation gaps found during repository review.

Current implementation snapshot:
- `Done`: `SETUP-001` to `SETUP-003`, `RUNNR-001` to `RUNNR-003`, `ARTIF-001` to `ARTIF-003`, `PARSE-001` to `PARSE-003`, `SAFEY-001` to `SAFEY-003`, `CLIUX-001`, `CLIUX-002`, `RULES-001` to `RULES-003`, `DOCUM-001` to `DOCUM-003`, `HARDE-001` to `HARDE-006`
- `In Progress`: none
- `Deferred`: none
- `Planned`: `HARDE-007`
- `GAJAE complete`: `GAJAE-009` documented that KAH normalizes existing KAT v0.1.0 artifacts without KAT source changes; `GAJAE-010` finalized durable operator docs/skill guidance after the KAH-side normalization contract

## SETUP: Project foundation

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| SETUP-001 | Done | Initialize Go module structure, CLI entrypoint placeholder, single-binary packaging baseline, formatter/lint/test command scaffolding, and repository README. | `KAT-REQ-RQCLI-001`, `ADR-0006`, `KAT-REQ-RQDOC-004` |
| SETUP-002 | Done | Implement config discovery, config override flag, schema validation, and fail-closed config diagnostics. | `KAT-REQ-RQCFG-001` to `KAT-REQ-RQCFG-006` |
| SETUP-003 | Done | Define shared domain models for command config, run metadata, artifact references, summary, status, spans, failures, warnings, and watcher hash inputs. | `KAT-REQ-RQART-003`, `KAT-REQ-RQWAT-001`, `KAT-REQ-RQWAT-002` |

## RUNNR: Command runner

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| RUNNR-001 | Done | Implement configured command execution with working-directory control, stdout/stderr capture, ordered log buffering, and raw-log persistence. | `KAT-REQ-RQCLI-002`, `KAT-REQ-RQRUN-001` to `KAT-REQ-RQRUN-004` |
| RUNNR-002 | Done | Implement ad-hoc command execution with `run --lane <lane> -- <command...>` and generated command IDs for standalone artifacts. | `KAT-REQ-RQCLI-003`, `KAT-REQ-RQRUN-001` |
| RUNNR-003 | Done | Implement timeout, killed/interrupted status, partial log preservation, and process-compatible exit-code handling. | `KAT-REQ-RQRUN-005`, `KAT-REQ-RQRUN-006`, `KAT-REQ-RQCLI-006` |

## ARTIF: Artifact writer

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| ARTIF-001 | Done | Implement artifact path planning for `.kat/` standalone output, caller-specified output directories, and optional `.kkachi/runs/<run_id>/...` layout. | `KAT-REQ-RQART-001`, `KAT-REQ-RQART-002`, `KAT-REQ-RQART-007` |
| ARTIF-002 | Done | Write summary JSON, summary Markdown, raw-log SHA-256, duration metadata, failure/warning counts, and artifact references. | `KAT-REQ-RQART-003`, `KAT-REQ-RQART-004` |
| ARTIF-003 | Done | Write status JSON, stable watcher hash inputs, and bounded failure excerpts suitable for no-agent watcher and compact human review. | `KAT-REQ-RQART-005`, `KAT-REQ-RQART-006`, `KAT-REQ-RQWAT-001` to `KAT-REQ-RQWAT-003` |

## PARSE: Extraction engine

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| PARSE-001 | Done | Implement generic parser for common failure, warning, file-line, stack-top, and test-name patterns with bounded spans. | `KAT-REQ-RQEXT-001`, `KAT-REQ-RQEXT-003`, `KAT-REQ-RQEXT-004` |
| PARSE-002 | Done | Implement parser registry and parser labels while requiring only `generic` in the first runnable slice and failing closed on unsupported specialized labels. | `KAT-REQ-RQEXT-002`, `ADR-0008`, `KAT-REQ-RQSEC-003` |
| PARSE-003 | Done | Implement extractor status computation and degraded extraction signals for non-zero exits with missing or overly broad spans. | `KAT-REQ-RQEXT-005`, `KAT-REQ-RQEXT-006`, `KAT-REQ-RQEXT-007` |

## SAFEY: Safety and filtering

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| SAFEY-001 | Done | Implement redaction pipeline for summaries, excerpts, status files, and console-safe output, with raw-log handling warnings clearly marked. | `KAT-REQ-RQSEC-001`, `KAT-REQ-RQSEC-002`, `ADR-0003` |
| SAFEY-002 | Done | Implement noise filtering for summaries without altering raw logs. | `KAT-REQ-RQCFG-004`, `KAT-REQ-RQSEC-005` |
| SAFEY-003 | Done | Add RE2-based regex validation plus bounds for regex input size, extracted block size, excerpt size, summary size, and overmatch diagnostics. | `KAT-REQ-RQSEC-003`, `KAT-REQ-RQSEC-004`, `KAT-REQ-RQRUL-006`, `ADR-0007` |

## CLIUX: Direct artifact commands

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| CLIUX-001 | Done | Implement `summarize <raw-log>` so existing logs can be converted into KAT summary and status artifacts without rerunning the command. | `KAT-REQ-RQCLI-004`, `KAT-REQ-RQART-003` to `KAT-REQ-RQART-006` |
| CLIUX-002 | Done | Implement deterministic excerpt retrieval with `excerpt --summary <summary-path> <failure-id>`. | `KAT-REQ-RQCLI-005`, `ADR-0002` |

## RULES: Rule management

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| RULES-001 | Done | Implement rule storage, list/search/show, disabled-rule handling, deletion reason, and project-local rule loading. | `KAT-REQ-RQRUL-001`, `KAT-REQ-RQRUL-002`, `KAT-REQ-RQRUL-004` |
| RULES-002 | Done | Implement create/update validation with provenance requirements, RE2-safe matching config, and capture group diagnostics. | `KAT-REQ-RQRUL-003`, `KAT-REQ-RQRUL-006`, `ADR-0007` |
| RULES-003 | Done | Implement rule test and rule propose from raw-log span, including run-local proposed rule separation. | `KAT-REQ-RQRUL-005`, `KAT-REQ-RQRUL-007`, `KAT-REQ-RQDOC-003` |

## DOCUM: Documentation and release readiness

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| DOCUM-001 | Done | Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes. | `KAT-REQ-RQDOC-001` |
| DOCUM-002 | Done | Add real CLI examples, config examples, and artifact examples after first runnable implementation. | `KAT-REQ-RQDOC-002`, `TD-DOC-001` |
| DOCUM-003 | Done | Add release-readiness checklist, fixture evidence expectations, and v0.1 packaging notes before tagging. | `KAT-REQ-RQDOC-004`, `TD-REL-001` |

## HARDE: Post-baseline hardening and contract closure

Implement these tasks as separate, reviewable PRs in numerical order. A task moves to `Done` only after its focused verification and the existing affected test suites pass. `HARDE-007` is the final end-to-end hardening gate.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| HARDE-001 | Done | Enforce fail-closed artifact containment for run IDs, configured command IDs, rule IDs, and excerpt references; reject absolute paths, traversal, cross-run access, and symlink escape. | Add traversal and symlink tests, then pass focused artifact/config/rules/CLI tests. | `KAT-REQ-RQHAR-001`, `KAT-REQ-RQCFG-006`, `KAT-REQ-RQART-001`, `KAT-REQ-RQSEC-003` |
| HARDE-002 | Done | Make command execution interruption-safe by preparing raw evidence before execution, handling and forwarding termination signals, and preserving partial raw/status artifacts with an explicit non-pass result. | Exercise a built binary with SIGINT and SIGTERM, verify partial evidence and status, then pass runner/CLI/E2E tests. | `KAT-REQ-RQHAR-002`, `KAT-REQ-RQRUN-003`, `KAT-REQ-RQRUN-005`, `KAT-REQ-RQRUN-006` |
| HARDE-003 | Done | Prevent standalone artifact overwrite by allocating collision-free run directories for repeated configured, ad-hoc, and summarize operations. | Run equivalent commands repeatedly within one timestamp interval, verify distinct paths and checksums, then pass artifact/CLI/E2E tests. | `KAT-REQ-RQHAR-003`, `KAT-REQ-RQART-002`, `KAT-REQ-RQART-007`, `ADR-0003` |
| HARDE-004 | Done | Complete the redaction boundary for surfaced summary, status, excerpt, and console-safe metadata while leaving original raw logs and literal artifact references unchanged. | Test secrets in argv, identifiers, lanes, evidence-origin paths, failures, and warnings; verify redacted surface fields, unchanged raw evidence, usable artifact references, and final status hashes, then pass safety/CLI/E2E tests. | `KAT-REQ-RQHAR-004`, `KAT-REQ-RQCFG-005`, `KAT-REQ-RQSEC-001`, `KAT-REQ-RQSEC-002`, `ADR-0003` |
| HARDE-005 | Done | Resolve and implement the specialized-parser miss and internal-error artifact contracts without allowing extraction behavior to override command truth. | Add contract tests for all extractor states and retained run states, then pass extract/CLI/guardrail tests. | `KAT-REQ-RQHAR-005`, `KAT-REQ-RQEXT-005` to `KAT-REQ-RQEXT-007`, `KAT-REQ-RQSEC-005`, `ADR-0002` |
| HARDE-006 | Done | Synchronize executable CLI behavior and durable documentation, including `--verbose`, `--no-color`, self-contained rule examples, Markdown output, version/toolchain resolver guidance, and roadmap/todo status wording. | Execute every documented command against a fresh fixture, compare generated output with examples, and pass CLI/toolchain E2E tests plus `git diff --check`. | `KAT-REQ-RQHAR-006`, `KAT-REQ-RQCLI-001` to `KAT-REQ-RQCLI-006`, `KAT-REQ-RQDOC-001` to `KAT-REQ-RQDOC-004` |
| HARDE-007 | Planned | Run the complete hardening regression and release-readiness gate across standalone and Kkachi-compatible layouts, then update hardening statuses only from observed evidence. | Pass `make test`, configured/ad-hoc/summarize/excerpt/rules smokes, path and signal probes, both artifact layouts, install/toolchain checks, and `git diff --check`. | `KAT-REQ-RQHAR-007`, `KAT-REQ-RQDOC-004`, `TD-HARDE-001` |

## GAJAE: KAS/KAH pilot-unblock integration contract

KAT remains a standalone deterministic tester. These GAJAE entries document integration-contract work only; they do not move KAS command semantics, KAH run-state ledger behavior, GJC session management, or Kkachi acceptance authority into KAT.

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| GAJAE-009 | Done | KAH-side normalization is sufficient for KAH attachment of existing KAT v0.1.0 factual status/summary/raw-log evidence without KAT source changes or a KAT-emitted compatibility snapshot. Preserve command exit code, extractor status, summary/raw refs, and no-authority semantics. | `KAT-REQ-RQART-003`, `KAT-REQ-RQART-005`, `KAT-REQ-RQWAT-001`, `ADR-0001`, `ADR-0002` |
| GAJAE-010 | Done | Updated durable KAS/KAH/KAT operator docs and skill guidance now that GAJAE-009 settled the final attach contract: raw KAT status remains unchanged, while KAH normalizes status/summary/raw-log refs for attachment. KAT remains factual-only and does not add source/schema behavior, review authority, MAR authority, waiver authority, or final acceptance authority for this closeout. | `KAT-REQ-RQDOC-001`, `KAT-REQ-RQDOC-002`, `KAT-REQ-RQWAT-002` |
