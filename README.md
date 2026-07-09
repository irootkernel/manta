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
- raw logs are preserved as original evidence and may contain unredacted values

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
kkachi-agent-tester summarize fixtures/unit.raw.log
```

Excerpt lookup:

```bash
kkachi-agent-tester excerpt --summary fixtures/unit.summary.json F001
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
.kat/runs/<timestamp-or-run-id>/
```

When `--run-id` is supplied, KAT writes to:

```text
.kkachi/runs/<run_id>/artifacts/test/
```

Summarize mode writes beside the input raw log by default, or into the selected output layout when `--run-id` or `--output-dir` is supplied.

## Documentation

Detailed requirements, architecture, UI examples, roadmap, and implementation notes live under `docs/`.
