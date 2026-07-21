# kkachi-agent-tester

KAT is a standalone Go CLI for running test commands, preserving raw logs, extracting bounded failure evidence, and writing compact summary/status artifacts.

## Current v0.1 scope

Implemented:
- configured runs: `kkachi-agent-tester run <command-id>`
- ad-hoc runs: `kkachi-agent-tester run --lane <lane> -- <command...>`
- raw-log summarization: `kkachi-agent-tester summarize <raw-log>`
- deterministic excerpt lookup: `kkachi-agent-tester excerpt --summary <summary-path> <failure-id>`
- rule management: `rules list/search/show/create/update/delete/test/propose`
- supported parsers: `generic`, `vitest`, `pytest`, `go-test`, `playwright`

Important behavior:
- command exit status is authoritative for `run`
- rules and parsers summarize evidence only; they never change pass/fail
- specialized parsers fail closed without retrying generic extraction; a miss is `no_match` for a passing command and `degraded` for a non-pass result
- an extraction internal error preserves an existing failed, timed-out, or killed result; after a passing command, artifacts retain command exit `0` with `status: internal_error` while KAT exits `4`
- raw evidence is opened before execution and streamed while the command runs
- on Unix, SIGINT/SIGTERM are forwarded to the command process group and recorded as `killed` with exit code `130`/`143`
- raw logs are preserved as original evidence and may contain unredacted values
- configured redaction applies to surfaced command metadata, failure/warning evidence, excerpts, status metadata, and human/JSON command output
- artifact paths remain literal, usable references; do not place secrets in run IDs, command IDs, output directories, or other artifact-bearing paths
- a rule's matched block plus before/after context may not exceed 160 lines; overbroad rules fail closed before execution

## Build

```bash
go build .
go install github.com/SeventeenthEarth/kkachi-agent-tester@v0.1.3
make install
VERSION=0.1.3 make install-toolchain
kkachi-agent-tester --version
```

`make install` installs the binary with embedded build metadata from a checkout. `make install-toolchain` installs a versioned toolchain copy under `~/.local/kkachi/toolchains/kat/v0.1.3/bin/`.

For deterministic operator and automation use, resolve an explicit binary with the bundled wrapper:

```bash
KKACHI_KAT_BIN=/absolute/path/to/kkachi-agent-tester scripts/kkachi-agent-tester-toolchain --toolchain-status
```

The resolver does not fall back to `PATH`. It selects `KKACHI_KAT_BIN` first, then `.kkachi/toolchain.yaml` `kat.binary_path`, then the versioned path selected by `kat.cli_version`. Metadata-selected binaries are checked against `kat.cli_version`; missing, relative, non-executable, or version-mismatched selections fail closed. KAS/KAH-generated local metadata may not contain a `kat` entry, in which case set `KKACHI_KAT_BIN` or add explicit local KAT metadata. See [the user interface guide](docs/user-interface.md#version-and-toolchain-selection) for metadata examples.

## Verify

```bash
make test
```

`make test` runs:
- `format`
- `lint`
- `vet`
- `guardrails`
- `unit-test`
- `integration-test`
- `e2e-test`

## Minimal config

```yaml
version: 1
commands:
  unit:
    command: ["sh", "test.sh"]
    lane: unit
    parser: generic
    timeout_sec: 10
redaction:
  patterns:
    - name: token
      regex: 'token=[^ ]+'
      replace: 'token=<redacted>'
```

Default config path:

```text
.kkachi/tester.yaml
```

Run IDs, configured command IDs, and rule IDs must match `[A-Za-z0-9][A-Za-z0-9_-]*`. KAT rejects path syntax in these identifiers before command execution or artifact writes.

Config and rule YAML accept exactly one document and reject unknown fields. Disabled rules require a non-empty `deletion_reason`; active rules must not carry one.

Project-local active rules live under:

```text
.kkachi/tester/rules/*.yaml
```

Run-local proposed rules are kept separate under:

```text
.kat/rule-proposals/
```

## Quick examples

These commands assume the self-contained fixture from [the user interface guide](docs/user-interface.md#tested-setup-fixture). Configured and ad-hoc fixture runs intentionally exit `1` because the fixture represents a failing test.

Configured run:

```bash
kkachi-agent-tester run unit
```

Ad-hoc run:

```bash
kkachi-agent-tester run --lane unit -- sh test.sh
```

Summarize an existing raw log:

```bash
kkachi-agent-tester --run-id summarize-example summarize fixtures/unit.raw.log
```

Excerpt lookup:

```bash
kkachi-agent-tester excerpt --summary .kkachi/runs/summarize-example/artifacts/test/unit.summary.json F001
```

Rule lifecycle:

```bash
kkachi-agent-tester rules create --file fixtures/generic-v1.yaml
kkachi-agent-tester rules list
kkachi-agent-tester rules search fixture-backed
kkachi-agent-tester rules show generic-v1
kkachi-agent-tester rules test --rule generic-v1 --log fixtures/unit.raw.log --expect-span 2:5
kkachi-agent-tester rules update generic-v1 --file fixtures/generic-v1-update.yaml
kkachi-agent-tester rules propose --lane unit --parser generic --raw-log fixtures/unit.raw.log --span 2:4
kkachi-agent-tester rules delete generic-v1 --reason "superseded by v2"
```

## Artifact layouts

Standalone mode writes to:

```text
.kat/runs/<UTC-timestamp>[-NNN]/
```

KAT reserves each standalone run directory atomically. The first operation in a UTC-second interval uses the timestamp alone; concurrent or repeated operations use `-001`, `-002`, and later suffixes without overwriting prior evidence. `--output-dir` uses the same allocation rule under `<output-dir>/runs/`.

When `--run-id` is supplied, KAT writes to:

```text
.kkachi/runs/<run_id>/artifacts/test/
```

Summarize mode copies the input raw log into a newly allocated standalone run by default, or into the selected output layout when `--run-id` or `--output-dir` is supplied. The original input remains unchanged.

Each summarize operation stores a complete raw-log copy in its artifact directory, so repeated summarization increases local storage usage in proportion to the source log size.

Excerpt references stored in summaries are relative to the summary directory, for example `excerpts/F001.log`. `excerpt --summary` accepts absolute summary paths, but rejects absolute, traversal, cross-run, dangling, or symlink-escaping embedded excerpt references.

## Documentation

Detailed requirements, architecture, UI examples, roadmap, implementation notes, and the requirements-to-test matrix live under `docs/`.
