package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/manta/internal/artifacts"
	"github.com/irootkernel/manta/internal/model"
)

func TestConfiguredRunAndExcerpt(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  unit:",
		"    command: [\"sh\", \"test.sh\"]",
		"    tags: [unit]",
		"    parser: generic",
		"    timeout_sec: 10",
		"redaction:",
		"  patterns:",
		"    - name: token",
		"      regex: 'token=[^ ]+'",
		"      replace: 'token=<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho 'noise: start'\necho 'TypeError: token=secret failed'\necho 'src/foo.test.ts:42:13'\necho '✗ renders empty state'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "unit"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%s", exitCode, stderr.String())
	}
	mantaDir := filepath.Join(repo, ".manta", "runs", "standalone")
	entries, err := os.ReadDir(mantaDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one run directory, err=%v entries=%d", err, len(entries))
	}
	runDir := filepath.Join(mantaDir, entries[0].Name())
	summaryJSONPath := filepath.Join(runDir, "unit.summary.json")
	rawLogPath := filepath.Join(runDir, "unit.raw.log")
	statusJSONPath := filepath.Join(runDir, "unit.status.json")
	summaryData, err := os.ReadFile(summaryJSONPath)
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
	if summary.ExtractorStatus == model.ExtractorStatusNoMatch {
		t.Fatalf("expected extracted failure, got %s", summary.ExtractorStatus)
	}
	if len(summary.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(summary.Failures))
	}
	if summary.Failures[0].Excerpt != "excerpts/F001.log" {
		t.Fatalf("expected summary-local excerpt reference, got %q", summary.Failures[0].Excerpt)
	}
	if strings.Contains(summary.Failures[0].Signature, "secret") {
		t.Fatalf("expected redacted failure signature, got %q", summary.Failures[0].Signature)
	}
	rawLog, err := os.ReadFile(rawLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawLog), "token=secret") {
		t.Fatalf("expected raw log to preserve original secret, got %q", string(rawLog))
	}
	statusData, err := os.ReadFile(statusJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	summaryFileBytes, err := os.ReadFile(summaryJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	summaryHash := sha256.Sum256(summaryFileBytes)
	expectedSummarySHA := "sha256:" + hex.EncodeToString(summaryHash[:])
	if status.SummarySHA256 != expectedSummarySHA {
		t.Fatalf("expected summary sha %s, got %s", expectedSummarySHA, status.SummarySHA256)
	}
	watcherInput := strings.Join([]string{
		status.CommandID,
		strings.Join(status.Tags, ","),
		string(status.Status),
		"1",
		string(status.ExtractorStatus),
		status.RawLogSHA256,
		strings.Join(status.FailureSignatures, ","),
		strings.Join(status.WarningSignatures, ","),
		status.SummaryPath,
		status.RawLogPath,
	}, "\n")
	watcherHash := sha256.Sum256([]byte(watcherInput))
	expectedStatusHash := "sha256:" + hex.EncodeToString(watcherHash[:])
	if status.StatusHash != expectedStatusHash {
		t.Fatalf("expected status hash %s, got %s", expectedStatusHash, status.StatusHash)
	}
	if len(status.WarningSignatures) != 0 {
		t.Fatalf("expected no warnings, got %+v", status.WarningSignatures)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "excerpt", "--summary", filepath.ToSlash(summaryJSONPath), "F001"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected excerpt command to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "token=<redacted>") {
		t.Fatalf("expected redacted excerpt output, got %q", stdout.String())
	}
}

func TestConfiguredRunRedactsSurfacedMetadata(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  command_secret_id:",
		"    command: [\"sh\", \"test.sh\", \"secret_arg\"]",
		"    tags: [unit, tag-secret_tag]",
		"    parser: generic",
		"    timeout_sec: 10",
		"redaction:",
		"  patterns:",
		"    - name: secret",
		"      regex: 'secret_[a-z_]+'",
		"      replace: '<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf '%s\\n' \"$1\"",
		"echo 'warning: secret_warning'",
		"echo 'TypeError: secret_failure failed'",
		"echo 'src/secret_path/foo.test.ts:42:13'",
		"echo '✗ secret_test'",
		"exit 1",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--json", "run", "command_secret_id"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%s", exitCode, stderr.String())
	}
	var result runResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON result %q: %v", stdout.String(), err)
	}
	if result.Command != "command_<redacted>" {
		t.Fatalf("expected redacted result command, got %q", result.Command)
	}
	for _, path := range []string{result.Summary, result.StatusJSON, result.RawLog} {
		if !strings.Contains(path, "command_secret_id") {
			t.Fatalf("expected literal artifact reference, got %q", path)
		}
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected artifact reference %q to resolve: %v", path, err)
		}
	}

	statusData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.StatusJSON)))
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	if status.CommandID != "command_<redacted>" || !slices.Equal(status.Tags, []string{"tag-<redacted>", "unit"}) {
		t.Fatalf("expected redacted status metadata, got command=%q tags=%q", status.CommandID, status.Tags)
	}
	if status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("status hash was not computed from final surfaced fields: %+v", status)
	}

	summaryData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(status.SummaryPath)))
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.CommandID != "command_<redacted>" || !slices.Equal(summary.Tags, []string{"tag-<redacted>", "unit"}) || summary.Parser != "generic" {
		t.Fatalf("expected redacted summary metadata, got command=%q tags=%q parser=%q", summary.CommandID, summary.Tags, summary.Parser)
	}
	if len(summary.CommandArgv) != 3 || summary.CommandArgv[2] != "<redacted>" {
		t.Fatalf("expected redacted command argv, got %+v", summary.CommandArgv)
	}
	if len(summary.Failures) == 0 || len(summary.Warnings) != 1 {
		t.Fatalf("expected failures and one warning, got failures=%+v warnings=%+v", summary.Failures, summary.Warnings)
	}
	for _, failure := range summary.Failures {
		assertNoSecret(t, "failure signature", failure.Signature)
		assertNoSecret(t, "failure test name", failure.TestName)
		assertNoSecret(t, "failure file", failure.File)
		for _, stackLine := range failure.StackTop {
			assertNoSecret(t, "failure stack", stackLine)
		}
	}
	assertNoSecret(t, "warning signature", summary.Warnings[0].Signature)
	if len(status.FailureSignatures) != len(summary.Failures) {
		t.Fatalf("expected failure hashes from redacted signatures, got %+v", status.FailureSignatures)
	}
	for _, failure := range summary.Failures {
		want := artifacts.SHA256([]byte(failure.Signature))
		if !slices.Contains(status.FailureSignatures, want) {
			t.Fatalf("expected redacted failure hash %q in %+v", want, status.FailureSignatures)
		}
	}
	if len(status.WarningSignatures) != 1 || status.WarningSignatures[0] != artifacts.SHA256([]byte(summary.Warnings[0].Signature)) {
		t.Fatalf("expected warning hash from redacted signature, got %+v", status.WarningSignatures)
	}

	for _, failure := range summary.Failures {
		excerptPath := filepath.Join(filepath.Dir(filepath.Join(repo, filepath.FromSlash(status.SummaryPath))), filepath.FromSlash(failure.Excerpt))
		excerptData, err := os.ReadFile(excerptPath)
		if err != nil {
			t.Fatal(err)
		}
		assertNoSecret(t, "excerpt", string(excerptData))
	}
	rawData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.RawLog)))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret_arg", "secret_warning", "secret_failure", "secret_path", "secret_test"} {
		if !strings.Contains(string(rawData), secret) {
			t.Fatalf("expected raw log to preserve %q, got %q", secret, rawData)
		}
	}
	if got := artifacts.SHA256(rawData); summary.RawLogSHA256 != got || status.RawLogSHA256 != got {
		t.Fatalf("raw checksum mismatch: summary=%q status=%q want=%q", summary.RawLogSHA256, status.RawLogSHA256, got)
	}

	markdownData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.Summary)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(markdownData), "# Manta Summary: command_<redacted>") {
		t.Fatalf("expected redacted markdown heading, got %q", markdownData)
	}
	if !strings.Contains(string(markdownData), "command_secret_id.raw.log") {
		t.Fatalf("expected literal raw-log reference in markdown, got %q", markdownData)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "run", "command_secret_id"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected human run exit code 1, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Command: command_<redacted>") {
		t.Fatalf("expected redacted human command metadata, got %q", stdout.String())
	}
}

func TestAdHocRunRedactsSurfacedMetadata(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"redaction:",
		"  patterns:",
		"    - name: secret",
		"      regex: 'secret_[a-z_]+'",
		"      replace: '<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "adhoc.sh"), []byte("#!/bin/sh\nprintf '%s\\n' \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := runJSONCommand(t, "--repo", repo, "--json", "run", "--tag", "unit", "--tag", "tag-secret_tag", "sh", "adhoc.sh", "secret_arg")
	if !strings.HasPrefix(result.Command, "adhoc-") || strings.Contains(result.Command, "secret_") {
		t.Fatalf("expected tag-independent generated command id, got %q", result.Command)
	}
	for _, path := range []string{result.Summary, result.StatusJSON, result.RawLog} {
		if !strings.Contains(path, "adhoc-") || strings.Contains(path, "secret_") {
			t.Fatalf("expected tag-independent literal artifact reference, got %q", path)
		}
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected artifact reference %q to resolve: %v", path, err)
		}
	}

	statusData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.StatusJSON)))
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	if status.CommandID != result.Command || !slices.Equal(status.Tags, []string{"tag-<redacted>", "unit"}) || status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("expected redacted, self-consistent ad-hoc status, got %+v", status)
	}
	summaryData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(status.SummaryPath)))
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.CommandID != result.Command || !slices.Equal(summary.Tags, []string{"tag-<redacted>", "unit"}) || summary.Parser != "generic" {
		t.Fatalf("expected redacted ad-hoc summary metadata, got command=%q tags=%q parser=%q", summary.CommandID, summary.Tags, summary.Parser)
	}
	if len(summary.CommandArgv) != 3 || summary.CommandArgv[2] != "<redacted>" {
		t.Fatalf("expected redacted ad-hoc argv, got %+v", summary.CommandArgv)
	}
	rawData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.RawLog)))
	if err != nil {
		t.Fatal(err)
	}
	if string(rawData) != "secret_arg\n" {
		t.Fatalf("expected exact unredacted ad-hoc raw log, got %q", rawData)
	}
	if want := artifacts.SHA256(rawData); summary.RawLogSHA256 != want || status.RawLogSHA256 != want {
		t.Fatalf("ad-hoc raw checksum mismatch: summary=%q status=%q want=%q", summary.RawLogSHA256, status.RawLogSHA256, want)
	}
}

func TestUnsafeRunIDFailsBeforeCommandExecution(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeMarkerCommandConfig(t, repo, "unit")
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--run-id", "../escape", "run", "unit"}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) {
		t.Fatalf("expected config exit code, got %d stderr=%s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("expected command not to execute, stat error=%v", err)
	}
}

func TestRawLogOpenFailurePreventsCommandExecution(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeMarkerCommandConfig(t, repo, "unit")
	base := filepath.Join(repo, ".manta", "runs", "scoped", "run-001", "artifacts", "test")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "outside.log")
	if err := os.WriteFile(external, []byte("unchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(base, "unit.raw.log")); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--run-id", "run-001", "run", "unit"}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("expected command not to execute, stat error=%v", err)
	}
	data, err := os.ReadFile(external)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged\n" {
		t.Fatalf("external raw target changed: %q", data)
	}
}

func TestRawLogWriteFailureDoesNotPublishDerivedArtifacts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeMarkerCommandConfig(t, repo, "unit")
	injectedErr := errors.New("injected raw-log write failure")
	partialRaw := []byte("partial raw evidence\n")
	execute := func(_ context.Context, _, _ string, _ []string, _ string, _ []string, _ int, raw io.Writer) (model.RunOutput, error) {
		if _, err := raw.Write(partialRaw); err != nil {
			return model.RunOutput{}, err
		}
		return model.RunOutput{}, model.NewMantaError(model.ExitCodeArtifactError, "write raw log", injectedErr)
	}

	var stdout, stderr bytes.Buffer
	exitCode := runCommand(globalOptions{RepoRoot: repo, RunID: "write-failure"}, []string{"unit"}, &stdout, &stderr, execute)

	if exitCode != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact exit code %d, got %d stdout=%s stderr=%s", model.ExitCodeArtifactError, exitCode, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "write raw log: "+injectedErr.Error()) {
		t.Fatalf("unexpected CLI output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	base := filepath.Join(repo, ".manta", "runs", "scoped", "write-failure", "artifacts", "test")
	raw, err := os.ReadFile(filepath.Join(base, "unit.raw.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, partialRaw) {
		t.Fatalf("partial raw log = %q, want %q", raw, partialRaw)
	}
	for _, name := range []string{"unit.summary.json", "unit.summary.md", "unit.status.json"} {
		if _, err := os.Stat(filepath.Join(base, name)); !os.IsNotExist(err) {
			t.Fatalf("derived artifact %s was published, stat error=%v", name, err)
		}
	}
}

func TestTimeoutPreservesPartialArtifacts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  timeout:",
		"    command: [\"sh\", \"timeout.sh\"]",
		"    tags: [unit]",
		"    parser: generic",
		"    timeout_sec: 1",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\necho started\nsleep 30\necho finished\n"
	if err := os.WriteFile(filepath.Join(repo, "timeout.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--run-id", "timeout-run", "run", "timeout"}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeTimeout) {
		t.Fatalf("expected timeout exit code %d, got %d stderr=%s", model.ExitCodeTimeout, exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Status: timed_out") {
		t.Fatalf("expected timed_out console result, got %q", stdout.String())
	}

	base := filepath.Join(repo, ".manta", "runs", "scoped", "timeout-run", "artifacts", "test")
	raw, err := os.ReadFile(filepath.Join(base, "timeout.raw.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "started\n") || strings.Contains(string(raw), "finished\n") {
		t.Fatalf("expected partial raw evidence, got %q", raw)
	}
	var summary model.Summary
	summaryData, err := os.ReadFile(filepath.Join(base, "timeout.summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	var status model.Status
	statusData, err := os.ReadFile(filepath.Join(base, "timeout.status.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	wantSHA := artifacts.SHA256(raw)
	if summary.Status != model.RunStatusTimedOut || summary.ExitCode != int(model.ExitCodeTimeout) {
		t.Fatalf("expected timed_out summary, got status=%s exit=%d", summary.Status, summary.ExitCode)
	}
	if status.Status != model.RunStatusTimedOut || status.ExitCode != int(model.ExitCodeTimeout) {
		t.Fatalf("expected timed_out status, got status=%s exit=%d", status.Status, status.ExitCode)
	}
	if summary.RawLogSHA256 != wantSHA || status.RawLogSHA256 != wantSHA {
		t.Fatalf("raw checksum mismatch: summary=%s status=%s actual=%s", summary.RawLogSHA256, status.RawLogSHA256, wantSHA)
	}
}

func TestUnsafeConfiguredCommandIDFailsBeforeExecution(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeMarkerCommandConfig(t, repo, "../unit")
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "../unit"}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) {
		t.Fatalf("expected config exit code, got %d stderr=%s", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("expected command not to execute, stat error=%v", err)
	}
}

func TestExcerptRejectsUnsafeReferences(t *testing.T) {
	t.Parallel()
	for name, reference := range map[string]string{
		"absolute":  "/tmp/F001.log",
		"traversal": "excerpts/../../other/F001.log",
		"cross-run": "../run-b/excerpts/F001.log",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			summaryPath := writeExcerptSummary(t, repo, reference)
			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
			if exitCode != int(model.ExitCodeArtifactError) {
				t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
			}
		})
	}
}

func TestExcerptSymlinkContainment(t *testing.T) {
	t.Parallel()
	t.Run("cross-run rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		runA := filepath.Join(repo, ".manta", "runs", "scoped", "run-a", "artifacts", "test")
		runB := filepath.Join(repo, ".manta", "runs", "scoped", "run-b", "artifacts", "test")
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
		if err := os.Symlink(target, filepath.Join(runA, "excerpts", "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, runA, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
		}
		if strings.Contains(stdout.String(), "other-run-secret") {
			t.Fatalf("cross-run excerpt content leaked: %q", stdout.String())
		}
	})

	t.Run("dangling rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		excerptsDir := filepath.Join(repo, "excerpts")
		if err := os.MkdirAll(excerptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(excerptsDir, "missing.log"), filepath.Join(excerptsDir, "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, repo, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
		}
	})

	t.Run("external rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		excerptsDir := filepath.Join(repo, "excerpts")
		if err := os.MkdirAll(excerptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		external := filepath.Join(t.TempDir(), "F001.log")
		if err := os.WriteFile(external, []byte("outside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, filepath.Join(excerptsDir, "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, repo, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact exit code, got %d stderr=%s", exitCode, stderr.String())
		}
	})

	t.Run("internal allowed", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		excerptsDir := filepath.Join(repo, "excerpts")
		if err := os.MkdirAll(excerptsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(excerptsDir, "actual.log")
		if err := os.WriteFile(target, []byte("inside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(excerptsDir, "F001.log")); err != nil {
			t.Fatal(err)
		}
		summaryPath := writeExcerptSummary(t, repo, "excerpts/F001.log")
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", summaryPath, "F001"}, &stdout, &stderr)
		if exitCode != 0 {
			t.Fatalf("expected success, got %d stderr=%s", exitCode, stderr.String())
		}
		if stdout.String() != "inside\n" {
			t.Fatalf("unexpected excerpt %q", stdout.String())
		}
	})
}

func writeExcerptSummary(t *testing.T, dir, reference string) string {
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

func writeMarkerCommandConfig(t *testing.T, repo, commandID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  \"" + commandID + "\":",
		"    command: [\"sh\", \"touch-marker.sh\"]",
		"    tags: [unit]",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "touch-marker.sh"), []byte("#!/bin/sh\ntouch command-ran\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestSummarizeRawLogUsesConfigRedaction(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"redaction:",
		"  patterns:",
		"    - name: token",
		"      regex: 'token=[^ ]+'",
		"      replace: 'token=<redacted>'",
		"    - name: identifier",
		"      regex: 'secret_[a-z_]+'",
		"      replace: '<redacted>'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	rawDir := filepath.Join(repo, "fixtures")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rawLogPath := filepath.Join(rawDir, "unit_secret_id.raw.log")
	rawText := strings.Join([]string{
		"noise: start",
		"TypeError: token=secret failed",
		"src/foo.test.ts:42:13",
		"✗ renders empty state",
		"",
	}, "\n")
	if err := os.WriteFile(rawLogPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	result := runJSONCommand(t, "--repo", repo, "--json", "summarize", filepath.ToSlash(rawLogPath))
	if result.Command != "unit_<redacted>" {
		t.Fatalf("expected redacted summarize JSON command, got %q", result.Command)
	}
	for _, path := range []string{result.Summary, result.StatusJSON, result.RawLog} {
		if !strings.Contains(path, "unit_secret_id") {
			t.Fatalf("expected literal summarize artifact reference, got %q", path)
		}
	}
	runsDir := filepath.Join(repo, ".manta", "runs", "standalone")
	entries, err := os.ReadDir(runsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one standalone summarize directory, err=%v entries=%d", err, len(entries))
	}
	runDir := filepath.Join(runsDir, entries[0].Name())
	summaryJSONPath := filepath.Join(runDir, "unit_secret_id.summary.json")
	summaryData, err := os.ReadFile(summaryJSONPath)
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
	if summary.CommandArgv == nil {
		t.Fatal("expected summarize command_argv to remain an empty array")
	}
	if summary.CommandID != "unit_<redacted>" || !slices.Equal(summary.Tags, []string{"unit_<redacted>"}) {
		t.Fatalf("expected redacted summarize identifiers, got command=%q tags=%q", summary.CommandID, summary.Tags)
	}
	if len(summary.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(summary.Failures))
	}
	if strings.Contains(summary.Failures[0].Signature, "secret") {
		t.Fatalf("expected redacted summarized failure signature, got %q", summary.Failures[0].Signature)
	}
	if got, want := summary.RawLog, artifacts.Rel(repo, filepath.Join(runDir, "unit_secret_id.raw.log")); got != want {
		t.Fatalf("expected copied raw log reference %q, got %q", want, got)
	}
	statusData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.StatusJSON)))
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	if status.CommandID != summary.CommandID || !slices.Equal(status.Tags, summary.Tags) || status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("expected redacted, self-consistent summarize status, got %+v", status)
	}
	for _, path := range []string{status.SummaryPath, status.RawLogPath} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("expected summarize status reference %q to resolve: %v", path, err)
		}
	}
	copiedRaw, err := os.ReadFile(filepath.Join(runDir, "unit_secret_id.raw.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedRaw) != rawText {
		t.Fatalf("expected copied raw evidence to match source, got %q", copiedRaw)
	}
	if want := artifacts.SHA256(copiedRaw); summary.RawLogSHA256 != want || status.RawLogSHA256 != want {
		t.Fatalf("summarize raw checksum mismatch: summary=%q status=%q want=%q", summary.RawLogSHA256, status.RawLogSHA256, want)
	}
	if _, err := os.Stat(filepath.Join(rawDir, "unit_secret_id.summary.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no summary beside input raw log, stat error=%v", err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "excerpt", "--summary", filepath.ToSlash(summaryJSONPath), "F001"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected excerpt command to succeed after summarize, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "token=<redacted>") {
		t.Fatalf("expected redacted excerpt output after summarize, got %q", stdout.String())
	}
}

func assertNoSecret(t *testing.T, label, value string) {
	t.Helper()
	if strings.Contains(value, "secret_") {
		t.Fatalf("expected redacted %s, got %q", label, value)
	}
}

func TestStandaloneOperationsUseDistinctRunDirectories(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  unit:",
		"    command: [\"sh\", \"configured.sh\"]",
		"    tags: [unit]",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "configured.sh"), []byte("#!/bin/sh\nprintf 'configured\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "adhoc.sh"), []byte("#!/bin/sh\nprintf 'adhoc\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	rawLogPath := filepath.Join(repo, "fixture.raw.log")
	if err := os.WriteFile(rawLogPath, []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results := []runResult{
		runJSONCommand(t, "--repo", repo, "--json", "run", "unit"),
		runJSONCommand(t, "--repo", repo, "--json", "run", "unit"),
		runJSONCommand(t, "--repo", repo, "--json", "run", "--tag", "unit", "sh", "adhoc.sh"),
		runJSONCommand(t, "--repo", repo, "--json", "run", "--tag", "unit", "sh", "adhoc.sh"),
		runJSONCommand(t, "--repo", repo, "--json", "summarize", filepath.ToSlash(rawLogPath)),
		runJSONCommand(t, "--repo", repo, "--json", "summarize", filepath.ToSlash(rawLogPath)),
		runJSONCommand(t, "--repo", repo, "--output-dir", "evidence", "--json", "summarize", filepath.ToSlash(rawLogPath)),
		runJSONCommand(t, "--repo", repo, "--output-dir", "evidence", "--json", "summarize", filepath.ToSlash(rawLogPath)),
	}

	seen := make(map[string]bool, len(results))
	for _, result := range results {
		rawPath := filepath.Join(repo, filepath.FromSlash(result.RawLog))
		baseDir := filepath.Dir(rawPath)
		if seen[baseDir] {
			t.Fatalf("standalone operation reused run directory %q", baseDir)
		}
		seen[baseDir] = true
		data, err := os.ReadFile(rawPath)
		if err != nil {
			t.Fatal(err)
		}
		statusData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.StatusJSON)))
		if err != nil {
			t.Fatal(err)
		}
		var status model.Status
		if err := json.Unmarshal(statusData, &status); err != nil {
			t.Fatal(err)
		}
		summaryData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(status.SummaryPath)))
		if err != nil {
			t.Fatal(err)
		}
		var summary model.Summary
		if err := json.Unmarshal(summaryData, &summary); err != nil {
			t.Fatal(err)
		}
		wantSHA := artifacts.SHA256(data)
		if summary.RawLogSHA256 != wantSHA || status.RawLogSHA256 != wantSHA {
			t.Fatalf("raw checksum mismatch for %q: summary=%q status=%q want=%q", rawPath, summary.RawLogSHA256, status.RawLogSHA256, wantSHA)
		}
	}
}

func runJSONCommand(t *testing.T, args ...string) runResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if exitCode := Main(args, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("command %q failed with exit %d: %s", args, exitCode, stderr.String())
	}
	var result runResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode command %q output %q: %v", args, stdout.String(), err)
	}
	return result
}

func TestAdHocRunWithoutConfig(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	script := "#!/bin/sh\necho 'ok'\nexit 0\n"
	if err := os.WriteFile(filepath.Join(repo, "ok.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "--tag", "unit", "sh", "ok.sh"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected ad-hoc run to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Status: passed") {
		t.Fatalf("expected pass output, got %q", stdout.String())
	}
}

func TestOversizedFailedRunPreservesRawLog(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".manta", "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  huge:",
		"    command: [\"sh\", \"huge.sh\"]",
		"    tags: [unit]",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\npython3 - <<'PY'\nprint('x' * 300000)\nPY\nexit 1\n"
	if err := os.WriteFile(filepath.Join(repo, "huge.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "run", "huge"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected underlying failed exit code, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Status: failed") || !strings.Contains(stdout.String(), "Exit code: 1") {
		t.Fatalf("expected failed result with original command exit, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no extraction diagnostic, got %q", stderr.String())
	}
	runsDir := filepath.Join(repo, ".manta", "runs", "standalone")
	entries, err := os.ReadDir(runsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one run directory, err=%v entries=%d", err, len(entries))
	}
	runDir := filepath.Join(runsDir, entries[0].Name())
	raw := assertDegradedArtifacts(t, runDir, "huge", model.RunStatusFailed, 1)
	if len(raw) == 0 {
		t.Fatal("expected non-empty raw log artifact")
	}
}

func TestConfiguredRunsUseSpecializedParsers(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		parser   string
		script   string
		file     string
		line     int
		testName string
	}{
		{
			parser: "vitest",
			script: `#!/bin/sh
echo ' RUN  v1.6.0 /repo'
echo ''
echo ' FAIL  src/foo.test.ts > renders empty state'
echo ' AssertionError: expected false to be true'
echo ' ❯ src/foo.ts:42:13'
exit 1
`,
			file:     "src/foo.ts",
			line:     42,
			testName: "renders empty state",
		},
		{
			parser: "playwright",
			script: `#!/bin/sh
echo '1) [chromium] › tests/checkout.spec.ts:73:5 › submits order'
echo ''
echo '  Error: expect(page).toHaveURL() failed'
echo ''
echo '    at tests/checkout.spec.ts:73:5'
exit 1
`,
			file:     "tests/checkout.spec.ts",
			line:     73,
			testName: "submits order",
		},
		{
			parser: "pytest",
			script: `#!/bin/sh
echo '=================================== FAILURES ==================================='
echo '_______________________________ test_empty_state _______________________________'
echo ''
echo 'E       AssertionError: expected ready'
echo ''
echo 'tests/test_app.py:42: AssertionError'
echo '=========================== short test summary info ============================'
echo 'FAILED tests/test_app.py::test_empty_state - AssertionError: expected ready'
exit 1
`,
			file:     "tests/test_app.py",
			line:     42,
			testName: "test_empty_state",
		},
	} {
		t.Run(test.parser, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
				t.Fatal(err)
			}
			config := strings.Join([]string{
				"version: 2",
				"commands:",
				"  test:",
				"    command: [\"sh\", \"test.sh\"]",
				"    tags: [test]",
				"    parser: " + test.parser,
				"    timeout_sec: 10",
			}, "\n") + "\n"
			if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, "test.sh"), []byte(test.script), 0o755); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "--json", "run", "test"}, &stdout, &stderr)
			if exitCode != 1 {
				t.Fatalf("expected %s run to preserve exit code 1, got %d stderr=%s", test.parser, exitCode, stderr.String())
			}
			var result runResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode run result %q: %v", stdout.String(), err)
			}
			summary := readRunSummary(t, repo, result)
			if summary.Parser != test.parser || summary.Status != model.RunStatusFailed || summary.ExitCode != 1 || summary.ExtractorStatus != model.ExtractorStatusPrecise {
				t.Fatalf("unexpected %s summary contract: %+v", test.parser, summary)
			}
			if len(summary.Failures) != 1 {
				t.Fatalf("expected one %s failure, got %d", test.parser, len(summary.Failures))
			}
			failure := summary.Failures[0]
			if failure.File != test.file || failure.Line != test.line || failure.TestName != test.testName {
				t.Fatalf("expected %s file/line/test capture, got %+v", test.parser, failure)
			}
		})
	}
}

func TestLegacyLaneInterfacesFailClosed(t *testing.T) {
	t.Parallel()
	t.Run("CLI flag", func(t *testing.T) {
		repo := t.TempDir()
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "run", "--lane", "unit", "true"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeConfigError) || !strings.Contains(stderr.String(), "flag provided but not defined") {
			t.Fatalf("legacy flag exit=%d stderr=%q", exitCode, stderr.String())
		}
	})
	t.Run("config schema", func(t *testing.T) {
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".manta"), 0o755); err != nil {
			t.Fatal(err)
		}
		legacy := "version: 1\ncommands:\n  unit:\n    command: [true]\n    lane: unit\n    parser: generic\n    timeout_sec: 10\n"
		if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(legacy), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		exitCode := Main([]string{"--repo", repo, "run", "unit"}, &stdout, &stderr)
		if exitCode != int(model.ExitCodeConfigError) || !strings.Contains(stderr.String(), "field lane not found") {
			t.Fatalf("legacy config exit=%d stderr=%q", exitCode, stderr.String())
		}
	})
}

func TestRunAndSummarizeSelectRulesByAllTags(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawText := writeTagSelectorFixture(t, repo)

	for _, test := range []struct {
		commandID string
		wantTags  []string
		wantRules []string
	}{
		{commandID: "unit", wantTags: []string{"go", "unit"}, wantRules: []string{"COMMON_ONLY", "UNIT_ONLY"}},
		{commandID: "integration", wantTags: []string{"go", "integration"}, wantRules: []string{"COMMON_ONLY", "INTEGRATION_ONLY"}},
	} {
		t.Run(test.commandID, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "--json", "run", test.commandID}, &stdout, &stderr)
			if exitCode != 1 {
				t.Fatalf("run exit=%d, want 1; stderr=%s", exitCode, stderr.String())
			}
			var result runResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode run result %q: %v", stdout.String(), err)
			}
			summary := readRunSummary(t, repo, result)
			assertSelectedRules(t, summary, test.wantTags, test.wantRules)
		})
	}

	rawPath := filepath.Join(repo, "selector.raw.log")
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	result := runJSONCommand(t, "--repo", repo, "--json", "summarize", "--tag", "integration", "--tag", "go", "--tag", "integration", rawPath)
	summary := readRunSummary(t, repo, result)
	assertSelectedRules(t, summary, []string{"go", "integration"}, []string{"COMMON_ONLY", "INTEGRATION_ONLY"})
	if summary.Status != model.RunStatusPassed {
		t.Fatalf("rules changed inferred summarize status: %s", summary.Status)
	}
}

func writeTagSelectorFixture(t *testing.T, repo string) string {
	t.Helper()
	rulesDir := filepath.Join(repo, ".manta", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  unit:",
		"    command: [sh, selector.sh]",
		"    tags: [unit, go, unit]",
		"    parser: generic",
		"    timeout_sec: 10",
		"  integration:",
		"    command: [sh, selector.sh]",
		"    tags: [integration, go]",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".manta", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	rawText := "COMMON_ONLY\n\nUNIT_ONLY\n\nINTEGRATION_ONLY\n\n"
	script := "#!/bin/sh\nprintf '%s' '" + rawText + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(repo, "selector.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []struct {
		id     string
		tags   string
		marker string
	}{
		{id: "common", tags: "[go]", marker: "COMMON_ONLY"},
		{id: "unit", tags: "[go, unit]", marker: "UNIT_ONLY"},
		{id: "integration", tags: "[go, integration]", marker: "INTEGRATION_ONLY"},
	} {
		content := strings.Join([]string{
			"id: " + rule.id,
			"tags: " + rule.tags,
			"parser: generic",
			"status: active",
			"provenance:",
			"  created_by: tester",
			"  source_run: selector-run",
			"  source_command: selector",
			"  source_log_sha256: sha256:abc",
			"  source_span:",
			"    start_line: 1",
			"    end_line: 1",
			"  reason: tag selector fixture",
			"match:",
			"  start:",
			"    regex: '^" + rule.marker + "$'",
			"  end:",
			"    any_of:",
			"      - regex: '^$'",
			"    max_block_lines: 2",
			"  include_context:",
			"    before: 0",
			"    after: 0",
			"confidence: high",
		}, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(rulesDir, rule.id+".yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return rawText
}

func readRunSummary(t *testing.T, repo string, result runResult) model.Summary {
	t.Helper()
	statusData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.StatusJSON)))
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	summaryData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(status.SummaryPath)))
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatal(err)
	}
	return summary
}

func assertSelectedRules(t *testing.T, summary model.Summary, wantTags, wantSignatures []string) {
	t.Helper()
	if !slices.Equal(summary.Tags, wantTags) {
		t.Fatalf("summary tags = %q, want %q", summary.Tags, wantTags)
	}
	got := make([]string, 0, len(summary.Failures))
	for _, failure := range summary.Failures {
		got = append(got, failure.Signature)
	}
	slices.Sort(got)
	want := slices.Clone(wantSignatures)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("selected rule signatures = %q, want %q", got, want)
	}
}
