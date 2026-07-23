package e2e

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/irootkernel/manta/internal/model"
	"github.com/irootkernel/manta/internal/safety"
)

func TestBinaryRejectsUnknownConfigFields(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
	configPath := filepath.Join(repo, ".manta", "tester.yaml")
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString("redactions:\n  patterns: []\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("append unknown config field: write=%v close=%v", writeErr, closeErr)
	}
	out, err := exec.Command(bin, "--repo", repo, "run", "unit").CombinedOutput()
	requireExitCode(t, err, int(model.ExitCodeConfigError), out)
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("command ran after invalid config: %v", err)
	}
}

func TestBinaryEnforcesRuleAndConfigInputSizeLimits(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)

	t.Run("config fails before command execution", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
		configPath := filepath.Join(repo, ".manta", "tester.yaml")
		config, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configPath, padYAMLToSize(config, safety.MaxConfigRuleInputBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}
		requireRunConfigErrorBeforeExecution(t, bin, repo)
	})

	t.Run("stored rule fails before command execution", func(t *testing.T) {
		repo := t.TempDir()
		writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
		rulesDir := filepath.Join(repo, ".manta", "tester", "rules")
		if err := os.MkdirAll(rulesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		rule := padYAMLToSize([]byte(auditRuleYAML("oversized-v1", 20, 0, 0)), safety.MaxConfigRuleInputBytes+1)
		if err := os.WriteFile(filepath.Join(rulesDir, "oversized-v1.yaml"), rule, 0o644); err != nil {
			t.Fatal(err)
		}
		requireRunConfigErrorBeforeExecution(t, bin, repo)
	})

	t.Run("rule source does not create rule", func(t *testing.T) {
		repo := t.TempDir()
		inputPath := filepath.Join(repo, "oversized.yaml")
		rule := padYAMLToSize([]byte(auditRuleYAML("oversized-v1", 20, 0, 0)), safety.MaxConfigRuleInputBytes+1)
		if err := os.WriteFile(inputPath, rule, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(bin, "--repo", repo, "rules", "create", "--file", inputPath).CombinedOutput()
		requireExitCode(t, err, int(model.ExitCodeConfigError), out)
		if _, err := os.Stat(filepath.Join(repo, ".manta", "tester", "rules", "oversized-v1.yaml")); !os.IsNotExist(err) {
			t.Fatalf("oversized source created rule: %v", err)
		}
	})

	t.Run("propose raw log does not create proposal", func(t *testing.T) {
		repo := t.TempDir()
		rawPath := filepath.Join(repo, "oversized.raw.log")
		if err := os.WriteFile(rawPath, bytes.Repeat([]byte("x"), safety.MaxConfigRuleInputBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(bin, "--repo", repo, "rules", "propose", "--tag", "unit", "--parser", "generic", "--raw-log", rawPath, "--span", "1:1").CombinedOutput()
		requireExitCode(t, err, int(model.ExitCodeConfigError), out)
		if _, err := os.Stat(filepath.Join(repo, ".manta", "rule-proposals")); !os.IsNotExist(err) {
			t.Fatalf("oversized raw log created proposal directory: %v", err)
		}
	})

	t.Run("rule test keeps parser-error contract", func(t *testing.T) {
		repo := t.TempDir()
		rulesDir := filepath.Join(repo, ".manta", "tester", "rules")
		if err := os.MkdirAll(rulesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rulesDir, "bounded-v1.yaml"), []byte(auditRuleYAML("bounded-v1", 20, 0, 0)), 0o644); err != nil {
			t.Fatal(err)
		}
		rawPath := filepath.Join(repo, "oversized.raw.log")
		if err := os.WriteFile(rawPath, bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(bin, "--repo", repo, "rules", "test", "--rule", "bounded-v1", "--log", rawPath, "--expect-span", "1:1").CombinedOutput()
		requireExitCode(t, err, int(model.ExitCodeParserError), out)
	})
}

func TestRequirementTraceabilityMatrixCoversCompletedRequirements(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	specData, err := os.ReadFile(filepath.Join(root, "docs", "requirements-specs.md"))
	if err != nil {
		t.Fatal(err)
	}
	matrixData, err := os.ReadFile(filepath.Join(root, "docs", "requirements-test-matrix.md"))
	if err != nil {
		t.Fatal(err)
	}
	testFunctions, err := repositoryTestFunctions(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, message := range requirementTraceabilityErrors(specData, matrixData, testFunctions, nonTestEvidenceRequirements) {
		t.Error(message)
	}
}

func TestRequirementTraceabilityAuditRejectsInvalidEvidence(t *testing.T) {
	t.Parallel()
	testFunctions := map[string]bool{"TestPresent": true}
	testCases := []struct {
		name       string
		spec       string
		matrix     string
		exceptions map[string]bool
		want       string
	}{
		{
			name:   "unresolved test citation",
			spec:   "- [x] `MANTA-REQ-RQCLI-001` Example.\n",
			matrix: "| `MANTA-REQ-RQCLI-001` | `TestMissing` |\n",
			want:   "references missing test TestMissing",
		},
		{
			name:   "unapproved non-test evidence",
			spec:   "- [x] `MANTA-REQ-RQCLI-001` Example.\n",
			matrix: "| `MANTA-REQ-RQCLI-001` | `make test` |\n",
			want:   "has no cited test and is not an explicit non-test exception",
		},
		{
			name:       "stale non-test exception",
			spec:       "- [x] `MANTA-REQ-RQDOC-004` Example.\n",
			matrix:     "| `MANTA-REQ-RQDOC-004` | `TestPresent` |\n",
			exceptions: map[string]bool{"MANTA-REQ-RQDOC-004": true},
			want:       "explicit non-test exception now cites a test",
		},
		{
			name:       "orphaned non-test exception",
			spec:       "- [x] `MANTA-REQ-RQCLI-001` Example.\n",
			matrix:     "| `MANTA-REQ-RQCLI-001` | `TestPresent` |\n",
			exceptions: map[string]bool{"MANTA-REQ-RQDOC-004": true},
			want:       "explicit non-test exception has no traceability row",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			messages := requirementTraceabilityErrors([]byte(testCase.spec), []byte(testCase.matrix), testFunctions, testCase.exceptions)
			if got := strings.Join(messages, "\n"); !strings.Contains(got, testCase.want) {
				t.Fatalf("expected %q in audit errors:\n%s", testCase.want, got)
			}
		})
	}
}

func TestRepositoryTestFunctions(t *testing.T) {
	t.Parallel()

	t.Run("finds only runnable Go tests", func(t *testing.T) {
		root := t.TempDir()
		writeAuditGoFixture(t, root, "functions_test.go", `package fixture

import "testing"

func TestValid(t *testing.T) {}

type suite struct{}

func TesticularCancer(t *testing.T) {}
func TestWrongSignature() {}
func TestReturnsValue(t *testing.T) error { return nil }
func TestTwoArguments(t *testing.T, value string) {}
func TestValueArgument(t testing.T) {}
func (suite) TestMethod(t *testing.T) {}
`)
		writeAuditGoFixture(t, root, "production.go", `package fixture

import "testing"

func TestProductionFunction(t *testing.T) {}
`)
		const ignoredTest = `package fixture

import "testing"

func TestIgnored(t *testing.T) {}
`
		for _, directory := range []string{".manta", "vendor"} {
			writeAuditGoFixture(t, root, filepath.Join(directory, "hidden_test.go"), ignoredTest)
		}

		functions, err := repositoryTestFunctions(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(functions) != 1 || !functions["TestValid"] {
			t.Fatalf("expected only TestValid, got %v", functions)
		}
	})

	t.Run("reports malformed test files", func(t *testing.T) {
		root := t.TempDir()
		writeAuditGoFixture(t, root, "malformed_test.go", "package fixture\nfunc TestMalformed(")
		if _, err := repositoryTestFunctions(root); err == nil || !strings.Contains(err.Error(), "parse Go test file") {
			t.Fatalf("expected Go test parse error, got %v", err)
		}
	})
}

func writeAuditGoFixture(t *testing.T, root, relativePath, contents string) {
	t.Helper()
	path := filepath.Join(root, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

var (
	completedRequirementPattern = regexp.MustCompile(`(?m)^- \[x\] \x60(MANTA-REQ-[A-Z0-9-]+)\x60`)
	traceabilityRowPattern      = regexp.MustCompile(`(?m)^\| \x60(MANTA-REQ-[A-Z0-9-]+)\x60 \| ([^|]+) \|$`)
	testCitationPattern         = regexp.MustCompile(`\x60(Test[A-Za-z0-9_]*)\x60`)
	nonTestEvidenceRequirements = map[string]bool{
		"MANTA-REQ-RQDOC-004": true,
		"MANTA-REQ-RQHAR-007": true,
	}
	skippedTestScanDirectories = map[string]bool{
		".git":                     true,
		".manta":                   true,
		".codegraph":               true,
		".omx":                     true,
		".omc":                     true,
		".external-review-sidecar": true,
		"bin":                      true,
		"vendor":                   true,
	}
)

func repositoryTestFunctions(root string) (map[string]bool, error) {
	testFunctions := make(map[string]bool)
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skippedTestScanDirectories[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse Go test file %s: %w", path, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && isRunnableGoTest(function) {
				testFunctions[function.Name.Name] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan repository Go tests: %w", err)
	}
	return testFunctions, nil
}

func isRunnableGoTest(function *ast.FuncDecl) bool {
	if function.Recv != nil || !isGoTestName(function.Name.Name) {
		return false
	}
	if function.Type.Results != nil && len(function.Type.Results.List) > 0 ||
		function.Type.Params.List == nil ||
		len(function.Type.Params.List) != 1 ||
		len(function.Type.Params.List[0].Names) > 1 {
		return false
	}
	pointer, ok := function.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	if identifier, ok := pointer.X.(*ast.Ident); ok {
		return identifier.Name == "T"
	}
	if selector, ok := pointer.X.(*ast.SelectorExpr); ok {
		return selector.Sel.Name == "T"
	}
	return false
}

func isGoTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") {
		return false
	}
	if len(name) == len("Test") {
		return true
	}
	firstRune, _ := utf8.DecodeRuneInString(name[len("Test"):])
	return !unicode.IsLower(firstRune)
}

func requirementTraceabilityErrors(specData, matrixData []byte, testFunctions map[string]bool, nonTestExceptions map[string]bool) []string {
	completed := make(map[string]bool)
	for _, match := range completedRequirementPattern.FindAllSubmatch(specData, -1) {
		completed[string(match[1])] = true
	}
	mapped := make(map[string]bool)
	var errors []string
	for _, match := range traceabilityRowPattern.FindAllSubmatch(matrixData, -1) {
		id := string(match[1])
		if mapped[id] {
			errors = append(errors, fmt.Sprintf("duplicate traceability row for %s", id))
		}
		mapped[id] = true
		evidence := strings.TrimSpace(string(match[2]))
		if evidence == "" {
			errors = append(errors, fmt.Sprintf("empty evidence for %s", id))
		}
		citations := testCitationPattern.FindAllStringSubmatch(evidence, -1)
		isNonTestException := nonTestExceptions[id]
		if len(citations) == 0 && !isNonTestException {
			errors = append(errors, fmt.Sprintf("traceability row %s has no cited test and is not an explicit non-test exception", id))
		}
		if len(citations) > 0 && isNonTestException {
			errors = append(errors, fmt.Sprintf("traceability row %s explicit non-test exception now cites a test", id))
		}
		for _, citation := range citations {
			name := citation[1]
			if !testFunctions[name] {
				errors = append(errors, fmt.Sprintf("traceability row %s references missing test %s", id, name))
			}
		}
	}
	for id := range completed {
		if !mapped[id] {
			errors = append(errors, fmt.Sprintf("completed requirement missing from traceability matrix: %s", id))
		}
	}
	for id := range mapped {
		if !completed[id] {
			errors = append(errors, fmt.Sprintf("traceability matrix references unknown or incomplete requirement: %s", id))
		}
	}
	if len(completed) != len(mapped) {
		errors = append(errors, fmt.Sprintf("traceability count mismatch: completed=%d mapped=%d", len(completed), len(mapped)))
	}
	for id := range nonTestExceptions {
		if !mapped[id] {
			errors = append(errors, fmt.Sprintf("explicit non-test exception has no traceability row: %s", id))
		}
	}
	return errors
}

func TestBinaryRuleTestDoesNotUseParserFallback(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".manta", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rule := strings.Join([]string{
		"id: never-match",
		"tags: [unit]",
		"parser: generic",
		"status: active",
		"provenance:",
		"  created_by: auditor",
		"  source_run: local-audit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 1",
		"    end_line: 1",
		"  reason: audit fixture",
		"match:",
		"  start:",
		"    regex: '^THIS-WILL-NEVER-MATCH$'",
		"  end:",
		"    max_block_lines: 8",
		"  include_context:",
		"    before: 0",
		"    after: 0",
		"extract: {}",
		"confidence: medium",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "never-match.yaml"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	if err := os.WriteFile(rawPath, []byte("Error: generic fallback must not count\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "--repo", repo, "rules", "test", "--rule", "never-match", "--log", rawPath, "--expect-span", "1:1").CombinedOutput()
	requireExitCode(t, err, int(model.ExitCodeParserError), out)
	if !strings.Contains(string(out), "produced no failures") {
		t.Fatalf("unexpected rule-test diagnostic: %s", out)
	}
}

func TestBinaryRejectsOversizedRuleContext(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)

	t.Run("create and update fail closed", func(t *testing.T) {
		repo := t.TempDir()
		validPath := filepath.Join(repo, "valid.yaml")
		invalidPath := filepath.Join(repo, "invalid.yaml")
		if err := os.WriteFile(validPath, []byte(auditRuleYAML("bounded-v1", 20, 0, 0)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(invalidPath, []byte(auditRuleYAML("bounded-v1", 160, 0, 1)), 0o644); err != nil {
			t.Fatal(err)
		}

		createOut, createErr := exec.Command(bin, "--repo", repo, "rules", "create", "--file", invalidPath).CombinedOutput()
		requireExitCode(t, createErr, int(model.ExitCodeConfigError), createOut)
		if _, err := os.Stat(filepath.Join(repo, ".manta", "tester", "rules", "bounded-v1.yaml")); !os.IsNotExist(err) {
			t.Fatalf("invalid create wrote a rule file: %v", err)
		}

		runExpectedExit(t, exec.Command(bin, "--repo", repo, "rules", "create", "--file", validPath), 0)
		updateOut, updateErr := exec.Command(bin, "--repo", repo, "rules", "update", "bounded-v1", "--file", invalidPath).CombinedOutput()
		requireExitCode(t, updateErr, int(model.ExitCodeConfigError), updateOut)
		stored, err := os.ReadFile(filepath.Join(repo, ".manta", "tester", "rules", "bounded-v1.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(stored), "max_block_lines: 160") {
			t.Fatalf("invalid update overwrote the valid rule:\n%s", stored)
		}
	})

	t.Run("test and run reject discovered invalid rule", func(t *testing.T) {
		repo := t.TempDir()
		rulesDir := filepath.Join(repo, ".manta", "tester", "rules")
		if err := os.MkdirAll(rulesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		invalid := auditRuleYAML("overflow-v1", 1, 0, int(^uint(0)>>1))
		if err := os.WriteFile(filepath.Join(rulesDir, "overflow-v1.yaml"), []byte(invalid), 0o644); err != nil {
			t.Fatal(err)
		}
		rawPath := filepath.Join(repo, "fixture.raw.log")
		if err := os.WriteFile(rawPath, []byte("MARKER\n\nafter\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		testOut, testErr := exec.Command(bin, "--repo", repo, "rules", "test", "--rule", "overflow-v1", "--log", rawPath, "--expect-span", "1:3").CombinedOutput()
		requireExitCode(t, testErr, int(model.ExitCodeConfigError), testOut)
		if strings.Contains(string(testOut), "panic:") {
			t.Fatalf("rules test panicked:\n%s", testOut)
		}

		writeE2EConfig(t, repo, "#!/bin/sh\ntouch command-ran\n")
		runOut, runErr := exec.Command(bin, "--repo", repo, "run", "unit").CombinedOutput()
		requireExitCode(t, runErr, int(model.ExitCodeConfigError), runOut)
		if strings.Contains(string(runOut), "panic:") {
			t.Fatalf("configured run panicked:\n%s", runOut)
		}
		if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
			t.Fatalf("command ran after invalid rule discovery: %v", err)
		}
	})
}

func TestBinaryPreservesConcurrentRuleProposals(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "fixture.raw.log")
	if err := os.WriteFile(rawPath, []byte("TypeError: boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	const count = 12
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(bin, "--repo", repo, "rules", "propose", "--tag", "unit", "--parser", "generic", "--raw-log", rawPath, "--span", "1:1")
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("%w: %s", err, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("proposal command failed: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(repo, ".manta", "rule-proposals"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != count {
		t.Fatalf("proposal files = %d, want %d", len(entries), count)
	}
}

func auditRuleYAML(id string, maxBlockLines, before, after int) string {
	return fmt.Sprintf(`id: %s
tags: [unit]
parser: generic
status: active
provenance:
  created_by: auditor
  source_run: local-audit
  source_command: unit
  source_log_sha256: sha256:abc
  source_span:
    start_line: 1
    end_line: 2
  reason: audit fixture
match:
  start:
    regex: '^MARKER$'
  end:
    any_of:
      - regex: '^$'
    max_block_lines: %d
  include_context:
    before: %d
    after: %d
extract: {}
confidence: medium
`, id, maxBlockLines, before, after)
}

func padYAMLToSize(data []byte, size int) []byte {
	padded := append(append([]byte(nil), data...), '#')
	return append(padded, bytes.Repeat([]byte("x"), size-len(padded))...)
}

func requireRunConfigErrorBeforeExecution(t *testing.T, bin, repo string) {
	t.Helper()
	out, err := exec.Command(bin, "--repo", repo, "run", "unit").CombinedOutput()
	requireExitCode(t, err, int(model.ExitCodeConfigError), out)
	if _, err := os.Stat(filepath.Join(repo, "command-ran")); !os.IsNotExist(err) {
		t.Fatalf("command ran after config error: %v", err)
	}
}
