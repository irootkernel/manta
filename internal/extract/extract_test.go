package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/safety"
)

func TestProcessGenericFailureProducesPreciseSpan(t *testing.T) {
	t.Parallel()
	raw := []byte("before\nTypeError: boom\nsrc/foo.test.ts:42:13\n- renders empty state\n")
	run := model.RunOutput{Status: model.RunStatusFailed}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if processed.ExtractorStatus != model.ExtractorStatusPrecise {
		t.Fatalf("expected precise extractor status, got %s", processed.ExtractorStatus)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "src/foo.test.ts" || failure.Line != 42 {
		t.Fatalf("unexpected file/line: %+v", failure)
	}
	if !strings.Contains(failure.TestName, "renders empty state") {
		t.Fatalf("expected test name capture, got %q", failure.TestName)
	}
}

func TestProcessExtractorStatusContract(t *testing.T) {
	t.Parallel()
	matched := []struct {
		name         string
		raw          string
		wantStatus   model.ExtractorStatus
		wantFailures int
	}{
		{
			name:         "precise generic match",
			raw:          "TypeError: boom\nsrc/foo.test.ts:42:13\n- renders empty state\n",
			wantStatus:   model.ExtractorStatusPrecise,
			wantFailures: 1,
		},
		{
			name:         "partial generic match",
			raw:          "Error: boom\n",
			wantStatus:   model.ExtractorStatusPartial,
			wantFailures: 1,
		},
	}
	for _, tt := range matched {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{
				Status:   model.RunStatusFailed,
				Metadata: model.RunMetadata{Parser: "generic"},
			}
			processed, err := Process([]byte(tt.raw), run, nil)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}
			if processed.ExtractorStatus != tt.wantStatus {
				t.Fatalf("expected extractor status %s, got %s", tt.wantStatus, processed.ExtractorStatus)
			}
			if len(processed.Failures) != tt.wantFailures {
				t.Fatalf("expected %d failures, got %d", tt.wantFailures, len(processed.Failures))
			}
		})
	}

	for _, status := range []model.RunStatus{
		model.RunStatusPassed,
		model.RunStatusFailed,
		model.RunStatusTimedOut,
		model.RunStatusKilled,
	} {
		t.Run("specialized parser miss after "+string(status), func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{
				Status:   status,
				Metadata: model.RunMetadata{Parser: "vitest"},
			}
			processed, err := Process([]byte("TypeError: boom\nsrc/foo.test.ts:42:13\n"), run, nil)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}
			wantStatus := model.ExtractorStatusDegraded
			if status == model.RunStatusPassed {
				wantStatus = model.ExtractorStatusNoMatch
			}
			if processed.Status != status || processed.ExtractorStatus != wantStatus {
				t.Fatalf("expected run/extractor status %s/%s, got %s/%s", status, wantStatus, processed.Status, processed.ExtractorStatus)
			}
			if len(processed.Failures) != 0 {
				t.Fatalf("specialized parser miss used generic fallback: %+v", processed.Failures)
			}
		})
	}
}

func TestProcessRuleAssistedExtraction(t *testing.T) {
	t.Parallel()
	raw := []byte("setup\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nsummary\n")
	rule := model.Rule{
		ID:     "generic-v1",
		Lane:   "unit",
		Parser: "generic",
		Status: model.RuleStatusActive,
		Match: model.RuleMatch{
			Start:          model.RuleRegex{Regex: `^TypeError:`},
			End:            model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: 20},
			IncludeContext: model.RuleContext{Before: 0, After: 0},
		},
		Extract: model.RuleExtract{
			FileLine: model.RuleExtractField{Regex: `(?P<file>[^\s:]+\.ts):(?P<line>\d+)`},
			TestName: model.RuleExtractField{Regex: `^\s*[✗×]\s+(?P<test>.+)$`},
		},
	}
	run := model.RunOutput{Status: model.RunStatusFailed}
	processed, err := Process(raw, run, []model.Rule{rule})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(processed.Failures))
	}
	if processed.Failures[0].File != "src/foo.ts" {
		t.Fatalf("expected rule-assisted file capture, got %+v", processed.Failures[0])
	}
}

func TestProcessRulesBoundsUnvalidatedContext(t *testing.T) {
	t.Parallel()
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i+1)
	}
	lines[99] = "MARKER"
	lines[100] = ""
	raw := []byte(strings.Join(lines, "\n"))
	maxInt := int(^uint(0) >> 1)
	rule := model.Rule{
		Match: model.RuleMatch{
			Start:          model.RuleRegex{Regex: `^MARKER$`},
			End:            model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: safety.MaxBlockLines},
			IncludeContext: model.RuleContext{Before: maxInt, After: maxInt},
		},
	}

	processed, err := ProcessRules(raw, model.RunOutput{Status: model.RunStatusFailed}, []model.Rule{rule})
	if err != nil {
		t.Fatalf("ProcessRules failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("failure count = %d, want 1", len(processed.Failures))
	}
	span := processed.Failures[0].RawSpan
	if got := span.EndLine - span.StartLine + 1; got != safety.MaxBlockLines {
		t.Fatalf("bounded span lines = %d, want %d: %+v", got, safety.MaxBlockLines, span)
	}
	if span.StartByte < 0 || span.EndByte > len(raw) || span.StartByte >= span.EndByte {
		t.Fatalf("invalid bounded byte span %+v for %d bytes", span, len(raw))
	}
}

func TestProcessVitestFixture(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "vitest.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "vitest"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one vitest failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "src/foo.ts" || failure.Line != 42 {
		t.Fatalf("expected vitest file/line capture, got %+v", failure)
	}
	if failure.TestName != "renders empty state" {
		t.Fatalf("expected vitest test name, got %q", failure.TestName)
	}
}

func TestProcessPytestFixture(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "pytest.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "pytest"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 2 {
		t.Fatalf("expected two pytest failures from repeated summary lines, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "tests/test_app.py" || failure.TestName != "test_empty_state" {
		t.Fatalf("expected pytest file/test capture, got %+v", failure)
	}
}

func TestProcessGoTestFixture(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "go-test.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "go-test"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one go-test failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "foo_test.go" || failure.Line != 42 || failure.TestName != "TestEmptyState" {
		t.Fatalf("expected go-test capture, got %+v", failure)
	}
}

func TestProcessPlaywrightFixture(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "playwright.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "playwright"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one playwright failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "tests/example.spec.ts" || failure.Line != 42 || failure.TestName != "renders empty state" {
		t.Fatalf("expected playwright capture, got %+v", failure)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
