# kkachi-agent-tester

KAT runs a test command, keeps its original output, and produces a compact failure summary that is easier for people and automation to consume.

Use it when you want to:

- run the same project test commands locally or from automation;
- keep a raw log for audit while reviewing a much smaller summary;
- give another tool stable JSON status and evidence paths;
- summarize a log that was produced outside KAT.

KAT never changes a command result: a failing test command remains failed even when no parser recognizes its output.

## Install

Install the current release with Go:

```bash
go install github.com/SeventeenthEarth/kkachi-agent-tester@v0.1.3
kkachi-agent-tester --version
```

From a source checkout, use:

```bash
make install
```

Projects that pin a local KAT toolchain can install the versioned binary at `~/.local/kkachi/toolchains/kat/v0.1.3/bin/`:

```bash
VERSION=0.1.3 make install-toolchain
```

## Try it in five minutes

The following disposable command intentionally fails so you can see the evidence KAT creates. Run it from any temporary directory:

```bash
mkdir -p .kkachi
cat > .kkachi/tester.yaml <<'YAML'
version: 1
commands:
  demo:
    command: ["sh", "kat-demo-test.sh"]
    lane: unit
    parser: generic
    timeout_sec: 30
redaction:
  patterns:
    - name: token
      regex: 'token=[^ ]+'
      replace: 'token=<redacted>'
YAML

cat > kat-demo-test.sh <<'SH'
#!/bin/sh
echo 'TypeError: token=secret failed'
echo 'src/demo.test.ts:12:3'
echo '✗ renders the demo'
exit 1
SH
chmod +x kat-demo-test.sh
```

Run the configured command:

```bash
kkachi-agent-tester run demo
```

The command exits `1`, and KAT prints the paths of the generated evidence. Open the latest human-readable summary:

```bash
latest_run="$(ls -dt .kat/runs/* | head -1)"
sed -n '1,120p' "$latest_run/demo.summary.md"
```

The summary contains `token=<redacted>`. The corresponding `demo.raw.log` intentionally retains the original `token=secret` value, so treat raw logs as sensitive local evidence.

## Configure your project

Commit `.kkachi/tester.yaml` with the commands your project wants to expose. Commands are argv arrays, so no shell quoting is added implicitly.

```yaml
version: 1
commands:
  unit:
    command: ["go", "test", "./..."]
    lane: unit
    parser: go-test
    timeout_sec: 600
  web:
    command: ["pnpm", "vitest", "run"]
    lane: unit
    parser: vitest
    timeout_sec: 600
```

Choose the parser that matches the command output:

| Test output | Parser |
|---|---|
| Other or project-specific text | `generic` |
| Vitest | `vitest` |
| Pytest | `pytest` |
| `go test` | `go-test` |
| Playwright | `playwright` |

Run a configured command by ID:

```bash
kkachi-agent-tester run unit
```

Use an ad-hoc command when you do not want to add it to the config:

```bash
kkachi-agent-tester run --lane unit -- go test ./internal/...
```

## Work with existing evidence

Summarize an existing raw log without rerunning its command:

```bash
kkachi-agent-tester summarize path/to/unit.raw.log
```

Use `--run-id` when a parent workflow needs a stable run-scoped location:

```bash
kkachi-agent-tester --run-id local-check run unit
```

This writes under:

```text
.kkachi/runs/local-check/artifacts/test/
```

For standalone runs, KAT creates a collision-free directory under `.kat/runs/`. Each run contains:

| Artifact | Use |
|---|---|
| `*.summary.md` | First stop for human review |
| `*.status.json` | Compact polling and completion state |
| `*.summary.json` | Structured failures, warnings, and spans |
| `excerpts/*.log` | Bounded evidence for one failure |
| `*.raw.log` | Original, potentially unredacted output |

Retrieve one failure excerpt without opening the full raw log:

```bash
kkachi-agent-tester excerpt \
  --summary .kkachi/runs/local-check/artifacts/test/unit.summary.json \
  F001
```

Add `--json` when a script needs compact command output. Use `--repo`, `--config`, or `--output-dir` to select a different project root, config, or standalone evidence directory.

## Safe defaults

- The executed command's exit code is authoritative.
- Summaries and excerpts are bounded; raw logs are preserved unchanged.
- Redaction applies to surfaced summaries, excerpts, status, and console metadata, not to raw logs or literal artifact paths.
- Do not put secrets in run IDs, command IDs, output directories, or filenames.
- Add `.kat/` and `.kkachi/runs/` to the parent project's ignore rules unless its evidence-retention policy says otherwise.

## Learn more

- [CLI reference and rule workflow](docs/user-interface.md)
- [Parent-project integration guide and current capability status](docs/integration-guide.md)
- [Documentation map](docs/README.md)
- [Architecture and artifact contracts](docs/architecture.md)
- [Development and verification guidance](AGENTS.md)
