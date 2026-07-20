package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
)

func TestConfiguredRunAndExcerpt(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"commands:",
		"  unit:",
		"    command: [\"sh\", \"test.sh\"]",
		"    lane: unit",
		"    parser: generic",
		"    timeout_sec: 10",
		"redaction:",
		"  patterns:",
		"    - name: token",
		"      regex: 'token=[^ ]+'",
		"      replace: 'token=<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho 'noise: start'\necho 'TypeError: token=secret failed'\necho 'src/foo.test.ts:42:13'\necho '✗ renders empty state'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "unit"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%s", exitCode, stderr.String())
	}
	katDir := filepath.Join(repo, ".kat", "runs")
	entries, err := os.ReadDir(katDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one run directory, err=%v entries=%d", err, len(entries))
	}
	runDir := filepath.Join(katDir, entries[0].Name())
	summaryJSONPath := filepath.Join(runDir, "unit.summary.json")
	rawLogPath := filepath.Join(runDir, "unit.raw.log")
	statusJSONPath := filepath.Join(runDir, "unit.status.json")
	summaryData, err := os.ReadFile(summaryJSONPath)
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
	if summary.ExtractorStatus == model.ExtractorStatusNoMatch {
		t.Fatalf("expected extracted failure, got %s", summary.ExtractorStatus)
	}
	if len(summary.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(summary.Failures))
	}
	if summary.Failures[0].Excerpt != "excerpts/F001.log" {
		t.Fatalf("expected summary-local excerpt reference, got %q", summary.Failures[0].Excerpt)
	}
	if strings.Contains(summary.Failures[0].Signature, "secret") {
		t.Fatalf("expected redacted failure signature, got %q", summary.Failures[0].Signature)
	}
	rawLog, err := os.ReadFile(rawLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawLog), "token=secret") {
		t.Fatalf("expected raw log to preserve original secret, got %q", string(rawLog))
	}
	statusData, err := os.ReadFile(statusJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	summaryFileBytes, err := os.ReadFile(summaryJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	summaryHash := sha256.Sum256(summaryFileBytes)
	expectedSummarySHA := "sha256:" + hex.EncodeToString(summaryHash[:])
	if status.SummarySHA256 != expectedSummarySHA {
		t.Fatalf("expected summary sha %s, got %s", expectedSummarySHA, status.SummarySHA256)
	}
	watcherInput := strings.Join([]string{
		status.CommandID,
		string(status.Status),
		"1",
		string(status.ExtractorStatus),
		status.RawLogSHA256,
		strings.Join(status.FailureSignatures, ","),
		strings.Join(status.WarningSignatures, ","),
		status.SummaryPath,
		status.RawLogPath,
	}, "\n")
	watcherHash := sha256.Sum256([]byte(watcherInput))
	expectedStatusHash := "sha256:" + hex.EncodeToString(watcherHash[:])
	if status.StatusHash != expectedStatusHash {
		t.Fatalf("expected status hash %s, got %s", expectedStatusHash, status.StatusHash)
	}
	if len(status.WarningSignatures) != 0 {
		t.Fatalf("expected no warnings, got %+v", status.WarningSignatures)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "excerpt", "--summary", filepath.ToSlash(summaryJSONPath), "F001"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected excerpt command to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "token=<redacted>") {
		t.Fatalf("expected redacted excerpt output, got %q", stdout.String())
	}
}

func TestUnsafeRunIDFailsBeforeCommandExecution(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeMarkerCommandConfig(t, repo, "unit")
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--run-id", "../escape", "run", "unit"}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) {
		t.Fatalf("expected config exit code, got %d stderr=%s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("expected command not to execute, stat error=%v", err)
	}
}

func TestUnsafeConfiguredCommandIDFailsBeforeExecution(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeMarkerCommandConfig(t, repo, "../unit")
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "../unit"}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) {
		t.Fatalf("expected config exit code, got %d stderr=%s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("expected command not to execute, stat error=%v", err)
	}
}

func TestExcerptRejectsUnsafeReferences(t *testing.T) {
	t.Parallel()
	for name, reference := range map[string]string{
		"absolute":  "/tmp/F001.log",
		"traversal": "excerpts/../../other/F001.log",
		"cross-run": "../run-b/excerpts/F001.log",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			summaryPath := writeExcerptSummary(t, repo, reference)
			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
			if exitCode != int(model.ExitCodeArtifactError) {
				t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
			}
		})
	}
}

func TestExcerptSymlinkContainment(t *testing.T) {
	t.Parallel()
	t.Run("cross-run rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		runA := filepath.Join(repo, ".kkachi", "runs", "run-a", "artifacts", "test")
		runB := filepath.Join(repo, ".kkachi", "runs", "run-b", "artifacts", "test")
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
		if err := os.Symlink(target, filepath.Join(runA, "excerpts", "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, runA, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
		}
		if strings.Contains(stdout.String(), "other-run-secret") {
			t.Fatalf("cross-run excerpt content leaked: %q", stdout.String())
		}
	})

	t.Run("dangling rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		excerptsDir := filepath.Join(repo, "excerpts")
		if err := os.MkdirAll(excerptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(excerptsDir, "missing.log"), filepath.Join(excerptsDir, "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, repo, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
		}
	})

	t.Run("external rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		excerptsDir := filepath.Join(repo, "excerpts")
		if err := os.MkdirAll(excerptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		external := filepath.Join(t.TempDir(), "F001.log")
		if err := os.WriteFile(external, []byte("outside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, filepath.Join(excerptsDir, "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, repo, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
		}
	})

	t.Run("internal allowed", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		excerptsDir := filepath.Join(repo, "excerpts")
		if err := os.MkdirAll(excerptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(excerptsDir, "actual.log")
		if err := os.WriteFile(target, []byte("inside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(excerptsDir, "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, repo, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != 0 {
			t.Fatalf("expected success, got %d stderr=%s", exitCode, stderr.String())
		}
		if stdout.String() != "inside\n" {
			t.Fatalf("unexpected excerpt %q", stdout.String())
		}
	})
}

func writeExcerptSummary(t *testing.T, dir, reference string) string {
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

func writeMarkerCommandConfig(t *testing.T, repo, commandID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"commands:",
		"  \"" + commandID + "\":",
		"    command: [\"sh\", \"touch-marker.sh\"]",
		"    lane: unit",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "touch-marker.sh"), []byte("#!/bin/sh\ntouch command-ran\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestSummarizeRawLogUsesConfigRedaction(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"redaction:",
		"  patterns:",
		"    - name: token",
		"      regex: 'token=[^ ]+'",
		"      replace: 'token=<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	rawDir := filepath.Join(repo, "fixtures")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rawLogPath := filepath.Join(rawDir, "unit.raw.log")
	rawText := strings.Join([]string{
		"noise: start",
		"TypeError: token=secret failed",
		"src/foo.test.ts:42:13",
		"✗ renders empty state",
		"",
	}, "\n")
	if err := os.WriteFile(rawLogPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "summarize", filepath.ToSlash(rawLogPath)}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected summarize command to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	summaryJSONPath := filepath.Join(rawDir, "unit.summary.json")
	summaryData, err := os.ReadFile(summaryJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != model.RunStatusFailed {
		t.Fatalf("expected failed summarized status, got %s", summary.Status)
	}
	if len(summary.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(summary.Failures))
	}
	if strings.Contains(summary.Failures[0].Signature, "secret") {
		t.Fatalf("expected redacted summarized failure signature, got %q", summary.Failures[0].Signature)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "excerpt", "--summary", filepath.ToSlash(summaryJSONPath), "F001"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected excerpt command to succeed after summarize, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "token=<redacted>") {
		t.Fatalf("expected redacted excerpt output after summarize, got %q", stdout.String())
	}
}

func TestAdHocRunWithoutConfig(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := "#!/bin/sh\necho 'ok'\nexit 0\n"
	if err := os.WriteFile(filepath.Join(repo, "ok.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "--lane", "unit", "sh", "ok.sh"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected ad-hoc run to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Status: passed") {
		t.Fatalf("expected pass output, got %q", stdout.String())
	}
}

func TestRawLogPersistsWhenExtractionFails(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"commands:",
		"  huge:",
		"    command: [\"sh\", \"huge.sh\"]",
		"    lane: unit",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\npython3 - <<'PY'\nprint('x' * 300000)\nPY\nexit 1\n"
	if err := os.WriteFile(filepath.Join(repo, "huge.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "huge"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected underlying failed exit code, got %d stderr=%s", exitCode, stderr.String())
	}
	runsDir := filepath.Join(repo, ".kat", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one run directory, err=%v entries=%d", err, len(entries))
	}
	runDir := filepath.Join(runsDir, entries[0].Name())
	rawLogPath := filepath.Join(runDir, "huge.raw.log")
	info, err := os.Stat(rawLogPath)
	if err != nil {
		t.Fatalf("expected raw log to persist despite extraction failure: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty raw log artifact")
	}
	summaryData, err := os.ReadFile(filepath.Join(runDir, "huge.summary.json"))
	if err != nil {
		t.Fatalf("expected summary json to be written with degraded extraction: %v", err)
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != model.RunStatusFailed || summary.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("expected failed/degraded summary, got status=%s extractor=%s", summary.Status, summary.ExtractorStatus)
	}
}

func TestConfiguredVitestRunUsesSpecializedParser(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 1",
		"commands:",
		"  unit:",
		"    command: [\"sh\", \"vitest.sh\"]",
		"    lane: unit",
		"    parser: vitest",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := strings.Join([]string{
		"#!/bin/sh",
		"echo ' RUN  v1.6.0 /repo'",
		"echo ''",
		"echo ' FAIL  src/foo.test.ts > renders empty state'",
		"echo ' AssertionError: expected false to be true'",
		"echo ' ❯ src/foo.ts:42:13'",
		"exit 1",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, "vitest.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "unit"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected vitest run to preserve exit code 1, got %d stderr=%s", exitCode, stderr.String())
	}
	runsDir := filepath.Join(repo, ".kat", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one run directory, err=%v entries=%d", err, len(entries))
	}
	summaryPath := filepath.Join(runsDir, entries[0].Name(), "unit.summary.json")
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Failures) != 1 {
		t.Fatalf("expected one specialized-parser failure, got %d", len(summary.Failures))
	}
	failure := summary.Failures[0]
	if failure.File != "src/foo.ts" || failure.Line != 42 || failure.TestName != "renders empty state" {
		t.Fatalf("expected vitest file/line/test capture, got %+v", failure)
	}
}
