# AGENTS.md

Guidance for coding agents working in `kkachi-agent-tester` local development.

KAT is a standalone deterministic Go CLI for running test commands, preserving raw logs, extracting bounded failure evidence, and writing compact summary/status artifacts. It may emit Kkachi-compatible evidence paths, but it must not own KAS command semantics, KAH run-state behavior, GJC session management, review authority, MAR authority, waiver authority, or final acceptance authority.

Current local-dev baseline:

- KAS: `kkachi-agent-skills 0.2.0`
- KAH: `kkachi-agent-helper 0.2.0`
- KAT: `kkachi-agent-tester 0.1.0`
- Project root: `/Users/draccoon/Workspace/SeventeenthEarth/kkachi/kkachi-agent-tester`
- KAS/KAH local metadata: `.kkachi/`
- KAT standalone evidence: `.kat/`

## 1. Source Of Truth

Start from repository authority before changing behavior:

- `README.md` for current CLI scope and user-facing examples.
- `docs/requirements-specs.md` for KAT requirements and non-goals.
- `docs/architecture.md` and `docs/architecture-decision-records.md` for architectural boundaries.
- `docs/roadmap.md` for completed/planned task context and GAJAE integration-contract notes.
- `docs/kkachi-docs-map.yaml` for KAH-managed docs map when present.

Treat docs carefully:

- KAT remains standalone and deterministic unless an approved task changes that contract.
- GAJAE integration entries describe artifact/contract compatibility only; they do not move KAS/KAH/GJC authority into KAT.
- KAT output is factual evidence. KAS/Blue/color/MAR/final gates remain acceptance authority.

## 2. KAS/KAH/KAT Local-Dev Flow

Run local commands with the real user home so Go, GJC, Codex, KAH, and KAT do not use a Hermes profile home by accident:

```bash
cd /Users/draccoon/Workspace/SeventeenthEarth/kkachi/kkachi-agent-tester
HOME=/Users/draccoon kkachi-agent-skills toolchain doctor --project-root "$PWD" --json
HOME=/Users/draccoon kkachi-agent-helper project doctor --json
HOME=/Users/draccoon kkachi-agent-tester --version
```

When KAH binary resolution matters, prefer the versioned local toolchain:

```bash
KKACHI_KAH_BIN=/Users/draccoon/.local/kkachi/toolchains/kah/v0.2.0/bin/kkachi-agent-helper
```

Use KAS for task classification, phase policy, review routing, MAR policy, and authority wording. Use KAH for deterministic project/run state, gates, GJC/KAT evidence refs, hashes, plan locks, callbacks, and status. Use KAT only for factual test execution and evidence compression.

## 3. Think Before Coding

Do not assume. Investigate discoverable repository facts first.

Before substantial edits:

- State assumptions and success criteria when they affect implementation.
- Prefer the smallest change that satisfies the requirement.
- Keep KAT standalone; do not import KAS or KAH internals.
- Push back on requests that make KAT a planner, reviewer, acceptance gate, waiver authority, or Kkachi runtime orchestrator.
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

## 5. KAT Evidence Semantics

Preserve these invariants:

- Command exit code is authoritative for `run` pass/fail.
- Parsers, rules, and summaries compress evidence only; they never change pass/fail.
- Raw logs are preserved and may contain unredacted values; share raw-log excerpts cautiously.
- Redaction/noise filtering applies to summaries/excerpts/status output, not to original raw logs.
- `--run-id` artifacts must stay inside the matching `.kkachi/runs/<run_id>/artifacts/test/` path and must not cross-run or symlink-escape.
- Standalone runs write under `.kat/runs/<timestamp-or-run-id>/`.
- Missing, malformed, unsupported, unsafe, overbroad, or stale evidence should fail closed or be reported as degraded according to the existing contract.

Do not claim same-thread wake, review acceptance, MAR acceptance, waiver, final acceptance, install, release, push, or runtime activation from KAT evidence alone.

## 6. Local Artifact And Git Safety

These are local runtime/evidence surfaces and should stay out of ordinary source commits unless explicitly scoped:

```text
.kkachi/
.kat/
.codegraph/
.omx/
.omc/
.external-review-sidecar/
```

`docs/kkachi-docs-map.yaml` is KAH-managed docs-map configuration and may be reviewed as source-facing configuration before commit. `.kkachi/toolchain.yaml` is generated local state and should not be promoted as a source doc.

Never run `git add`, `git commit`, or `git push` unless the user explicitly asks for that exact action after verification. Do not revert or overwrite user changes.

## 7. Verification

Run the narrowest meaningful verification first, then broaden when shared behavior changed.

Common commands:

```bash
HOME=/Users/draccoon kkachi-agent-tester run unit
HOME=/Users/draccoon kkachi-agent-tester run integration
HOME=/Users/draccoon kkachi-agent-tester run e2e
HOME=/Users/draccoon go test ./...
git diff --check
```

Use `HOME=/Users/draccoon kkachi-agent-tester run all` or `HOME=/Users/draccoon make test` when full local release-style verification is needed. The full `make test` path includes format/lint/vet/guardrails/unit/integration/e2e and may fail if optional lint tooling is unavailable.

Verification expectations:

- Parser/rule changes: focused parser/rule tests plus a KAT run or summarize smoke.
- Runner/artifact/path changes: runner tests, integration/E2E coverage, and path-safety checks.
- CLI behavior changes: help/output examples, integration or E2E tests, and README/docs sync.
- Docs/AGENTS/skill-guidance-only changes: file readback, cross-reference sanity, and `git diff --check` are usually sufficient unless executable commands were changed.

Before reporting completion, include changed files, commands run, command exits, evidence paths when relevant, and remaining risks or skipped checks.

## 8. Reporting

Use English for code, docs, tests, commit messages, and run artifacts. Use Korean for direct 주군-facing status reports unless requested otherwise.

Keep chat reports compact. Put detailed logs, evidence paths, and raw snippets in artifacts or explicit summaries rather than flooding chat. Distinguish:

- mechanical KAT/KAH/KAS PASS,
- development-gate completion,
- color/MAR/final acceptance,
- commit/push/release/runtime activation boundaries.
