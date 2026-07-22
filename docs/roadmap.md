# Manta Roadmap

Status: v0.1 standalone MVP, HARDE hardening epic, `TAGS-001`, and `RELRV-001` through `RELRV-003` complete
Scope: Implementation tracking for the Manta v0.1 standalone baseline, post-baseline hardening, schema-v2 tag migration, and release-readiness follow-up

This roadmap is a delivery record, not an operator guide or a promise that out-of-scope capabilities will be added. See the [integration guide](integration-guide.md) for the current supported/unsupported capability boundary and `todo.md` for explicitly accepted open work.

Task status values: `Planned`, `In Progress`, `Blocked`, `Done`, `Deferred`.

Existing `Done` entries record completion of the original v0.1 implementation slices. They do not supersede or satisfy the later `HARDE` tasks, which close correctness, safety, verification, and documentation gaps found during repository review.

Current implementation snapshot:
- `Done`: `SETUP-001` to `SETUP-003`, `RUNNR-001` to `RUNNR-003`, `ARTIF-001` to `ARTIF-003`, `PARSE-001` to `PARSE-003`, `SAFEY-001` to `SAFEY-003`, `CLIUX-001`, `CLIUX-002`, `RULES-001` to `RULES-003`, `DOCUM-001` to `DOCUM-003`, `HARDE-001` to `HARDE-007`, `TAGS-001`, `RELRV-001`, `RELRV-002`, `RELRV-003`
- `In Progress`: none
- `Deferred`: none
- `Planned`: none

## SETUP: Project foundation

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| SETUP-001 | Done | Initialize Go module structure, CLI entrypoint placeholder, single-binary packaging baseline, formatter/lint/test command scaffolding, and repository README. | `MANTA-REQ-RQCLI-001`, `ADR-0006`, `MANTA-REQ-RQDOC-004` |
| SETUP-002 | Done | Implement config discovery, config override flag, schema validation, and fail-closed config diagnostics. | `MANTA-REQ-RQCFG-001` to `MANTA-REQ-RQCFG-006` |
| SETUP-003 | Done | Define shared domain models for command config, run metadata, artifact references, summary, status, spans, failures, warnings, and watcher hash inputs. | `MANTA-REQ-RQART-003`, `MANTA-REQ-RQWAT-001`, `MANTA-REQ-RQWAT-002` |

## RUNNR: Command runner

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| RUNNR-001 | Done | Implement configured command execution with working-directory control, stdout/stderr capture, ordered log buffering, and raw-log persistence. | `MANTA-REQ-RQCLI-002`, `MANTA-REQ-RQRUN-001` to `MANTA-REQ-RQRUN-004` |
| RUNNR-002 | Done | Implement ad-hoc command execution with repeatable `--tag` selectors and `adhoc-<UTC timestamp>` command IDs for standalone artifacts. | `MANTA-REQ-RQCLI-003`, `MANTA-REQ-RQRUN-001` |
| RUNNR-003 | Done | Implement timeout, killed/interrupted status, partial log preservation, and process-compatible exit-code handling. | `MANTA-REQ-RQRUN-005`, `MANTA-REQ-RQRUN-006`, `MANTA-REQ-RQCLI-006` |

## ARTIF: Artifact writer

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| ARTIF-001 | Done | Implement artifact path planning for `.manta/` standalone output, caller-specified output directories, and optional `.manta/runs/scoped/<run_id>/...` layout. | `MANTA-REQ-RQART-001`, `MANTA-REQ-RQART-002`, `MANTA-REQ-RQART-007` |
| ARTIF-002 | Done | Write summary JSON, summary Markdown, raw-log SHA-256, duration metadata, failure/warning counts, and artifact references. | `MANTA-REQ-RQART-003`, `MANTA-REQ-RQART-004` |
| ARTIF-003 | Done | Write status JSON, stable watcher hash inputs, and bounded failure excerpts suitable for no-agent watcher and compact human review. | `MANTA-REQ-RQART-005`, `MANTA-REQ-RQART-006`, `MANTA-REQ-RQWAT-001` to `MANTA-REQ-RQWAT-003` |

## PARSE: Extraction engine

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| PARSE-001 | Done | Implement generic parser for common failure, warning, file-line, stack-top, and test-name patterns with bounded spans. | `MANTA-REQ-RQEXT-001`, `MANTA-REQ-RQEXT-003`, `MANTA-REQ-RQEXT-004` |
| PARSE-002 | Done | Implement parser registry and parser labels while requiring only `generic` in the first runnable slice and failing closed on unsupported specialized labels. | `MANTA-REQ-RQEXT-002`, `ADR-0008`, `MANTA-REQ-RQSEC-003` |
| PARSE-003 | Done | Implement extractor status computation and degraded extraction signals for non-zero exits with missing or overly broad spans. | `MANTA-REQ-RQEXT-005`, `MANTA-REQ-RQEXT-006`, `MANTA-REQ-RQEXT-007` |

## SAFEY: Safety and filtering

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| SAFEY-001 | Done | Implement redaction pipeline for summaries, excerpts, status files, and console-safe output, with raw-log handling warnings clearly marked. | `MANTA-REQ-RQSEC-001`, `MANTA-REQ-RQSEC-002`, `ADR-0003` |
| SAFEY-002 | Done | Implement noise filtering for summaries without altering raw logs. | `MANTA-REQ-RQCFG-004`, `MANTA-REQ-RQSEC-005` |
| SAFEY-003 | Done | Add RE2-based regex validation plus bounds for regex input size, extracted block size, excerpt size, summary size, and overmatch diagnostics. | `MANTA-REQ-RQSEC-003`, `MANTA-REQ-RQSEC-004`, `MANTA-REQ-RQRUL-006`, `ADR-0007` |

## CLIUX: Direct artifact commands

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| CLIUX-001 | Done | Implement `summarize <raw-log>` so existing logs can be converted into Manta summary and status artifacts without rerunning the command. | `MANTA-REQ-RQCLI-004`, `MANTA-REQ-RQART-003` to `MANTA-REQ-RQART-006` |
| CLIUX-002 | Done | Implement deterministic excerpt retrieval with `excerpt --summary <summary-path> <failure-id>`. | `MANTA-REQ-RQCLI-005`, `ADR-0002` |

## RULES: Rule management

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| RULES-001 | Done | Implement rule storage, list/search/show, disabled-rule handling, deletion reason, and project-local rule loading. | `MANTA-REQ-RQRUL-001`, `MANTA-REQ-RQRUL-002`, `MANTA-REQ-RQRUL-004` |
| RULES-002 | Done | Implement create/update validation with provenance requirements, RE2-safe matching config, and capture group diagnostics. | `MANTA-REQ-RQRUL-003`, `MANTA-REQ-RQRUL-006`, `ADR-0007` |
| RULES-003 | Done | Implement rule test and rule propose from raw-log span, including run-local proposed rule separation. | `MANTA-REQ-RQRUL-005`, `MANTA-REQ-RQRUL-007`, `MANTA-REQ-RQDOC-003` |

## DOCUM: Documentation and release readiness

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| DOCUM-001 | Done | Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes. | `MANTA-REQ-RQDOC-001` |
| DOCUM-002 | Done | Add real CLI examples, config examples, and artifact examples after first runnable implementation. | `MANTA-REQ-RQDOC-002` |
| DOCUM-003 | Done | Add release-readiness checklist, fixture evidence expectations, and v0.1 packaging notes before tagging. | `MANTA-REQ-RQDOC-004` |

## HARDE: Post-baseline hardening and contract closure

These tasks were implemented as separate, reviewable units in numerical order. A task moved to `Done` only after its focused verification and the existing affected test suites passed. `HARDE-007` was the final end-to-end hardening gate.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| HARDE-001 | Done | Enforce fail-closed artifact containment for run IDs, configured command IDs, rule IDs, and excerpt references; reject absolute paths, traversal, cross-run access, and symlink escape. | Add traversal and symlink tests, then pass focused artifact/config/rules/CLI tests. | `MANTA-REQ-RQHAR-001`, `MANTA-REQ-RQCFG-006`, `MANTA-REQ-RQART-001`, `MANTA-REQ-RQSEC-003` |
| HARDE-002 | Done | Make command execution interruption-safe by preparing raw evidence before execution, handling and forwarding termination signals, and preserving partial raw/status artifacts with an explicit non-pass result. | Exercise a built binary with SIGINT and SIGTERM, verify partial evidence and status, then pass runner/CLI/E2E tests. | `MANTA-REQ-RQHAR-002`, `MANTA-REQ-RQRUN-003`, `MANTA-REQ-RQRUN-005`, `MANTA-REQ-RQRUN-006` |
| HARDE-003 | Done | Prevent standalone artifact overwrite by allocating collision-free run directories for repeated configured, ad-hoc, and summarize operations. | Run equivalent commands repeatedly within one timestamp interval, verify distinct paths and checksums, then pass artifact/CLI/E2E tests. | `MANTA-REQ-RQHAR-003`, `MANTA-REQ-RQART-002`, `MANTA-REQ-RQART-007`, `ADR-0003` |
| HARDE-004 | Done | Complete the redaction boundary for surfaced summary, status, excerpt, and console-safe metadata while leaving original raw logs and literal artifact references unchanged. | Test secrets in argv, identifiers, tags, evidence-origin paths, failures, and warnings; verify redacted surface fields, unchanged raw evidence, usable artifact references, and final status hashes, then pass safety/CLI/E2E tests. | `MANTA-REQ-RQHAR-004`, `MANTA-REQ-RQCFG-005`, `MANTA-REQ-RQSEC-001`, `MANTA-REQ-RQSEC-002`, `ADR-0003` |
| HARDE-005 | Done | Resolve and implement the specialized-parser miss and internal-error artifact contracts without allowing extraction behavior to override command truth. | Add contract tests for all extractor states and retained run states, then pass extract/CLI/guardrail tests. | `MANTA-REQ-RQHAR-005`, `MANTA-REQ-RQEXT-005` to `MANTA-REQ-RQEXT-007`, `MANTA-REQ-RQSEC-005`, `ADR-0002` |
| HARDE-006 | Done | Synchronize executable CLI behavior and durable documentation, including `--verbose`, `--no-color`, self-contained rule examples, Markdown output, version/toolchain resolver guidance, and roadmap/todo status wording. | Execute every documented command against a fresh fixture, compare generated output with examples, and pass CLI/toolchain E2E tests plus `git diff --check`. | `MANTA-REQ-RQHAR-006`, `MANTA-REQ-RQCLI-001` to `MANTA-REQ-RQCLI-006`, `MANTA-REQ-RQDOC-001` to `MANTA-REQ-RQDOC-004` |
| HARDE-007 | Done | Run the complete hardening regression and release-readiness gate across standalone and fixed run-scoped layouts, then update hardening statuses only from observed evidence. | Pass `make test`, configured/ad-hoc/summarize/excerpt/rules smokes, path and signal probes, both artifact layouts, install/toolchain checks, and `git diff --check`. | `MANTA-REQ-RQHAR-007`, `MANTA-REQ-RQDOC-004` |

## TAGS: Rule selection metadata

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| TAGS-001 | Done | Replace the single execution grouping label with canonical multi-value tags across schema v2, CLI, rule selection, artifacts, watcher hashes, tests, and documentation. | `MANTA-REQ-RQCLI-003`, `MANTA-REQ-RQCFG-003`, `MANTA-REQ-RQRUL-008`, `MANTA-REQ-RQWAT-002` |

## RELRV: v0.1.4 release-readiness follow-up

Completed release-readiness findings are retained here; remaining accepted findings stay in `todo.md`. A completed item records its development gate only and does not claim release, tag, or final review acceptance.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| RELRV-001 | Done | Process oversized runtime and summarize logs through a bounded complete-line tail without converting passing commands into internal errors, while preserving full raw evidence and absolute spans. | Cover boundary handling, runtime rules, specialized parsers, passing/failing/summarize artifacts, command exits, hashes, and excerpts; pass the full release-style test suite. | `MANTA-REQ-RQEXT-003`, `MANTA-REQ-RQEXT-005`, `MANTA-REQ-RQEXT-007`, `MANTA-REQ-RQSEC-004`, `ADR-0002`, `ADR-0007` |
| RELRV-002 | Done | Bound surfaced failure and warning records so noisy logs still produce compact terminal summary/status artifacts without changing authoritative command results. | Cover 50-record boundaries, actual JSON/Markdown byte budgets, redaction/noise ordering, truncation fields, noisy passing/failing exits, hashes, and retained excerpts; pass the full release-style test suite. | `MANTA-REQ-RQART-003` to `MANTA-REQ-RQART-006`, `MANTA-REQ-RQEXT-005`, `MANTA-REQ-RQEXT-007`, `MANTA-REQ-RQSEC-004`, `MANTA-REQ-RQWAT-001`, `ADR-0002`, `ADR-0003` |
| RELRV-003 | Done | Accept Playwright failure headers with or without trailing padding while preserving file, line, and test-name capture. | Cover padded and unpadded fixture headers with distinct capture values; pass focused parser tests, the unit gate, and the full Go test suite. | `MANTA-REQ-RQEXT-002`, `MANTA-REQ-RQEXT-004`, `MANTA-REQ-RQSEC-005` |
