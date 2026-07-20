package rules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/safety"
)

var knownRuleParsers = map[string]bool{
	"generic":    true,
	"vitest":     true,
	"pytest":     true,
	"go-test":    true,
	"playwright": true,
}

func Discover(repoRoot string) ([]string, error) {
	rulesDir := RulesDir(repoRoot)
	entries, err := safety.ReadDirWithin(repoRoot, rulesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, model.NewKATError(model.ExitCodeConfigError, "discover rule files", err)
	}
	matches := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			matches = append(matches, filepath.Join(rulesDir, entry.Name()))
		}
	}
	return matches, nil
}

func LoadApplicable(repoRoot, lane, parser string) ([]model.Rule, error) {
	allRules, err := LoadAll(repoRoot)
	if err != nil {
		return nil, err
	}
	applicable := make([]model.Rule, 0, len(allRules))
	for _, rule := range allRules {
		if !isApplicable(rule, lane, parser) {
			continue
		}
		if rule.Status == model.RuleStatusDisabled {
			continue
		}
		applicable = append(applicable, rule)
	}
	return applicable, nil
}

func isApplicable(rule model.Rule, lane, parser string) bool {
	if rule.Parser != parser {
		return false
	}
	if rule.Lane != lane {
		return false
	}
	return true
}

func ValidateApplicable(rule model.Rule) error {
	if err := safety.ValidateArtifactIdentifier("rule id", rule.ID); err != nil {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("%w (%s)", err, rule.SourcePath))
	}
	if rule.Status != model.RuleStatusActive && rule.Status != model.RuleStatusDisabled {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q has invalid status %q", rule.ID, rule.Status))
	}
	if strings.TrimSpace(rule.Parser) == "" {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q must define parser", rule.ID))
	}
	if !knownRuleParsers[rule.Parser] {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q has unsupported parser label %q", rule.ID, rule.Parser))
	}
	if strings.TrimSpace(rule.Lane) == "" {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q must define lane", rule.ID))
	}
	if strings.TrimSpace(rule.Match.Start.Regex) == "" {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q must define start regex", rule.ID))
	}
	if rule.Match.End.MaxBlockLines <= 0 || rule.Match.End.MaxBlockLines > safety.MaxBlockLines {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q max_block_lines must be between 1 and %d", rule.ID, safety.MaxBlockLines))
	}
	if _, err := validateRegex(rule.Match.Start.Regex, rule.ID, "start"); err != nil {
		return err
	}
	for _, expr := range rule.Match.End.AnyOf {
		if _, err := validateRegex(expr.Regex, rule.ID, "end"); err != nil {
			return err
		}
	}
	if rule.Extract.FileLine.Regex != "" {
		re, err := validateRegex(rule.Extract.FileLine.Regex, rule.ID, "extract.file_line")
		if err != nil {
			return err
		}
		if !hasNamedGroup(re, "file") || !hasNamedGroup(re, "line") {
			return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q extract.file_line must define named capture groups file and line", rule.ID))
		}
	}
	if rule.Extract.TestName.Regex != "" {
		re, err := validateRegex(rule.Extract.TestName.Regex, rule.ID, "extract.test_name")
		if err != nil {
			return err
		}
		if re.NumSubexp() < 1 {
			return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q extract.test_name must define at least one capture group", rule.ID))
		}
	}
	return nil
}

func validateRegex(expr, ruleID, field string) (*regexp.Regexp, error) {
	if err := safety.ValidateRegex(expr); err != nil {
		return nil, model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q %s invalid regex: %w", ruleID, field, err))
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q %s invalid regex: %w", ruleID, field, err))
	}
	return re, nil
}

func hasNamedGroup(re *regexp.Regexp, name string) bool {
	for _, group := range re.SubexpNames() {
		if group == name {
			return true
		}
	}
	return false
}
