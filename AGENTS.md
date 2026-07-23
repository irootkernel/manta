# AGENTS.md

Guidance for coding agents working in `manta` local development.

Manta is a standalone deterministic Go CLI for running test commands, preserving raw logs, extracting bounded failure evidence, and writing compact summary/status artifacts.

Current local-dev baseline:

- Manta: `manta 0.1.5`
- Project root: `/Users/draccoon/Workspace/SeventeenthEarth/kkachi/kkachi-agent-tester`
- Manta standalone evidence: `.manta/`

## 1. Source Of Truth

Start from repository authority before changing behavior:

- `README.md` for current CLI scope and user-facing examples.
- `docs/README.md` for document roles and source-of-truth order.
- `docs/integration-guide.md` for parent-project capability, ownership, and artifact-consumption contracts.
- `docs/requirements-specs.md` for Manta requirements and non-goals.
- `docs/architecture.md` and `docs/architecture-decision-records.md` for architectural boundaries.
- `docs/roadmap.md` for completed and planned task context.

Treat docs carefully:

- Manta remains standalone and deterministic unless an approved task changes that contract.


## 3. Think Before Coding

Do not assume. Investigate discoverable repository facts first.

Before substantial edits:

- State assumptions and success criteria when they affect implementation.
- Prefer the smallest change that satisfies the requirement.
- Push back on requests that make Manta a planner, reviewer, acceptance gate, waiver authority, or runtime orchestrator.
- Ask only for user judgment when scope, authority, credentials, destructive actions, production effects, or materially branching choices remain unclear.

For trivial, reversible, low-risk work, proceed without ceremony and verify the result.

## 4. Simplicity And Surgical Changes

Touch only what the task requires.

- Match existing Go, test, docs, and CLI-output style.
- Do not refactor unrelated code.
- Do not add speculative configurability or new dependencies without explicit need.
- Do not broaden parser behavior, redaction behavior, or artifact semantics without tests.
- Remove imports, variables, files, or helpers made unused by your own changes.
- If unrelated drift exists, mention it instead of silently fixing it.

Every changed line should trace to the user request, an active roadmap task, or a required verification fix.

## 5. Manta Evidence Semantics

Preserve these invariants:

- Command exit code is authoritative for `run` pass/fail.
- Parsers, rules, and summaries compress evidence only; they never change pass/fail.
- Tags are canonical rule selectors: parser labels match exactly and every rule tag must be present on the run.
- Raw logs are preserved and may contain unredacted values; share raw-log excerpts cautiously.
- Redaction/noise filtering applies to summaries/excerpts/status output, not to original raw logs.
- `--run-id` artifacts must stay inside the matching `.manta/runs/scoped/<run_id>/artifacts/test/` path and must not cross-run or symlink-escape.
- Standalone runs write under `.manta/runs/standalone/<UTC-timestamp>[-NNN]/`.
- Missing, malformed, unsupported, unsafe, overbroad, or stale evidence should fail closed or be reported as degraded according to the existing contract.

Do not claim review acceptance, waiver, final acceptance, install, release, push, or runtime activation from Manta evidence alone.

## 6. Local Artifact And Git Safety

These are local runtime/evidence surfaces and must stay out of source commits:

```text
.manta/
.codegraph/
.omx/
.omc/
.external-review-sidecar/
```

Never run `git add`, `git commit`, or `git push` unless the user explicitly asks for that exact action after verification. Do not revert or overwrite user changes.

## 7. Verification

Run the narrowest meaningful verification first, then broaden when shared behavior changed.

Common commands:

```bash
HOME=/Users/draccoon manta run unit
HOME=/Users/draccoon manta run integration
HOME=/Users/draccoon manta run e2e
HOME=/Users/draccoon go test ./...
git diff --check
```

Use `HOME=/Users/draccoon manta run all` or `HOME=/Users/draccoon make test` when full local release-style verification is needed. The full `make test` path includes format/lint/vet/guardrails/unit/integration/e2e and may fail if optional lint tooling is unavailable.

Verification expectations:

- Parser/rule changes: focused parser/rule tests plus a Manta run or summarize smoke.
- Runner/artifact/path changes: runner tests, integration/E2E coverage, and path-safety checks.
- CLI behavior changes: help/output examples, integration or E2E tests, and README/docs sync.
- Docs/AGENTS/skill-guidance-only changes: file readback, cross-reference sanity, and `git diff --check` are usually sufficient unless executable commands were changed.

Before reporting completion, include changed files, commands run, command exits, evidence paths when relevant, and remaining risks or skipped checks.

## 8. Reporting

Use English for code, docs, tests, commit messages, and run artifacts. Use Korean for direct user-facing status reports unless requested otherwise.

Keep chat reports compact. Put detailed logs, evidence paths, and raw snippets in artifacts or explicit summaries rather than flooding chat. Distinguish:

- development-gate completion,
- review/final acceptance,
- commit/push/release/runtime activation boundaries.
