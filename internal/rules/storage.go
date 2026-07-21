package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/extract"
	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/safety"
)

func RulesDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".kkachi", "tester", "rules")
}

func ProposedRulesDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".kat", "rule-proposals")
}

func LoadAll(repoRoot string) ([]model.Rule, error) {
	paths, err := Discover(repoRoot)
	if err != nil {
		return nil, err
	}
	rules := make([]model.Rule, 0, len(paths))
	ids := map[string]string{}
	for _, path := range paths {
		rule, err := readRuleFile(repoRoot, path)
		if err != nil {
			return nil, err
		}
		if existing, ok := ids[rule.ID]; ok && rule.ID != "" {
			return nil, model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("duplicate rule id %q in %s and %s", rule.ID, existing, path))
		}
		ids[rule.ID] = path
		if err := ValidateStoredRule(rule); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules, nil
}

func LoadByID(repoRoot, id string) (model.Rule, error) {
	if err := safety.ValidateArtifactIdentifier("rule id", id); err != nil {
		return model.Rule{}, model.NewKATError(model.ExitCodeConfigError, "load rule", err)
	}
	rules, err := LoadAll(repoRoot)
	if err != nil {
		return model.Rule{}, err
	}
	for _, rule := range rules {
		if rule.ID == id {
			return rule, nil
		}
	}
	return model.Rule{}, model.NewKATError(model.ExitCodeConfigError, "load rule", fmt.Errorf("unknown rule id %q", id))
}

func Search(repoRoot, query string) ([]model.Rule, error) {
	rules, err := LoadAll(repoRoot)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return rules, nil
	}
	filtered := make([]model.Rule, 0, len(rules))
	for _, rule := range rules {
		haystack := strings.ToLower(strings.Join([]string{
			rule.ID,
			rule.Lane,
			rule.Parser,
			string(rule.Status),
			rule.Provenance.CreatedBy,
			rule.Provenance.SourceRun,
			rule.Provenance.SourceCommand,
			rule.Provenance.Reason,
			rule.DeletionReason,
			filepath.Base(rule.SourcePath),
		}, " "))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, rule)
		}
	}
	return filtered, nil
}

func Create(repoRoot string, rule model.Rule) (model.Rule, error) {
	if err := ensureRuleIDAvailable(repoRoot, rule.ID); err != nil {
		return model.Rule{}, err
	}
	path := filepath.Join(RulesDir(repoRoot), rule.ID+".yaml")
	return writeRuleFile(repoRoot, path, rule)
}

func Update(repoRoot, id string, rule model.Rule) (model.Rule, error) {
	if rule.ID != id {
		return model.Rule{}, model.NewKATError(model.ExitCodeConfigError, "update rule", fmt.Errorf("rule file id %q does not match target id %q", rule.ID, id))
	}
	existing, err := LoadByID(repoRoot, id)
	if err != nil {
		return model.Rule{}, err
	}
	path := existing.SourcePath
	if path == "" {
		path = filepath.Join(RulesDir(repoRoot), id+".yaml")
	}
	return writeRuleFile(repoRoot, path, rule)
}

func Delete(repoRoot, id, reason string) (model.Rule, error) {
	rule, err := LoadByID(repoRoot, id)
	if err != nil {
		return model.Rule{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return model.Rule{}, model.NewKATError(model.ExitCodeConfigError, "delete rule", fmt.Errorf("--reason is required"))
	}
	rule.Status = model.RuleStatusDisabled
	rule.DeletionReason = reason
	return writeRuleFile(repoRoot, rule.SourcePath, rule)
}

func TestRule(repoRoot, id, rawLogPath string, expectStart, expectEnd int) (model.RuleTestResult, error) {
	rule, err := LoadByID(repoRoot, id)
	if err != nil {
		return model.RuleTestResult{}, err
	}
	resolved := rawLogPath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(repoRoot, rawLogPath)
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return model.RuleTestResult{}, model.NewKATError(model.ExitCodeConfigError, "read raw log", err)
	}
	run := model.RunOutput{Status: model.RunStatusFailed}
	processed, err := extract.ProcessRules(raw, run, []model.Rule{rule})
	if err != nil {
		return model.RuleTestResult{}, err
	}
	result := model.RuleTestResult{RuleID: rule.ID, RawLogPath: resolved, ExpectedStartLine: expectStart, ExpectedEndLine: expectEnd, FailureCount: len(processed.Failures)}
	if len(processed.Failures) == 0 {
		return result, model.NewKATError(model.ExitCodeParserError, "test rule", fmt.Errorf("rule %q produced no failures", id))
	}
	if len(processed.Failures) > 1 {
		return result, model.NewKATError(model.ExitCodeParserError, "test rule", fmt.Errorf("rule %q overmatched %d failure spans", id, len(processed.Failures)))
	}
	first := processed.Failures[0]
	result.ActualStartLine = first.RawSpan.StartLine
	result.ActualEndLine = first.RawSpan.EndLine
	result.Signature = first.Signature
	result.Passed = first.RawSpan.StartLine == expectStart && first.RawSpan.EndLine == expectEnd
	if !result.Passed {
		return result, model.NewKATError(model.ExitCodeParserError, "test rule", fmt.Errorf("expected span %d:%d, got %d:%d", expectStart, expectEnd, first.RawSpan.StartLine, first.RawSpan.EndLine))
	}
	return result, nil
}

func Propose(repoRoot, lane, parser, rawLogPath string, startLine, endLine int) (model.RuleProposal, error) {
	return proposeAt(repoRoot, lane, parser, rawLogPath, startLine, endLine, time.Now().UTC())
}

func proposeAt(repoRoot, lane, parser, rawLogPath string, startLine, endLine int, now time.Time) (model.RuleProposal, error) {
	if strings.TrimSpace(lane) == "" {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeConfigError, "propose rule", fmt.Errorf("--lane is required"))
	}
	if strings.TrimSpace(parser) == "" {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeConfigError, "propose rule", fmt.Errorf("--parser is required"))
	}
	if !knownRuleParsers[parser] {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeConfigError, "propose rule", fmt.Errorf("unsupported parser label %q", parser))
	}
	if startLine <= 0 || endLine < startLine {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeConfigError, "propose rule", fmt.Errorf("invalid span %d:%d", startLine, endLine))
	}
	resolved := rawLogPath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(repoRoot, rawLogPath)
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeConfigError, "read raw log", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if endLine > len(lines) {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeConfigError, "propose rule", fmt.Errorf("span end line %d exceeds %d", endLine, len(lines)))
	}
	segment := lines[startLine-1 : endLine]
	startPattern := quoteFirstMeaningfulLine(segment)
	if startPattern == "" {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeConfigError, "propose rule", fmt.Errorf("span %d:%d does not contain a usable start line", startLine, endLine))
	}
	base := sanitizeRuleID(strings.TrimSuffix(filepath.Base(resolved), filepath.Ext(resolved)))
	if base == "" {
		base = "rule"
	}
	proposalID := fmt.Sprintf("%s-%s-%d-%d", parser, base, startLine, endLine)
	rule := model.Rule{
		ID:     proposalID,
		Lane:   lane,
		Parser: parser,
		Status: model.RuleStatusActive,
		Provenance: model.RuleProvenance{
			CreatedBy:       "kat-rules-propose",
			SourceRun:       inferSourceRun(resolved),
			SourceCommand:   inferSourceCommand(resolved),
			SourceLogSHA256: sha256String(raw),
			SourceSpan: model.RawSpan{
				StartLine: startLine,
				EndLine:   endLine,
			},
			Reason: fmt.Sprintf("Proposed from %s lines %d:%d", filepath.Base(resolved), startLine, endLine),
		},
		Match: model.RuleMatch{
			Start: model.RuleRegex{Regex: startPattern},
			End: model.RuleEnd{
				AnyOf:         []model.RuleRegex{{Regex: `^$`}},
				MaxBlockLines: minInt(160, maxInt(8, endLine-startLine+8)),
			},
			IncludeContext: model.RuleContext{Before: 1, After: 1},
		},
		Extract: model.RuleExtract{
			FileLine: model.RuleExtractField{Regex: `(?P<file>[^\s:]+\.[A-Za-z0-9]+):(?P<line>\d+)`},
			TestName: model.RuleExtractField{Regex: `^\s*[×✗-]\s+(?P<test>.+)$`},
		},
		Confidence: "medium",
	}
	if err := ValidateStoredRule(rule); err != nil {
		return model.RuleProposal{}, err
	}
	if err := safety.MkdirAllWithin(repoRoot, ProposedRulesDir(repoRoot), 0o755); err != nil {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeArtifactError, "create proposal directory", err)
	}
	data, err := yaml.Marshal(&rule)
	if err != nil {
		return model.RuleProposal{}, model.NewKATError(model.ExitCodeArtifactError, "marshal proposal rule", err)
	}
	path, err := writeUniqueProposal(repoRoot, proposalID, now, data)
	if err != nil {
		return model.RuleProposal{}, err
	}
	return model.RuleProposal{Rule: rule, Path: path}, nil
}

func writeUniqueProposal(repoRoot, proposalID string, now time.Time, data []byte) (string, error) {
	base := fmt.Sprintf("%s-%s", proposalID, now.UTC().Format("20060102t150405"))
	for sequence := 0; ; sequence++ {
		name := base
		if sequence > 0 {
			name = fmt.Sprintf("%s-%03d", base, sequence)
		}
		path := filepath.Join(ProposedRulesDir(repoRoot), name+".yaml")
		file, err := safety.OpenFileWithin(repoRoot, path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", model.NewKATError(model.ExitCodeArtifactError, "write proposal rule", err)
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return "", model.NewKATError(model.ExitCodeArtifactError, "write proposal rule", err)
		}
		return path, nil
	}
}

func ValidateStoredRule(rule model.Rule) error {
	if err := ValidateApplicable(rule); err != nil {
		return err
	}
	if strings.TrimSpace(rule.Provenance.CreatedBy) == "" || strings.TrimSpace(rule.Provenance.SourceRun) == "" || strings.TrimSpace(rule.Provenance.SourceCommand) == "" || strings.TrimSpace(rule.Provenance.SourceLogSHA256) == "" || strings.TrimSpace(rule.Provenance.Reason) == "" {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q must define provenance created_by, source_run, source_command, source_log_sha256, and reason", rule.ID))
	}
	if rule.Provenance.SourceSpan.StartLine <= 0 || rule.Provenance.SourceSpan.EndLine < rule.Provenance.SourceSpan.StartLine {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("rule %q must define valid provenance source_span", rule.ID))
	}
	if rule.Status == model.RuleStatusDisabled && strings.TrimSpace(rule.DeletionReason) == "" {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("disabled rule %q must define deletion_reason", rule.ID))
	}
	if rule.Status == model.RuleStatusActive && strings.TrimSpace(rule.DeletionReason) != "" {
		return model.NewKATError(model.ExitCodeConfigError, "validate rule file", fmt.Errorf("active rule %q must not define deletion_reason", rule.ID))
	}
	return nil
}

func readRuleFile(repoRoot, path string) (model.Rule, error) {
	data, err := safety.ReadFileWithin(repoRoot, path)
	if err != nil {
		return model.Rule{}, model.NewKATError(model.ExitCodeConfigError, "read rule file", err)
	}
	var rule model.Rule
	if err := safety.DecodeYAMLStrict(data, &rule); err != nil {
		return model.Rule{}, model.NewKATError(model.ExitCodeConfigError, "parse rule file", fmt.Errorf("%s: %w", path, err))
	}
	rule.SourcePath = path
	return rule, nil
}

func writeRuleFile(repoRoot, path string, rule model.Rule) (model.Rule, error) {
	if err := ValidateStoredRule(rule); err != nil {
		return model.Rule{}, err
	}
	if err := safety.MkdirAllWithin(repoRoot, filepath.Dir(path), 0o755); err != nil {
		return model.Rule{}, model.NewKATError(model.ExitCodeArtifactError, "create rule directory", err)
	}
	data, err := yaml.Marshal(&rule)
	if err != nil {
		return model.Rule{}, model.NewKATError(model.ExitCodeArtifactError, "marshal rule file", err)
	}
	if err := safety.WriteFileWithin(repoRoot, path, data, 0o644); err != nil {
		return model.Rule{}, model.NewKATError(model.ExitCodeArtifactError, "write rule file", err)
	}
	rule.SourcePath = path
	return rule, nil
}

func ensureRuleIDAvailable(repoRoot, id string) error {
	if err := safety.ValidateArtifactIdentifier("rule id", id); err != nil {
		return model.NewKATError(model.ExitCodeConfigError, "create rule", err)
	}
	path := filepath.Join(RulesDir(repoRoot), id+".yaml")
	if _, err := safety.StatWithin(repoRoot, path); err == nil {
		return model.NewKATError(model.ExitCodeConfigError, "create rule", fmt.Errorf("rule id %q already exists", id))
	} else if err != nil && !os.IsNotExist(err) {
		return model.NewKATError(model.ExitCodeConfigError, "create rule", err)
	}
	return nil
}

func quoteFirstMeaningfulLine(lines []string) string {
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		return "^" + regexp.QuoteMeta(strings.TrimSuffix(line, "\r")) + "$"
	}
	return ""
}

func sanitizeRuleID(value string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, value)
	return strings.Trim(mapped, "-")
}

func inferSourceRun(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.TrimSuffix(base, ".raw")
	base = sanitizeRuleID(base)
	if base == "" {
		return "local-log"
	}
	return "local-" + base
}

func inferSourceCommand(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.TrimSuffix(base, ".raw")
	base = sanitizeRuleID(base)
	if base == "" {
		return "unknown"
	}
	return base
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func sha256String(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
