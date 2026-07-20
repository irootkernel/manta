package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
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

func TestUpdateRulePersistsChanges(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rule := validRule("generic-v1")
	if _, err := Create(repo, rule); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rule.Confidence = "high"
	rule.Provenance.Reason = "fixture-backed rule updated"
	updated, err := Update(repo, rule.ID, rule)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Confidence != "high" || updated.Provenance.Reason != "fixture-backed rule updated" {
		t.Fatalf("Update returned stale rule: %+v", updated)
	}
	reloaded, err := LoadByID(repo, rule.ID)
	if err != nil {
		t.Fatalf("LoadByID failed: %v", err)
	}
	if reloaded.Confidence != "high" || reloaded.Provenance.Reason != "fixture-backed rule updated" {
		t.Fatalf("updated fields were not persisted: %+v", reloaded)
	}
}

func TestCreateRejectsUnsafeRuleIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"../generic", "/tmp/generic", "nested/generic", ".", "규칙"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			if _, err := Create(repo, validRule(id)); err == nil {
				t.Fatalf("expected unsafe rule id %q to fail", id)
			}
		})
	}
}

func TestLoadAllRejectsRuleSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := RulesDir(repo)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external.yaml")
	data, err := yaml.Marshal(validRule("external"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(external, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(rulesDir, "external.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAll(repo); err == nil {
		t.Fatal("expected external rule symlink to fail closed")
	}
}

func TestLoadAllRejectsDanglingRuleSymlink(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := RulesDir(repo)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "missing.yaml"), filepath.Join(rulesDir, "dangling.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAll(repo); err == nil {
		t.Fatal("expected dangling rule symlink to fail closed")
	}
}

func TestLoadAllAllowsInternalRuleSymlink(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := RulesDir(repo)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(validRule("internal"))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repo, "internal-rule.yaml")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(rulesDir, "internal.yaml")); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAll(repo)
	if err != nil {
		t.Fatalf("expected internal rule symlink to be allowed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "internal" {
		t.Fatalf("unexpected rules %+v", loaded)
	}
}

func TestCreateRuleSymlinkContainment(t *testing.T) {
	t.Parallel()
	t.Run("external rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "tester"), 0o755); err != nil {
			t.Fatal(err)
		}
		external := t.TempDir()
		if err := os.Symlink(external, RulesDir(repo)); err != nil {
			t.Fatal(err)
		}
		_, err := Create(repo, validRule("external-write"))
		if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact error, got %v", err)
		}
		entries, err := os.ReadDir(external)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected no external rule writes, got %d entries", len(entries))
		}
	})

	t.Run("internal allowed", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "tester"), 0o755); err != nil {
			t.Fatal(err)
		}
		internal := filepath.Join(repo, "actual-rules")
		if err := os.MkdirAll(internal, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(internal, RulesDir(repo)); err != nil {
			t.Fatal(err)
		}
		created, err := Create(repo, validRule("internal-write"))
		if err != nil {
			t.Fatalf("expected internal symlink write to succeed: %v", err)
		}
		if created.ID != "internal-write" {
			t.Fatalf("unexpected created rule %+v", created)
		}
		if _, err := os.Stat(filepath.Join(internal, "internal-write.yaml")); err != nil {
			t.Fatalf("expected rule in internal target: %v", err)
		}
	})
}

func TestProposeRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".kat"), 0o755); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := os.Symlink(external, ProposedRulesDir(repo)); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "noise\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Propose(repo, "unit", "generic", rawPath, 2, 4)
	if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact error, got %v", err)
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no external proposal writes, got %d entries", len(entries))
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
