//go:build unix

package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
)

func TestExecuteForwardsTerminationAndNormalizesResult(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := filepath.Join(repo, "wait.sh")
	content := "#!/bin/sh\ntrap 'echo interrupted; exit 0' TERM\necho started\ntouch ready\nwhile :; do :; done\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(repo, "raw.log")
	raw, err := os.OpenFile(rawPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	interrupts := make(chan os.Signal, 2)
	type result struct {
		output model.RunOutput
		err    error
	}
	finished := make(chan result, 1)
	go func() {
		output, runErr := executeWithSignals(context.Background(), repo, "wait", "unit", "generic", []string{"sh", "wait.sh"}, 10, raw, interrupts, 500*time.Millisecond)
		finished <- result{output: output, err: runErr}
	}()
	waitForFile(t, filepath.Join(repo, "ready"))
	interrupts <- syscall.SIGTERM
	resultValue := <-finished
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if resultValue.err != nil {
		t.Fatalf("executeWithSignals failed: %v", resultValue.err)
	}
	if resultValue.output.Status != model.RunStatusKilled || resultValue.output.Metadata.ExitCode != 143 {
		t.Fatalf("expected killed/143, got status=%s exit=%d", resultValue.output.Status, resultValue.output.Metadata.ExitCode)
	}
	data, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "started\n") || !strings.Contains(string(data), "interrupted\n") {
		t.Fatalf("expected streamed partial evidence, got %q", data)
	}
}

func TestExecuteCleansDescendantsAfterLeaderExits(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := filepath.Join(repo, "descendant.sh")
	content := strings.Join([]string{
		"#!/bin/sh",
		"trap 'echo interrupted; exit 0' TERM",
		"sh -c 'trap \"\" TERM; echo $$ > descendant.pid; while :; do sleep 1; done' </dev/null >/dev/null 2>&1 &",
		"while [ ! -s descendant.pid ]; do sleep 1; done",
		"touch ready",
		"while :; do sleep 1; done",
	}, "\n") + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.OpenFile(filepath.Join(repo, "raw.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := raw.Close(); err != nil {
			t.Errorf("close raw log: %v", err)
		}
	})
	interrupts := make(chan os.Signal, 2)
	type result struct {
		output model.RunOutput
		err    error
	}
	finished := make(chan result, 1)
	gracePeriod := 5 * time.Second
	go func() {
		output, runErr := executeWithSignals(context.Background(), repo, "descendant", "unit", "generic", []string{"sh", "descendant.sh"}, 10, raw, interrupts, gracePeriod)
		finished <- result{output: output, err: runErr}
	}()
	waitForFile(t, filepath.Join(repo, "ready"))
	pid := readPID(t, filepath.Join(repo, "descendant.pid"))
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	started := time.Now()
	interrupts <- syscall.SIGTERM
	resultValue := <-finished
	if resultValue.err != nil {
		t.Fatalf("executeWithSignals failed: %v", resultValue.err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("expected command leader to exit before grace expiry, took %s", elapsed)
	}
	if resultValue.output.Status != model.RunStatusKilled || resultValue.output.Metadata.ExitCode != 143 {
		t.Fatalf("expected killed/143, got status=%s exit=%d", resultValue.output.Status, resultValue.output.Metadata.ExitCode)
	}
	requirePIDGone(t, pid)
}

func TestExecuteRecoversLeaderResultAfterWaitDelay(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		exitCode int
		status   model.RunStatus
	}{
		{name: "passed leader", exitCode: 0, status: model.RunStatusPassed},
		{name: "failed leader", exitCode: 7, status: model.RunStatusFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			script := filepath.Join(repo, "wait-delay.sh")
			content := strings.Join([]string{
				"#!/bin/sh",
				"sh -c 'echo $$ > child.pid; sleep 30; echo late' &",
				"while [ ! -s child.pid ]; do sleep 0.01; done",
				"echo started",
				"exit " + strconv.Itoa(test.exitCode),
			}, "\n") + "\n"
			if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
				t.Fatal(err)
			}
			var raw bytes.Buffer
			started := time.Now()
			output, err := executeWithSignals(context.Background(), repo, "wait-delay", "unit", "generic", []string{"sh", "wait-delay.sh"}, 10, &raw, make(chan os.Signal, 2), 50*time.Millisecond)
			if err != nil {
				t.Fatalf("executeWithSignals failed: %v", err)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("wait-delay recovery took %s", elapsed)
			}
			if output.Status != test.status || output.Metadata.ExitCode != test.exitCode {
				t.Fatalf("status=%s exit=%d, want %s/%d", output.Status, output.Metadata.ExitCode, test.status, test.exitCode)
			}
			if raw.String() != "started\n" {
				t.Fatalf("raw output = %q", raw.String())
			}
			pid := readPID(t, filepath.Join(repo, "child.pid"))
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			requirePIDGone(t, pid)
		})
	}
}

func TestExecuteCleansDescendantsAfterSuccessfulLeaderExit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := filepath.Join(repo, "background.sh")
	content := strings.Join([]string{
		"#!/bin/sh",
		"sh -c 'echo $$ > child.pid; sleep 30' </dev/null >/dev/null 2>&1 &",
		"while [ ! -s child.pid ]; do sleep 0.01; done",
		"echo complete",
	}, "\n") + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	output, err := executeWithSignals(context.Background(), repo, "background", "unit", "generic", []string{"sh", "background.sh"}, 10, &raw, make(chan os.Signal, 2), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("executeWithSignals failed: %v", err)
	}
	if output.Status != model.RunStatusPassed || output.Metadata.ExitCode != 0 {
		t.Fatalf("status=%s exit=%d, want passed/0", output.Status, output.Metadata.ExitCode)
	}
	pid := readPID(t, filepath.Join(repo, "child.pid"))
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	requirePIDGone(t, pid)
}

func TestExecuteEscalatesSecondTermination(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := filepath.Join(repo, "ignore.sh")
	content := "#!/bin/sh\ntrap '' TERM\ntouch ready\nwhile :; do :; done\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.OpenFile(filepath.Join(repo, "raw.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := raw.Close(); err != nil {
			t.Errorf("close raw log: %v", err)
		}
	})
	interrupts := make(chan os.Signal, 2)
	finished := make(chan model.RunOutput, 1)
	go func() {
		output, _ := executeWithSignals(context.Background(), repo, "ignore", "unit", "generic", []string{"sh", "ignore.sh"}, 10, raw, interrupts, 2*time.Second)
		finished <- output
	}()
	waitForFile(t, filepath.Join(repo, "ready"))
	started := time.Now()
	interrupts <- syscall.SIGTERM
	interrupts <- syscall.SIGTERM
	output := <-finished
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("expected prompt escalation, took %s", elapsed)
	}
	if output.Status != model.RunStatusKilled || output.Metadata.ExitCode != 143 {
		t.Fatalf("expected killed/143 after escalation, got status=%s exit=%d", output.Status, output.Metadata.ExitCode)
	}
}

func TestExecuteEscalatesAfterGracePeriod(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := filepath.Join(repo, "ignore.sh")
	content := "#!/bin/sh\ntrap '' TERM\ntouch ready\nwhile :; do :; done\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.OpenFile(filepath.Join(repo, "raw.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := raw.Close(); err != nil {
			t.Errorf("close raw log: %v", err)
		}
	})
	interrupts := make(chan os.Signal, 2)
	type result struct {
		output model.RunOutput
		err    error
	}
	finished := make(chan result, 1)
	gracePeriod := 100 * time.Millisecond
	go func() {
		output, runErr := executeWithSignals(context.Background(), repo, "ignore", "unit", "generic", []string{"sh", "ignore.sh"}, 10, raw, interrupts, gracePeriod)
		finished <- result{output: output, err: runErr}
	}()
	waitForFile(t, filepath.Join(repo, "ready"))
	started := time.Now()
	interrupts <- syscall.SIGTERM
	resultValue := <-finished
	elapsed := time.Since(started)
	if resultValue.err != nil {
		t.Fatalf("executeWithSignals failed: %v", resultValue.err)
	}
	if elapsed < gracePeriod || elapsed > time.Second {
		t.Fatalf("expected escalation after %s grace period, took %s", gracePeriod, elapsed)
	}
	if resultValue.output.Status != model.RunStatusKilled || resultValue.output.Metadata.ExitCode != 143 {
		t.Fatalf("expected killed/143 after grace expiry, got status=%s exit=%d", resultValue.output.Status, resultValue.output.Metadata.ExitCode)
	}
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

func requirePIDGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant process %d still exists", pid)
}
