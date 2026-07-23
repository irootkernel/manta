package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDocumentedCLIWorkflowAgainstFreshFixture(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	userInterface := readDocumentation(t, filepath.Join(root, "docs", "user-interface.md"))
	runDocumentationBlock(t, bin, repo, 0, markdownCodeBlockAfter(t, userInterface, "## Tested setup fixture", "bash"))

	version := runDocumentationBlock(t, bin, repo, 0, markdownCodeBlockAfter(t, userInterface, "Use either version surface", "bash"))
	versionLines := strings.Split(strings.TrimSpace(version), "\n")
	if len(versionLines) != 2 {
		t.Fatalf("unexpected version output %q", version)
	}
	const versionPrefix = "manta v"
	if !strings.HasPrefix(versionLines[0], versionPrefix) {
		t.Fatalf("unexpected human version output %q", versionLines[0])
	}
	humanVersion := strings.TrimPrefix(versionLines[0], versionPrefix)
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?(\+[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$`).MatchString(humanVersion) {
		t.Fatalf("human version is not semantic: %q", humanVersion)
	}
	var jsonVersion struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(versionLines[1]), &jsonVersion); err != nil {
		t.Fatalf("decode JSON version output %q: %v", versionLines[1], err)
	}
	if jsonVersion.Name != "manta" || jsonVersion.Version != humanVersion {
		t.Fatalf("version surfaces disagree: human=%q JSON=%+v", versionLines[0], jsonVersion)
	}
	if got := strings.Count(userInterface, `cli_version: "`+humanVersion+`"`); got != 2 {
		t.Fatalf("user interface contains %d toolchain metadata versions for %s, want 2", got, humanVersion)
	}
	for _, option := range []string{"--verbose", "--no-color"} {
		output := runDocumentedCommand(t, bin, repo, 2, option, "--version")
		if !strings.Contains(output, "flag provided but not defined") {
			t.Fatalf("unsupported option %s did not fail closed: %s", option, output)
		}
	}

	configured := runDocumentationBlock(t, bin, repo, 1,
		markdownCodeBlockAfter(t, userInterface, "Configured run with deterministic artifact paths", "bash"))
	if !strings.Contains(configured, "Status: failed") {
		t.Fatalf("configured output does not report failure: %s", configured)
	}
	assertDocumentedArtifacts(t, repo, "example-run")

	adhoc := runDocumentationBlock(t, bin, repo, 1,
		markdownCodeBlockAfter(t, userInterface, "Ad-hoc run without project config commands", "bash"))
	if !strings.Contains(adhoc, "Status: failed") {
		t.Fatalf("ad-hoc output does not report failure: %s", adhoc)
	}

	summarized := runDocumentationBlock(t, bin, repo, 0,
		markdownCodeBlockAfter(t, userInterface, "Summarize an existing raw log without rerunning the command", "bash"))
	if !strings.Contains(summarized, "Summary: .manta/runs/scoped/summarize-example/artifacts/test/unit.summary.md") {
		t.Fatalf("summarize output does not report documented summary: %s", summarized)
	}
	assertDocumentedArtifacts(t, repo, "summarize-example")
	markdownPath := filepath.Join(repo, ".manta", "runs", "scoped", "summarize-example", "artifacts", "test", "unit.summary.md")
	markdownData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatal(err)
	}
	shaLine := regexp.MustCompile(`(?m)^Raw log SHA-256: sha256:[0-9a-f]+$`)
	gotMarkdown := shaLine.ReplaceAllString(string(markdownData), "Raw log SHA-256: sha256:...")
	wantMarkdown := markdownCodeBlockAfter(t, userInterface, "## Markdown summary shape", "markdown")
	if gotMarkdown != wantMarkdown {
		t.Fatalf("generated Markdown does not match documentation\ngot:\n%s\nwant:\n%s", gotMarkdown, wantMarkdown)
	}

	excerpt := runDocumentationBlock(t, bin, repo, 0,
		markdownCodeBlockAfter(t, userInterface, "Deterministic excerpt lookup after either", "bash"))
	if !strings.Contains(excerpt, "token=<redacted>") || strings.Contains(excerpt, "token=secret") {
		t.Fatalf("unexpected excerpt output %q", excerpt)
	}

	jsonOutput := runDocumentationBlock(t, bin, repo, 0,
		markdownCodeBlockAfter(t, userInterface, "Compact JSON output for scripts", "bash"))
	var result binaryRunResult
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		t.Fatalf("decode summarize JSON %q: %v", jsonOutput, err)
	}
	for _, path := range []string{result.Summary, result.RawLog} {
		if !strings.HasPrefix(path, "evidence/runs/") {
			t.Fatalf("output-dir artifact %q is outside documented layout", path)
		}
	}

	ruleWorkflow := runDocumentationBlock(t, bin, repo, 0,
		markdownCodeBlockAfter(t, userInterface, "Fixture-backed rule workflow examples", "bash"))
	for _, want := range []string{"PASS generic-v1 expected=2:5 actual=2:5", "Proposed rule:", ".manta/rule-proposals/", "disabled"} {
		if !strings.Contains(ruleWorkflow, want) {
			t.Fatalf("rule workflow output missing %q: %s", want, ruleWorkflow)
		}
	}
	storedRule, err := os.ReadFile(filepath.Join(repo, ".manta", "tester", "rules", "generic-v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	storedRuleText := string(storedRule)
	for _, want := range []string{"reason: fixture-backed rule updated", "confidence: high", "deletion_reason: superseded by v2"} {
		if !strings.Contains(storedRuleText, want) {
			t.Fatalf("updated rule does not persist %q:\n%s", want, storedRule)
		}
	}

	implementationNote := readDocumentation(t, filepath.Join(root, "docs", "implementation-note.md"))
	vitestRulePath := filepath.Join(repo, "vitest-empty-state-v1.yaml")
	if err := os.WriteFile(vitestRulePath, []byte(markdownCodeBlockAfter(t, implementationNote, "Fixture-backed example using the Vitest log", "yaml")), 0o644); err != nil {
		t.Fatal(err)
	}
	runDocumentedCommand(t, bin, repo, 0, "rules", "create", "--file", vitestRulePath)
	runDocumentedCommand(t, bin, repo, 0, "rules", "test", "--rule", "vitest-empty-state-v1", "--log",
		filepath.Join(root, "internal", "extract", "testdata", "vitest.raw.log"), "--expect-span", "6:15")

	readme := readDocumentation(t, filepath.Join(root, "README.md"))
	integrationGuide := readDocumentation(t, filepath.Join(root, "docs", "integration-guide.md"))
	documentationIndex := readDocumentation(t, filepath.Join(root, "docs", "README.md"))
	for _, want := range []string{"@v" + humanVersion, "VERSION=" + humanVersion, "/v" + humanVersion + "/bin/"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README does not match binary version %s: missing %q", humanVersion, want)
		}
	}
	for name, document := range map[string]string{
		"integration guide":   integrationGuide,
		"documentation index": documentationIndex,
	} {
		if !strings.Contains(document, "v"+humanVersion) {
			t.Fatalf("%s does not match binary version %s", name, humanVersion)
		}
	}
	readmeRepo := t.TempDir()
	quickStart := markdownBashBlocksInSection(t, readme, "## Try it in five minutes")
	if len(quickStart) != 3 {
		t.Fatalf("README five-minute example contains %d bash blocks, want 3", len(quickStart))
	}
	for i, expectedExit := range []int{0, 1, 0} {
		output := runDocumentationBlock(t, bin, readmeRepo, expectedExit, quickStart[i])
		if i == 2 && (!strings.Contains(output, "# Manta Summary: demo") || !strings.Contains(output, "token=<redacted>")) {
			t.Fatalf("README summary inspection did not show the documented redacted evidence: %s", output)
		}
	}
	entries, err := os.ReadDir(filepath.Join(readmeRepo, ".manta", "runs", "standalone"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("README example created %d standalone runs, want 1", len(entries))
	}
	readmeRunDir := filepath.Join(readmeRepo, ".manta", "runs", "standalone", entries[0].Name())
	for _, name := range []string{"demo.raw.log", "demo.summary.json", "demo.summary.md", "demo.status.json", "excerpts/F001.log"} {
		if _, err := os.Stat(filepath.Join(readmeRunDir, filepath.FromSlash(name))); err != nil {
			t.Fatalf("README example artifact %s is missing: %v", name, err)
		}
	}

	integrationRepo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(integrationRepo, ".manta"), 0o755); err != nil {
		t.Fatal(err)
	}
	integrationConfig := markdownCodeBlockAfter(t, integrationGuide, "Create `.manta/tester.yaml`", "yaml")
	if err := os.WriteFile(filepath.Join(integrationRepo, ".manta", "tester.yaml"), []byte(integrationConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(integrationRepo, "go.mod"), []byte("module example.com/manta-docs\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(integrationRepo, "documented_test.go"), []byte("package docs\n\nimport \"testing\"\n\nfunc TestDocumentedIntegration(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	integrationOutput := runDocumentedCommand(t, bin, integrationRepo, 0, "run", "unit")
	if !strings.Contains(integrationOutput, "Status: passed") {
		t.Fatalf("integration guide config did not produce a passing configured run: %s", integrationOutput)
	}
}

func runDocumentedCommand(t *testing.T, bin, repo string, expectedExit int, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"--repo", repo}, args...)
	cmd := exec.Command(bin, commandArgs...)
	cmd.Dir = repo
	return runExpectedExit(t, cmd, expectedExit)
}

func assertDocumentedArtifacts(t *testing.T, repo, runID string) {
	t.Helper()
	base := filepath.Join(repo, ".manta", "runs", "scoped", runID, "artifacts", "test")
	for _, name := range []string{"unit.raw.log", "unit.summary.json", "unit.summary.md", "unit.status.json", "excerpts/F001.log"} {
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(name))); err != nil {
			t.Fatalf("documented artifact %s is missing: %v", name, err)
		}
	}
}

func readDocumentation(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func markdownCodeBlockAfter(t *testing.T, document, anchor, language string) string {
	t.Helper()
	anchorIndex := strings.Index(document, anchor)
	if anchorIndex < 0 {
		t.Fatalf("documentation anchor %q not found", anchor)
	}
	blocks := markdownCodeBlocks(t, document[anchorIndex+len(anchor):], language)
	if len(blocks) == 0 {
		t.Fatalf("%s code block after %q not found", language, anchor)
	}
	return blocks[0]
}

func markdownBashBlocksInSection(t *testing.T, document, heading string) []string {
	t.Helper()
	start := strings.Index(document, heading)
	if start < 0 {
		t.Fatalf("documentation heading %q not found", heading)
	}
	section := document[start+len(heading):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return markdownCodeBlocks(t, section, "bash")
}

func markdownCodeBlocks(t *testing.T, document, language string) []string {
	t.Helper()
	fence := "```" + language + "\n"
	var blocks []string
	for {
		start := strings.Index(document, fence)
		if start < 0 {
			return blocks
		}
		document = document[start+len(fence):]
		end := strings.Index(document, "```\n")
		if end < 0 {
			t.Fatalf("unterminated %s code block", language)
		}
		blocks = append(blocks, document[:end])
		document = document[end+len("```\n"):]
	}
}

func runDocumentationBlock(t *testing.T, bin, repo string, expectedExit int, block string) string {
	t.Helper()
	cmd := exec.Command("sh", "-eu", "-c", block)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	return runExpectedExit(t, cmd, expectedExit)
}

func runExpectedExit(t *testing.T, cmd *exec.Cmd, expectedExit int) string {
	t.Helper()
	output, err := cmd.CombinedOutput()
	if expectedExit == 0 {
		if err != nil {
			t.Fatalf("command %q failed: %v\n%s", cmd.Args, err, output)
		}
	} else {
		requireExitCode(t, err, expectedExit, output)
	}
	return string(output)
}
