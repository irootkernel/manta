# Manta Documentation

Status: Current for `manta v0.1.4`

This directory contains Manta's integration contracts, technical design, delivery history, and maintainer guidance. Start with the document that matches your role instead of reading the directory in filename order.

## Recommended reading paths

For a person running Manta directly:

1. Read the repository [README](../README.md) and complete its five-minute example.
2. Use the [CLI reference](user-interface.md) for options, rule management, exit codes, and tested examples.

For a parent project integrating Manta:

1. Read the [integration guide](integration-guide.md) for the supported capability matrix, ownership boundaries, project files, invocation, and rollout checklist.
2. Read the [architecture](architecture.md) for the summary/status schemas, artifact layout, watcher hash, and degraded-evidence behavior.
3. Consult the [architecture decisions](architecture-decision-records.md) before proposing a change to Manta's authority or evidence semantics.

For Manta maintainers:

1. Follow [AGENTS.md](../AGENTS.md) for repository workflow and verification expectations.
2. Use the [requirements](requirements-specs.md) as the behavioral source of truth.
3. Use the [requirements-to-test matrix](requirements-test-matrix.md) to find executable evidence.
4. Read the [implementation note](implementation-note.md) before changing runner, parser, artifact, redaction, or rule behavior.
5. Use the [roadmap](roadmap.md) and [todo](todo.md) for recorded delivery and open-work state.

## Current delivery state

The standalone v0.1 baseline, the recorded `HARDE-001` through `HARDE-007` hardening requirements, the schema-v2 canonical tag selector contract, and release-readiness follow-up `RELRV-001` through `RELRV-003` are implemented. The current supported surfaces include configured and ad-hoc command execution, raw-log summarization, excerpt lookup, five parsers, rule lifecycle commands, bounded/redacted derived evidence with explicit record truncation, collision-free standalone artifacts, fixed run-scoped artifacts, and deterministic status JSON. All `.manta/` content is local-only state.

The remaining six open items from the v0.1.4 release-readiness review are recorded in `todo.md`. The delivery statement above is intentionally narrower than “Manta provides every testing or orchestration capability.” Features listed as unsupported or out of scope in the [integration guide](integration-guide.md#not-provided-by-manta-v01) are not implicitly planned or promised.

## Document catalog

| Document | Audience | Authority |
|---|---|---|
| [Repository README](../README.md) | First-time and daily users | Installation, quick start, common workflows |
| [CLI reference](user-interface.md) | Operators and script authors | Commands, options, examples, exit behavior |
| [Integration guide](integration-guide.md) | Parent-project owners | Capability status, ownership boundary, adoption contract |
| [Architecture](architecture.md) | Integrators and maintainers | Components, data flow, schemas, artifact and watcher contracts |
| [Architecture decisions](architecture-decision-records.md) | Maintainers and reviewers | Accepted design constraints and their rationale |
| [Requirements](requirements-specs.md) | Maintainers and reviewers | Normative behavioral requirements and v0.1 non-goals |
| [Requirements-to-test matrix](requirements-test-matrix.md) | Maintainers and auditors | Primary evidence for each completed requirement |
| [Implementation note](implementation-note.md) | Contributors | Package boundaries, risk areas, tests, release checklist |
| [Roadmap](roadmap.md) | Project maintainers | Completed delivery history and integration-contract tasks |
| [Todo](todo.md) | Project maintainers | Explicitly accepted open work; currently the v0.1.4 release-readiness items |

## Source-of-truth order

When documents appear to disagree, use this order:

1. `requirements-specs.md` and accepted ADRs for intended behavior.
2. Executable behavior and tests for what the current binary actually does.
3. `architecture.md` and `integration-guide.md` for stable consumer contracts.
4. `user-interface.md` and the root README for operator instructions.
5. `roadmap.md`, `todo.md`, and `implementation-note.md` for project history and development context.

Treat a mismatch between the first two levels as a defect. Update user-facing and integration documents in the same change whenever executable CLI or artifact behavior changes.
