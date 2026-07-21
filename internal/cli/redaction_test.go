package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/manta/internal/artifacts"
	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

func TestRedactSummaryCoversSurfacedMetadata(t *testing.T) {
	t.Parallel()
	summary := model.Summary{
		CommandID:   "command_secret_id",
		Tags:        []string{"unit", "tag-secret_tag"},
		Parser:      "secret_parser",
		CommandArgv: []string{"runner", "secret_arg"},
		RawLog:      ".manta/runs/standalone/secret_path/unit.raw.log",
		Failures: []model.Failure{{
			Signature: "secret_failure",
			File:      "src/secret_path/test.go",
			TestName:  "secret_test",
			Excerpt:   "excerpts/secret_ref.log",
			StackTop:  []string{"src/secret_stack/test.go:42"},
		}},
		Warnings: []model.Warning{{Signature: "secret_warning"}},
	}
	patterns := []model.RedactionPattern{{
		Name:    "secret",
		Regex:   `secret_[a-z_]+`,
		Replace: "<redacted>",
	}}

	redactor, err := safety.NewRedactor(patterns)
	if err != nil {
		t.Fatal(err)
	}
	redactSummary(&summary, redactor, nil)
	checks := []struct {
		label string
		got   string
		want  string
	}{
		{"command id", summary.CommandID, "command_<redacted>"},
		{"tag", summary.Tags[0], "tag-<redacted>"},
		{"parser", summary.Parser, "<redacted>"},
		{"argv", summary.CommandArgv[1], "<redacted>"},
		{"failure signature", summary.Failures[0].Signature, "<redacted>"},
		{"failure file", summary.Failures[0].File, "src/<redacted>/test.go"},
		{"failure test", summary.Failures[0].TestName, "<redacted>"},
		{"failure stack", summary.Failures[0].StackTop[0], "src/<redacted>/test.go:42"},
		{"warning signature", summary.Warnings[0].Signature, "<redacted>"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("unexpected redacted %s: got %q want %q", check.label, check.got, check.want)
		}
	}
	if summary.RawLog != ".manta/runs/standalone/secret_path/unit.raw.log" || summary.Failures[0].Excerpt != "excerpts/secret_ref.log" {
		t.Fatalf("artifact references must remain literal: raw=%q excerpt=%q", summary.RawLog, summary.Failures[0].Excerpt)
	}
}

func TestCloneFailuresDoesNotShareStackTop(t *testing.T) {
	t.Parallel()
	original := []model.Failure{{Signature: "failure", StackTop: []string{"src/original.go:1"}}}
	cloned := cloneFailures(original)

	cloned[0].StackTop[0] = "src/changed.go:2"

	if original[0].StackTop[0] != "src/original.go:1" {
		t.Fatalf("clone mutation changed original stack: %+v", original[0].StackTop)
	}
}

func TestMaterializeArtifactsBoundsFilteredAndExpandedWarnings(t *testing.T) {
	t.Parallel()
	filteredWarnings := make([]model.Warning, 0, 120)
	for i := 1; i <= 60; i++ {
		filteredWarnings = append(filteredWarnings, model.Warning{ID: fmt.Sprintf("W%03d", i), Signature: "noise: warning"})
	}
	for i := 61; i <= 120; i++ {
		filteredWarnings = append(filteredWarnings, model.Warning{ID: fmt.Sprintf("W%03d", i), Signature: "warning: useful"})
	}
	tests := []struct {
		name          string
		config        model.Config
		warnings      []model.Warning
		wantCount     int
		wantFirstID   string
		wantLastID    string
		wantSignature string
	}{
		{
			name:          "noise filtering precedes record cap",
			config:        model.Config{NoiseFilters: []string{"noise:"}},
			warnings:      filteredWarnings,
			wantCount:     safety.MaxSummaryWarnings,
			wantFirstID:   "W061",
			wantLastID:    "W110",
			wantSignature: "warning: useful",
		},
		{
			name: "redaction expansion precedes byte budget",
			config: model.Config{Redaction: model.RedactionConfig{Patterns: []model.RedactionPattern{{
				Name:    "expand",
				Regex:   "EXPAND",
				Replace: strings.Repeat("x", safety.MaxSummaryBytes),
			}}}},
			warnings: []model.Warning{
				{ID: "W001", Signature: "warning: useful"},
				{ID: "W002", Signature: "warning: EXPAND"},
			},
			wantCount:     1,
			wantFirstID:   "W001",
			wantLastID:    "W001",
			wantSignature: "warning: useful",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			paths, err := artifacts.PreparePaths(repo, "", "warning-bounds", "unit")
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("raw warning evidence\n")
			rawSHA, err := artifacts.WriteRawLog(paths, raw)
			if err != nil {
				t.Fatal(err)
			}
			runOutput := model.RunOutput{
				Metadata:        model.RunMetadata{CommandID: "unit", Tags: []string{"unit"}, Parser: "generic"},
				Status:          model.RunStatusPassed,
				ExtractorStatus: model.ExtractorStatusNoMatch,
				RawLogBytes:     raw,
				Warnings:        tt.warnings,
			}
			result, err := materializeArtifactsWithExtractor(
				model.RunRequest{RepoRoot: repo, RunID: "warning-bounds"},
				tt.config,
				paths,
				rawSHA,
				artifacts.Rel(repo, paths.RawLogPath),
				runOutput,
				nil,
				materializationExecutedCommand,
				func(_ []byte, output model.RunOutput, _ []model.Rule) (model.RunOutput, error) { return output, nil },
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != model.RunStatusPassed || result.ExitCode != 0 || result.Extractor != string(model.ExtractorStatusDegraded) {
				t.Fatalf("unexpected materialized result: %+v", result)
			}

			var summary model.Summary
			readJSONArtifact(t, filepath.Join(paths.BaseDir, "unit.summary.json"), &summary)
			if summary.WarningCount != tt.wantCount || len(summary.Warnings) != tt.wantCount || !summary.WarningsTruncated {
				t.Fatalf("unexpected warning bound: count=%d len=%d truncated=%t", summary.WarningCount, len(summary.Warnings), summary.WarningsTruncated)
			}
			if summary.Warnings[0].ID != tt.wantFirstID || summary.Warnings[len(summary.Warnings)-1].ID != tt.wantLastID {
				t.Fatalf("unexpected warning prefix: first=%q last=%q", summary.Warnings[0].ID, summary.Warnings[len(summary.Warnings)-1].ID)
			}
			for _, warning := range summary.Warnings {
				if warning.Signature != tt.wantSignature {
					t.Fatalf("unexpected retained warning: %+v", warning)
				}
			}
		})
	}
}
