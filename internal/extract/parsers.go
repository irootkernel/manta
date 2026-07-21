package extract

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/irootkernel/manta/internal/model"
)

var (
	vitestFailRE     = regexp.MustCompile(`^\s*FAIL\s+(.+?)(?:\s+>\s+(.+))?$`)
	vitestCaseRE     = regexp.MustCompile(`^\s*[×✗]\s+(.+?)(?:\s+\d+ms)?$`)
	pytestSummaryRE  = regexp.MustCompile(`^FAILED\s+([^\s:]+(?:/[^\s:]+)*)::([^\s]+)\s+-\s+(.+)$`)
	goTestFailRE     = regexp.MustCompile(`^--- FAIL: ([^(\s]+)`)
	playwrightFailRE = regexp.MustCompile(`^\s*\d+\)\s+\[[^\]]+\]\s+›\s+(.+?):(\d+):\d+\s+›\s+(.+?)\s+─*$`)
)

func parserFailures(parser string, lines []lineIndex, text string) []model.Failure {
	switch parser {
	case "vitest":
		return vitestFailures(lines, text)
	case "pytest":
		return pytestFailures(lines, text)
	case "go-test":
		return goTestFailures(lines, text)
	case "playwright":
		return playwrightFailures(lines, text)
	default:
		return genericFailures(lines)
	}
}

func vitestFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := vitestFailRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		endLine := spanUntilBlank(lines, idx, 8)
		span := spanFor(lines, idx, endLine)
		segment := sliceText(text, span)
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), RawSpan: span, StackTop: stackTop(segment)}
		failure.File = strings.TrimSpace(match[1])
		if len(match) > 2 {
			failure.TestName = strings.TrimSpace(match[2])
		}
		if failure.TestName == "" {
			captureTestName(vitestCaseRE, segment, &failure)
		}
		captureFileLine(fileLineRE, segment, &failure)
		failures = append(failures, failure)
	}
	return failures
}

func pytestFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := pytestSummaryRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		span := spanFor(lines, idx, idx)
		failure := model.Failure{Signature: strings.TrimSpace(match[3]), RawSpan: span}
		failure.File = strings.TrimSpace(match[1])
		failure.TestName = strings.TrimSpace(match[2])
		failures = append(failures, failure)
	}
	return failures
}

func goTestFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := goTestFailRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		endLine := spanUntilBlank(lines, idx, 4)
		span := spanFor(lines, idx, endLine)
		segment := sliceText(text, span)
		failure := model.Failure{Signature: strings.TrimSpace(line.text), RawSpan: span, TestName: strings.TrimSpace(match[1]), StackTop: stackTop(segment)}
		captureFileLine(fileLineRE, segment, &failure)
		failures = append(failures, failure)
	}
	return failures
}

func playwrightFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := playwrightFailRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		endLine := spanUntilBlank(lines, idx, 8)
		span := spanFor(lines, idx, endLine)
		segment := sliceText(text, span)
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), RawSpan: span, File: strings.TrimSpace(match[1]), TestName: strings.TrimSpace(match[3]), StackTop: stackTop(segment)}
		if lineNo, err := strconv.Atoi(match[2]); err == nil {
			failure.Line = lineNo
		}
		failures = append(failures, failure)
	}
	return failures
}

func spanUntilBlank(lines []lineIndex, start, maxAhead int) int {
	end := min(len(lines)-1, start+maxAhead)
	for idx := start + 1; idx <= end; idx++ {
		if strings.TrimSpace(lines[idx].text) == "" {
			return idx
		}
	}
	return end
}

func firstMeaningfulLine(segment, fallback string) string {
	for _, line := range strings.Split(segment, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "Error") || strings.Contains(trimmed, "expect(") || strings.HasPrefix(trimmed, "FAILED") {
			return trimmed
		}
	}
	return strings.TrimSpace(fallback)
}
