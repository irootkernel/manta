package safety

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/SeventeenthEarth/kkachi-agent-tester/internal/model"
)

const (
	MaxRegexInputBytes = 256 * 1024
	MaxSummaryBytes    = 64 * 1024
	MaxExcerptBytes    = 16 * 1024
	MaxBlockLines      = 160
)

func ValidateRegex(regex string) error {
	if _, err := regexp.Compile(regex); err != nil {
		return model.NewKATError(model.ExitCodeConfigError, "validate regex", err)
	}
	return nil
}

func BoundBytes(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit]
}

func BoundLines(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	return append([]string(nil), lines[:limit]...)
}

func ApplyRedaction(text string, patterns []model.RedactionPattern) (string, error) {
	redacted := text
	for _, pattern := range patterns {
		if err := ValidateRegex(pattern.Regex); err != nil {
			return "", err
		}
		re, _ := regexp.Compile(pattern.Regex)
		redacted = re.ReplaceAllString(redacted, pattern.Replace)
	}
	return redacted, nil
}

func FilterNoise(text string, filters []string) string {
	if len(filters) == 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		drop := false
		for _, filter := range filters {
			if filter != "" && strings.Contains(line, filter) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func EnsureInputWithinLimit(text string) error {
	if len(text) > MaxRegexInputBytes {
		return model.NewKATError(model.ExitCodeParserError, "regex input bound", fmt.Errorf("input exceeds %d bytes", MaxRegexInputBytes))
	}
	return nil
}
