package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kkachi-agent-tester/internal/model"
)

func TestSummarizeRebuildsArtifactsFromRawLogOnly(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawDir := filepath.Join(repo, "logs")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(rawDir, "unit.raw.log")
	rawText := "noise: start\nTypeError: secret failed\nsrc/foo.test.ts:42:13\n✗ renders empty state\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "summarize", filepath.ToSlash(rawPath)}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected summarize command to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	summaryPath := filepath.Join(rawDir, "unit.summary.json")
	statusPath := filepath.Join(rawDir, "unit.status.json")
	excerptPath := filepath.Join(rawDir, "excerpts", "F001.log")
	for _, path := range []string{summaryPath, statusPath, excerptPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected summarize to create %s: %v", path, err)
		}
	}
	summaryData, err := os.ReadFile(summaryPath)
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
	if summary.CommandID != "unit" || summary.Lane != "unit" {
		t.Fatalf("expected unit command/lane inference, got command=%q lane=%q", summary.CommandID, summary.Lane)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Excerpt == "" {
		t.Fatalf("expected one failure with excerpt, got %+v", summary.Failures)
	}
}
