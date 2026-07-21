package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/rules"
	"github.com/irootkernel/manta/internal/safety"
)

func rulesCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeLine(stderr, "usage: manta rules <list|search|show|create|update|delete|test|propose>")
		return int(model.ExitCodeConfigError)
	}
	switch args[0] {
	case "list":
		return rulesListCommand(opts, args[1:], stdout, stderr)
	case "search":
		return rulesSearchCommand(opts, args[1:], stdout, stderr)
	case "show":
		return rulesShowCommand(opts, args[1:], stdout, stderr)
	case "create":
		return rulesCreateCommand(opts, args[1:], stdout, stderr)
	case "update":
		return rulesUpdateCommand(opts, args[1:], stdout, stderr)
	case "delete":
		return rulesDeleteCommand(opts, args[1:], stdout, stderr)
	case "test":
		return rulesTestCommand(opts, args[1:], stdout, stderr)
	case "propose":
		return rulesProposeCommand(opts, args[1:], stdout, stderr)
	default:
		writef(stderr, "unknown rules subcommand %q\n", args[0])
		return int(model.ExitCodeConfigError)
	}
}

func rulesListCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	loaded, err := rules.LoadAll(opts.RepoRoot)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(loaded)
		writeLine(stdout, string(data))
		return 0
	}
	for _, rule := range loaded {
		writeRuleLine(stdout, rule)
	}
	return 0
}

func rulesSearchCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeLine(stderr, "usage: manta rules search <query>")
		return int(model.ExitCodeConfigError)
	}
	loaded, err := rules.Search(opts.RepoRoot, args[0])
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(loaded)
		writeLine(stdout, string(data))
		return 0
	}
	for _, rule := range loaded {
		writeRuleLine(stdout, rule)
	}
	return 0
}

func rulesShowCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeLine(stderr, "usage: manta rules show <rule-id>")
		return int(model.ExitCodeConfigError)
	}
	rule, err := rules.LoadByID(opts.RepoRoot, args[0])
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(rule)
		writeLine(stdout, string(data))
		return 0
	}
	data, err := yaml.Marshal(&rule)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeArtifactError)
	}
	writeString(stdout, string(data))
	return 0
}

func rulesCreateCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var file string
	fs.StringVar(&file, "file", "", "rule yaml file")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if file == "" {
		writeLine(stderr, "usage: manta rules create --file <rule.yaml>")
		return int(model.ExitCodeConfigError)
	}
	rule, err := readRuleInput(file)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	created, err := rules.Create(opts.RepoRoot, rule)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	return writeRuleResponse(stdout, created, opts.JSON)
}

func rulesUpdateCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeLine(stderr, "usage: manta rules update <rule-id> --file <rule.yaml>")
		return int(model.ExitCodeConfigError)
	}
	ruleID := args[0]
	fs := flag.NewFlagSet("rules update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var file string
	fs.StringVar(&file, "file", "", "rule yaml file")
	if err := fs.Parse(args[1:]); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if file == "" || len(fs.Args()) != 0 {
		writeLine(stderr, "usage: manta rules update <rule-id> --file <rule.yaml>")
		return int(model.ExitCodeConfigError)
	}
	rule, err := readRuleInput(file)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	updated, err := rules.Update(opts.RepoRoot, ruleID, rule)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	return writeRuleResponse(stdout, updated, opts.JSON)
}

func rulesDeleteCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeLine(stderr, "usage: manta rules delete <rule-id> --reason <reason>")
		return int(model.ExitCodeConfigError)
	}
	ruleID := args[0]
	fs := flag.NewFlagSet("rules delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var reason string
	fs.StringVar(&reason, "reason", "", "deletion reason")
	if err := fs.Parse(args[1:]); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if strings.TrimSpace(reason) == "" || len(fs.Args()) != 0 {
		writeLine(stderr, "usage: manta rules delete <rule-id> --reason <reason>")
		return int(model.ExitCodeConfigError)
	}
	disabled, err := rules.Delete(opts.RepoRoot, ruleID, reason)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	return writeRuleResponse(stdout, disabled, opts.JSON)
}

func rulesTestCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var ruleID, rawLogPath, expectSpan string
	fs.StringVar(&ruleID, "rule", "", "rule id")
	fs.StringVar(&rawLogPath, "log", "", "raw log path")
	fs.StringVar(&expectSpan, "expect-span", "", "expected span start:end")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	start, end, err := parseSpan(expectSpan)
	if err != nil || ruleID == "" || rawLogPath == "" {
		writeLine(stderr, "usage: manta rules test --rule <rule-id> --log <raw-log> --expect-span <start:end>")
		return int(model.ExitCodeConfigError)
	}
	result, err := rules.TestRule(opts.RepoRoot, ruleID, rawLogPath, start, end)
	if opts.JSON {
		data, _ := json.Marshal(result)
		writeLine(stdout, string(data))
	}
	if err != nil {
		if !opts.JSON {
			writef(stdout, "FAIL %s expected=%d:%d actual=%d:%d signature=%s\n", result.RuleID, result.ExpectedStartLine, result.ExpectedEndLine, result.ActualStartLine, result.ActualEndLine, result.Signature)
		}
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if !opts.JSON {
		writef(stdout, "PASS %s expected=%d:%d actual=%d:%d signature=%s\n", result.RuleID, result.ExpectedStartLine, result.ExpectedEndLine, result.ActualStartLine, result.ActualEndLine, result.Signature)
	}
	return 0
}

func rulesProposeCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules propose", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var tags stringList
	var parser, rawLogPath, span string
	fs.Var(&tags, "tag", "tag (repeatable)")
	fs.StringVar(&parser, "parser", "", "parser")
	fs.StringVar(&rawLogPath, "raw-log", "", "raw log path")
	fs.StringVar(&span, "span", "", "source span start:end")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	start, end, err := parseSpan(span)
	if err != nil || len(tags) == 0 || parser == "" || rawLogPath == "" {
		writeLine(stderr, "usage: manta rules propose --tag <tag> [--tag <tag> ...] --parser <parser> --raw-log <raw-log> --span <start:end>")
		return int(model.ExitCodeConfigError)
	}
	proposal, err := rules.Propose(opts.RepoRoot, tags, parser, rawLogPath, start, end)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(proposal)
		writeLine(stdout, string(data))
		return 0
	}
	writef(stdout, "Proposed rule: %s\n", proposal.Rule.ID)
	writef(stdout, "Saved to: %s\n", filepath.ToSlash(proposal.Path))
	return 0
}

func readRuleInput(path string) (model.Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Rule{}, model.NewMantaError(model.ExitCodeConfigError, "read rule input", err)
	}
	var rule model.Rule
	if err := safety.DecodeYAMLStrict(data, &rule); err != nil {
		return model.Rule{}, model.NewMantaError(model.ExitCodeConfigError, "parse rule input", err)
	}
	return rule, nil
}

func writeRuleResponse(stdout io.Writer, rule model.Rule, jsonMode bool) int {
	if jsonMode {
		data, _ := json.Marshal(rule)
		writeLine(stdout, string(data))
		return 0
	}
	writeRuleLine(stdout, rule)
	return 0
}

func writeRuleLine(stdout io.Writer, rule model.Rule) {
	writef(stdout, "%s\t%s\t%s\t%s\t%s\n", rule.ID, rule.Status, rule.Parser, strings.Join(rule.Tags, ","), filepath.ToSlash(rule.SourcePath))
}

func parseSpan(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid span %q", value)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if start <= 0 || end < start {
		return 0, 0, fmt.Errorf("invalid span %q", value)
	}
	return start, end, nil
}
