package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/irootkernel/manta/internal/artifacts"
	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

type binaryRunResult struct {
	Command    string          `json:"command"`
	Status     model.RunStatus `json:"status"`
	ExitCode   int             `json:"exit_code"`
	Summary    string          `json:"summary"`
	StatusJSON string          `json:"status_json"`
	RawLog     string          `json:"raw_log"`
	Failures   int             `json:"failures"`
	Extractor  string          `json:"extractor"`
}

type binaryCommandOutput struct {
	output []byte
	err    error
}

func TestBinaryConfiguredRunAndExcerpt(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"commands:",
		"  command_secret_id:",
		"    command: [\"sh\", \"test.sh\", \"secret_arg\"]",
		"    lane: lane-secret_lane",
		"    parser: generic",
		"    timeout_sec: 10",
		"redaction:",
		"  patterns:",
		"    - name: token",
		"      regex: 'token=[^ ]+'",
		"      replace: 'token=<redacted>'",
		"    - name: secret",
		"      regex: 'secret_[a-z_]+'",
		"      replace: '<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$1\"\necho 'warning: secret_warning'\necho 'TypeError: token=secret secret_failure failed'\necho 'src/secret_path/foo.test.ts:42:13'\necho '✗ secret_test'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	runCmd := exec.Command(bin, "--repo", repo, "run", "command_secret_id")
	runCmd.Dir = repo
	runOut, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected configured run to exit non-zero, output=%s", string(runOut))
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit error, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected underlying exit code 1, got %d output=%s", exitErr.ExitCode(), string(runOut))
	}
	if !strings.Contains(string(runOut), "Command: command_<redacted>") || !strings.Contains(string(runOut), "Summary: .manta/runs/standalone/") {
		t.Fatalf("expected run output to report artifact paths, got %q", string(runOut))
	}
	if !strings.Contains(string(runOut), "command_secret_id.summary.md") {
		t.Fatalf("expected run output to retain literal artifact reference, got %q", string(runOut))
	}

	runsDir := filepath.Join(repo, ".manta", "runs", "standalone")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one run directory, got %d", len(entries))
	}
	runDir := filepath.Join(runsDir, entries[0].Name())
	summaryPath := filepath.Join(runDir, "command_secret_id.summary.json")
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != model.RunStatusFailed {
		t.Fatalf("expected failed status, got %s", summary.Status)
	}
	if summary.CommandID != "command_<redacted>" || summary.Lane != "lane-<redacted>" {
		t.Fatalf("expected redacted binary summary metadata, got command=%q lane=%q", summary.CommandID, summary.Lane)
	}
	if len(summary.CommandArgv) != 3 || summary.CommandArgv[2] != "<redacted>" {
		t.Fatalf("expected redacted binary argv, got %+v", summary.CommandArgv)
	}
	if len(summary.Failures) == 0 || len(summary.Warnings) != 1 {
		t.Fatalf("expected extracted failures and warning, got failures=%+v warnings=%+v", summary.Failures, summary.Warnings)
	}
	for _, failure := range summary.Failures {
		for _, value := range []string{failure.Signature, failure.File, failure.TestName, strings.Join(failure.StackTop, "\n")} {
			if strings.Contains(value, "secret_") {
				t.Fatalf("expected redacted binary failure metadata, got %q", value)
			}
		}
	}
	if strings.Contains(summary.Warnings[0].Signature, "secret_") {
		t.Fatalf("expected redacted binary warning, got %q", summary.Warnings[0].Signature)
	}
	statusData, err := os.ReadFile(filepath.Join(runDir, "command_secret_id.status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	if status.CommandID != summary.CommandID || status.Lane != summary.Lane || status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("expected redacted, self-consistent binary status, got %+v", status)
	}
	rawData, err := os.ReadFile(filepath.Join(runDir, "command_secret_id.raw.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret_arg", "secret_warning", "secret_failure", "secret_path", "secret_test"} {
		if !strings.Contains(string(rawData), secret) {
			t.Fatalf("expected binary raw log to preserve %q, got %q", secret, rawData)
		}
	}

	excerptCmd := exec.Command(bin, "--repo", repo, "excerpt", "--summary", filepath.ToSlash(summaryPath), "F001")
	excerptCmd.Dir = repo
	excerptOut, err := excerptCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected excerpt command to succeed, err=%v output=%s", err, string(excerptOut))
	}
	if !strings.Contains(string(excerptOut), "token=<redacted>") {
		t.Fatalf("expected redacted excerpt output, got %q", string(excerptOut))
	}
	if strings.Contains(string(excerptOut), "secret_") {
		t.Fatalf("expected excerpt metadata to be redacted, got %q", string(excerptOut))
	}
	if err := os.Remove(summaryPath); err != nil {
		t.Fatalf("remove summary before summarize: %v", err)
	}
	if err := os.Remove(filepath.Join(runDir, "command_secret_id.summary.md")); err != nil {
		t.Fatalf("remove markdown before summarize: %v", err)
	}
	if err := os.Remove(filepath.Join(runDir, "command_secret_id.status.json")); err != nil {
		t.Fatalf("remove status before summarize: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(runDir, "excerpts")); err != nil {
		t.Fatalf("remove excerpts before summarize: %v", err)
	}

	summarizeCmd := exec.Command(bin, "--repo", repo, "summarize", filepath.ToSlash(filepath.Join(runDir, "command_secret_id.raw.log")))
	summarizeCmd.Dir = repo
	summarizeOut, err := summarizeCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected summarize command to succeed, err=%v output=%s", err, string(summarizeOut))
	}
	if !strings.Contains(string(summarizeOut), "Command: command_<redacted>") || !strings.Contains(string(summarizeOut), "Summary: .manta/runs/standalone/") {
		t.Fatalf("expected summarize output to report artifact paths, got %q", string(summarizeOut))
	}
	entries, err = os.ReadDir(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected summarize to allocate a second run directory, got %d", len(entries))
	}
	var summarizeDir string
	for _, entry := range entries {
		candidate := filepath.Join(runsDir, entry.Name())
		if candidate != runDir {
			summarizeDir = candidate
			break
		}
	}
	if summarizeDir == "" {
		t.Fatal("expected a distinct summarize directory")
	}
	summaryPath = filepath.Join(summarizeDir, "command_secret_id.summary.json")
	summaryData, err = os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("expected summarize to create summary in a new run: %v", err)
	}
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != model.RunStatusFailed || len(summary.Failures) == 0 {
		t.Fatalf("expected summarize to create failed summary with failures, got %+v", summary)
	}
	if summary.CommandID != "command_<redacted>" || summary.Lane != "command_<redacted>" {
		t.Fatalf("expected redacted binary summarize metadata, got command=%q lane=%q", summary.CommandID, summary.Lane)
	}
	excerptOut, err = exec.Command(bin, "--repo", repo, "excerpt", "--summary", filepath.ToSlash(summaryPath), "F001").CombinedOutput()
	if err != nil {
		t.Fatalf("expected excerpt after summarize to succeed, err=%v output=%s", err, string(excerptOut))
	}
	if !strings.Contains(string(excerptOut), "token=<redacted>") {
		t.Fatalf("expected redacted excerpt output after summarize, got %q", string(excerptOut))
	}
	if strings.Contains(string(excerptOut), "secret_") {
		t.Fatalf("expected summarized excerpt metadata to be redacted, got %q", string(excerptOut))
	}
}

func TestBinaryJSONRedactsCommandMetadata(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"commands:",
		"  json_secret_id:",
		"    command: [\"sh\", \"test.sh\", \"secret_arg\"]",
		"    lane: lane-secret_lane",
		"    parser: generic",
		"    timeout_sec: 10",
		"redaction:",
		"  patterns:",
		"    - name: secret",
		"      regex: 'secret_[a-z_]+'",
		"      replace: '<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte("#!/bin/sh\nprintf '%s\\n' \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := runBinaryJSON(t, bin, repo, "run", "json_secret_id")
	if result.Command != "json_<redacted>" {
		t.Fatalf("expected redacted binary JSON command, got %q", result.Command)
	}
	for _, path := range []string{result.Summary, result.StatusJSON, result.RawLog} {
		if !strings.Contains(path, "json_secret_id") {
			t.Fatalf("expected literal binary JSON artifact reference, got %q", path)
		}
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected binary JSON artifact reference %q to resolve: %v", path, err)
		}
	}
}

func TestBinaryExtractionContracts(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)

	t.Run("specialized parser miss does not use generic fallback", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfigWithParser(t, repo, "vitest", "#!/bin/sh\necho 'TypeError: generic-looking failure'\necho 'src/foo.test.ts:42:13'\nexit 7\n")

		result, stderr := runBinaryJSONWithExit(t, bin, repo, 7, "run", "unit")
		if stderr != "" {
			t.Fatalf("expected no diagnostic for a parser miss, got %q", stderr)
		}
		summary, status, _ := loadBinaryRunArtifacts(t, repo, result)
		assertBinaryExtractionContract(t, result, summary, status, model.RunStatusFailed, 7)
	})

	t.Run("passing command extraction error preserves command exit", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfig(t, repo, "#!/bin/sh\ncat huge.raw.log\nexit 0\n")
		raw := bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1)
		if err := os.WriteFile(filepath.Join(repo, "huge.raw.log"), raw, 0o644); err != nil {
			t.Fatal(err)
		}

		result, stderr := runBinaryJSONWithExit(t, bin, repo, int(model.ExitCodeParserError), "run", "unit")
		if !strings.Contains(stderr, "regex input bound") {
			t.Fatalf("expected bounded extraction diagnostic, got %q", stderr)
		}
		summary, status, copiedRaw := loadBinaryRunArtifacts(t, repo, result)
		if !bytes.Equal(copiedRaw, raw) {
			t.Fatal("passing-command internal error did not preserve raw evidence")
		}
		assertBinaryExtractionContract(t, result, summary, status, model.RunStatusInternalErr, 0)
		assertBinaryMarkdownContract(t, repo, result, "internal_error", "0", "degraded")
	})

	t.Run("summarize extraction error materializes internal error", func(t *testing.T) {
		repo := t.TempDir()
		raw := bytes.Repeat([]byte("y"), safety.MaxRegexInputBytes+1)
		rawPath := filepath.Join(repo, "unit.raw.log")
		if err := os.WriteFile(rawPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}

		result, stderr := runBinaryJSONWithExit(t, bin, repo, int(model.ExitCodeParserError), "summarize", filepath.ToSlash(rawPath))
		if !strings.Contains(stderr, "regex input bound") {
			t.Fatalf("expected bounded summarize diagnostic, got %q", stderr)
		}
		summary, status, copiedRaw := loadBinaryRunArtifacts(t, repo, result)
		if !bytes.Equal(copiedRaw, raw) {
			t.Fatal("summarize internal error did not preserve copied raw evidence")
		}
		assertBinaryExtractionContract(t, result, summary, status, model.RunStatusInternalErr, int(model.ExitCodeParserError))
		assertBinaryMarkdownContract(t, repo, result, "internal_error", "4", "degraded")
	})
}

func TestBinaryStandaloneCollisionResistance(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	writeE2EConfig(t, repo, "#!/bin/sh\nprintf 'configured\\n'\n")
	if err := os.WriteFile(filepath.Join(repo, "adhoc.sh"), []byte("#!/bin/sh\nprintf 'adhoc\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	const concurrentRuns = 8
	start := make(chan struct{})
	outputs := make(chan binaryCommandOutput, concurrentRuns)
	for range concurrentRuns {
		go func() {
			<-start
			cmd := exec.Command(bin, "--repo", repo, "--json", "run", "unit")
			cmd.Dir = repo
			output, err := cmd.CombinedOutput()
			outputs <- binaryCommandOutput{output: output, err: err}
		}()
	}
	close(start)

	results := make([]binaryRunResult, 0, concurrentRuns+4)
	for range concurrentRuns {
		commandOutput := <-outputs
		if commandOutput.err != nil {
			t.Fatalf("concurrent configured run failed: %v output=%s", commandOutput.err, commandOutput.output)
		}
		results = append(results, decodeBinaryRunResult(t, commandOutput.output))
	}
	results = append(results,
		runBinaryJSON(t, bin, repo, "run", "--lane", "unit", "sh", "adhoc.sh"),
		runBinaryJSON(t, bin, repo, "run", "--lane", "unit", "sh", "adhoc.sh"),
	)
	assertDistinctBinaryRunDirectories(t, repo, results)
	for _, result := range results {
		snapshotBinaryRunArtifacts(t, repo, result, false)
	}

	rawLogPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "noise: start\nTypeError: failed\nsrc/foo.test.ts:42:13\n✗ renders empty state\n"
	if err := os.WriteFile(rawLogPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	firstSummarize := runBinaryJSON(t, bin, repo, "summarize", filepath.ToSlash(rawLogPath))
	firstSnapshot := snapshotBinaryRunArtifacts(t, repo, firstSummarize, true)
	secondSummarize := runBinaryJSON(t, bin, repo, "summarize", filepath.ToSlash(rawLogPath))
	snapshotBinaryRunArtifacts(t, repo, secondSummarize, true)
	assertDistinctBinaryRunDirectories(t, repo, append(results, firstSummarize, secondSummarize))
	for path, want := range firstSnapshot {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("re-read binary artifact %q: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("binary artifact changed at %q", path)
		}
	}
	sourceRaw, err := os.ReadFile(rawLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceRaw) != rawText {
		t.Fatalf("source raw log changed: want %q got %q", rawText, sourceRaw)
	}
}

func TestBinaryArtifactContainment(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)

	t.Run("unsafe run id fails before execution", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
		cmd := exec.Command(bin, "--repo", repo, "--run-id", "../escape", "run", "unit")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		requireExitCode(t, err, int(model.ExitCodeConfigError), out)
		if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
			t.Fatalf("expected command not to execute, stat error=%v", err)
		}
	})

	t.Run("valid run id writes fixed layout", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfig(t, repo, "#!/bin/sh\necho ok\n")
		cmd := exec.Command(bin, "--repo", repo, "--run-id", "run-001", "run", "unit")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected successful run-scoped execution, err=%v output=%s", err, out)
		}
		base := filepath.Join(repo, ".manta", "runs", "scoped", "run-001", "artifacts", "test")
		for _, name := range []string{"unit.raw.log", "unit.summary.json", "unit.summary.md", "unit.status.json"} {
			if _, err := os.Stat(filepath.Join(base, name)); err != nil {
				t.Fatalf("expected %s in run-scoped layout: %v", name, err)
			}
		}
		if info, err := os.Stat(filepath.Join(base, "excerpts")); err != nil || !info.IsDir() {
			t.Fatalf("expected run-scoped excerpts directory, info=%v err=%v", info, err)
		}
	})

	for _, test := range []struct {
		name    string
		runsDir string
		runID   string
	}{
		{name: "standalone runs", runsDir: filepath.Join(".manta", "runs", "standalone")},
		{name: "run-id artifacts", runsDir: filepath.Join(".manta", "runs", "scoped"), runID: "run-001"},
	} {
		t.Run(test.name+" symlink escape is rejected", func(t *testing.T) {
			repo := t.TempDir()
			writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
			runsPath := filepath.Join(repo, test.runsDir)
			if err := os.MkdirAll(filepath.Dir(runsPath), 0o755); err != nil {
				t.Fatal(err)
			}
			external := t.TempDir()
			if err := os.Symlink(external, runsPath); err != nil {
				t.Fatal(err)
			}
			args := []string{"--repo", repo}
			if test.runID != "" {
				args = append(args, "--run-id", test.runID)
			}
			args = append(args, "run", "unit")
			cmd := exec.Command(bin, args...)
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			requireExitCode(t, err, int(model.ExitCodeArtifactError), out)
			if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
				t.Fatalf("expected command not to execute, stat error=%v", err)
			}
			entries, err := os.ReadDir(external)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("expected no artifacts outside the repository, got %d entries", len(entries))
			}
		})
	}

	for _, test := range []struct {
		name      string
		reference string
		symlink   bool
	}{
		{name: "traversal excerpt", reference: "excerpts/../../run-b/excerpts/F001.log"},
		{name: "cross-run excerpt symlink", reference: "excerpts/F001.log", symlink: true},
	} {
		t.Run(test.name+" is rejected", func(t *testing.T) {
			repo := t.TempDir()
			runA := filepath.Join(repo, ".manta", "runs", "scoped", "run-a", "artifacts", "test")
			runB := filepath.Join(repo, ".manta", "runs", "scoped", "run-b", "artifacts", "test")
			if err := os.MkdirAll(filepath.Join(runA, "excerpts"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(runB, "excerpts"), 0o755); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(runB, "excerpts", "F001.log")
			if err := os.WriteFile(target, []byte("other-run-secret\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if test.symlink {
				if err := os.Symlink(target, filepath.Join(runA, "excerpts", "F001.log")); err != nil {
					t.Fatal(err)
				}
			}
			summaryPath := writeE2ESummary(t, runA, test.reference)
			cmd := exec.Command(bin, "--repo", repo, "excerpt", "--summary", summaryPath, "F001")
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			requireExitCode(t, err, int(model.ExitCodeArtifactError), out)
			if strings.Contains(string(out), "other-run-secret") {
				t.Fatalf("cross-run excerpt content leaked: %q", out)
			}
		})
	}
}

func writeE2EConfig(t *testing.T, repo, script string) {
	t.Helper()
	writeE2EConfigWithParser(t, repo, "generic", script)
}

func writeE2EConfigWithParser(t *testing.T, repo, parser, script string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"commands:",
		"  unit:",
		"    command: [\"sh\", \"test.sh\"]",
		"    lane: unit",
		"    parser: " + parser,
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runBinaryJSON(t *testing.T, bin, repo string, args ...string) binaryRunResult {
	t.Helper()
	result, stderr := runBinaryJSONWithExit(t, bin, repo, 0, args...)
	if stderr != "" {
		t.Fatalf("expected no binary diagnostic, got %q", stderr)
	}
	return result
}

func runBinaryJSONWithExit(t *testing.T, bin, repo string, expectedExit int, args ...string) (binaryRunResult, string) {
	t.Helper()
	commandArgs := append([]string{"--repo", repo, "--json"}, args...)
	cmd := exec.Command(bin, commandArgs...)
	cmd.Dir = repo
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if expectedExit == 0 {
		if err != nil {
			t.Fatalf("binary command %q failed: %v stdout=%s stderr=%s", commandArgs, err, stdout.String(), stderr.String())
		}
	} else {
		output := append([]byte(nil), stdout.Bytes()...)
		output = append(output, stderr.Bytes()...)
		requireExitCode(t, err, expectedExit, output)
	}
	return decodeBinaryRunResult(t, stdout.Bytes()), stderr.String()
}

func decodeBinaryRunResult(t *testing.T, output []byte) binaryRunResult {
	t.Helper()
	var result binaryRunResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode binary result %q: %v", output, err)
	}
	return result
}

func assertDistinctBinaryRunDirectories(t *testing.T, repo string, results []binaryRunResult) {
	t.Helper()
	seen := make(map[string]bool, len(results))
	for _, result := range results {
		baseDir := filepath.Dir(filepath.Join(repo, filepath.FromSlash(result.RawLog)))
		if seen[baseDir] {
			t.Fatalf("binary operations reused standalone run directory %q", baseDir)
		}
		seen[baseDir] = true
	}
}

func snapshotBinaryRunArtifacts(t *testing.T, repo string, result binaryRunResult, requireExcerpt bool) map[string][]byte {
	t.Helper()
	summary, status, _ := loadBinaryRunArtifacts(t, repo, result)
	statusPath := filepath.Join(repo, filepath.FromSlash(result.StatusJSON))
	summaryJSONPath := filepath.Join(repo, filepath.FromSlash(status.SummaryPath))
	rawPath := filepath.Join(repo, filepath.FromSlash(result.RawLog))

	paths := []string{
		rawPath,
		summaryJSONPath,
		filepath.Join(repo, filepath.FromSlash(result.Summary)),
		statusPath,
	}
	for _, failure := range summary.Failures {
		paths = append(paths, filepath.Join(filepath.Dir(summaryJSONPath), filepath.FromSlash(failure.Excerpt)))
	}
	if requireExcerpt && len(summary.Failures) == 0 {
		t.Fatal("expected summarized failure excerpt")
	}
	snapshot := make(map[string][]byte, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read binary artifact %q: %v", path, err)
		}
		snapshot[path] = data
	}
	return snapshot
}

func loadBinaryRunArtifacts(t *testing.T, repo string, result binaryRunResult) (model.Summary, model.Status, []byte) {
	t.Helper()
	statusPath := filepath.Join(repo, filepath.FromSlash(result.StatusJSON))
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	summaryPath := filepath.Join(repo, filepath.FromSlash(status.SummaryPath))
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(repo, filepath.FromSlash(result.RawLog))
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	wantRawSHA := artifacts.SHA256(raw)
	if summary.RawLogSHA256 != wantRawSHA || status.RawLogSHA256 != wantRawSHA {
		t.Fatalf("raw checksum mismatch: summary=%q status=%q want=%q", summary.RawLogSHA256, status.RawLogSHA256, wantRawSHA)
	}
	if status.SummarySHA256 != artifacts.SHA256(summaryData) {
		t.Fatalf("summary checksum mismatch: got=%q want=%q", status.SummarySHA256, artifacts.SHA256(summaryData))
	}
	if status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("status hash mismatch: got %q", status.StatusHash)
	}
	return summary, status, raw
}

func assertBinaryExtractionContract(t *testing.T, result binaryRunResult, summary model.Summary, status model.Status, wantStatus model.RunStatus, wantExitCode int) {
	t.Helper()
	if result.Status != wantStatus || result.ExitCode != wantExitCode || result.Extractor != string(model.ExtractorStatusDegraded) || result.Failures != 0 {
		t.Fatalf("unexpected binary result: %+v", result)
	}
	if summary.Status != wantStatus || summary.ExitCode != wantExitCode || summary.ExtractorStatus != model.ExtractorStatusDegraded || summary.FailureCount != 0 || summary.WarningCount != 0 {
		t.Fatalf("unexpected binary summary: %+v", summary)
	}
	if status.Status != wantStatus || status.ExitCode != wantExitCode || status.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("unexpected binary status: %+v", status)
	}
}

func assertBinaryMarkdownContract(t *testing.T, repo string, result binaryRunResult, wantStatus, wantExit, wantExtractor string) {
	t.Helper()
	markdown, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.Summary)))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Status: " + wantStatus, "Exit code: " + wantExit, "Extractor: " + wantExtractor} {
		if !strings.Contains(string(markdown), want) {
			t.Fatalf("expected Markdown summary to contain %q, got %q", want, markdown)
		}
	}
}

func writeE2ESummary(t *testing.T, dir, reference string) string {
	t.Helper()
	summary := model.Summary{Failures: []model.Failure{{ID: "F001", Excerpt: reference}}}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "unit.summary.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireExitCode(t *testing.T, err error, expected int, output []byte) {
	t.Helper()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected exit code %d, err=%v output=%s", expected, err, output)
	}
	if exitErr.ExitCode() != expected {
		t.Fatalf("expected exit code %d, got %d output=%s", expected, exitErr.ExitCode(), output)
	}
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "manta")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/manta")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(out))
	}
	return bin
}

func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(file))
}
