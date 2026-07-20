# KAT Todo

Status: Hardening epic in progress
Scope: Open documentation and implementation follow-up notes

## Todo status legend

- `Open`: not started.
- `Active`: currently being worked.
- `Blocked`: waiting on a decision or dependency.
- `Done`: completed and referenced by roadmap or requirements.

## Active items

### TD-HARDE-001: Complete the post-baseline hardening epic

Status: Active
References: `HARDE-001` to `HARDE-007`, `KAT-REQ-RQHAR-001` to `KAT-REQ-RQHAR-007`

`HARDE-001` through `HARDE-006` are complete. Continue with `HARDE-007` as the final separate reviewable PR. It must retain its focused evidence and pass the affected existing suites before its roadmap status changes to `Done`; passing an earlier focused test does not close the epic.

## Completed items

### TD-RULE-001: Add parser and rule examples from real fixture logs

Status: Done
References: `KAT-REQ-RQDOC-003`, `RULES-003`

Parser and rule examples now reference real fixture logs under `internal/extract/testdata/`.

### TD-REL-001: Add v0.1 release-readiness checklist

Status: Done
References: `KAT-REQ-RQDOC-004`, `DOCUM-003`

Implementation notes now include the release-readiness checklist covering build, make-test, fixture coverage, artifact verification, watcher compatibility, and known limitations.

### TD-DOC-001: Add real CLI examples after implementation exists

Status: Done
References: `KAT-REQ-RQDOC-002`, `DOCUM-002`

User-interface docs now include copy-paste-tested examples for configured runs, ad-hoc runs, summarize, excerpt lookup, and JSON output.

### TD-DOC-BOOTSTRAP-001: Create initial docs directory

Status: Done
References: `KAT-REQ-RQDOC-001`, `DOCUM-001`

Initial documentation baseline created with seven Markdown documents: requirements specs, architecture, user interface, ADRs, roadmap, todo, and implementation note.

### TD-ARCH-001: Decide implementation language and packaging

Status: Done
References: `ADR-0006`, `SETUP-001`

Implementation baseline fixed to Go with a standalone single-binary packaging target.

### TD-SAFE-001: Decide safe regex strategy

Status: Done
References: `ADR-0007`, `SAFEY-003`

Regex policy fixed to Go RE2 semantics plus explicit bounds on regex input size and surfaced artifact size.

### TD-RAW-001: Clarify raw-log redaction policy

Status: Done
References: `ADR-0003`, `SAFEY-001`

Raw logs remain original evidence and are not redacted by default. Surfaced artifacts are redacted and docs must warn about raw-log sharing.

### TD-PARSE-001: Choose first specialized parser targets

Status: Done
References: `ADR-0008`, `PARSE-002`

Initial runnable implementation started with only the `generic` parser. Fixture-backed support now also exists for `vitest`, `pytest`, `go-test`, and `playwright`.

### TD-WATCH-001: Define stable status-hash fields

Status: Done
References: `ADR-0005`, `ARTIF-003`

Watcher compatibility is fixed around an explicit ordered field set: command ID, status, exit code, extractor status, raw-log checksum, failure signatures, warning signatures, summary path, and raw-log path.
