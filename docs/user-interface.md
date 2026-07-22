# Manta User Interface

Status: Complete through `HARDE-007`, `TAGS-001`, and `RELRV-004`
Scope: CLI-first interface for Manta v0.1, including schema-v2 tag selectors and release-readiness follow-up

This is the complete command reference. First-time users should begin with the repository [README](../README.md); parent-project owners should use the [integration guide](integration-guide.md) for ownership boundaries and adoption steps.

## Interface principles

- CLI-first and script-friendly.
- Deterministic output paths.
- Human-readable Markdown summaries plus machine-readable JSON.
- No hidden pass/fail overrides.
- Compact console output; details live in artifacts.
- Raw logs are preserved as source evidence and may contain unredacted values.
- Redaction covers surfaced command metadata and extracted failure/warning content, including argv, identifiers, tags, source paths, test names, and stack entries.
- Artifact-reference fields remain literal and usable. Do not put secrets in run IDs, command IDs, output directories, or other path components.

## Primary commands

```bash
manta run <command-id>
manta run --tag <tag> [--tag <tag> ...] -- <command...>
manta summarize [--tag <tag> ...] <raw-log>
manta excerpt --summary <summary-path> <failure-id>
```

## Rule commands

```bash
manta rules list
manta rules search <query>
manta rules show <rule-id>
manta rules create --file <rule.yaml>
manta rules update <rule-id> --file <rule.yaml>
manta rules delete <rule-id> --reason <reason>
manta rules test --rule <rule-id> --log <raw-log> --expect-span <start:end>
manta rules propose --tag <tag> [--tag <tag> ...] --parser <parser> --raw-log <raw-log> --span <start:end>
```

## Supported parser labels

Implemented parser labels:

- `generic`
- `vitest`
- `pytest`
- `go-test`
- `playwright`

Applicable project rules are evaluated first. The selected parser is a fallback and runs only when no rule produces a failure. The `generic` label uses generic extraction patterns; specialized labels use only their own parser patterns and never retry generic extraction. A specialized-parser miss reports `no_match` after a pass and `degraded` after a non-pass result.

Parser-specific examples in this repository are backed by fixture logs under `internal/extract/testdata/`.


## Global options

Recommended global options:

| Option | Purpose |
|---|---|
| `--config <path>` | Override default `.manta/tester.yaml`. |
| `--repo <path>` | Set repository root / working directory. |
| `--output-dir <path>` | Write standalone artifacts outside `.manta/`. |
| `--run-id <id>` | Use the fixed `.manta/runs/scoped/<run_id>/...` run-scoped artifact layout. |
| `--json` | Print compact JSON result to stdout. |

Run IDs, configured command IDs, rule IDs, and tags must match `[A-Za-z0-9][A-Za-z0-9_-]*`. Invalid identifiers fail with config exit code `2`; run identifiers and tags are checked before the test command starts. Tags are sorted and deduplicated before matching or serialization.

Manta output is plain text and does not emit ANSI color. Historical `--no-color` and `--verbose` placeholders had no executable behavior and are not supported options; either flag now fails closed with config exit code `2`.

Schema v1, `lane` fields, and `--lane` are not compatibility aliases. They fail closed with config exit code `2`; see the [schema-v2 migration guide](integration-guide.md#schema-v2-tag-migration) for the required replacements and artifact changes.

## Version and toolchain selection

Use either version surface to inspect a selected binary:

```bash
manta --version
manta version --json
```

For deterministic automation, the bundled `scripts/manta-toolchain` resolver uses this precedence:

1. Absolute executable path from `MANTA_BIN`.
2. Absolute `manta.binary_path` in `.manta/toolchain.yaml`.
3. `manta.cli_version` resolved as `${MANTA_TOOLCHAIN_ROOT:-$HOME/.local/manta/toolchains}/v<version>/bin/manta`.

The resolver never falls back to `PATH`. It verifies that the selected path is executable and that `--version` succeeds. When `manta.cli_version` accompanies a metadata-selected binary, the reported semantic version must match exactly. Missing Manta metadata, unsupported schema versions, relative or non-executable paths, malformed versions, and version mismatches fail closed.

An explicit local binary can be inspected without editing generated metadata:

```bash
MANTA_BIN=/absolute/path/to/manta scripts/manta-toolchain --toolchain-status
```

Versioned selection uses local metadata such as:

```yaml
schema_version: "manta.toolchain.v1"
manta:
  cli_version: "0.1.4"
```

An absolute override may be recorded with an optional version assertion:

```yaml
schema_version: "manta.toolchain.v1"
manta:
  cli_version: "0.1.4"
  binary_path: "/absolute/path/to/manta"
```

If `.manta/toolchain.yaml` omits the `manta` block, set `MANTA_BIN` or add explicit local Manta metadata. The entire `.manta/` directory, including config, rules, toolchain metadata, proposals, and evidence, is local-only state and should be ignored by Git.

## Tested setup fixture

The examples below are grounded in the current automated tests. They use this minimal fixture:

```bash
mkdir -p .manta/tester fixtures
cat > .manta/tester.yaml <<'YAML'
version: 2
commands:
  unit:
    command: ["sh", "test.sh"]
    tags: [generic, unit]
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
tags: [generic, unit]
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
manta --config .manta/tester.yaml --run-id example-run run unit
# exits 1 because the fixture command fails
# writes .manta/runs/scoped/example-run/artifacts/test/unit.raw.log
# writes .manta/runs/scoped/example-run/artifacts/test/unit.summary.json
# writes .manta/runs/scoped/example-run/artifacts/test/unit.summary.md
# writes .manta/runs/scoped/example-run/artifacts/test/unit.status.json
# writes .manta/runs/scoped/example-run/artifacts/test/excerpts/F001.log
```

Ad-hoc run without project config commands:

```bash
manta run --tag generic --tag unit -- sh test.sh
# exits 1 because the fixture command fails
```

Summarize an existing raw log without rerunning the command:

```bash
manta --run-id summarize-example summarize fixtures/unit.raw.log
# copies .manta/runs/scoped/summarize-example/artifacts/test/unit.raw.log
# writes .manta/runs/scoped/summarize-example/artifacts/test/unit.summary.json
# writes .manta/runs/scoped/summarize-example/artifacts/test/unit.summary.md
# writes .manta/runs/scoped/summarize-example/artifacts/test/unit.status.json
# writes .manta/runs/scoped/summarize-example/artifacts/test/excerpts/F001.log
```

Deterministic excerpt lookup after either `run` or `summarize`:

```bash
manta excerpt --summary .manta/runs/scoped/summarize-example/artifacts/test/unit.summary.json F001
```

Compact JSON output for scripts:

```bash
manta --output-dir evidence --json summarize fixtures/unit.raw.log
```

Fixture-backed rule workflow examples:

```bash
manta rules create --file fixtures/generic-v1.yaml
manta rules list
manta rules search fixture-backed
manta rules show generic-v1
manta rules test --rule generic-v1 --log fixtures/unit.raw.log --expect-span 2:5
manta rules update generic-v1 --file fixtures/generic-v1-update.yaml
manta rules propose --tag generic --tag unit --parser generic --raw-log fixtures/unit.raw.log --span 2:4
manta rules delete generic-v1 --reason "superseded by v2"
```

For project rules, `max_block_lines` counts the matched block including its start line. The matched block plus `include_context.before` and `include_context.after` must not exceed 160 lines; overbroad or overflow-sized values fail closed with config exit code `2`.

Tags are rule selectors, not command selectors or automatic rule generators. The parser must match exactly, and every tag declared by a rule must be present on the run. For a run tagged `[go, unit]`, rules tagged `[go]`, `[unit]`, and `[go, unit]` are applicable, while `[integration]` is not. All applicable active rules inspect the raw log first; the selected parser runs only when those rules produce no failure. `rules propose` writes only a local candidate under `.manta/rule-proposals/`; an operator must review, test, and explicitly create it before it becomes active under `.manta/tester/rules/`.

## Summarize mode notes

- `summarize <raw-log>` uses the `generic` parser plus any matching project rules.
- When tags are omitted, Manta infers `command_id` from the raw-log basename and uses that command ID as a single tag. For example, `unit.raw.log` produces `command_id: unit` and `tags: [unit]`. Repeat `--tag` to provide an explicit selector set instead.
- Because original execution metadata is unavailable, summarize infers `status` and `exit_code` from raw-log evidence. Use `run` when authoritative execution metadata is required.
- For inputs larger than 256 KiB, summarize preserves the complete copied raw log but extracts only from the final 256 KiB beginning at a complete line. The artifacts retain inferred status and exit code, report `extractor_status: degraded`, and the command exits `0` unless another error occurs.
- Summary artifacts retain at most 50 failures and 50 warnings after redaction and noise filtering. The count fields equal retained array lengths; `failures_truncated` or `warnings_truncated` reports omitted evidence and makes `extractor_status` degraded without changing the command result.
- If extraction fails internally, summarize still preserves the copied raw log and writes degraded `internal_error` summary/status artifacts with exit code `4`; the diagnostic is emitted on stderr.
- Without `--run-id` or `--output-dir`, summarize copies the input raw log into a newly allocated `.manta/runs/standalone/<UTC-timestamp>[-NNN]/` directory and writes derived artifacts there. `--output-dir` uses the same collision-free allocation under `<output-dir>/runs/`; `--run-id` retains the fixed run-scoped layout. The original input remains unchanged.
- Each summarize operation stores a complete raw-log copy in its artifact directory, so repeated summarization increases local storage usage in proportion to the source log size.
- Summary JSON stores excerpt references relative to the summary directory, such as `excerpts/F001.log`. An absolute `--summary` input remains valid, while absolute, traversal, cross-run, dangling, and symlink-escaping embedded references fail with artifact exit code `3`.
- Inferred `command_id` and tag values are redacted in summary, status, and console metadata. The copied raw-log and derived artifact references retain their literal filenames so they remain resolvable.

## Exit code guidance

| Condition | CLI exit code |
|---|---:|
| Test command passed | `0` |
| Test command failed | underlying command exit code when available |
| Test command timed out | documented timeout code, recommended `124` |
| Test command interrupted by SIGINT/SIGTERM on Unix | `130` / `143`, with `status: killed` |
| Manta config error | documented internal code, recommended `2` |
| Manta artifact write error | documented internal code, recommended `3` |
| Bounded-tail extraction after a passing test command | CLI `0`; artifacts use `passed` / `0` with `extractor_status: degraded` |
| Bounded-tail extraction after a failing test command | underlying command exit code and status with `extractor_status: degraded` |
| Evidence truncation after a passing test command | CLI `0`; artifacts use `passed` / `0` with `extractor_status: degraded` |
| Evidence truncation after a failing test command | underlying command exit code and status with `extractor_status: degraded` |
| Extraction internal error after a passing test command | CLI `4`; artifacts retain command exit `0` with `status: internal_error` |
| Extraction internal error after a failed, timed-out, or killed command | original command exit code and status |
| Extraction internal error during `summarize` | CLI and artifact exit code `4` with `status: internal_error` |
| Other Manta parser/rule internal error | documented internal code, recommended `4` |
| Successful `summarize` or `excerpt` | `0` |

## Markdown summary shape

```markdown
# Manta Summary: unit

Status: failed
Exit code: 1
Duration: 0.0s
Extractor: precise
Failures: 1 (truncated: false)
Warnings: 0 (truncated: false)
Raw log: .manta/runs/scoped/summarize-example/artifacts/test/unit.raw.log
Raw log SHA-256: sha256:...

## Failures

### F001: TypeError: token=<redacted> failed

- File: src/foo.test.ts:42
- Test: renders empty state
- Excerpt: excerpts/F001.log

## Notes

Command exit code is authoritative. Extraction rules only summarize evidence.
```
