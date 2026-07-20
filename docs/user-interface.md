# KAT User Interface

Status: Complete through `HARDE-006`
Scope: CLI-first interface for KAT v0.1

## Interface principles

- CLI-first and script-friendly.
- Deterministic output paths.
- Human-readable Markdown summaries plus machine-readable JSON.
- No hidden pass/fail overrides.
- Compact console output; details live in artifacts.
- Raw logs are preserved as source evidence and may contain unredacted values.
- Redaction covers surfaced command metadata and extracted failure/warning content, including argv, identifiers, lanes, source paths, test names, and stack entries.
- Artifact-reference fields remain literal and usable. Do not put secrets in run IDs, command IDs, output directories, or other path components.

## Primary commands

```bash
kkachi-agent-tester run <command-id>
kkachi-agent-tester run --lane <lane> -- <command...>
kkachi-agent-tester summarize <raw-log>
kkachi-agent-tester excerpt --summary <summary-path> <failure-id>
```

## Rule commands

```bash
kkachi-agent-tester rules list
kkachi-agent-tester rules search <query>
kkachi-agent-tester rules show <rule-id>
kkachi-agent-tester rules create --file <rule.yaml>
kkachi-agent-tester rules update <rule-id> --file <rule.yaml>
kkachi-agent-tester rules delete <rule-id> --reason <reason>
kkachi-agent-tester rules test --rule <rule-id> --log <raw-log> --expect-span <start:end>
kkachi-agent-tester rules propose --lane <lane> --parser <parser> --raw-log <raw-log> --span <start:end>
```

## Supported parser labels

Implemented parser labels:

- `generic`
- `vitest`
- `pytest`
- `go-test`
- `playwright`

Applicable project rules run before the selected parser. The `generic` label uses generic extraction patterns; specialized labels use only their own parser patterns and never retry generic extraction. A specialized-parser miss reports `no_match` after a pass and `degraded` after a non-pass result.

Parser-specific examples in this repository are backed by fixture logs under `internal/extract/testdata/`.


## Global options

Recommended global options:

| Option | Purpose |
|---|---|
| `--config <path>` | Override default `.kkachi/tester.yaml`. |
| `--repo <path>` | Set repository root / working directory. |
| `--output-dir <path>` | Write standalone artifacts outside `.kat/`. |
| `--run-id <id>` | Use Kkachi-compatible `.kkachi/runs/<run_id>/...` artifact layout. |
| `--json` | Print compact JSON result to stdout. |

Run IDs, configured command IDs, and rule IDs must match `[A-Za-z0-9][A-Za-z0-9_-]*`. Invalid identifiers fail with config exit code `2`; run identifiers are checked before the test command starts.

KAT output is plain text and does not emit ANSI color. Historical `--no-color` and `--verbose` placeholders had no executable behavior and are not supported options; either flag now fails closed with config exit code `2`.

## Version and toolchain selection

Use either version surface to inspect a selected binary:

```bash
kkachi-agent-tester --version
kkachi-agent-tester version --json
```

For deterministic automation, the bundled `scripts/kkachi-agent-tester-toolchain` resolver uses this precedence:

1. Absolute executable path from `KKACHI_KAT_BIN`.
2. Absolute `kat.binary_path` in `.kkachi/toolchain.yaml`.
3. `kat.cli_version` resolved as `${KKACHI_TOOLCHAIN_ROOT:-$HOME/.local/kkachi/toolchains}/kat/v<version>/bin/kkachi-agent-tester`.

The resolver never falls back to `PATH`. It verifies that the selected path is executable and that `--version` succeeds. When `kat.cli_version` accompanies a metadata-selected binary, the reported semantic version must match exactly. Missing KAT metadata, unsupported schema versions, relative or non-executable paths, malformed versions, and version mismatches fail closed.

An explicit local binary can be inspected without editing generated metadata:

```bash
KKACHI_KAT_BIN=/absolute/path/to/kkachi-agent-tester scripts/kkachi-agent-tester-toolchain --toolchain-status
```

Versioned selection uses local metadata such as:

```yaml
schema_version: "kkachi.toolchain.v1"
kat:
  cli_version: "0.1.3"
```

An absolute override may be recorded with an optional version assertion:

```yaml
schema_version: "kkachi.toolchain.v1"
kat:
  cli_version: "0.1.3"
  binary_path: "/absolute/path/to/kkachi-agent-tester"
```

KAS/KAH-generated `.kkachi/toolchain.yaml` may omit the `kat` block; set `KKACHI_KAT_BIN` or add explicit local KAT metadata. Treat the generated file as local state, not source documentation.

## Tested setup fixture

The examples below are grounded in the current automated tests. They use this minimal fixture:

```bash
mkdir -p .kkachi/tester fixtures
cat > .kkachi/tester.yaml <<'YAML'
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
YAML

cat > test.sh <<'SH'
#!/bin/sh
echo 'noise: start'
echo 'TypeError: token=secret failed'
echo 'src/foo.test.ts:42:13'
echo '✗ renders empty state'
echo
exit 1
SH
chmod +x test.sh

cat > fixtures/unit.raw.log <<'LOG'
noise: start
TypeError: token=secret failed
src/foo.test.ts:42:13
✗ renders empty state

LOG

cat > fixtures/generic-v1.yaml <<'YAML'
id: generic-v1
lane: unit
parser: generic
status: active
provenance:
  created_by: tester
  source_run: local-unit
  source_command: unit
  source_log_sha256: sha256:abc
  source_span:
    start_line: 2
    end_line: 4
  reason: fixture-backed rule
match:
  start:
    regex: '^TypeError:'
  end:
    any_of:
      - regex: '^$'
    max_block_lines: 20
  include_context:
    before: 0
    after: 0
extract:
  file_line:
    regex: '(?P<file>[^\s:]+\.ts):(?P<line>\d+)'
  test_name:
    regex: '^\s*[✗×]\s+(?P<test>.+)$'
confidence: medium
YAML

sed \
  -e 's/reason: fixture-backed rule/reason: fixture-backed rule updated/' \
  -e 's/confidence: medium/confidence: high/' \
  fixtures/generic-v1.yaml > fixtures/generic-v1-update.yaml
```

## Tested command examples

Configured run with deterministic artifact paths:

```bash
kkachi-agent-tester --config .kkachi/tester.yaml --run-id example-run run unit
# exits 1 because the fixture command fails
# writes .kkachi/runs/example-run/artifacts/test/unit.raw.log
# writes .kkachi/runs/example-run/artifacts/test/unit.summary.json
# writes .kkachi/runs/example-run/artifacts/test/unit.summary.md
# writes .kkachi/runs/example-run/artifacts/test/unit.status.json
# GAJAE-009 note: KAH normalizes raw KAT v0.1.0 status/summary/raw-log refs for attachment; KAT emits factual evidence only.
# writes .kkachi/runs/example-run/artifacts/test/excerpts/F001.log
```

Ad-hoc run without project config commands:

```bash
kkachi-agent-tester run --lane unit -- sh test.sh
# exits 1 because the fixture command fails
```

Summarize an existing raw log without rerunning the command:

```bash
kkachi-agent-tester --run-id summarize-example summarize fixtures/unit.raw.log
# copies .kkachi/runs/summarize-example/artifacts/test/unit.raw.log
# writes .kkachi/runs/summarize-example/artifacts/test/unit.summary.json
# writes .kkachi/runs/summarize-example/artifacts/test/unit.summary.md
# writes .kkachi/runs/summarize-example/artifacts/test/unit.status.json
# writes .kkachi/runs/summarize-example/artifacts/test/excerpts/F001.log
```

Deterministic excerpt lookup after either `run` or `summarize`:

```bash
kkachi-agent-tester excerpt --summary .kkachi/runs/summarize-example/artifacts/test/unit.summary.json F001
```

Compact JSON output for scripts:

```bash
kkachi-agent-tester --output-dir evidence --json summarize fixtures/unit.raw.log
```

Fixture-backed rule workflow examples:

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

## Summarize mode notes

- `summarize <raw-log>` uses the `generic` parser plus any matching project rules.
- When only a raw log is available, KAT infers `command_id` and `lane` from the raw-log basename. For example, `unit.raw.log` produces `command_id: unit` and `lane: unit`.
- Because original execution metadata is unavailable, summarize infers `status` and `exit_code` from raw-log evidence. Use `run` when authoritative execution metadata is required.
- If extraction fails internally, summarize still preserves the copied raw log and writes degraded `internal_error` summary/status artifacts with exit code `4`; the diagnostic is emitted on stderr.
- Without `--run-id` or `--output-dir`, summarize copies the input raw log into a newly allocated `.kat/runs/<UTC-timestamp>[-NNN]/` directory and writes derived artifacts there. `--output-dir` uses the same collision-free allocation under `<output-dir>/runs/`; `--run-id` retains the fixed Kkachi-compatible layout. The original input remains unchanged.
- Each summarize operation stores a complete raw-log copy in its artifact directory, so repeated summarization increases local storage usage in proportion to the source log size.
- Summary JSON stores excerpt references relative to the summary directory, such as `excerpts/F001.log`. An absolute `--summary` input remains valid, while absolute, traversal, cross-run, dangling, and symlink-escaping embedded references fail with artifact exit code `3`.
- Inferred `command_id` and `lane` values are redacted in summary, status, and console metadata. The copied raw-log and derived artifact references retain their literal filenames so they remain resolvable.

## Exit code guidance

| Condition | CLI exit code |
|---|---:|
| Test command passed | `0` |
| Test command failed | underlying command exit code when available |
| Test command timed out | documented timeout code, recommended `124` |
| Test command interrupted by SIGINT/SIGTERM on Unix | `130` / `143`, with `status: killed` |
| KAT config error | documented internal code, recommended `2` |
| KAT artifact write error | documented internal code, recommended `3` |
| Extraction internal error after a passing test command | CLI `4`; artifacts retain command exit `0` with `status: internal_error` |
| Extraction internal error after a failed, timed-out, or killed command | original command exit code and status |
| Extraction internal error during `summarize` | CLI and artifact exit code `4` with `status: internal_error` |
| Other KAT parser/rule internal error | documented internal code, recommended `4` |
| Successful `summarize` or `excerpt` | `0` |

## Markdown summary shape

```markdown
# KAT Summary: unit

Status: failed
Exit code: 1
Duration: 0.0s
Extractor: precise
Raw log: .kkachi/runs/summarize-example/artifacts/test/unit.raw.log
Raw log SHA-256: sha256:...

## Failures

### F001: TypeError: token=<redacted> failed

- File: src/foo.test.ts:42
- Test: renders empty state
- Excerpt: excerpts/F001.log

## Notes

Command exit code is authoritative. Extraction rules only summarize evidence.
```
