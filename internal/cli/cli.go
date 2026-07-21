package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/irootkernel/manta/internal/artifacts"
	"github.com/irootkernel/manta/internal/config"
	"github.com/irootkernel/manta/internal/extract"
	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/rules"
	"github.com/irootkernel/manta/internal/runner"
	"github.com/irootkernel/manta/internal/safety"
)

type globalOptions struct {
	ConfigPath  string
	RepoRoot    string
	OutputDir   string
	RunID       string
	JSON        bool
	ShowVersion bool
}

type runResult struct {
	Command    string          `json:"command"`
	Status     model.RunStatus `json:"status"`
	ExitCode   int             `json:"exit_code"`
	DurationMS int64           `json:"duration_ms"`
	Summary    string          `json:"summary"`
	StatusJSON string          `json:"status_json"`
	RawLog     string          `json:"raw_log,omitempty"`
	Failures   int             `json:"failures"`
	Extractor  string          `json:"extractor"`
	diagnostic string
}

type materializationSource uint8

const (
	materializationExecutedCommand materializationSource = iota
	materializationSummarizedRaw
)

func Main(args []string, stdout, stderr io.Writer) int {
	return Run(args, stdout, stderr, NewBuildInfo("manta", "0.1.4", "unknown", "unknown"))
}

func Run(args []string, stdout, stderr io.Writer, info BuildInfo) int {
	opts, remaining, err := parseGlobalOptions(args)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if opts.ShowVersion {
		writeVersion(stdout, info, opts.JSON)
		return 0
	}
	if len(remaining) == 0 {
		writeLine(stderr, "usage: manta [global options] <version|run|excerpt|summarize|rules>")
		return int(model.ExitCodeConfigError)
	}
	if remaining[0] == "version" {
		return versionCommand(opts, remaining[1:], stdout, stderr, info)
	}

	repoRoot, err := resolveRepoRoot(opts.RepoRoot)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	opts.RepoRoot = repoRoot

	switch remaining[0] {
	case "run":
		return runCommand(opts, remaining[1:], stdout, stderr)
	case "excerpt":
		return excerptCommand(opts, remaining[1:], stdout, stderr)
	case "summarize":
		return summarizeCommand(opts, remaining[1:], stdout, stderr)
	case "rules":
		return rulesCommand(opts, remaining[1:], stdout, stderr)
	default:
		writef(stderr, "unknown command %q\n", remaining[0])
		return int(model.ExitCodeConfigError)
	}
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	var opts globalOptions
	fs := flag.NewFlagSet("global", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.ConfigPath, "config", "", "config override")
	fs.StringVar(&opts.RepoRoot, "repo", "", "repo root")
	fs.StringVar(&opts.OutputDir, "output-dir", "", "output dir")
	fs.StringVar(&opts.RunID, "run-id", "", "run id")
	fs.BoolVar(&opts.JSON, "json", false, "json output")
	fs.BoolVar(&opts.ShowVersion, "version", false, "show version")
	if err := fs.Parse(args); err != nil {
		return opts, nil, err
	}
	return opts, fs.Args(), nil
}

func versionCommand(opts globalOptions, args []string, stdout, stderr io.Writer, info BuildInfo) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonMode := opts.JSON
	fs.BoolVar(&jsonMode, "json", jsonMode, "json output")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if len(fs.Args()) != 0 {
		writeLine(stderr, "usage: manta version [--json]")
		return int(model.ExitCodeConfigError)
	}
	writeVersion(stdout, info, jsonMode)
	return 0
}

func runCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var lane string
	fs.StringVar(&lane, "lane", "", "lane")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	rest := fs.Args()
	req := model.RunRequest{
		RepoRoot:   opts.RepoRoot,
		ConfigPath: opts.ConfigPath,
		OutputDir:  opts.OutputDir,
		RunID:      opts.RunID,
		JSON:       opts.JSON,
	}
	if lane != "" {
		req.Mode = model.RunModeAdHoc
		req.Lane = lane
		req.CommandArgv = append([]string(nil), rest...)
		if len(req.CommandArgv) == 0 {
			writeLine(stderr, "ad-hoc run requires command after --")
			return int(model.ExitCodeConfigError)
		}
	} else {
		if len(rest) != 1 {
			writeLine(stderr, "configured run requires a command id")
			return int(model.ExitCodeConfigError)
		}
		req.Mode = model.RunModeConfigured
		req.CommandID = rest[0]
	}

	result, exitCode, err := executeRun(req)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if result.diagnostic != "" {
		writeLine(stderr, result.diagnostic)
	}
	if opts.JSON {
		data, _ := json.Marshal(result)
		writeLine(stdout, string(data))
	} else {
		printRunResult(stdout, result)
	}
	return exitCode
}

func summarizeCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("summarize", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	rest := fs.Args()
	if len(rest) != 1 {
		writeLine(stderr, "usage: manta summarize <raw-log>")
		return int(model.ExitCodeConfigError)
	}
	req := model.RunRequest{
		RepoRoot:   opts.RepoRoot,
		ConfigPath: opts.ConfigPath,
		OutputDir:  opts.OutputDir,
		RunID:      opts.RunID,
		JSON:       opts.JSON,
	}
	result, exitCode, err := executeSummarize(req, rest[0])
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if result.diagnostic != "" {
		writeLine(stderr, result.diagnostic)
	}
	if opts.JSON {
		data, _ := json.Marshal(result)
		writeLine(stdout, string(data))
	} else {
		printRunResult(stdout, result)
	}
	return exitCode
}

func executeRun(req model.RunRequest) (runResult, int, error) {
	allowMissing := req.Mode == model.RunModeAdHoc
	cfg, _, err := config.Load(req.RepoRoot, req.ConfigPath, allowMissing)
	if err != nil {
		return runResult{}, 0, err
	}

	var commandID, lane, parser string
	var argv []string
	var timeoutSec int
	if req.Mode == model.RunModeConfigured {
		cmd, ok := cfg.Commands[req.CommandID]
		if !ok {
			return runResult{}, 0, model.NewMantaError(model.ExitCodeConfigError, "resolve command", fmt.Errorf("unknown command id %q", req.CommandID))
		}
		commandID = req.CommandID
		lane = cmd.Lane
		parser = cmd.Parser
		argv = append([]string(nil), cmd.Command...)
		timeoutSec = cmd.TimeoutSec
	} else {
		if err := config.ValidateAdHocLane(req.Lane); err != nil {
			return runResult{}, 0, err
		}
		commandID = generatedCommandID(req.Lane)
		lane = req.Lane
		parser = "generic"
		argv = append([]string(nil), req.CommandArgv...)
		timeoutSec = 600
	}
	if err := config.ValidateParserLabel(parser); err != nil {
		return runResult{}, 0, err
	}

	applicableRules, err := rules.LoadApplicable(req.RepoRoot, lane, parser)
	if err != nil {
		return runResult{}, 0, err
	}
	paths, err := artifacts.PreparePaths(req.RepoRoot, req.OutputDir, req.RunID, commandID)
	if err != nil {
		return runResult{}, 0, err
	}
	rawFile, err := artifacts.OpenRawLog(paths)
	if err != nil {
		return runResult{}, 0, err
	}
	runOutput, runErr := runner.Execute(context.Background(), req.RepoRoot, commandID, lane, parser, argv, timeoutSec, rawFile)
	closeErr := rawFile.Close()
	if closeErr != nil {
		return runResult{}, 0, model.NewMantaError(model.ExitCodeArtifactError, "close raw log", closeErr)
	}
	if runErr != nil {
		return runResult{}, 0, runErr
	}
	if err := artifacts.ValidateRawLog(paths); err != nil {
		return runResult{}, 0, err
	}
	rawSHA := artifacts.SHA256(runOutput.RawLogBytes)
	relRaw := artifacts.Rel(req.RepoRoot, paths.RawLogPath)

	result, processed, err := materializeArtifacts(req, cfg, paths, rawSHA, relRaw, runOutput, applicableRules, materializationExecutedCommand)
	if err != nil {
		return runResult{}, 0, err
	}
	return result, exitCodeFromRun(processed), nil
}

func executeSummarize(req model.RunRequest, rawLogArg string) (runResult, int, error) {
	cfg, _, err := config.Load(req.RepoRoot, req.ConfigPath, true)
	if err != nil {
		return runResult{}, 0, err
	}
	resolved := rawLogArg
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(req.RepoRoot, rawLogArg)
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return runResult{}, 0, model.NewMantaError(model.ExitCodeConfigError, "read raw log", err)
	}
	commandID := summarizeCommandID(resolved)
	lane := commandID
	parser := "generic"
	if err := config.ValidateParserLabel(parser); err != nil {
		return runResult{}, 0, err
	}
	applicableRules, err := rules.LoadApplicable(req.RepoRoot, lane, parser)
	if err != nil {
		return runResult{}, 0, err
	}
	paths, err := artifacts.PreparePaths(req.RepoRoot, req.OutputDir, req.RunID, commandID)
	if err != nil {
		return runResult{}, 0, err
	}
	rawSHA, err := artifacts.WriteRawLog(paths, raw)
	if err != nil {
		return runResult{}, 0, err
	}
	relRaw := artifacts.Rel(req.RepoRoot, paths.RawLogPath)
	status, exitCode := inferSummarizeStatus(raw)
	runOutput := model.RunOutput{
		Metadata: model.RunMetadata{
			CommandID:   commandID,
			Lane:        lane,
			Parser:      parser,
			CommandArgv: []string{},
			ExitCode:    exitCode,
		},
		Status:      status,
		RawLogBytes: raw,
	}
	result, _, err := materializeArtifacts(req, cfg, paths, rawSHA, relRaw, runOutput, applicableRules, materializationSummarizedRaw)
	if err != nil {
		return runResult{}, 0, err
	}
	if result.Status == model.RunStatusInternalErr {
		return result, int(model.ExitCodeParserError), nil
	}
	return result, 0, nil
}

func materializeArtifacts(req model.RunRequest, cfg model.Config, paths model.ArtifactPaths, rawSHA, relRaw string, runOutput model.RunOutput, applicableRules []model.Rule, source materializationSource) (runResult, model.RunOutput, error) {
	runOutput, extractionErr := extract.Process(runOutput.RawLogBytes, runOutput, applicableRules)
	if extractionErr != nil {
		runOutput.Failures = nil
		runOutput.Warnings = nil
		runOutput.ExtractorStatus = model.ExtractorStatusDegraded
		if source == materializationSummarizedRaw {
			runOutput.Status = model.RunStatusInternalErr
			runOutput.Metadata.ExitCode = int(model.ExitCodeParserError)
		} else if runOutput.Status == model.RunStatusPassed {
			runOutput.Status = model.RunStatusInternalErr
		}
	}
	metadata := runOutput.Metadata
	redactor, err := safety.NewRedactor(cfg.Redaction.Patterns)
	if err != nil {
		return runResult{}, model.RunOutput{}, err
	}

	summary := model.Summary{
		Status:          runOutput.Status,
		CommandID:       metadata.CommandID,
		Lane:            metadata.Lane,
		Parser:          metadata.Parser,
		CommandArgv:     slices.Clone(metadata.CommandArgv),
		ExitCode:        metadata.ExitCode,
		StartedAt:       metadata.StartedAt,
		EndedAt:         metadata.EndedAt,
		DurationMS:      metadata.DurationMS,
		RawLog:          relRaw,
		RawLogSHA256:    rawSHA,
		ExtractorStatus: runOutput.ExtractorStatus,
		FailureCount:    len(runOutput.Failures),
		WarningCount:    len(runOutput.Warnings),
		Failures:        cloneFailures(runOutput.Failures),
		Warnings:        slices.Clone(runOutput.Warnings),
	}
	if err := writeExcerpts(redactor, cfg.NoiseFilters, paths, runOutput.RawLogBytes, &summary); err != nil {
		return runResult{}, model.RunOutput{}, err
	}
	redactSummary(&summary, redactor, cfg.NoiseFilters)
	summarySHA, err := artifacts.WriteSummaryJSON(paths, summary)
	if err != nil {
		return runResult{}, model.RunOutput{}, err
	}
	if err := artifacts.WriteSummaryMarkdown(paths, summary); err != nil {
		return runResult{}, model.RunOutput{}, err
	}
	statusDoc := model.Status{
		Status:            runOutput.Status,
		CommandID:         summary.CommandID,
		Lane:              summary.Lane,
		ExitCode:          metadata.ExitCode,
		ExtractorStatus:   runOutput.ExtractorStatus,
		SummaryPath:       artifacts.Rel(req.RepoRoot, paths.SummaryJSON),
		SummarySHA256:     summarySHA,
		RawLogPath:        relRaw,
		RawLogSHA256:      rawSHA,
		FailureSignatures: signatureHashes(summary.Failures),
		WarningSignatures: warningSignatureHashes(summary.Warnings),
		UpdatedAt:         time.Now().UTC(),
	}
	statusDoc.StatusHash = artifacts.ComputeStatusHash(statusDoc)
	if err := artifacts.WriteStatusJSON(paths, statusDoc); err != nil {
		return runResult{}, model.RunOutput{}, err
	}

	result := runResult{
		Command:    summary.CommandID,
		Status:     runOutput.Status,
		ExitCode:   metadata.ExitCode,
		DurationMS: metadata.DurationMS,
		Summary:    artifacts.Rel(req.RepoRoot, paths.SummaryMD),
		StatusJSON: artifacts.Rel(req.RepoRoot, paths.StatusJSON),
		RawLog:     relRaw,
		Failures:   len(summary.Failures),
		Extractor:  string(summary.ExtractorStatus),
	}
	if extractionErr != nil {
		result.diagnostic = safety.BoundBytes(redactor.Apply(extractionErr.Error()), safety.MaxExcerptBytes)
	}
	return result, runOutput, nil
}

func inferSummarizeStatus(raw []byte) (model.RunStatus, int) {
	text := string(raw)
	for _, marker := range []string{"Error:", "TypeError:", "ReferenceError:", "AssertionError:", "panic:", "Traceback", "FAIL", "FAILED", "✗"} {
		if strings.Contains(text, marker) {
			return model.RunStatusFailed, 1
		}
	}
	return model.RunStatusPassed, 0
}

func writeExcerpts(redactor safety.Redactor, noiseFilters []string, paths model.ArtifactPaths, raw []byte, summary *model.Summary) error {
	text := string(raw)
	for i := range summary.Failures {
		failure := &summary.Failures[i]
		content := excerptContent(text, failure.RawSpan)
		redacted := safety.FilterNoise(redactor.Apply(content), noiseFilters)
		redacted = safety.BoundBytes(redacted, safety.MaxExcerptBytes)
		if err := safety.ValidateArtifactIdentifier("failure id", failure.ID); err != nil {
			return model.NewMantaError(model.ExitCodeArtifactError, "write excerpt", err)
		}
		excerptPath := filepath.Join(paths.ExcerptsDir, failure.ID+".log")
		if err := artifacts.WriteExcerpt(paths, excerptPath, redacted); err != nil {
			return err
		}
		failure.Excerpt = filepath.ToSlash(filepath.Join("excerpts", failure.ID+".log"))
	}
	return nil
}

func redactSummary(summary *model.Summary, redactor safety.Redactor, noiseFilters []string) {
	summary.CommandID = redactor.Apply(summary.CommandID)
	summary.Lane = redactor.Apply(summary.Lane)
	summary.Parser = redactor.Apply(summary.Parser)
	for i := range summary.CommandArgv {
		summary.CommandArgv[i] = redactor.Apply(summary.CommandArgv[i])
	}
	redactEvidence := func(text string) string {
		return strings.TrimSpace(safety.FilterNoise(redactor.Apply(text), noiseFilters))
	}
	for i := range summary.Failures {
		failure := &summary.Failures[i]
		failure.Signature = redactEvidence(failure.Signature)
		if failure.TestName != "" {
			failure.TestName = redactEvidence(failure.TestName)
		}
		if failure.File != "" {
			failure.File = strings.TrimSpace(redactor.Apply(failure.File))
		}
		for j := range failure.StackTop {
			failure.StackTop[j] = strings.TrimSpace(redactor.Apply(failure.StackTop[j]))
		}
	}
	warnings := make([]model.Warning, 0, len(summary.Warnings))
	for _, warning := range summary.Warnings {
		redacted := redactEvidence(warning.Signature)
		if redacted == "" {
			continue
		}
		warning.Signature = redacted
		warnings = append(warnings, warning)
	}
	summary.Warnings = warnings
	summary.WarningCount = len(summary.Warnings)
	summary.FailureCount = len(summary.Failures)
}

func cloneFailures(failures []model.Failure) []model.Failure {
	cloned := slices.Clone(failures)
	for i := range cloned {
		cloned[i].StackTop = slices.Clone(failures[i].StackTop)
	}
	return cloned
}

func excerptContent(text string, span model.RawSpan) string {
	if span.StartByte < 0 || span.EndByte > len(text) || span.StartByte >= span.EndByte {
		return ""
	}
	return text[span.StartByte:span.EndByte]
}

func signatureHashes(failures []model.Failure) []string {
	out := make([]string, 0, len(failures))
	for _, failure := range failures {
		out = append(out, artifacts.SHA256([]byte(failure.Signature)))
	}
	sort.Strings(out)
	return out
}

func warningSignatureHashes(warnings []model.Warning) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, artifacts.SHA256([]byte(warning.Signature)))
	}
	sort.Strings(out)
	return out
}

func exitCodeFromRun(runOutput model.RunOutput) int {
	switch runOutput.Status {
	case model.RunStatusTimedOut:
		return int(model.ExitCodeTimeout)
	case model.RunStatusInternalErr:
		return int(model.ExitCodeParserError)
	default:
		if runOutput.Metadata.ExitCode != 0 {
			return runOutput.Metadata.ExitCode
		}
		return 0
	}
}

func printRunResult(w io.Writer, result runResult) {
	writeLine(w, "Manta run complete")
	writef(w, "Command: %s\n", result.Command)
	writef(w, "Status: %s\n", result.Status)
	writef(w, "Exit code: %d\n", result.ExitCode)
	writef(w, "Duration: %.1fs\n", float64(result.DurationMS)/1000)
	if result.Failures > 0 {
		writef(w, "Failures: %d\n", result.Failures)
	}
	writef(w, "Extractor: %s\n", result.Extractor)
	writef(w, "Summary: %s\n", result.Summary)
	writef(w, "Status JSON: %s\n", result.StatusJSON)
	if result.RawLog != "" {
		writef(w, "Raw log: %s\n", result.RawLog)
		writeLine(w, "Warning: raw logs may contain unredacted values; share cautiously.")
	}
}

func excerptCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("excerpt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var summaryPath string
	fs.StringVar(&summaryPath, "summary", "", "summary path")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	rest := fs.Args()
	if summaryPath == "" || len(rest) != 1 {
		writeLine(stderr, "usage: manta excerpt --summary <summary-path> <failure-id>")
		return int(model.ExitCodeConfigError)
	}
	resolved := summaryPath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(opts.RepoRoot, summaryPath)
	}
	canonicalSummary, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	data, err := os.ReadFile(canonicalSummary)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	var summary model.Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	failureID := rest[0]
	for _, failure := range summary.Failures {
		if failure.ID != failureID {
			continue
		}
		reference := failure.Excerpt
		if !isSafeExcerptReference(reference) {
			writef(stderr, "unsafe excerpt reference %q\n", reference)
			return int(model.ExitCodeArtifactError)
		}
		summaryDir := filepath.Dir(canonicalSummary)
		excerptsDir := filepath.Join(summaryDir, "excerpts")
		if err := safety.ValidateExistingPathWithin(summaryDir, excerptsDir); err != nil {
			writeLine(stderr, err)
			return int(model.ExitCodeArtifactError)
		}
		excerptPath := filepath.Join(summaryDir, filepath.FromSlash(reference))
		content, err := safety.ReadFileWithin(excerptsDir, excerptPath)
		if err != nil {
			writeLine(stderr, err)
			return int(model.ExitCodeArtifactError)
		}
		if opts.JSON {
			payload := map[string]any{"failure_id": failureID, "excerpt_path": reference, "content": string(content)}
			encoded, _ := json.Marshal(payload)
			writeLine(stdout, string(encoded))
		} else {
			writeString(stdout, string(content))
		}
		return 0
	}
	writef(stderr, "failure id %q not found in summary\n", failureID)
	return int(model.ExitCodeConfigError)
}

func isSafeExcerptReference(reference string) bool {
	native := filepath.FromSlash(reference)
	return reference != "" &&
		!filepath.IsAbs(native) &&
		filepath.ToSlash(filepath.Clean(native)) == reference &&
		strings.HasPrefix(reference, "excerpts/")
}

func writeVersion(w io.Writer, info BuildInfo, jsonMode bool) {
	if jsonMode {
		_ = json.NewEncoder(w).Encode(versionOutput{Name: info.Name, Version: info.Version})
		return
	}
	writef(w, "%s v%s\n", info.Name, info.Version)
}

func writeLine(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeString(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}
func resolveRepoRoot(repo string) (string, error) {
	if repo == "" {
		return os.Getwd()
	}
	if filepath.IsAbs(repo) {
		return repo, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, repo), nil
}

func generatedCommandID(lane string) string {
	clean := sanitizeIdentifier(lane)
	if clean == "" {
		clean = "command"
	}
	return fmt.Sprintf("%s-%s", clean, time.Now().UTC().Format("20060102t150405"))
}

func summarizeCommandID(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = strings.TrimSuffix(name, ".raw")
	clean := sanitizeIdentifier(name)
	if clean == "" {
		return "summarized"
	}
	return clean
}

func sanitizeIdentifier(value string) string {
	clean := strings.Map(func(r rune) rune {
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
	return strings.Trim(clean, "-")
}
