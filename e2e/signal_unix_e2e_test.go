//go:build unix

package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/irootkernel/manta/internal/artifacts"
	"github.com/irootkernel/manta/internal/model"
)

func TestBinaryPreservesInterruptedEvidence(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)

	for _, test := range []struct {
		name     string
		signal   syscall.Signal
		exitCode int
		runID    string
	}{
		{name: "sigint standalone", signal: syscall.SIGINT, exitCode: 130},
		{name: "sigint run id", signal: syscall.SIGINT, exitCode: 130, runID: "int-run"},
		{name: "sigterm standalone", signal: syscall.SIGTERM, exitCode: 143},
		{name: "sigterm run id", signal: syscall.SIGTERM, exitCode: 143, runID: "term-run"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			script := strings.Join([]string{
				"#!/bin/sh",
				"sleep 30 &",
				"child=$!",
				"echo \"$child\" > child.pid",
				"trap 'echo interrupted; exit 0' INT TERM",
				"echo started",
				"while :; do sleep 1; done",
			}, "\n") + "\n"
			writeE2EConfig(t, repo, script)

			args := []string{"--repo", repo}
			if test.runID != "" {
				args = append(args, "--run-id", test.runID)
			}
			args = append(args, "run", "unit")
			cmd := exec.Command(bin, args...)
			cmd.Dir = repo
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}

			rawPath := waitForInterruptedRaw(t, repo, test.runID)
			waitForRawMarker(t, rawPath, "started\n")
			if err := cmd.Process.Signal(test.signal); err != nil {
				t.Fatalf("send %s: %v", test.signal, err)
			}
			err := cmd.Wait()
			requireExitCode(t, err, test.exitCode, append(stdout.Bytes(), stderr.Bytes()...))

			raw, err := os.ReadFile(rawPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "started\n") || !strings.Contains(string(raw), "interrupted\n") {
				t.Fatalf("expected partial raw evidence after %s, got %q", test.signal, raw)
			}
			if strings.Contains(string(raw), "finished") {
				t.Fatalf("unexpected completion marker in interrupted raw log: %q", raw)
			}

			base := filepath.Dir(rawPath)
			summary := readSummary(t, filepath.Join(base, "unit.summary.json"))
			status := readStatus(t, filepath.Join(base, "unit.status.json"))
			if summary.Status != model.RunStatusKilled || summary.ExitCode != test.exitCode {
				t.Fatalf("expected summary killed/%d, got status=%s exit=%d", test.exitCode, summary.Status, summary.ExitCode)
			}
			if status.Status != model.RunStatusKilled || status.ExitCode != test.exitCode {
				t.Fatalf("expected status killed/%d, got status=%s exit=%d", test.exitCode, status.Status, status.ExitCode)
			}
			if status.RawLogSHA256 != artifacts.SHA256(raw) || summary.RawLogSHA256 != status.RawLogSHA256 {
				t.Fatalf("raw checksum mismatch: summary=%s status=%s actual=%s", summary.RawLogSHA256, status.RawLogSHA256, artifacts.SHA256(raw))
			}
			if !strings.Contains(stdout.String(), "Status: killed") {
				t.Fatalf("expected killed console result, stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			requireProcessGone(t, filepath.Join(repo, "child.pid"))
		})
	}
}

func waitForInterruptedRaw(t *testing.T, repo, runID string) string {
	t.Helper()
	if runID != "" {
		path := filepath.Join(repo, ".manta", "runs", "scoped", runID, "artifacts", "test", "unit.raw.log")
		waitForPath(t, path)
		return path
	}
	pattern := filepath.Join(repo, ".manta", "runs", "standalone", "*", "unit.raw.log")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 1 {
			return matches[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for raw log matching %s", pattern)
	return ""
}

func waitForRawMarker(t *testing.T, path, marker string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), marker) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in %s", marker, path)
}

func waitForPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func readSummary(t *testing.T, path string) model.Summary {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	return summary
}

func readStatus(t *testing.T, path string) model.Status {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func requireProcessGone(t *testing.T, pidPath string) {
	t.Helper()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child process %d still exists", pid)
}
