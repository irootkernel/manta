package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
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

	t.Run("valid run id writes Kkachi layout", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfig(t, repo, "#!/bin/sh\necho ok\n")
		cmd := exec.Command(bin, "--repo", repo, "--run-id", "run-001", "run", "unit")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("expected successful Kkachi run, err=%v output=%s", err, out)
		}
		base := filepath.Join(repo, ".kkachi", "runs", "run-001", "artifacts", "test")
		for _, name := range []string{"unit.raw.log", "unit.summary.json", "unit.summary.md", "unit.status.json"} {
			if _, err := os.Stat(filepath.Join(base, name)); err != nil {
				t.Fatalf("expected %s in Kkachi layout: %v", name, err)
			}
		}
		if info, err := os.Stat(filepath.Join(base, "excerpts")); err != nil || !info.IsDir() {
			t.Fatalf("expected Kkachi excerpts directory, info=%v err=%v", info, err)
		}
	})

	t.Run("Kkachi runs symlink escape is rejected", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfig(t, repo, "#!/bin/sh\necho ok\n")
		external := t.TempDir()
		if err := os.Symlink(external, filepath.Join(repo, ".kkachi", "runs")); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, "--repo", repo, "--run-id", "run-001", "run", "unit")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		requireExitCode(t, err, int(model.ExitCodeArtifactError), out)
		entries, err := os.ReadDir(external)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected no artifacts outside the repository, got %d entries", len(entries))
		}
	})

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
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi"), 0o755); err != nil {
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
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
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
