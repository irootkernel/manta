package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/manta/internal/artifacts"
	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

func TestMaterializeArtifactsExtractionErrorContract(t *testing.T) {
	t.Parallel()
	// Inject failures so the documented artifact contract remains covered independently of current extractor error triggers.
	tests := []struct {
		name         string
		status       model.RunStatus
		exitCode     int
		source       materializationSource
		wantStatus   model.RunStatus
		wantExitCode int
	}{
		{name: "failed", status: model.RunStatusFailed, exitCode: 7, source: materializationExecutedCommand, wantStatus: model.RunStatusFailed, wantExitCode: 7},
		{name: "timed-out", status: model.RunStatusTimedOut, exitCode: int(model.ExitCodeTimeout), source: materializationExecutedCommand, wantStatus: model.RunStatusTimedOut, wantExitCode: int(model.ExitCodeTimeout)},
		{name: "killed", status: model.RunStatusKilled, exitCode: 143, source: materializationExecutedCommand, wantStatus: model.RunStatusKilled, wantExitCode: 143},
		{name: "passed", status: model.RunStatusPassed, exitCode: 0, source: materializationExecutedCommand, wantStatus: model.RunStatusInternalErr, wantExitCode: 0},
		{name: "summarized", status: model.RunStatusPassed, exitCode: 0, source: materializationSummarizedRaw, wantStatus: model.RunStatusInternalErr, wantExitCode: int(model.ExitCodeParserError)},
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
			raw := []byte("raw evidence\n")
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
			result, processed, err := materializeArtifactsWithExtractor(
				req,
				model.Config{},
				paths,
				rawSHA,
				artifacts.Rel(repo, paths.RawLogPath),
				runOutput,
				nil,
				tt.source,
				func(_ []byte, output model.RunOutput, _ []model.Rule) (model.RunOutput, error) {
					return output, errors.New("forced extraction failure")
				},
			)
			if err != nil {
				t.Fatalf("materializeArtifacts failed: %v", err)
			}
			if processed.Status != tt.wantStatus || processed.Metadata.ExitCode != tt.wantExitCode {
				t.Fatalf("expected status/exit %s/%d, got %s/%d", tt.wantStatus, tt.wantExitCode, processed.Status, processed.Metadata.ExitCode)
			}
			if processed.ExtractorStatus != model.ExtractorStatusDegraded {
				t.Fatalf("expected degraded extraction, got %s", processed.ExtractorStatus)
			}
			if !strings.Contains(result.diagnostic, "forced extraction failure") {
				t.Fatalf("expected extraction diagnostic, got %q", result.diagnostic)
			}
			assertDegradedArtifacts(t, paths.BaseDir, tt.name, tt.wantStatus, tt.wantExitCode)
		})
	}
}

func TestOversizedPassingRunUsesBoundedExtraction(t *testing.T) {
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
	exitCode := Main([]string{"--repo", repo, "--run-id", "oversized-pass", "--json", "run", "huge-pass"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected passing command exit 0, got %d stderr=%s", exitCode, stderr.String())
	}
	var result runResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode run result %q: %v", stdout.String(), err)
	}
	if result.Status != model.RunStatusPassed || result.ExitCode != 0 || result.Extractor != string(model.ExtractorStatusDegraded) {
		t.Fatalf("expected passed/degraded result with command exit 0, got %+v", result)
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
	if stderr.Len() != 0 {
		t.Fatalf("expected no extraction diagnostic, got %q", stderr.String())
	}

	baseDir := filepath.Join(repo, ".manta", "runs", "scoped", "oversized-pass", "artifacts", "test")
	copiedRaw := assertDegradedArtifacts(t, baseDir, "huge-pass", model.RunStatusPassed, 0)
	if !bytes.Equal(copiedRaw, raw) {
		t.Fatal("expected run to preserve the original raw bytes")
	}
}

func TestOversizedSummarizeUsesBoundedExtraction(t *testing.T) {
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
			exitCode := Main([]string{"--repo", repo, "--run-id", "oversized-summarize-" + tt.name, "summarize", rawPath}, &stdout, &stderr)
			if exitCode != 0 {
				t.Fatalf("expected summarize exit 0, got %d stderr=%s", exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Status: "+string(tt.inferredStatus)) || !strings.Contains(stdout.String(), fmt.Sprintf("Exit code: %d", tt.inferredExitCode)) || !strings.Contains(stdout.String(), "Extractor: degraded") {
				t.Fatalf("expected bounded summarize result, got %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no extraction diagnostic, got %q", stderr.String())
			}

			baseDir := filepath.Join(repo, ".manta", "runs", "scoped", "oversized-summarize-"+tt.name, "artifacts", "test")
			copiedRaw := assertDegradedArtifacts(t, baseDir, "unit", tt.inferredStatus, tt.inferredExitCode)
			if !bytes.Equal(copiedRaw, tt.raw) {
				t.Fatal("expected summarize to preserve the original raw bytes")
			}
		})
	}
}

func assertDegradedArtifacts(t *testing.T, baseDir, commandID string, wantStatus model.RunStatus, wantExitCode int) []byte {
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
		t.Fatalf("expected empty degraded evidence, got %+v", summary)
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
