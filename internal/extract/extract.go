package extract

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kkachi-agent-tester/internal/model"
	"kkachi-agent-tester/internal/safety"
)

var (
	genericFailureMarkers = []string{"Error:", "TypeError:", "ReferenceError:", "AssertionError:", "panic:", "Traceback", "FAIL", "FAILED", "✗"}
	fileLineRE            = regexp.MustCompile(`([^\s:]+\.[A-Za-z0-9]+):(\d+)(?::(\d+))?`)
	testNameRE            = regexp.MustCompile(`^\s*[×✗-]\s+(.+)$`)
)

type lineIndex struct {
	text  string
	start int
	end   int
}

func Process(raw []byte, run model.RunOutput, rules []model.Rule) (model.RunOutput, error) {
	text := string(raw)
	if err := safety.EnsureInputWithinLimit(text); err != nil {
		return run, err
	}
	lines := buildLineIndex(text)
	failures := applyRules(lines, text, rules)
	if len(failures) == 0 {
		failures = parserFailures(run.Metadata.Parser, lines, text)
	}
	warnings := genericWarnings(lines)
	for i := range failures {
		failures[i].ID = fmt.Sprintf("F%03d", i+1)
		failures[i].Kind = "test_failure"
	}
	for i := range warnings {
		warnings[i].ID = fmt.Sprintf("W%03d", i+1)
	}
	run.Failures = failures
	run.Warnings = warnings
	run.ExtractorStatus = extractorStatus(run.Status, failures)
	return run, nil
}

func buildLineIndex(text string) []lineIndex {
	parts := strings.SplitAfter(text, "\n")
	if len(parts) == 0 {
		return nil
	}
	lines := make([]lineIndex, 0, len(parts))
	offset := 0
	for _, part := range parts {
		trimmed := strings.TrimSuffix(part, "\n")
		lines = append(lines, lineIndex{text: trimmed, start: offset, end: offset + len(trimmed)})
		offset += len(part)
	}
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		last := lines[len(lines)-1]
		last.end = len(text)
		lines[len(lines)-1] = last
	}
	return lines
}

func applyRules(lines []lineIndex, text string, rules []model.Rule) []model.Failure {
	failures := make([]model.Failure, 0)
	seen := map[string]bool{}
	for _, rule := range rules {
		startRE, _ := regexp.Compile(rule.Match.Start.Regex)
		endREs := make([]*regexp.Regexp, 0, len(rule.Match.End.AnyOf))
		for _, expr := range rule.Match.End.AnyOf {
			re, _ := regexp.Compile(expr.Regex)
			endREs = append(endREs, re)
		}
		var fileRE, testRE *regexp.Regexp
		if rule.Extract.FileLine.Regex != "" {
			fileRE, _ = regexp.Compile(rule.Extract.FileLine.Regex)
		}
		if rule.Extract.TestName.Regex != "" {
			testRE, _ = regexp.Compile(rule.Extract.TestName.Regex)
		}
		for idx, line := range lines {
			if !startRE.MatchString(line.text) {
				continue
			}
			startLine := max(0, idx-rule.Match.IncludeContext.Before)
			endLine := min(len(lines)-1, idx+rule.Match.End.MaxBlockLines)
			for j := idx; j <= endLine; j++ {
				if j > idx && (lines[j].text == "" || matchesAny(lines[j].text, endREs)) {
					endLine = min(len(lines)-1, j+rule.Match.IncludeContext.After)
					break
				}
			}
			span := spanFor(lines, startLine, endLine)
			key := fmt.Sprintf("%d:%d", span.StartByte, span.EndByte)
			if seen[key] {
				continue
			}
			seen[key] = true
			segment := sliceText(text, span)
			signature := strings.TrimSpace(lines[idx].text)
			failure := model.Failure{Signature: signature, RawSpan: span, StackTop: stackTop(segment)}
			if fileRE != nil {
				captureNamed(fileRE, segment, &failure)
			}
			if testRE != nil {
				captureTestName(testRE, segment, &failure)
			}
			failures = append(failures, failure)
		}
	}
	return failures
}

func genericFailures(lines []lineIndex) []model.Failure {
	failures := make([]model.Failure, 0)
	seen := map[string]bool{}
	for idx, line := range lines {
		if !containsAny(line.text, genericFailureMarkers) {
			continue
		}
		startLine := max(0, idx-3)
		endLine := min(len(lines)-1, idx+12)
		for j := idx + 1; j <= endLine; j++ {
			if lines[j].text == "" {
				endLine = min(len(lines)-1, j+2)
				break
			}
		}
		span := spanFor(lines, startLine, endLine)
		key := fmt.Sprintf("%d:%d", span.StartByte, span.EndByte)
		if seen[key] {
			continue
		}
		seen[key] = true
		failure := model.Failure{Signature: strings.TrimSpace(line.text), RawSpan: span}
		segment := joinLines(lines[startLine : endLine+1])
		captureFileLine(fileLineRE, segment, &failure)
		captureTestName(testNameRE, segment, &failure)
		failure.StackTop = stackTop(segment)
		failures = append(failures, failure)
	}
	return failures
}

func genericWarnings(lines []lineIndex) []model.Warning {
	warnings := make([]model.Warning, 0)
	for idx, line := range lines {
		lower := strings.ToLower(line.text)
		if !strings.Contains(lower, "warning") && !strings.Contains(lower, "deprecated") {
			continue
		}
		span := spanFor(lines, idx, idx)
		warnings = append(warnings, model.Warning{Signature: strings.TrimSpace(line.text), RawSpan: span})
	}
	return warnings
}

func extractorStatus(status model.RunStatus, failures []model.Failure) model.ExtractorStatus {
	if len(failures) == 0 {
		if status == model.RunStatusFailed || status == model.RunStatusTimedOut || status == model.RunStatusKilled {
			return model.ExtractorStatusDegraded
		}
		return model.ExtractorStatusNoMatch
	}
	precise := true
	for _, failure := range failures {
		if failure.File == "" && failure.TestName == "" {
			precise = false
			break
		}
	}
	if precise {
		return model.ExtractorStatusPrecise
	}
	return model.ExtractorStatusPartial
}

func spanFor(lines []lineIndex, startLine, endLine int) model.RawSpan {
	return model.RawSpan{StartLine: startLine + 1, EndLine: endLine + 1, StartByte: lines[startLine].start, EndByte: lines[endLine].end}
}

func joinLines(lines []lineIndex) string {
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, line.text)
	}
	return strings.Join(parts, "\n")
}

func sliceText(text string, span model.RawSpan) string {
	if span.StartByte < 0 || span.EndByte > len(text) || span.StartByte >= span.EndByte {
		return ""
	}
	return text[span.StartByte:span.EndByte]
}

func captureNamed(re *regexp.Regexp, text string, failure *model.Failure) {
	match := re.FindStringSubmatch(text)
	if len(match) == 0 {
		return
	}
	names := re.SubexpNames()
	for idx, name := range names {
		switch name {
		case "file":
			failure.File = match[idx]
		case "line":
			if value, err := strconv.Atoi(match[idx]); err == nil {
				failure.Line = value
			}
		}
	}
}

func captureFileLine(re *regexp.Regexp, text string, failure *model.Failure) {
	match := re.FindStringSubmatch(text)
	if len(match) < 3 {
		return
	}
	failure.File = match[1]
	if value, err := strconv.Atoi(match[2]); err == nil {
		failure.Line = value
	}
}

func captureTestName(re *regexp.Regexp, text string, failure *model.Failure) {
	for _, line := range strings.Split(text, "\n") {
		match := re.FindStringSubmatch(line)
		if len(match) >= 2 {
			failure.TestName = strings.TrimSpace(match[1])
			return
		}
	}
}

func stackTop(text string) []string {
	matches := fileLineRE.FindAllString(text, 2)
	if len(matches) == 0 {
		return nil
	}
	return matches
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func matchesAny(text string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
