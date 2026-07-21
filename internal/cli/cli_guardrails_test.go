package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/irootkernel/manta/internal/model"
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
	runsDir := filepath.Join(repo, ".manta", "runs", "standalone")
	entries, err := os.ReadDir(runsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one standalone summarize directory, err=%v entries=%d", err, len(entries))
	}
	baseDir := filepath.Join(runsDir, entries[0].Name())
	summaryPath := filepath.Join(baseDir, "unit.summary.json")
	statusPath := filepath.Join(baseDir, "unit.status.json")
	excerptPath := filepath.Join(baseDir, "excerpts", "F001.log")
	copiedRawPath := filepath.Join(baseDir, "unit.raw.log")
	for _, path := range []string{copiedRawPath, summaryPath, statusPath, excerptPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected summarize to create %s: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(rawDir, "unit.summary.json"),
		filepath.Join(rawDir, "unit.status.json"),
		filepath.Join(rawDir, "excerpts"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected no derived artifact beside source at %s, stat error=%v", path, err)
		}
	}
	copiedRaw, err := os.ReadFile(copiedRawPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedRaw) != rawText {
		t.Fatalf("expected copied raw log to match source, got %q", copiedRaw)
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
	if summary.CommandID != "unit" || len(summary.Tags) != 1 || summary.Tags[0] != "unit" {
		t.Fatalf("expected unit command/tag inference, got command=%q tags=%q", summary.CommandID, summary.Tags)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Excerpt == "" {
		t.Fatalf("expected one failure with excerpt, got %+v", summary.Failures)
	}
}
