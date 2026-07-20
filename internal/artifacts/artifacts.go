package artifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/safety"
)

func PlanPaths(repoRoot, outputDir, runID, commandID string) (model.ArtifactPaths, error) {
	if runID != "" {
		if err := safety.ValidateArtifactIdentifier("run id", runID); err != nil {
			return model.ArtifactPaths{}, model.NewKATError(model.ExitCodeConfigError, "plan artifact paths", err)
		}
	}
	if err := safety.ValidateArtifactIdentifier("command id", commandID); err != nil {
		return model.ArtifactPaths{}, model.NewKATError(model.ExitCodeConfigError, "plan artifact paths", err)
	}

	stamp := runID
	if stamp == "" {
		stamp = time.Now().UTC().Format("20060102T150405")
	}

	boundaryDir := repoRoot
	var baseDir string
	if runID != "" {
		baseDir = filepath.Join(repoRoot, ".kkachi", "runs", runID, "artifacts", "test")
	} else if outputDir != "" {
		if filepath.IsAbs(outputDir) {
			boundaryDir = outputDir
		} else {
			boundaryDir = filepath.Join(repoRoot, outputDir)
		}
		baseDir = filepath.Join(boundaryDir, "runs", stamp)
	} else {
		baseDir = filepath.Join(repoRoot, ".kat", "runs", stamp)
	}

	return model.ArtifactPaths{
		BoundaryDir: boundaryDir,
		BaseDir:     baseDir,
		RawLogPath:  filepath.Join(baseDir, commandID+".raw.log"),
		SummaryJSON: filepath.Join(baseDir, commandID+".summary.json"),
		SummaryMD:   filepath.Join(baseDir, commandID+".summary.md"),
		StatusJSON:  filepath.Join(baseDir, commandID+".status.json"),
		ExcerptsDir: filepath.Join(baseDir, "excerpts"),
	}, nil
}

func EnsureParents(paths model.ArtifactPaths) error {
	if err := safety.MkdirAllWithin(paths.BoundaryDir, paths.ExcerptsDir, 0o755); err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, "create artifact directory", err)
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
		return nil, model.NewKATError(model.ExitCodeArtifactError, "open raw log", err)
	}
	return file, nil
}

func ValidateRawLog(paths model.ArtifactPaths) error {
	info, err := safety.StatWithin(paths.BoundaryDir, paths.RawLogPath)
	if err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, "validate raw log", err)
	}
	if !info.Mode().IsRegular() {
		return model.NewKATError(model.ExitCodeArtifactError, "validate raw log", fmt.Errorf("raw log is not a regular file"))
	}
	return nil
}

func WriteSummaryJSON(paths model.ArtifactPaths, summary model.Summary) (string, error) {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", model.NewKATError(model.ExitCodeArtifactError, "marshal summary json", err)
	}
	if len(data) > safety.MaxSummaryBytes {
		return "", model.NewKATError(model.ExitCodeArtifactError, "write summary json", fmt.Errorf("summary json exceeds %d bytes", safety.MaxSummaryBytes))
	}
	written := append(data, '\n')
	if err := writeArtifact(paths, paths.SummaryJSON, written, "write summary json"); err != nil {
		return "", err
	}
	return SHA256(written), nil
}

func WriteStatusJSON(paths model.ArtifactPaths, status model.Status) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, "marshal status json", err)
	}
	return writeArtifact(paths, paths.StatusJSON, append(data, '\n'), "write status json")
}

func WriteSummaryMarkdown(paths model.ArtifactPaths, summary model.Summary) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# KAT Summary: %s\n\n", summary.CommandID)
	fmt.Fprintf(&b, "Status: %s\n", summary.Status)
	fmt.Fprintf(&b, "Exit code: %d\n", summary.ExitCode)
	fmt.Fprintf(&b, "Duration: %.1fs\n", float64(summary.DurationMS)/1000)
	fmt.Fprintf(&b, "Extractor: %s\n", summary.ExtractorStatus)
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
	markdown := b.String()
	if len(markdown) > safety.MaxSummaryBytes {
		return model.NewKATError(model.ExitCodeArtifactError, "write summary markdown", fmt.Errorf("summary markdown exceeds %d bytes", safety.MaxSummaryBytes))
	}
	return writeArtifact(paths, paths.SummaryMD, []byte(markdown), "write summary markdown")
}

func WriteExcerpt(paths model.ArtifactPaths, path string, content string) error {
	return writeArtifact(paths, path, []byte(content), "write excerpt")
}

func writeArtifact(paths model.ArtifactPaths, path string, data []byte, operation string) error {
	if err := safety.WriteFileWithin(paths.BoundaryDir, path, data, 0o644); err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, operation, err)
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
