package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/manta/internal/artifacts"
	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

func TestMaterializeArtifactsExtractionErrorRetainsNonPassRunState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		status   model.RunStatus
		exitCode int
	}{
		{name: "failed", status: model.RunStatusFailed, exitCode: 7},
		{name: "timed-out", status: model.RunStatusTimedOut, exitCode: int(model.ExitCodeTimeout)},
		{name: "killed", status: model.RunStatusKilled, exitCode: 143},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			req := model.RunRequest{RepoRoot: repo, RunID: "contract-" + tt.name}
			paths, err := artifacts.PreparePaths(repo, "", req.RunID, tt.name)
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte(strings.Repeat("x", safety.MaxRegexInputBytes+1))
			rawSHA, err := artifacts.WriteRawLog(paths, raw)
			if err != nil {
				t.Fatal(err)
			}
			runOutput := model.RunOutput{
				Metadata: model.RunMetadata{
					CommandID: tt.name,
					Tags:      []string{"unit"},
					Parser:    "generic",
					ExitCode:  tt.exitCode,
				},
				Status:      tt.status,
				RawLogBytes: raw,
			}
			result, processed, err := materializeArtifacts(
				req,
				model.Config{},
				paths,
				rawSHA,
				artifacts.Rel(repo, paths.RawLogPath),
				runOutput,
				nil,
				materializationExecutedCommand,
			)
			if err != nil {
				t.Fatalf("materializeArtifacts failed: %v", err)
			}
			if processed.Status != tt.status || processed.Metadata.ExitCode != tt.exitCode {
				t.Fatalf("expected retained status/exit %s/%d, got %s/%d", tt.status, tt.exitCode, processed.Status, processed.Metadata.ExitCode)
			}
			if processed.ExtractorStatus != model.ExtractorStatusDegraded {
				t.Fatalf("expected degraded extraction, got %s", processed.ExtractorStatus)
			}
			if !strings.Contains(result.diagnostic, "regex input bound") {
				t.Fatalf("expected extraction diagnostic, got %q", result.diagnostic)
			}
			assertExtractionErrorArtifacts(t, paths.BaseDir, tt.name, tt.status, tt.exitCode)
		})
	}
}

func TestRunInternalErrorAfterPassedCommandMaterializesArtifacts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  huge-pass:",
		"    command: [\"sh\", \"huge-pass.sh\"]",
		"    tags: [unit]",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := []byte(strings.Repeat("x", safety.MaxRegexInputBytes+1))
	if err := os.WriteFile(filepath.Join(repo, "huge.raw.log"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat huge.raw.log\nexit 0\n"
	if err := os.WriteFile(filepath.Join(repo, "huge-pass.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--run-id", "passed-internal-error", "--json", "run", "huge-pass"}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeParserError) {
		t.Fatalf("expected Manta parser exit %d, got %d stderr=%s", model.ExitCodeParserError, exitCode, stderr.String())
	}
	var result runResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode run result %q: %v", stdout.String(), err)
	}
	if result.Status != model.RunStatusInternalErr || result.ExitCode != 0 || result.Extractor != string(model.ExtractorStatusDegraded) {
		t.Fatalf("expected materialized internal-error result with command exit 0, got %+v", result)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"diagnostic", "error"} {
		if _, ok := fields[field]; ok {
			t.Fatalf("internal diagnostic must not add JSON field %q", field)
		}
	}
	if !strings.Contains(stderr.String(), "regex input bound") {
		t.Fatalf("expected extraction diagnostic on stderr, got %q", stderr.String())
	}

	baseDir := filepath.Join(repo, ".manta", "runs", "scoped", "passed-internal-error", "artifacts", "test")
	assertExtractionErrorArtifacts(t, baseDir, "huge-pass", model.RunStatusInternalErr, 0)
}

func TestSummarizeInternalErrorMaterializesArtifacts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		raw              []byte
		inferredStatus   model.RunStatus
		inferredExitCode int
	}{
		{
			name:             "inferred-pass",
			raw:              bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1),
			inferredStatus:   model.RunStatusPassed,
			inferredExitCode: 0,
		},
		{
			name:             "inferred-failure",
			raw:              append([]byte("Error: inferred failure\n"), bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1)...),
			inferredStatus:   model.RunStatusFailed,
			inferredExitCode: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inferredStatus, inferredExitCode := inferSummarizeStatus(tt.raw)
			if inferredStatus != tt.inferredStatus || inferredExitCode != tt.inferredExitCode {
				t.Fatalf("unexpected summarize inference: status=%s exit=%d", inferredStatus, inferredExitCode)
			}

			repo := t.TempDir()
			rawPath := filepath.Join(repo, "unit.raw.log")
			if err := os.WriteFile(rawPath, tt.raw, 0o644); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "--run-id", "summarize-internal-error-" + tt.name, "summarize", rawPath}, &stdout, &stderr)
			if exitCode != int(model.ExitCodeParserError) {
				t.Fatalf("expected Manta parser exit %d, got %d stderr=%s", model.ExitCodeParserError, exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Status: internal_error") || !strings.Contains(stdout.String(), "Exit code: 4") {
				t.Fatalf("expected summarized internal-error result with exit 4, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "regex input bound") {
				t.Fatalf("expected extraction diagnostic on stderr, got %q", stderr.String())
			}

			baseDir := filepath.Join(repo, ".manta", "runs", "scoped", "summarize-internal-error-"+tt.name, "artifacts", "test")
			copiedRaw := assertExtractionErrorArtifacts(t, baseDir, "unit", model.RunStatusInternalErr, int(model.ExitCodeParserError))
			if !bytes.Equal(copiedRaw, tt.raw) {
				t.Fatal("expected summarize to preserve the original raw bytes")
			}
		})
	}
}

func assertExtractionErrorArtifacts(t *testing.T, baseDir, commandID string, wantStatus model.RunStatus, wantExitCode int) []byte {
	t.Helper()
	summaryPath := filepath.Join(baseDir, commandID+".summary.json")
	statusPath := filepath.Join(baseDir, commandID+".status.json")
	rawPath := filepath.Join(baseDir, commandID+".raw.log")
	for _, path := range []string{rawPath, summaryPath, filepath.Join(baseDir, commandID+".summary.md"), statusPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}

	var summary model.Summary
	readJSONArtifact(t, summaryPath, &summary)
	if summary.Status != wantStatus || summary.ExitCode != wantExitCode || summary.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("unexpected summary contract: status=%s exit=%d extractor=%s", summary.Status, summary.ExitCode, summary.ExtractorStatus)
	}
	if summary.FailureCount != 0 || summary.WarningCount != 0 || len(summary.Failures) != 0 || len(summary.Warnings) != 0 {
		t.Fatalf("expected empty compressed evidence after extraction error, got %+v", summary)
	}

	var status model.Status
	readJSONArtifact(t, statusPath, &status)
	if status.Status != wantStatus || status.ExitCode != wantExitCode || status.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("unexpected status contract: status=%s exit=%d extractor=%s", status.Status, status.ExitCode, status.ExtractorStatus)
	}
	if status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("status hash mismatch: got %q", status.StatusHash)
	}
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if status.SummarySHA256 != artifacts.SHA256(summaryData) {
		t.Fatalf("summary hash mismatch: got %q", status.SummarySHA256)
	}
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	wantRawSHA := artifacts.SHA256(raw)
	if summary.RawLogSHA256 != wantRawSHA || status.RawLogSHA256 != wantRawSHA {
		t.Fatalf("raw hash mismatch: summary=%q status=%q want=%q", summary.RawLogSHA256, status.RawLogSHA256, wantRawSHA)
	}
	return raw
}

func readJSONArtifact(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
