package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kkachi-agent-tester/internal/model"
)

func TestBinaryConfiguredRunAndExcerpt(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
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

	runCmd := exec.Command(bin, "--repo", repo, "run", "unit")
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
	if !strings.Contains(string(runOut), "Summary: .kat/runs/") {
		t.Fatalf("expected run output to report artifact paths, got %q", string(runOut))
	}

	runsDir := filepath.Join(repo, ".kat", "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one run directory, got %d", len(entries))
	}
	runDir := filepath.Join(runsDir, entries[0].Name())
	summaryPath := filepath.Join(runDir, "unit.summary.json")
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
	if len(summary.Failures) != 1 || summary.Failures[0].ID != "F001" {
		t.Fatalf("expected one extracted failure, got %+v", summary.Failures)
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
	if err := os.Remove(summaryPath); err != nil {
		t.Fatalf("remove summary before summarize: %v", err)
	}
	if err := os.Remove(filepath.Join(runDir, "unit.summary.md")); err != nil {
		t.Fatalf("remove markdown before summarize: %v", err)
	}
	if err := os.Remove(filepath.Join(runDir, "unit.status.json")); err != nil {
		t.Fatalf("remove status before summarize: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(runDir, "excerpts")); err != nil {
		t.Fatalf("remove excerpts before summarize: %v", err)
	}

	summarizeCmd := exec.Command(bin, "--repo", repo, "summarize", filepath.ToSlash(filepath.Join(runDir, "unit.raw.log")))
	summarizeCmd.Dir = repo
	summarizeOut, err := summarizeCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected summarize command to succeed, err=%v output=%s", err, string(summarizeOut))
	}
	if !strings.Contains(string(summarizeOut), "Summary: .kat/runs/") {
		t.Fatalf("expected summarize output to report artifact paths, got %q", string(summarizeOut))
	}
	summaryData, err = os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("expected summarize to recreate summary: %v", err)
	}
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Status != model.RunStatusFailed || len(summary.Failures) != 1 {
		t.Fatalf("expected summarize to recreate failed summary with one failure, got %+v", summary)
	}
	excerptOut, err = exec.Command(bin, "--repo", repo, "excerpt", "--summary", filepath.ToSlash(summaryPath), "F001").CombinedOutput()
	if err != nil {
		t.Fatalf("expected excerpt after summarize to succeed, err=%v output=%s", err, string(excerptOut))
	}
	if !strings.Contains(string(excerptOut), "token=<redacted>") {
		t.Fatalf("expected redacted excerpt output after summarize, got %q", string(excerptOut))
	}
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kkachi-agent-tester")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/kkachi-agent-tester")
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
