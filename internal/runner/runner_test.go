package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kkachi-agent-tester/internal/model"
)

func TestExecuteTimeout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := filepath.Join(repo, "sleep.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	output, err := Execute(context.Background(), repo, "sleep", "unit", "generic", []string{"sh", "sleep.sh"}, 1)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.Status != model.RunStatusTimedOut {
		t.Fatalf("expected timed_out, got %s", output.Status)
	}
}
