package tagset

import (
	"fmt"
	"slices"

	"github.com/irootkernel/manta/internal/safety"
)

func Canonicalize(values []string) ([]string, error) {
	if err := Validate(values); err != nil {
		return nil, err
	}
	return Normalize(values), nil
}

func Validate(values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("at least one tag is required")
	}
	for _, value := range values {
		if err := safety.ValidateArtifactIdentifier("tag", value); err != nil {
			return err
		}
	}
	return nil
}

func Normalize(values []string) []string {
	normalized := slices.Clone(values)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}
