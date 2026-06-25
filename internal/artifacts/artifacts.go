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

	"kkachi-agent-tester/internal/model"
	"kkachi-agent-tester/internal/safety"
)

func PlanPaths(repoRoot, outputDir, runID, commandID string) model.ArtifactPaths {
	stamp := runID
	if stamp == "" {
		stamp = time.Now().UTC().Format("20060102T150405")
	}

	var baseDir string
	if runID != "" {
		baseDir = filepath.Join(repoRoot, ".kkachi", "runs", runID, "artifacts", "test")
	} else if outputDir != "" {
		if filepath.IsAbs(outputDir) {
			baseDir = filepath.Join(outputDir, "runs", stamp)
		} else {
			baseDir = filepath.Join(repoRoot, outputDir, "runs", stamp)
		}
	} else {
		baseDir = filepath.Join(repoRoot, ".kat", "runs", stamp)
	}

	raw := filepath.Join(baseDir, commandID+".raw.log")
	summaryJSON := filepath.Join(baseDir, commandID+".summary.json")
	summaryMD := filepath.Join(baseDir, commandID+".summary.md")
	statusJSON := filepath.Join(baseDir, commandID+".status.json")
	excerptsDir := filepath.Join(baseDir, "excerpts")
	if runID != "" {
		excerptsDir = filepath.Join(baseDir, "excerpts")
	}

	return model.ArtifactPaths{
		BaseDir:     baseDir,
		RawLogPath:  raw,
		SummaryJSON: summaryJSON,
		SummaryMD:   summaryMD,
		StatusJSON:  statusJSON,
		ExcerptsDir: excerptsDir,
	}
}

func EnsureParents(paths model.ArtifactPaths) error {
	for _, path := range []string{paths.RawLogPath, paths.SummaryJSON, paths.SummaryMD, paths.StatusJSON, filepath.Join(paths.ExcerptsDir, ".keep")} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return model.NewKATError(model.ExitCodeArtifactError, "create artifact directory", err)
		}
	}
	return nil
}

func WriteRawLog(path string, raw []byte) (string, error) {
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", model.NewKATError(model.ExitCodeArtifactError, "write raw log", err)
	}
	return SHA256(raw), nil
}

func WriteSummaryJSON(path string, summary model.Summary) (string, error) {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", model.NewKATError(model.ExitCodeArtifactError, "marshal summary json", err)
	}
	if len(data) > safety.MaxSummaryBytes {
		return "", model.NewKATError(model.ExitCodeArtifactError, "write summary json", fmt.Errorf("summary json exceeds %d bytes", safety.MaxSummaryBytes))
	}
	written := append(data, '\n')
	if err := os.WriteFile(path, written, 0o644); err != nil {
		return "", model.NewKATError(model.ExitCodeArtifactError, "write summary json", err)
	}
	return SHA256(written), nil
}

func WriteStatusJSON(path string, status model.Status) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, "marshal status json", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, "write status json", err)
	}
	return nil
}

func WriteSummaryMarkdown(path string, summary model.Summary) error {
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
	if err := os.WriteFile(path, []byte(markdown), 0o644); err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, "write summary markdown", err)
	}
	return nil
}

func WriteExcerpt(path string, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return model.NewKATError(model.ExitCodeArtifactError, "write excerpt", err)
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
