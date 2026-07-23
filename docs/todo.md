# Manta Todo

Status: 2 open items from the v0.1.4 release-readiness review
Scope: Documentation and implementation follow-up notes

## Todo status legend

- `Open`: not started.
- `Active`: currently being worked.
- `Blocked`: waiting on a decision or dependency.

## Release gate

Tag and publish `v0.1.4` only after the items below are closed and the release-readiness review passes again. Until that tag exists, the documented `go install github.com/irootkernel/manta@v0.1.4` command does not resolve: local tags currently stop at `v0.1.3`, and the remote has no tags or releases.

## Open items

Items are listed in recommended fix order. Line references were verified against the current tree and may drift as fixes land.

### RELRV-008 `Open` — Architecture JSON contract examples omit real fields

- Severity: docs.
- Problem: the Summary and Status JSON contract examples omit fields the binary always writes: `started_at` and `ended_at` in summary JSON, and `status_hash` in status JSON (`status_hash` is discussed in ADR-0005 and the integration guide but missing from the architecture contract example).
- Evidence: `docs/architecture.md:155-239`; `internal/model/types.go:169-170,194-195`.
- Done when: both contract examples list every field a fresh run actually produces, cross-checked against generated artifacts.

### RELRV-009 `Open` — Stale AGENTS.md project root; traceability audit does not resolve cited tests

- Severity: docs/test.
- Problem: (a) `AGENTS.md` records project root `/Users/draccoon/Workspace/SeventeenthEarth/manta`, but the repository lives at `/Users/draccoon/Workspace/SeventeenthEarth/kkachi/kkachi-agent-tester`. (b) The audit regression test only requires each matrix evidence cell to be non-empty text; it never verifies that the cited `Test*` names exist (all 45 currently do, by discipline rather than enforcement), and the `MANTA-REQ-RQWAT-002` row cites a function (`ComputeStatusHash`) instead of a test.
- Evidence: `AGENTS.md:10`; `e2e/audit_regression_e2e_test.go:39-81`; `docs/requirements-test-matrix.md:56`.
- Done when: the recorded project root matches the real checkout, and the audit test resolves every cited `Test*` identifier against the codebase, with an explicit rule for non-test evidence rows.

## Out-of-scope reminder

Unsupported and out-of-scope v0.1 capabilities are listed in the [integration guide](integration-guide.md#not-provided-by-manta-v01). They are not implicitly planned; add an approved requirement and roadmap item before treating one as future work.
