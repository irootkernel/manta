package artifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/safety"
)

func TestWriteSummaryJSONFailsWhenTooLarge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	summary := model.Summary{
		Status:          model.RunStatusFailed,
		CommandID:       "unit",
		Lane:            "unit",
		Parser:          "generic",
		CommandArgv:     []string{"sh", "test.sh"},
		ExitCode:        1,
		RawLog:          ".kat/runs/x/unit.raw.log",
		RawLogSHA256:    "sha256:abc",
		ExtractorStatus: model.ExtractorStatusPrecise,
		Failures: []model.Failure{{
			ID:        "F001",
			Kind:      "test_failure",
			Signature: strings.Repeat("x", safety.MaxSummaryBytes),
		}},
	}
	paths := model.ArtifactPaths{BoundaryDir: dir, SummaryJSON: filepath.Join(dir, "summary.json")}
	if _, err := WriteSummaryJSON(paths, summary); err == nil {
		t.Fatal("expected oversized summary json to fail")
	}
}

func TestPlanPathsRejectsUnsafeIdentifiers(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, value := range []string{"../other", "/tmp/other", "nested/id", ".", "with space"} {
		t.Run("run_id_"+strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			t.Parallel()
			if _, err := PlanPaths(repo, "", value, "unit"); err == nil {
				t.Fatalf("expected unsafe run id %q to fail", value)
			}
		})
		t.Run("command_id_"+strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			t.Parallel()
			if _, err := PlanPaths(repo, "", "safe-run", value); err == nil {
				t.Fatalf("expected unsafe command id %q to fail", value)
			}
		})
	}
}

func TestArtifactWritesRejectSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(repo, ".kat")); err != nil {
		t.Fatal(err)
	}
	paths, err := PlanPaths(repo, "", "", "unit")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureParents(paths); err == nil {
		t.Fatal("expected external .kat symlink to fail closed")
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no artifacts outside the repository, got %d entries", len(entries))
	}
}

func TestArtifactWritesAllowInternalSymlink(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	internal := filepath.Join(repo, "evidence")
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(internal, filepath.Join(repo, ".kat")); err != nil {
		t.Fatal(err)
	}
	paths, err := PlanPaths(repo, "", "", "unit")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureParents(paths); err != nil {
		t.Fatalf("expected internal .kat symlink to be allowed: %v", err)
	}
	if _, err := WriteRawLog(paths, []byte("ok\n")); err != nil {
		t.Fatalf("expected raw log write through internal symlink: %v", err)
	}
	data, err := os.ReadFile(paths.RawLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ok\n" {
		t.Fatalf("unexpected raw log %q", data)
	}
}

func TestKkachiArtifactLayout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	paths, err := PlanPaths(repo, "", "run-001", "unit")
	if err != nil {
		t.Fatal(err)
	}
	expectedBase := filepath.Join(repo, ".kkachi", "runs", "run-001", "artifacts", "test")
	if paths.BoundaryDir != repo || paths.BaseDir != expectedBase {
		t.Fatalf("unexpected Kkachi paths %+v", paths)
	}
	if err := EnsureParents(paths); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRawLog(paths, []byte("raw\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(expectedBase, "unit.raw.log")); err != nil {
		t.Fatalf("expected Kkachi raw log: %v", err)
	}
}

func TestKkachiArtifactWritesRejectSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(repo, ".kkachi")); err != nil {
		t.Fatal(err)
	}
	paths, err := PlanPaths(repo, "", "run-001", "unit")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureParents(paths); model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact error for external .kkachi symlink, got %v", err)
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no artifacts outside the repository, got %d entries", len(entries))
	}
}

func TestArtifactOutputDirectories(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	absoluteOutput := t.TempDir()
	for name, outputDir := range map[string]string{
		"relative": "custom-output",
		"absolute": absoluteOutput,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			paths, err := PlanPaths(repo, outputDir, "", "unit")
			if err != nil {
				t.Fatal(err)
			}
			expectedBoundary := outputDir
			if !filepath.IsAbs(expectedBoundary) {
				expectedBoundary = filepath.Join(repo, expectedBoundary)
			}
			if paths.BoundaryDir != expectedBoundary {
				t.Fatalf("expected boundary %q, got %q", expectedBoundary, paths.BoundaryDir)
			}
			if err := EnsureParents(paths); err != nil {
				t.Fatal(err)
			}
			if _, err := WriteRawLog(paths, []byte("raw\n")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(paths.RawLogPath); err != nil {
				t.Fatalf("expected output-dir raw log: %v", err)
			}
		})
	}
}

func TestArtifactWriteRejectsFinalFileSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	paths, err := PlanPaths(repo, "", "", "unit")
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureParents(paths); err != nil {
		t.Fatal(err)
	}
	externalPath := filepath.Join(t.TempDir(), "outside.log")
	if err := os.WriteFile(externalPath, []byte("unchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalPath, paths.RawLogPath); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRawLog(paths, []byte("escaped\n")); model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact error for final file symlink, got %v", err)
	}
	data, err := os.ReadFile(externalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged\n" {
		t.Fatalf("external file was modified: %q", data)
	}
}
