package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

func TestRulesLifecycleCommands(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	inputPath := filepath.Join(repo, "generic-v1.yaml")
	ruleText := ruleInputYAML("generic-v1", "fixture-backed rule")
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
	exitCode = Main([]string{"--repo", repo, "rules", "propose", "--tag", "unit", "--tag", "go", "--parser", "generic", "--raw-log", filepath.ToSlash(rawPath), "--span", "2:4"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "Proposed rule:") {
		t.Fatalf("expected rules propose to succeed, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	proposalEntries, err := os.ReadDir(filepath.Join(repo, ".manta", "rule-proposals"))
	if err != nil || len(proposalEntries) != 1 {
		t.Fatalf("expected one proposal file, err=%v entries=%d", err, len(proposalEntries))
	}
	proposal, err := readRuleInput(filepath.Join(repo, ".manta", "rule-proposals", proposalEntries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(proposal.Tags, []string{"go", "unit"}) {
		t.Fatalf("proposal tags = %q, want [go unit]", proposal.Tags)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "delete", "generic-v1", "--reason", "superseded by v2"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "disabled") {
		t.Fatalf("expected delete to disable rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRuleSourceFilesEnforceInputSizeLimit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	exactPath := filepath.Join(repo, "exact.yaml")
	if err := os.WriteFile(exactPath, paddedRuleInput("bounded-v1", safety.MaxConfigRuleInputBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "rules", "create", "--file", exactPath}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exact-limit create failed: exit=%d stderr=%q", exitCode, stderr.String())
	}
	storedPath := filepath.Join(repo, ".manta", "tester", "rules", "bounded-v1.yaml")
	before, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatal(err)
	}

	oversizedUpdate := filepath.Join(repo, "oversized-update.yaml")
	if err := os.WriteFile(oversizedUpdate, paddedRuleInput("bounded-v1", safety.MaxConfigRuleInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode := Main([]string{"--repo", repo, "rules", "update", "bounded-v1", "--file", oversizedUpdate}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) || !strings.Contains(stderr.String(), "input exceeds 262144 bytes") {
		t.Fatalf("oversized update exit=%d stderr=%q", exitCode, stderr.String())
	}
	after, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("oversized update changed the stored rule")
	}

	oversizedCreate := filepath.Join(repo, "oversized-create.yaml")
	if err := os.WriteFile(oversizedCreate, paddedRuleInput("oversized-v1", safety.MaxConfigRuleInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "create", "--file", oversizedCreate}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) || !strings.Contains(stderr.String(), "input exceeds 262144 bytes") {
		t.Fatalf("oversized create exit=%d stderr=%q", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".manta", "tester", "rules", "oversized-v1.yaml")); !os.IsNotExist(err) {
		t.Fatalf("oversized create wrote a rule: %v", err)
	}
}

func paddedRuleInput(id string, size int) []byte {
	base := ruleInputYAML(id, "bounded input fixture") + "#"
	return append([]byte(base), bytes.Repeat([]byte("x"), size-len(base))...)
}

func ruleInputYAML(id, reason string) string {
	return strings.Join([]string{
		"id: " + id,
		"tags: [unit]",
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
		"  reason: " + reason,
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
}
