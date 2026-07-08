package artifacts

import (
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
	if _, err := WriteSummaryJSON(filepath.Join(dir, "summary.json"), summary); err == nil {
		t.Fatal("expected oversized summary json to fail")
	}
}
