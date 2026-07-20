# KAT User Interface

Status: Complete
Scope: CLI-first interface for KAT v0.1

## Interface principles

- CLI-first and script-friendly.
- Deterministic output paths.
- Human-readable Markdown summaries plus machine-readable JSON.
- No hidden pass/fail overrides.
- Compact console output; details live in artifacts.
- Raw logs are preserved as source evidence and may contain unredacted values.

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
| `--no-color` | Disable ANSI colors. |
| `--verbose` | Print additional local diagnostics without dumping raw logs. |

Run IDs, configured command IDs, and rule IDs must match `[A-Za-z0-9][A-Za-z0-9_-]*`. Invalid identifiers fail with config exit code `2`; run identifiers are checked before the test command starts.

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
exit 1
SH
chmod +x test.sh

cat > fixtures/unit.raw.log <<'LOG'
noise: start
TypeError: token=secret failed
src/foo.test.ts:42:13
✗ renders empty state
LOG
```

## Tested command examples

Configured run with deterministic artifact paths:

```bash
kkachi-agent-tester --run-id example-run run unit
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
kkachi-agent-tester --json summarize fixtures/unit.raw.log
```

Fixture-backed rule workflow examples:

```bash
kkachi-agent-tester rules list
kkachi-agent-tester rules show generic-v1
kkachi-agent-tester rules test --rule generic-v1 --log internal/extract/testdata/vitest.raw.log --expect-span 7:14
kkachi-agent-tester rules propose --lane unit --parser vitest --raw-log internal/extract/testdata/vitest.raw.log --span 7:9
```

## Summarize mode notes

- `summarize <raw-log>` uses the `generic` parser plus any matching project rules.
- When only a raw log is available, KAT infers `command_id` and `lane` from the raw-log basename. For example, `unit.raw.log` produces `command_id: unit` and `lane: unit`.
- Because original execution metadata is unavailable, summarize infers `status` and `exit_code` from raw-log evidence. Use `run` when authoritative execution metadata is required.
- Without `--run-id` or `--output-dir`, summarize copies the input raw log into a newly allocated `.kat/runs/<UTC-timestamp>[-NNN]/` directory and writes derived artifacts there. `--output-dir` uses the same collision-free allocation under `<output-dir>/runs/`; `--run-id` retains the fixed Kkachi-compatible layout. The original input remains unchanged.
- Each summarize operation stores a complete raw-log copy in its artifact directory, so repeated summarization increases local storage usage in proportion to the source log size.
- Summary JSON stores excerpt references relative to the summary directory, such as `excerpts/F001.log`. An absolute `--summary` input remains valid, while absolute, traversal, cross-run, dangling, and symlink-escaping embedded references fail with artifact exit code `3`.

## Exit code guidance

| Condition | CLI exit code |
|---|---:|
| Test command passed | `0` |
| Test command failed | underlying command exit code when available |
| Test command timed out | documented timeout code, recommended `124` |
| Test command interrupted by SIGINT/SIGTERM on Unix | `130` / `143`, with `status: killed` |
| KAT config error | documented internal code, recommended `2` |
| KAT artifact write error | documented internal code, recommended `3` |
| KAT parser/rule internal error | documented internal code, recommended `4` unless command itself failed first |
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

Command exit code is authoritative for `run`. Extraction rules only summarize evidence. Use `kkachi-agent-tester excerpt --summary .kkachi/runs/summarize-example/artifacts/test/unit.summary.json F001` for deterministic excerpt lookup.
```

## UI backlog references

- Rule examples should be based on real fixtures: `todo.md#TD-RULE-001`.
