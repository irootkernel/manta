package safety

import (
	"testing"

	"github.com/irootkernel/manta/internal/model"
)

func TestRedactorUsesConfiguredOrder(t *testing.T) {
	t.Parallel()
	patterns := []model.RedactionPattern{
		{Name: "token", Regex: `token=[^ ]+`, Replace: "token=<redacted>"},
		{Name: "label", Regex: `<redacted>`, Replace: "safe"},
	}

	redactor, err := NewRedactor(patterns)
	if err != nil {
		t.Fatal(err)
	}
	got := redactor.Apply("token=secret unchanged")
	if got != "token=safe unchanged" {
		t.Fatalf("unexpected redaction result %q", got)
	}
	if got := redactor.Apply("plain metadata"); got != "plain metadata" {
		t.Fatalf("unexpected unmatched redaction result %q", got)
	}
}

func TestNewRedactorRejectsInvalidRegex(t *testing.T) {
	t.Parallel()
	if _, err := NewRedactor([]model.RedactionPattern{{Regex: "("}}); err == nil {
		t.Fatal("expected invalid redaction regex to fail")
	}
}
