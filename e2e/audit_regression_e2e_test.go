package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
)

func TestBinaryRejectsUnknownConfigFields(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
	configPath := filepath.Join(repo, ".kkachi", "tester.yaml")
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

func TestBinaryRuleTestDoesNotUseParserFallback(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".kkachi", "tester", "rules")
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
	entries, err := os.ReadDir(filepath.Join(repo, ".kat", "rule-proposals"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != count {
		t.Fatalf("proposal files = %d, want %d", len(entries), count)
	}
}
