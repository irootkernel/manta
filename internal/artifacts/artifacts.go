package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

func PreparePaths(repoRoot, outputDir, runID, commandID string) (model.ArtifactPaths, error) {
	return preparePathsAt(repoRoot, outputDir, runID, commandID, time.Now().UTC())
}

func preparePathsAt(repoRoot, outputDir, runID, commandID string, now time.Time) (model.ArtifactPaths, error) {
	if runID != "" {
		if err := safety.ValidateArtifactIdentifier("run id", runID); err != nil {
			return model.ArtifactPaths{}, model.NewMantaError(model.ExitCodeConfigError, "plan artifact paths", err)
		}
	}
	if err := safety.ValidateArtifactIdentifier("command id", commandID); err != nil {
		return model.ArtifactPaths{}, model.NewMantaError(model.ExitCodeConfigError, "plan artifact paths", err)
	}
	if runID != "" {
		paths := pathsForBase(repoRoot, filepath.Join(repoRoot, ".manta", "runs", "scoped", runID, "artifacts", "test"), commandID)
		if err := ensureParents(paths); err != nil {
			return model.ArtifactPaths{}, err
		}
		return paths, nil
	}

	boundaryDir := repoRoot
	runsDir := filepath.Join(repoRoot, ".manta", "runs", "standalone")
	if outputDir != "" {
		if filepath.IsAbs(outputDir) {
			boundaryDir = outputDir
		} else {
			boundaryDir = filepath.Join(repoRoot, outputDir)
		}
		runsDir = filepath.Join(boundaryDir, "runs")
	}
	if err := safety.MkdirAllWithin(boundaryDir, runsDir, 0o755); err != nil {
		return model.ArtifactPaths{}, model.NewMantaError(model.ExitCodeArtifactError, "create artifact runs directory", err)
	}

	baseStamp := now.UTC().Format("20060102T150405")
	for sequence := 0; ; sequence++ {
		stamp := baseStamp
		if sequence > 0 {
			stamp = fmt.Sprintf("%s-%03d", baseStamp, sequence)
		}
		baseDir := filepath.Join(runsDir, stamp)
		if err := safety.MkdirWithin(boundaryDir, baseDir, 0o755); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return model.ArtifactPaths{}, model.NewMantaError(model.ExitCodeArtifactError, "reserve artifact directory", err)
		}

		paths := pathsForBase(boundaryDir, baseDir, commandID)
		if err := ensureParents(paths); err != nil {
			return model.ArtifactPaths{}, err
		}
		return paths, nil
	}
}

func pathsForBase(boundaryDir, baseDir, commandID string) model.ArtifactPaths {
	return model.ArtifactPaths{
		BoundaryDir: boundaryDir,
		BaseDir:     baseDir,
		RawLogPath:  filepath.Join(baseDir, commandID+".raw.log"),
		SummaryJSON: filepath.Join(baseDir, commandID+".summary.json"),
		SummaryMD:   filepath.Join(baseDir, commandID+".summary.md"),
		StatusJSON:  filepath.Join(baseDir, commandID+".status.json"),
		ExcerptsDir: filepath.Join(baseDir, "excerpts"),
	}
}

func ensureParents(paths model.ArtifactPaths) error {
	if err := safety.MkdirAllWithin(paths.BoundaryDir, paths.ExcerptsDir, 0o755); err != nil {
		return model.NewMantaError(model.ExitCodeArtifactError, "create artifact directory", err)
	}
	return nil
}

func WriteRawLog(paths model.ArtifactPaths, raw []byte) (string, error) {
	if err := writeArtifact(paths, paths.RawLogPath, raw, "write raw log"); err != nil {
		return "", err
	}
	return SHA256(raw), nil
}

func OpenRawLog(paths model.ArtifactPaths) (*os.File, error) {
	file, err := safety.OpenFileWithin(paths.BoundaryDir, paths.RawLogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, model.NewMantaError(model.ExitCodeArtifactError, "open raw log", err)
	}
	return file, nil
}

func ValidateRawLog(paths model.ArtifactPaths) error {
	info, err := safety.StatWithin(paths.BoundaryDir, paths.RawLogPath)
	if err != nil {
		return model.NewMantaError(model.ExitCodeArtifactError, "validate raw log", err)
	}
	if !info.Mode().IsRegular() {
		return model.NewMantaError(model.ExitCodeArtifactError, "validate raw log", fmt.Errorf("raw log is not a regular file"))
	}
	return nil
}

func WriteSummaryJSON(paths model.ArtifactPaths, summary model.Summary) (string, error) {
	data, err := marshalSummaryJSON(summary)
	if err != nil {
		return "", model.NewMantaError(model.ExitCodeArtifactError, "marshal summary json", err)
	}
	written := append(data, '\n')
	if len(written) > safety.MaxSummaryBytes {
		return "", model.NewMantaError(model.ExitCodeArtifactError, "write summary json", fmt.Errorf("summary json exceeds %d bytes", safety.MaxSummaryBytes))
	}
	if err := writeArtifact(paths, paths.SummaryJSON, written, "write summary json"); err != nil {
		return "", err
	}
	return SHA256(written), nil
}

func WriteStatusJSON(paths model.ArtifactPaths, status model.Status) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return model.NewMantaError(model.ExitCodeArtifactError, "marshal status json", err)
	}
	return writeArtifact(paths, paths.StatusJSON, append(data, '\n'), "write status json")
}

func WriteSummaryMarkdown(paths model.ArtifactPaths, summary model.Summary) error {
	markdown := renderSummaryMarkdown(summary)
	if len(markdown) > safety.MaxSummaryBytes {
		return model.NewMantaError(model.ExitCodeArtifactError, "write summary markdown", fmt.Errorf("summary markdown exceeds %d bytes", safety.MaxSummaryBytes))
	}
	return writeArtifact(paths, paths.SummaryMD, []byte(markdown), "write summary markdown")
}

// BoundSummaryEvidence keeps deterministic evidence prefixes that fit both
// summary artifact formats. Failures receive byte-budget priority because they
// explain the authoritative non-pass result.
func BoundSummaryEvidence(summary model.Summary) (model.Summary, error) {
	failureLimit := min(len(summary.Failures), safety.MaxSummaryFailures)
	warningLimit := min(len(summary.Warnings), safety.MaxSummaryWarnings)
	failures := append([]model.Failure(nil), summary.Failures[:failureLimit]...)
	warnings := append([]model.Warning(nil), summary.Warnings[:warningLimit]...)
	failuresTruncated := summary.FailuresTruncated || failureLimit < len(summary.Failures)
	warningsTruncated := summary.WarningsTruncated || warningLimit < len(summary.Warnings)

	candidate := func(failureCount, warningCount int) model.Summary {
		bounded := summary
		bounded.Failures = failures[:failureCount]
		bounded.Warnings = warnings[:warningCount]
		bounded.FailuresTruncated = failuresTruncated || failureCount < len(failures)
		bounded.WarningsTruncated = warningsTruncated || warningCount < len(warnings)
		syncSummaryEvidenceMetadata(&bounded)
		return bounded
	}
	fits := func(bounded model.Summary) (bool, error) {
		jsonData, err := marshalSummaryJSON(bounded)
		if err != nil {
			return false, model.NewMantaError(model.ExitCodeArtifactError, "marshal summary json", err)
		}
		return len(jsonData)+1 <= safety.MaxSummaryBytes && len(renderSummaryMarkdown(bounded)) <= safety.MaxSummaryBytes, nil
	}
	allEvidence := candidate(len(failures), len(warnings))
	allFits, err := fits(allEvidence)
	if err != nil {
		return model.Summary{}, err
	}
	if allFits {
		return allEvidence, nil
	}
	emptyEvidence := candidate(0, 0)
	emptyFits, err := fits(emptyEvidence)
	if err != nil {
		return model.Summary{}, err
	}
	if !emptyFits {
		return model.Summary{}, model.NewMantaError(model.ExitCodeArtifactError, "bound summary evidence", fmt.Errorf("summary metadata exceeds %d bytes", safety.MaxSummaryBytes))
	}

	failureCount := len(failures)
	for failureCount > 0 {
		failuresFit, err := fits(candidate(failureCount, 0))
		if err != nil {
			return model.Summary{}, err
		}
		if failuresFit {
			break
		}
		failureCount--
	}
	warningCount := 0
	for warningCount < len(warnings) {
		warningsFit, err := fits(candidate(failureCount, warningCount+1))
		if err != nil {
			return model.Summary{}, err
		}
		if !warningsFit {
			break
		}
		warningCount++
	}
	return candidate(failureCount, warningCount), nil
}

func syncSummaryEvidenceMetadata(summary *model.Summary) {
	summary.FailureCount = len(summary.Failures)
	summary.WarningCount = len(summary.Warnings)
	if summary.FailuresTruncated || summary.WarningsTruncated {
		summary.ExtractorStatus = model.ExtractorStatusDegraded
	}
}

func marshalSummaryJSON(summary model.Summary) ([]byte, error) {
	return json.MarshalIndent(summary, "", "  ")
}

func renderSummaryMarkdown(summary model.Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Manta Summary: %s\n\n", summary.CommandID)
	fmt.Fprintf(&b, "Status: %s\n", summary.Status)
	fmt.Fprintf(&b, "Exit code: %d\n", summary.ExitCode)
	fmt.Fprintf(&b, "Duration: %.1fs\n", float64(summary.DurationMS)/1000)
	fmt.Fprintf(&b, "Extractor: %s\n", summary.ExtractorStatus)
	fmt.Fprintf(&b, "Failures: %d (truncated: %t)\n", summary.FailureCount, summary.FailuresTruncated)
	fmt.Fprintf(&b, "Warnings: %d (truncated: %t)\n", summary.WarningCount, summary.WarningsTruncated)
	fmt.Fprintf(&b, "Raw log: %s\n", summary.RawLog)
	fmt.Fprintf(&b, "Raw log SHA-256: %s\n\n", summary.RawLogSHA256)
	if len(summary.Failures) > 0 {
		b.WriteString("## Failures\n\n")
		for _, failure := range summary.Failures {
			fmt.Fprintf(&b, "### %s: %s\n\n", failure.ID, failure.Signature)
			if failure.File != "" {
				fmt.Fprintf(&b, "- File: %s:%d\n", failure.File, failure.Line)
			}
			if failure.TestName != "" {
				fmt.Fprintf(&b, "- Test: %s\n", failure.TestName)
			}
			if failure.Excerpt != "" {
				fmt.Fprintf(&b, "- Excerpt: %s\n", failure.Excerpt)
			}
			b.WriteString("\n")
		}
	}
	if len(summary.Warnings) > 0 {
		b.WriteString("## Warnings\n\n")
		for _, warning := range summary.Warnings {
			fmt.Fprintf(&b, "- %s: %s\n", warning.ID, warning.Signature)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Notes\n\n")
	b.WriteString("Command exit code is authoritative. Extraction rules only summarize evidence.\n")
	return b.String()
}

func WriteExcerpt(paths model.ArtifactPaths, path string, content string) error {
	return writeArtifact(paths, path, []byte(content), "write excerpt")
}

func writeArtifact(paths model.ArtifactPaths, path string, data []byte, operation string) error {
	if err := safety.WriteFileWithin(paths.BoundaryDir, path, data, 0o644); err != nil {
		return model.NewMantaError(model.ExitCodeArtifactError, operation, err)
	}
	return nil
}

func Rel(repoRoot, path string) string {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ComputeStatusHash(status model.Status) string {
	ordered := []string{
		status.CommandID,
		strings.Join(status.Tags, ","),
		string(status.Status),
		fmt.Sprintf("%d", status.ExitCode),
		string(status.ExtractorStatus),
		status.RawLogSHA256,
		strings.Join(status.FailureSignatures, ","),
		strings.Join(status.WarningSignatures, ","),
		status.SummaryPath,
		status.RawLogPath,
	}
	return SHA256([]byte(strings.Join(ordered, "\n")))
}
