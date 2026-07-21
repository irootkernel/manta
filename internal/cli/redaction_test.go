package cli

import (
	"testing"

	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

func TestRedactSummaryCoversSurfacedMetadata(t *testing.T) {
	t.Parallel()
	summary := model.Summary{
		CommandID:   "command_secret_id",
		Lane:        "lane-secret_lane",
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
		{"lane", summary.Lane, "lane-<redacted>"},
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
