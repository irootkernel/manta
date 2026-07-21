package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulesLifecycleCommands(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	inputPath := filepath.Join(repo, "generic-v1.yaml")
	ruleText := strings.Join([]string{
		"id: generic-v1",
		"lane: unit",
		"parser: generic",
		"status: active",
		"provenance:",
		"  created_by: tester",
		"  source_run: local-unit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 2",
		"    end_line: 4",
		"  reason: fixture-backed rule",
		"match:",
		"  start:",
		"    regex: '^TypeError:'",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 0",
		"    after: 0",
		"extract:",
		"  file_line:",
		"    regex: '(?P<file>[^\\s:]+\\.ts):(?P<line>\\d+)'",
		"  test_name:",
		"    regex: '^\\s*[✗×]\\s+(?P<test>.+)$'",
		"confidence: medium",
	}, "\n") + "\n"
	if err := os.WriteFile(inputPath, []byte(ruleText), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "rules", "create", "--file", inputPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected create to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "list"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "generic-v1") {
		t.Fatalf("expected list to show created rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "search", "fixture-backed"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "generic-v1") {
		t.Fatalf("expected search to find created rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "show", "generic-v1"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "created_by: tester") {
		t.Fatalf("expected show to print yaml, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "before\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "test", "--rule", "generic-v1", "--log", filepath.ToSlash(rawPath), "--expect-span", "2:5"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "PASS generic-v1") {
		t.Fatalf("expected rules test to pass, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	updatedPath := filepath.Join(repo, "generic-v1-update.yaml")
	updatedRuleText := strings.ReplaceAll(ruleText, "reason: fixture-backed rule", "reason: fixture-backed rule updated")
	updatedRuleText = strings.ReplaceAll(updatedRuleText, "confidence: medium", "confidence: high")
	if err := os.WriteFile(updatedPath, []byte(updatedRuleText), 0o644); err != nil {
		t.Fatal(err)
	}
	exitCode = Main([]string{"--repo", repo, "rules", "update", "generic-v1", "--file", updatedPath}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "generic-v1") {
		t.Fatalf("expected update to succeed, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "show", "generic-v1"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "reason: fixture-backed rule updated") || !strings.Contains(stdout.String(), "confidence: high") {
		t.Fatalf("expected show to report persisted update, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "propose", "--lane", "unit", "--parser", "generic", "--raw-log", filepath.ToSlash(rawPath), "--span", "2:4"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "Proposed rule:") {
		t.Fatalf("expected rules propose to succeed, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	proposalEntries, err := os.ReadDir(filepath.Join(repo, ".manta", "rule-proposals"))
	if err != nil || len(proposalEntries) != 1 {
		t.Fatalf("expected one proposal file, err=%v entries=%d", err, len(proposalEntries))
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "delete", "generic-v1", "--reason", "superseded by v2"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "disabled") {
		t.Fatalf("expected delete to disable rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}
