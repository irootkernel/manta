# manta

Manta runs a test command, keeps its original output, and produces a compact failure summary that is easier for people and automation to consume.

Use it when you want to:

- run the same project test commands locally or from automation;
- keep a raw log for audit while reviewing a much smaller summary;
- give another tool stable JSON status and evidence paths;
- summarize a log that was produced outside Manta.

Manta never changes a command result: a failing test command remains failed even when no parser recognizes its output.

## Install

Install the current release with Go:

```bash
go install github.com/irootkernel/manta@v0.1.5
manta --version
```

From a source checkout, use:

```bash
make install
```

Projects that pin a local Manta toolchain can install the versioned binary at `~/.local/manta/toolchains/v0.1.5/bin/`:

```bash
VERSION=0.1.5 make install-toolchain
```

## Try it in five minutes

The following disposable command intentionally fails so you can see the evidence Manta creates. Run it from any temporary directory:

```bash
mkdir -p .manta
cat > .manta/tester.yaml <<'YAML'
version: 2
commands:
  demo:
    command: ["sh", "manta-demo-test.sh"]
    tags: [demo, unit]
    parser: generic
    timeout_sec: 30
redaction:
  patterns:
    - name: token
      regex: 'token=[^ ]+'
      replace: 'token=<redacted>'
YAML

cat > manta-demo-test.sh <<'SH'
#!/bin/sh
echo 'TypeError: token=secret failed'
echo 'src/demo.test.ts:12:3'
echo '✗ renders the demo'
exit 1
SH
chmod +x manta-demo-test.sh
```

Run the configured command:

```bash
manta run demo
```

The command exits `1`, and Manta prints the paths of the generated evidence. Open the latest human-readable summary:

```bash
latest_run="$(ls -dt .manta/runs/standalone/* | head -1)"
sed -n '1,120p' "$latest_run/demo.summary.md"
```

The summary contains `token=<redacted>`. The corresponding `demo.raw.log` intentionally retains the original `token=secret` value, so treat raw logs as sensitive local evidence.

## Configure your local project

Create `.manta/tester.yaml` with the commands you want to expose locally. The entire `.manta/` directory is local state and should be ignored by Git. Commands are argv arrays, so no shell quoting is added implicitly.

```yaml
version: 2
commands:
  unit:
    command: ["go", "test", "./..."]
    tags: [go, unit]
    parser: go-test
    timeout_sec: 600
  web:
    command: ["pnpm", "vitest", "run"]
    tags: [unit, web]
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
manta run unit
```

Use an ad-hoc command when you do not want to add it to the config:

```bash
manta run --tag go --tag unit -- go test ./internal/...
```

Tags select which local extraction rules may inspect a raw log; they do not select the command or change pass/fail. A rule applies only when its parser matches and all of its tags are present on the run. This lets a `tags: [go]` rule apply to both Go unit and integration runs while a `tags: [go, unit]` rule remains unit-specific. Multiple applicable rules may run against the same log.

## Work with existing evidence

Summarize an existing raw log without rerunning its command:

```bash
manta summarize path/to/unit.raw.log
```

Add repeatable tags when the filename alone does not describe the applicable rule scope:

```bash
manta summarize --tag go --tag unit path/to/unit.raw.log
```

Use `--run-id` when a parent workflow needs a stable run-scoped location:

```bash
manta --run-id local-check run unit
```

This writes under:

```text
.manta/runs/scoped/local-check/artifacts/test/
```

For standalone runs, Manta creates a collision-free directory under `.manta/runs/standalone/`. Each run contains:

| Artifact | Use |
|---|---|
| `*.summary.md` | First stop for human review |
| `*.status.json` | Compact polling and completion state |
| `*.summary.json` | Structured failures, warnings, and spans |
| `excerpts/*.log` | Bounded evidence for one failure |
| `*.raw.log` | Original, potentially unredacted output |

Retrieve one failure excerpt without opening the full raw log:

```bash
manta excerpt \
  --summary .manta/runs/scoped/local-check/artifacts/test/unit.summary.json \
  F001
```

Add `--json` when a script needs compact command output. Use `--repo`, `--config`, or `--output-dir` to select a different project root, config, or standalone evidence directory.

## Safe defaults

- The executed command's exit code is authoritative.
- Summaries and excerpts are bounded; raw logs are preserved unchanged. Summaries retain at most 50 failures and 50 warnings, report truncation explicitly, and remain within their byte budget. Logs larger than 256 KiB use degraded extraction from a bounded complete-line tail instead of becoming internal errors.
- Config YAML, stored and imported rule YAML, and `rules propose --raw-log` inputs are limited to 256 KiB and fail with config exit code `2` when oversized.
- Redaction applies to surfaced summaries, excerpts, status, and console metadata, not to raw logs or literal artifact paths.
- Do not put secrets in run IDs, command IDs, output directories, or filenames.
- Ignore the entire `.manta/` directory. Config, rules, toolchain metadata, proposals, and evidence are local-only state.

## Learn more

- [CLI reference and rule workflow](docs/user-interface.md)
- [Parent-project integration guide and current capability status](docs/integration-guide.md)
- [Documentation map](docs/README.md)
- [Architecture and artifact contracts](docs/architecture.md)
- [Development and verification guidance](AGENTS.md)
