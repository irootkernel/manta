package tagset

import (
	"slices"
	"testing"
)

func TestCanonicalizeSortsAndDeduplicates(t *testing.T) {
	t.Parallel()
	got, err := Canonicalize([]string{"unit", "go", "unit"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"go", "unit"}) {
		t.Fatalf("Canonicalize() = %q, want [go unit]", got)
	}
}

func TestValidateRejectsMissingAndUnsafeTags(t *testing.T) {
	t.Parallel()
	for _, values := range [][]string{nil, {}, {"unit/test"}, {" unit"}} {
		if err := Validate(values); err == nil {
			t.Fatalf("expected %q to fail", values)
		}
	}
}
