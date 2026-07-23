package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

const preparePathsProcessHelper = "MANTA_TEST_PREPARE_PATHS_PROCESS_HELPER"

func TestPreparePathsReservesDistinctStandaloneDirectoriesAcrossProcesses(t *testing.T) {
	if os.Getenv(preparePathsProcessHelper) == "1" {
		repo := os.Getenv("MANTA_TEST_PREPARE_PATHS_REPO")
		resultPath := os.Getenv("MANTA_TEST_PREPARE_PATHS_RESULT")
		now := time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC)
		paths, err := preparePathsAt(repo, "", "", "unit", now)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(resultPath, []byte(filepath.Base(paths.BaseDir)), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	repo := t.TempDir()
	resultsDir := t.TempDir()
	const count = 8
	commands := make([]*exec.Cmd, 0, count)
	resultPaths := make([]string, 0, count)
	for i := range count {
		resultPath := filepath.Join(resultsDir, fmt.Sprintf("result-%02d", i))
		cmd := exec.Command(os.Args[0], "-test.run=^TestPreparePathsReservesDistinctStandaloneDirectoriesAcrossProcesses$")
		cmd.Env = append(os.Environ(),
			preparePathsProcessHelper+"=1",
			"MANTA_TEST_PREPARE_PATHS_REPO="+repo,
			"MANTA_TEST_PREPARE_PATHS_RESULT="+resultPath,
		)
		if err := cmd.Start(); err != nil {
			t.Fatalf("start allocator process %d: %v", i, err)
		}
		commands = append(commands, cmd)
		resultPaths = append(resultPaths, resultPath)
	}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("wait for allocator process %d: %v", i, err)
		}
	}

	seen := make(map[string]bool, count)
	for _, resultPath := range resultPaths {
		data, err := os.ReadFile(resultPath)
		if err != nil {
			t.Fatal(err)
		}
		seen[string(data)] = true
	}
	for sequence := range count {
		want := "20260720T010203"
		if sequence > 0 {
			want = fmt.Sprintf("20260720T010203-%03d", sequence)
		}
		if !seen[want] {
			t.Fatalf("expected process allocation %q, got %+v", want, seen)
		}
	}
}

func TestPreparePathsReservesDistinctStandaloneDirectories(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	now := time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC)

	first, err := preparePathsAt(repo, "", "", "unit", now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := preparePathsAt(repo, "", "", "unit", now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := filepath.Base(first.BaseDir), "20260720T010203"; got != want {
		t.Fatalf("expected first run directory %q, got %q", want, got)
	}
	if got, want := filepath.Base(second.BaseDir), "20260720T010203-001"; got != want {
		t.Fatalf("expected collision suffix %q, got %q", want, got)
	}
	if first.BaseDir == second.BaseDir {
		t.Fatal("expected distinct standalone directories")
	}
}

func TestPreparePathsReservesDistinctOutputDirectoriesConcurrently(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	now := time.Date(2026, time.July, 20, 1, 2, 3, 0, time.UTC)
	const count = 16

	type allocationResult struct {
		paths model.ArtifactPaths
		err   error
	}
	results := make(chan allocationResult, count)
	for range count {
		go func() {
			allocated, err := preparePathsAt(repo, "custom-output", "", "unit", now)
			results <- allocationResult{paths: allocated, err: err}
		}()
	}

	seen := make(map[string]bool, count)
	for range count {
		allocatedResult := <-results
		if allocatedResult.err != nil {
			t.Fatal(allocatedResult.err)
		}
		allocated := allocatedResult.paths
		if seen[allocated.BaseDir] {
			t.Fatalf("duplicate allocated directory %q", allocated.BaseDir)
		}
		seen[allocated.BaseDir] = true
		if got, want := allocated.BoundaryDir, filepath.Join(repo, "custom-output"); got != want {
			t.Fatalf("expected output boundary %q, got %q", want, got)
		}
	}
	if len(seen) != count {
		t.Fatalf("expected %d unique directories, got %d", count, len(seen))
	}
}

func TestPreparePathsKeepsExplicitRunIDLayout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	paths, err := preparePathsAt(repo, "ignored", "run-001", "unit", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(repo, ".manta", "runs", "scoped", "run-001", "artifacts", "test")
	if paths.BaseDir != want {
		t.Fatalf("expected fixed run-scoped path %q, got %q", want, paths.BaseDir)
	}
	if _, err := os.Stat(paths.ExcerptsDir); err != nil {
		t.Fatalf("expected prepared run-scoped directory: %v", err)
	}
}

func TestWriteSummaryJSONFailsWhenTooLarge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	summary := model.Summary{
		Status:          model.RunStatusFailed,
		CommandID:       "unit",
		Tags:            []string{"unit"},
		Parser:          "generic",
		CommandArgv:     []string{"sh", "test.sh"},
		ExitCode:        1,
		RawLog:          ".manta/runs/standalone/x/unit.raw.log",
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

func TestWriteSummaryJSONIncludesFalseTruncationFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "summary.json")
	paths := model.ArtifactPaths{BoundaryDir: dir, SummaryJSON: path}
	summary := model.Summary{
		Status:          model.RunStatusPassed,
		CommandID:       "unit",
		ExtractorStatus: model.ExtractorStatusNoMatch,
		Failures:        []model.Failure{},
		Warnings:        []model.Warning{},
	}
	if _, err := WriteSummaryJSON(paths, summary); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"failures_truncated", "warnings_truncated"} {
		value, ok := fields[field]
		if !ok || string(value) != "false" {
			t.Fatalf("summary JSON field %q = %s, want false", field, value)
		}
	}
}

func TestBoundSummaryEvidenceCapsRecordsAndKeepsCountsAligned(t *testing.T) {
	t.Parallel()
	summary := model.Summary{
		ExtractorStatus: model.ExtractorStatusPrecise,
		Failures:        make([]model.Failure, safety.MaxSummaryFailures+1),
		Warnings:        make([]model.Warning, safety.MaxSummaryWarnings+1),
	}
	for i := range summary.Failures {
		summary.Failures[i] = model.Failure{ID: fmt.Sprintf("F%03d", i+1), Signature: "failure"}
	}
	for i := range summary.Warnings {
		summary.Warnings[i] = model.Warning{ID: fmt.Sprintf("W%03d", i+1), Signature: "warning"}
	}
	atCap := summary
	atCap.Failures = atCap.Failures[:safety.MaxSummaryFailures]
	atCap.Warnings = atCap.Warnings[:safety.MaxSummaryWarnings]
	atCapBounded, err := BoundSummaryEvidence(atCap)
	if err != nil {
		t.Fatal(err)
	}
	if atCapBounded.FailuresTruncated || atCapBounded.WarningsTruncated || atCapBounded.ExtractorStatus != model.ExtractorStatusPrecise {
		t.Fatalf("evidence at the record caps must not be truncated: %+v", atCapBounded)
	}

	bounded, err := BoundSummaryEvidence(summary)
	if err != nil {
		t.Fatal(err)
	}
	if bounded.FailureCount != safety.MaxSummaryFailures || len(bounded.Failures) != safety.MaxSummaryFailures || !bounded.FailuresTruncated {
		t.Fatalf("unexpected bounded failures: count=%d len=%d truncated=%t", bounded.FailureCount, len(bounded.Failures), bounded.FailuresTruncated)
	}
	if bounded.WarningCount != safety.MaxSummaryWarnings || len(bounded.Warnings) != safety.MaxSummaryWarnings || !bounded.WarningsTruncated {
		t.Fatalf("unexpected bounded warnings: count=%d len=%d truncated=%t", bounded.WarningCount, len(bounded.Warnings), bounded.WarningsTruncated)
	}
	if bounded.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("extractor status = %s, want degraded", bounded.ExtractorStatus)
	}
	if bounded.Failures[len(bounded.Failures)-1].ID != "F050" || bounded.Warnings[len(bounded.Warnings)-1].ID != "W050" {
		t.Fatalf("expected deterministic prefixes, got failure=%q warning=%q", bounded.Failures[len(bounded.Failures)-1].ID, bounded.Warnings[len(bounded.Warnings)-1].ID)
	}
}

func TestBoundSummaryEvidenceUsesRenderedByteBudget(t *testing.T) {
	t.Parallel()
	summary := model.Summary{
		Status:          model.RunStatusFailed,
		CommandID:       "unit",
		ExtractorStatus: model.ExtractorStatusPrecise,
		RawLog:          ".manta/unit.raw.log",
		Failures: []model.Failure{
			{ID: "F001", Signature: "short"},
			{ID: "F002", Signature: strings.Repeat("\x01", safety.MaxSummaryBytes)},
		},
		Warnings: []model.Warning{{ID: "W001", Signature: strings.Repeat("warning", safety.MaxSummaryBytes)}},
	}

	bounded, err := BoundSummaryEvidence(summary)
	if err != nil {
		t.Fatal(err)
	}
	jsonData, err := marshalSummaryJSON(bounded)
	if err != nil {
		t.Fatal(err)
	}
	markdown := renderSummaryMarkdown(bounded)
	if len(jsonData)+1 > safety.MaxSummaryBytes || len(markdown) > safety.MaxSummaryBytes {
		t.Fatalf("bounded artifacts exceed limit: json=%d markdown=%d", len(jsonData), len(markdown))
	}
	if bounded.FailureCount != 1 || bounded.WarningCount != 0 || !bounded.FailuresTruncated || !bounded.WarningsTruncated {
		t.Fatalf("unexpected byte-budget result: %+v", bounded)
	}
	if bounded.Failures[0].ID != "F001" {
		t.Fatalf("expected first failure to be retained, got %+v", bounded.Failures)
	}
}

func TestBoundSummaryEvidenceIncludesJSONTrailingNewlineInByteBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	paths := model.ArtifactPaths{BoundaryDir: dir, SummaryJSON: filepath.Join(dir, "summary.json")}
	summary := model.Summary{
		Status:          model.RunStatusFailed,
		CommandID:       "unit",
		ExtractorStatus: model.ExtractorStatusPrecise,
		RawLog:          ".manta/unit.raw.log",
		Failures: []model.Failure{
			{ID: "F001", Signature: "short"},
			{ID: "F002"},
		},
	}
	syncSummaryEvidenceMetadata(&summary)
	jsonData, err := marshalSummaryJSON(summary)
	if err != nil {
		t.Fatal(err)
	}
	padding := safety.MaxSummaryBytes - len(jsonData)
	if padding <= 0 {
		t.Fatalf("summary metadata leaves no boundary-test padding: %d", padding)
	}
	summary.Failures[1].Signature = strings.Repeat("x", padding)
	jsonData, err = marshalSummaryJSON(summary)
	if err != nil {
		t.Fatal(err)
	}
	if len(jsonData) != safety.MaxSummaryBytes {
		t.Fatalf("test summary JSON size=%d, want %d before trailing newline", len(jsonData), safety.MaxSummaryBytes)
	}

	bounded, err := BoundSummaryEvidence(summary)
	if err != nil {
		t.Fatal(err)
	}
	if bounded.FailureCount != 1 || !bounded.FailuresTruncated || bounded.Failures[0].ID != "F001" {
		t.Fatalf("expected boundary evidence to drop F002, got %+v", bounded)
	}
	if _, err := WriteSummaryJSON(paths, bounded); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(paths.SummaryJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) > safety.MaxSummaryBytes {
		t.Fatalf("written summary JSON size=%d, limit=%d", len(written), safety.MaxSummaryBytes)
	}
}

func TestBoundSummaryEvidenceUsesRemainingBudgetForWarnings(t *testing.T) {
	t.Parallel()
	summary := model.Summary{
		Status:          model.RunStatusFailed,
		CommandID:       "unit",
		ExtractorStatus: model.ExtractorStatusPrecise,
		Failures: []model.Failure{{
			ID:        "F001",
			Signature: strings.Repeat("failure", safety.MaxSummaryBytes),
		}},
		Warnings: []model.Warning{{ID: "W001", Signature: "short warning"}},
	}

	bounded, err := BoundSummaryEvidence(summary)
	if err != nil {
		t.Fatal(err)
	}
	if bounded.FailureCount != 0 || !bounded.FailuresTruncated {
		t.Fatalf("unexpected bounded failures: %+v", bounded)
	}
	if bounded.WarningCount != 1 || bounded.WarningsTruncated || bounded.Warnings[0].ID != "W001" {
		t.Fatalf("expected remaining budget to retain the warning: %+v", bounded)
	}
}

func TestComputeStatusHashIncludesTags(t *testing.T) {
	t.Parallel()
	status := model.Status{CommandID: "unit", Tags: []string{"go", "unit"}, Status: model.RunStatusFailed, ExitCode: 1}
	withUnitTags := ComputeStatusHash(status)
	status.Tags = []string{"go", "integration"}
	withIntegrationTags := ComputeStatusHash(status)
	if withUnitTags == withIntegrationTags {
		t.Fatal("expected tag changes to affect status hash")
	}
}

func TestWriteSummaryMarkdownMatchesDocumentedShape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "unit.summary.md")
	paths := model.ArtifactPaths{BoundaryDir: dir, SummaryMD: path}
	summary := model.Summary{
		Status:          model.RunStatusFailed,
		CommandID:       "unit",
		ExitCode:        1,
		RawLog:          ".manta/runs/scoped/summarize-example/artifacts/test/unit.raw.log",
		RawLogSHA256:    "sha256:abc",
		ExtractorStatus: model.ExtractorStatusPrecise,
		FailureCount:    1,
		Failures: []model.Failure{{
			ID:        "F001",
			Signature: "TypeError: token=<redacted> failed",
			File:      "src/foo.test.ts",
			Line:      42,
			TestName:  "renders empty state",
			Excerpt:   "excerpts/F001.log",
		}},
	}

	if err := WriteSummaryMarkdown(paths, summary); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"# Manta Summary: unit",
		"",
		"Status: failed",
		"Exit code: 1",
		"Duration: 0.0s",
		"Extractor: precise",
		"Failures: 1 (truncated: false)",
		"Warnings: 0 (truncated: false)",
		"Raw log: .manta/runs/scoped/summarize-example/artifacts/test/unit.raw.log",
		"Raw log SHA-256: sha256:abc",
		"",
		"## Failures",
		"",
		"### F001: TypeError: token=<redacted> failed",
		"",
		"- File: src/foo.test.ts:42",
		"- Test: renders empty state",
		"- Excerpt: excerpts/F001.log",
		"",
		"## Notes",
		"",
		"Command exit code is authoritative. Extraction rules only summarize evidence.",
		"",
	}, "\n")
	if got := string(data); got != want {
		t.Fatalf("markdown mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestOpenRawLogCreatesEvidenceBeforeCompletion(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	paths, err := PreparePaths(repo, "", "run-001", "unit")
	if err != nil {
		t.Fatal(err)
	}
	file, err := OpenRawLog(paths)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.RawLogPath); err != nil {
		t.Fatalf("expected raw log to exist before command execution: %v", err)
	}
	if _, err := file.WriteString("started\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRawLog(paths); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(paths.RawLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "started\n" {
		t.Fatalf("unexpected raw evidence %q", raw)
	}
	if err := os.Remove(paths.RawLogPath); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRawLog(paths); model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected missing raw log to fail validation, got %v", err)
	}
}

func TestPreparePathsRejectsUnsafeIdentifiers(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, value := range []string{"../other", "/tmp/other", "nested/id", ".", "with space"} {
		t.Run("run_id_"+strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			t.Parallel()
			if _, err := PreparePaths(repo, "", value, "unit"); err == nil {
				t.Fatalf("expected unsafe run id %q to fail", value)
			}
		})
		t.Run("command_id_"+strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			t.Parallel()
			if _, err := PreparePaths(repo, "", "safe-run", value); err == nil {
				t.Fatalf("expected unsafe command id %q to fail", value)
			}
		})
	}
}

func TestArtifactWritesRejectSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(repo, ".manta")); err != nil {
		t.Fatal(err)
	}
	if _, err := PreparePaths(repo, "", "", "unit"); err == nil {
		t.Fatal("expected external .manta symlink to fail closed")
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
	if err := os.Symlink(internal, filepath.Join(repo, ".manta")); err != nil {
		t.Fatal(err)
	}
	paths, err := PreparePaths(repo, "", "", "unit")
	if err != nil {
		t.Fatalf("expected internal .manta symlink to be allowed: %v", err)
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

func TestRunIDArtifactLayout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	paths, err := PreparePaths(repo, "", "run-001", "unit")
	if err != nil {
		t.Fatal(err)
	}
	expectedBase := filepath.Join(repo, ".manta", "runs", "scoped", "run-001", "artifacts", "test")
	if paths.BoundaryDir != repo || paths.BaseDir != expectedBase {
		t.Fatalf("unexpected run-scoped paths %+v", paths)
	}
	if _, err := WriteRawLog(paths, []byte("raw\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(expectedBase, "unit.raw.log")); err != nil {
		t.Fatalf("expected run-scoped raw log: %v", err)
	}
}

func TestRunIDArtifactWritesRejectSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(repo, ".manta")); err != nil {
		t.Fatal(err)
	}
	_, err := PreparePaths(repo, "", "run-001", "unit")
	if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact error for external .manta symlink, got %v", err)
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
			paths, err := PreparePaths(repo, outputDir, "", "unit")
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
	paths, err := PreparePaths(repo, "", "", "unit")
	if err != nil {
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
	if _, err := OpenRawLog(paths); model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected raw open error for final file symlink, got %v", err)
	}
	if err := ValidateRawLog(paths); model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected raw validation error for final file symlink, got %v", err)
	}
	data, err := os.ReadFile(externalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged\n" {
		t.Fatalf("external file was modified: %q", data)
	}
}
