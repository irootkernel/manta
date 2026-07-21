package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/manta/internal/model"
)

func TestExecuteTimeout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	var raw bytes.Buffer
	output, err := Execute(context.Background(), repo, "sleep", "unit", "generic", []string{"sh", "-c", "echo started; sleep 2"}, 1, &raw)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.Status != model.RunStatusTimedOut {
		t.Fatalf("expected timed_out, got %s", output.Status)
	}
	if output.Metadata.ExitCode != int(model.ExitCodeTimeout) {
		t.Fatalf("expected timeout exit code %d, got %d", model.ExitCodeTimeout, output.Metadata.ExitCode)
	}
	if raw.String() != "started\n" {
		t.Fatalf("expected partial raw output, got %q", raw.String())
	}
}

func TestExecuteCanceledContext(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ready := filepath.Join(repo, "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		output model.RunOutput
		err    error
	}
	var raw bytes.Buffer
	finished := make(chan result, 1)
	go func() {
		output, err := Execute(ctx, repo, "cancel", "unit", "generic", []string{"sh", "-c", "echo started; touch ready; while :; do sleep 1; done"}, 30, &raw)
		finished <- result{output: output, err: err}
	}()

	waitForFile(t, ready)
	cancel()
	resultValue := <-finished
	if resultValue.err != nil {
		t.Fatalf("Execute failed: %v", resultValue.err)
	}
	if resultValue.output.Status != model.RunStatusKilled || resultValue.output.Metadata.ExitCode != 137 {
		t.Fatalf("expected killed/137, got status=%s exit=%d", resultValue.output.Status, resultValue.output.Metadata.ExitCode)
	}
	if !strings.Contains(raw.String(), "started\n") {
		t.Fatalf("expected partial raw output, got %q", raw.String())
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
