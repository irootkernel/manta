package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/manta/internal/model"
)

var errInjectedRawLogWrite = errors.New("injected raw-log write failure")

type faultWriter struct {
	persisted bytes.Buffer
	attempted chan struct{}
}

func newFaultWriter() *faultWriter {
	return &faultWriter{attempted: make(chan struct{}, 1)}
}

func (w *faultWriter) Write(p []byte) (int, error) {
	n := len(p) / 2
	if n == 0 && len(p) > 0 {
		n = 1
	}
	_, _ = w.persisted.Write(p[:n])
	select {
	case w.attempted <- struct{}{}:
	default:
	}
	return n, errInjectedRawLogWrite
}

func requireRawLogWriteError(t *testing.T, output model.RunOutput, err error, raw *faultWriter, complete string) {
	t.Helper()
	if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact error %d, got output=%+v err=%v", model.ExitCodeArtifactError, output, err)
	}
	if !errors.Is(err, errInjectedRawLogWrite) {
		t.Fatalf("expected injected writer error, got %v", err)
	}
	if output.Status != "" || len(output.RawLogBytes) != 0 {
		t.Fatalf("expected no publishable run output, got %+v", output)
	}
	if raw.persisted.Len() == 0 || raw.persisted.Len() >= len(complete) {
		t.Fatalf("expected partial persisted bytes, got %q", raw.persisted.String())
	}
}

func TestExecuteReportsRawLogWriteFailure(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	raw := newFaultWriter()

	output, err := Execute(context.Background(), repo, "write-failure", []string{"unit"}, "generic", []string{"sh", "-c", "printf complete"}, 10, raw)

	requireRawLogWriteError(t, output, err, raw, "complete")
}

func TestExecuteTimeoutReportsRawLogWriteFailure(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	raw := newFaultWriter()

	output, err := Execute(context.Background(), repo, "timeout-write-failure", []string{"unit"}, "generic", []string{"sh", "-c", "printf started; while :; do sleep 1; done"}, 1, raw)

	requireRawLogWriteError(t, output, err, raw, "started")
}

func TestExecuteTimeout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	var raw bytes.Buffer
	output, err := Execute(context.Background(), repo, "sleep", []string{"unit"}, "generic", []string{"sh", "-c", "echo started; sleep 2"}, 1, &raw)
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
		output, err := Execute(ctx, repo, "cancel", []string{"unit"}, "generic", []string{"sh", "-c", "echo started; touch ready; while :; do sleep 1; done"}, 30, &raw)
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
