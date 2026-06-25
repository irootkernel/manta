package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kkachi-agent-tester/internal/model"
)

func TestLoadApplicableFailsOnInvalidDiscoveredFutureParserRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".kkachi", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	futureRule := []byte(strings.Join([]string{
		"id: future-v1",
		"lane: unit",
		"parser: vitest",
		"status: active",
		"provenance:",
		"  created_by: tester",
		"  source_run: local-unit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 2",
		"    end_line: 4",
		"  reason: invalid regex coverage",
		"match:",
		"  start:",
		"    regex: '(['",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 1",
		"    after: 1",
	}, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(rulesDir, "future.yaml"), futureRule, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApplicable(repo, "unit", "generic"); err == nil {
		t.Fatal("expected any discovered invalid rule file to fail closed")
	}
}

func TestLoadApplicableSkipsValidNonMatchingFutureParserRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".kkachi", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	futureRule := []byte(strings.Join([]string{
		"id: future-v1",
		"lane: unit",
		"parser: vitest",
		"status: active",
		"provenance:",
		"  created_by: tester",
		"  source_run: local-unit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 2",
		"    end_line: 4",
		"  reason: future parser fixture",
		"match:",
		"  start:",
		"    regex: '^TypeError:'",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 1",
		"    after: 1",
		"extract:",
		"  file_line:",
		"    regex: '(?P<file>[^\\s:]+\\.ts):(?P<line>\\d+)'",
		"  test_name:",
		"    regex: '^[✗×] (.+)$'",
	}, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(rulesDir, "future.yaml"), futureRule, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadApplicable(repo, "unit", "generic")
	if err != nil {
		t.Fatalf("expected valid non-matching rule to validate and skip, got %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected no applicable rules, got %d", len(loaded))
	}
}

func TestLoadApplicableFailsOnInvalidMatchingRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".kkachi", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	invalidRule := []byte(strings.Join([]string{
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
		"  reason: invalid matching rule",
		"match:",
		"  start:",
		"    regex: '^TypeError:'",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 1",
		"    after: 1",
		"extract:",
		"  file_line:",
		"    regex: '([^\\s:]+\\.ts):(\\d+)'",
	}, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(rulesDir, "generic.yaml"), invalidRule, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApplicable(repo, "unit", "generic"); err == nil {
		t.Fatal("expected invalid matching rule to fail closed")
	}
}

func TestCreateSearchAndDeleteRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rule := validRule("generic-v1")
	created, err := Create(repo, rule)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !strings.HasSuffix(created.SourcePath, filepath.ToSlash("generic-v1.yaml")) && filepath.Base(created.SourcePath) != "generic-v1.yaml" {
		t.Fatalf("unexpected source path %q", created.SourcePath)
	}
	loaded, err := Search(repo, "generic-v1")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "generic-v1" {
		t.Fatalf("expected one searchable rule, got %+v", loaded)
	}
	disabled, err := Delete(repo, "generic-v1", "superseded by v2")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if disabled.Status != model.RuleStatusDisabled || disabled.DeletionReason != "superseded by v2" {
		t.Fatalf("expected disabled rule with deletion reason, got %+v", disabled)
	}
}

func TestTestRuleMatchesExpectedSpan(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if _, err := Create(repo, validRule("generic-v1")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "before\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := TestRule(repo, "generic-v1", rawPath, 2, 5)
	if err != nil {
		t.Fatalf("TestRule failed: %+v %v", result, err)
	}
	if !result.Passed || result.FailureCount != 1 {
		t.Fatalf("expected passing rule test, got %+v", result)
	}
}

func TestProposeWritesRunLocalProposal(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "fixtures", "unit.raw.log")
	if err := os.MkdirAll(filepath.Dir(rawPath), 0o755); err != nil {
		t.Fatal(err)
	}
	rawText := "noise\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	proposal, err := Propose(repo, "unit", "generic", rawPath, 2, 4)
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}
	if !strings.HasPrefix(proposal.Path, ProposedRulesDir(repo)) {
		t.Fatalf("expected proposal path under proposed-rules dir, got %q", proposal.Path)
	}
	if strings.HasPrefix(proposal.Path, RulesDir(repo)) {
		t.Fatalf("expected proposal to stay separate from active rules, got %q", proposal.Path)
	}
	if proposal.Rule.Provenance.SourceSpan.StartLine != 2 || proposal.Rule.Provenance.SourceSpan.EndLine != 4 {
		t.Fatalf("unexpected proposal span %+v", proposal.Rule.Provenance.SourceSpan)
	}
}

func TestRuleDetectsOvermatch(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if _, err := Create(repo, validRule("generic-v1")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "before\nTypeError: first\nsrc/foo.ts:11:1\n✗ first\n\nTypeError: second\nsrc/foo.ts:22:1\n✗ second\n\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := TestRule(repo, "generic-v1", rawPath, 2, 5)
	if err == nil {
		t.Fatalf("expected overmatch failure, got %+v", result)
	}
	if !strings.Contains(err.Error(), "overmatched") {
		t.Fatalf("expected overmatch diagnostic, got %v", err)
	}
}

func validRule(id string) model.Rule {
	return model.Rule{
		ID:     id,
		Lane:   "unit",
		Parser: "generic",
		Status: model.RuleStatusActive,
		Provenance: model.RuleProvenance{
			CreatedBy:       "tester",
			SourceRun:       "local-unit",
			SourceCommand:   "unit",
			SourceLogSHA256: "sha256:abc",
			SourceSpan:      model.RawSpan{StartLine: 2, EndLine: 4},
			Reason:          "fixture-backed rule",
		},
		Match: model.RuleMatch{
			Start:          model.RuleRegex{Regex: `^TypeError:`},
			End:            model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: 20},
			IncludeContext: model.RuleContext{Before: 0, After: 0},
		},
		Extract: model.RuleExtract{
			FileLine: model.RuleExtractField{Regex: `(?P<file>[^\s:]+\.ts):(?P<line>\d+)`},
			TestName: model.RuleExtractField{Regex: `^\s*[✗×]\s+(?P<test>.+)$`},
		},
		Confidence: "medium",
	}
}
