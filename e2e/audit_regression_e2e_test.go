package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/irootkernel/manta/internal/model"
)

func TestBinaryRejectsUnknownConfigFields(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
	configPath := filepath.Join(repo, ".manta", "tester.yaml")
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString("redactions:\n  patterns: []\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("append unknown config field: write=%v close=%v", writeErr, closeErr)
	}
	out, err := exec.Command(bin, "--repo", repo, "run", "unit").CombinedOutput()
	requireExitCode(t, err, int(model.ExitCodeConfigError), out)
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("command ran after invalid config: %v", err)
	}
}

func TestRequirementTraceabilityMatrixCoversCompletedRequirements(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	specData, err := os.ReadFile(filepath.Join(root, "docs", "requirements-specs.md"))
	if err != nil {
		t.Fatal(err)
	}
	matrixData, err := os.ReadFile(filepath.Join(root, "docs", "requirements-test-matrix.md"))
	if err != nil {
		t.Fatal(err)
	}

	completedPattern := regexp.MustCompile(`(?m)^- \[x\] \x60(MANTA-REQ-[A-Z0-9-]+)\x60`)
	rowPattern := regexp.MustCompile(`(?m)^\| \x60(MANTA-REQ-[A-Z0-9-]+)\x60 \| ([^|]+) \|$`)
	completed := make(map[string]bool)
	for _, match := range completedPattern.FindAllSubmatch(specData, -1) {
		completed[string(match[1])] = true
	}
	mapped := make(map[string]bool)
	for _, match := range rowPattern.FindAllSubmatch(matrixData, -1) {
		id := string(match[1])
		if mapped[id] {
			t.Errorf("duplicate traceability row for %s", id)
		}
		mapped[id] = true
		if strings.TrimSpace(string(match[2])) == "" {
			t.Errorf("empty evidence for %s", id)
		}
	}
	for id := range completed {
		if !mapped[id] {
			t.Errorf("completed requirement missing from traceability matrix: %s", id)
		}
	}
	for id := range mapped {
		if !completed[id] {
			t.Errorf("traceability matrix references unknown or incomplete requirement: %s", id)
		}
	}
	if len(completed) != len(mapped) {
		t.Errorf("traceability count mismatch: completed=%d mapped=%d", len(completed), len(mapped))
	}
}

func TestBinaryRuleTestDoesNotUseParserFallback(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".manta", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rule := strings.Join([]string{
		"id: never-match",
		"lane: unit",
		"parser: generic",
		"status: active",
		"provenance:",
		"  created_by: auditor",
		"  source_run: local-audit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 1",
		"    end_line: 1",
		"  reason: audit fixture",
		"match:",
		"  start:",
		"    regex: '^THIS-WILL-NEVER-MATCH$'",
		"  end:",
		"    max_block_lines: 8",
		"  include_context:",
		"    before: 0",
		"    after: 0",
		"extract: {}",
		"confidence: medium",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "never-match.yaml"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	if err := os.WriteFile(rawPath, []byte("Error: generic fallback must not count\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "--repo", repo, "rules", "test", "--rule", "never-match", "--log", rawPath, "--expect-span", "1:1").CombinedOutput()
	requireExitCode(t, err, int(model.ExitCodeParserError), out)
	if !strings.Contains(string(out), "produced no failures") {
		t.Fatalf("unexpected rule-test diagnostic: %s", out)
	}
}

func TestBinaryRejectsOversizedRuleContext(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)

	t.Run("create and update fail closed", func(t *testing.T) {
		repo := t.TempDir()
		validPath := filepath.Join(repo, "valid.yaml")
		invalidPath := filepath.Join(repo, "invalid.yaml")
		if err := os.WriteFile(validPath, []byte(auditRuleYAML("bounded-v1", 20, 0, 0)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(invalidPath, []byte(auditRuleYAML("bounded-v1", 160, 0, 1)), 0o644); err != nil {
			t.Fatal(err)
		}

		createOut, createErr := exec.Command(bin, "--repo", repo, "rules", "create", "--file", invalidPath).CombinedOutput()
		requireExitCode(t, createErr, int(model.ExitCodeConfigError), createOut)
		if _, err := os.Stat(filepath.Join(repo, ".manta", "tester", "rules", "bounded-v1.yaml")); !os.IsNotExist(err) {
			t.Fatalf("invalid create wrote a rule file: %v", err)
		}

		runExpectedExit(t, exec.Command(bin, "--repo", repo, "rules", "create", "--file", validPath), 0)
		updateOut, updateErr := exec.Command(bin, "--repo", repo, "rules", "update", "bounded-v1", "--file", invalidPath).CombinedOutput()
		requireExitCode(t, updateErr, int(model.ExitCodeConfigError), updateOut)
		stored, err := os.ReadFile(filepath.Join(repo, ".manta", "tester", "rules", "bounded-v1.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(stored), "max_block_lines: 160") {
			t.Fatalf("invalid update overwrote the valid rule:\n%s", stored)
		}
	})

	t.Run("test and run reject discovered invalid rule", func(t *testing.T) {
		repo := t.TempDir()
		rulesDir := filepath.Join(repo, ".manta", "tester", "rules")
		if err := os.MkdirAll(rulesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		invalid := auditRuleYAML("overflow-v1", 1, 0, int(^uint(0)>>1))
		if err := os.WriteFile(filepath.Join(rulesDir, "overflow-v1.yaml"), []byte(invalid), 0o644); err != nil {
			t.Fatal(err)
		}
		rawPath := filepath.Join(repo, "fixture.raw.log")
		if err := os.WriteFile(rawPath, []byte("MARKER\n\nafter\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		testOut, testErr := exec.Command(bin, "--repo", repo, "rules", "test", "--rule", "overflow-v1", "--log", rawPath, "--expect-span", "1:3").CombinedOutput()
		requireExitCode(t, testErr, int(model.ExitCodeConfigError), testOut)
		if strings.Contains(string(testOut), "panic:") {
			t.Fatalf("rules test panicked:\n%s", testOut)
		}

		writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
		runOut, runErr := exec.Command(bin, "--repo", repo, "run", "unit").CombinedOutput()
		requireExitCode(t, runErr, int(model.ExitCodeConfigError), runOut)
		if strings.Contains(string(runOut), "panic:") {
			t.Fatalf("configured run panicked:\n%s", runOut)
		}
		if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
			t.Fatalf("command ran after invalid rule discovery: %v", err)
		}
	})
}

func TestBinaryPreservesConcurrentRuleProposals(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "fixture.raw.log")
	if err := os.WriteFile(rawPath, []byte("TypeError: boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	const count = 12
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bin, "--repo", repo, "rules", "propose", "--lane", "unit", "--parser", "generic", "--raw-log", rawPath, "--span", "1:1")
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("%w: %s", err, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("proposal command failed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(repo, ".manta", "rule-proposals"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != count {
		t.Fatalf("proposal files = %d, want %d", len(entries), count)
	}
}

func auditRuleYAML(id string, maxBlockLines, before, after int) string {
	return fmt.Sprintf(`id: %s
lane: unit
parser: generic
status: active
provenance:
  created_by: auditor
  source_run: local-audit
  source_command: unit
  source_log_sha256: sha256:abc
  source_span:
    start_line: 1
    end_line: 2
  reason: audit fixture
match:
  start:
    regex: '^MARKER$'
  end:
    any_of:
      - regex: '^$'
    max_block_lines: %d
  include_context:
    before: %d
    after: %d
extract: {}
confidence: medium
`, id, maxBlockLines, before, after)
}
