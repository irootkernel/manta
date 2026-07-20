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
- raw evidence is opened before execution and streamed while the command runs
- on Unix, SIGINT/SIGTERM are forwarded to the command process group and recorded as `killed` with exit code `130`/`143`
- raw logs are preserved as original evidence and may contain unredacted values
- configured redaction applies to surfaced command metadata, failure/warning evidence, excerpts, status metadata, and human/JSON command output
- artifact paths remain literal, usable references; do not place secrets in run IDs, command IDs, output directories, or other artifact-bearing paths

## Build

```bash
go build .
go install github.com/SeventeenthEarth/kkachi-agent-tester@v0.1.3
make install
make install-toolchain
kkachi-agent-tester --version
```

`make install` installs the binary with embedded build metadata from a checkout. `make install-toolchain` installs a versioned toolchain copy under `~/.local/kkachi/toolchains/kat/v0.1.3/bin/`.

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

Project-local active rules live under:

```text
.kkachi/tester/rules/*.yaml
```

Run-local proposed rules are kept separate under:

```text
.kat/rule-proposals/
```

## Quick examples

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
kkachi-agent-tester rules list
kkachi-agent-tester rules show generic-v1
kkachi-agent-tester rules test --rule generic-v1 --log internal/extract/testdata/vitest.raw.log --expect-span 7:14
kkachi-agent-tester rules propose --lane unit --parser vitest --raw-log internal/extract/testdata/vitest.raw.log --span 7:9
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

Detailed requirements, architecture, UI examples, roadmap, and implementation notes live under `docs/`.
