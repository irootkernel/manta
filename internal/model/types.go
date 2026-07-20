package model

import "time"

type CommandConfig struct {
	Command    []string `yaml:"command" json:"command_argv"`
	Lane       string   `yaml:"lane" json:"lane"`
	Parser     string   `yaml:"parser" json:"parser"`
	TimeoutSec int      `yaml:"timeout_sec" json:"timeout_sec"`
}

type RedactionPattern struct {
	Name    string `yaml:"name" json:"name"`
	Regex   string `yaml:"regex" json:"regex"`
	Replace string `yaml:"replace" json:"replace"`
}

type RedactionConfig struct {
	Patterns []RedactionPattern `yaml:"patterns" json:"patterns"`
}

type Config struct {
	Version      int                      `yaml:"version" json:"version"`
	Commands     map[string]CommandConfig `yaml:"commands" json:"commands"`
	NoiseFilters []string                 `yaml:"noise_filters" json:"noise_filters"`
	Redaction    RedactionConfig          `yaml:"redaction" json:"redaction"`
}

type RuleStatus string

const (
	RuleStatusActive   RuleStatus = "active"
	RuleStatusDisabled RuleStatus = "disabled"
)

type RuleProvenance struct {
	CreatedBy       string  `yaml:"created_by" json:"created_by"`
	SourceRun       string  `yaml:"source_run" json:"source_run"`
	SourceCommand   string  `yaml:"source_command" json:"source_command"`
	SourceLogSHA256 string  `yaml:"source_log_sha256" json:"source_log_sha256"`
	SourceSpan      RawSpan `yaml:"source_span" json:"source_span"`
	Reason          string  `yaml:"reason" json:"reason"`
}

type RuleRegex struct {
	Regex string `yaml:"regex" json:"regex"`
}

type RuleEnd struct {
	AnyOf         []RuleRegex `yaml:"any_of" json:"any_of"`
	MaxBlockLines int         `yaml:"max_block_lines" json:"max_block_lines"`
}

type RuleContext struct {
	Before int `yaml:"before" json:"before"`
	After  int `yaml:"after" json:"after"`
}

type RuleMatch struct {
	Start          RuleRegex   `yaml:"start" json:"start"`
	End            RuleEnd     `yaml:"end" json:"end"`
	IncludeContext RuleContext `yaml:"include_context" json:"include_context"`
}

type RuleExtractField struct {
	Regex string `yaml:"regex" json:"regex"`
}

type RuleExtract struct {
	FileLine RuleExtractField `yaml:"file_line" json:"file_line"`
	TestName RuleExtractField `yaml:"test_name" json:"test_name"`
}

type Rule struct {
	ID             string         `yaml:"id" json:"id"`
	Lane           string         `yaml:"lane" json:"lane"`
	Parser         string         `yaml:"parser" json:"parser"`
	Status         RuleStatus     `yaml:"status" json:"status"`
	Provenance     RuleProvenance `yaml:"provenance" json:"provenance"`
	Match          RuleMatch      `yaml:"match" json:"match"`
	Extract        RuleExtract    `yaml:"extract" json:"extract"`
	Confidence     string         `yaml:"confidence" json:"confidence"`
	DeletionReason string         `yaml:"deletion_reason,omitempty" json:"deletion_reason,omitempty"`
	SourcePath     string         `yaml:"-" json:"source_path"`
}

type RunRequest struct {
	RepoRoot   string
	ConfigPath string
	OutputDir  string
	RunID      string
	JSON       bool
	NoColor    bool
	Verbose    bool

	Mode        RunMode
	CommandID   string
	Lane        string
	CommandArgv []string
}

type RunMode string

const (
	RunModeConfigured RunMode = "configured"
	RunModeAdHoc      RunMode = "ad_hoc"
)

type RunStatus string

const (
	RunStatusPassed      RunStatus = "passed"
	RunStatusFailed      RunStatus = "failed"
	RunStatusTimedOut    RunStatus = "timed_out"
	RunStatusKilled      RunStatus = "killed"
	RunStatusInternalErr RunStatus = "internal_error"
)

type ExtractorStatus string

const (
	ExtractorStatusPrecise  ExtractorStatus = "precise"
	ExtractorStatusPartial  ExtractorStatus = "partial"
	ExtractorStatusDegraded ExtractorStatus = "degraded"
	ExtractorStatusNoMatch  ExtractorStatus = "no_match"
)

type RawSpan struct {
	StartLine int `yaml:"start_line" json:"start_line"`
	EndLine   int `yaml:"end_line" json:"end_line"`
	StartByte int `yaml:"start_byte,omitempty" json:"start_byte"`
	EndByte   int `yaml:"end_byte,omitempty" json:"end_byte"`
}

type Failure struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Signature string   `json:"signature"`
	File      string   `json:"file,omitempty"`
	Line      int      `json:"line,omitempty"`
	TestName  string   `json:"test_name,omitempty"`
	RawSpan   RawSpan  `json:"raw_span"`
	Excerpt   string   `json:"excerpt,omitempty"`
	StackTop  []string `json:"stack_top,omitempty"`
}

type Warning struct {
	ID        string  `json:"id"`
	Signature string  `json:"signature"`
	RawSpan   RawSpan `json:"raw_span"`
}

type RunMetadata struct {
	CommandID   string    `json:"command_id"`
	Lane        string    `json:"lane"`
	Parser      string    `json:"parser"`
	CommandArgv []string  `json:"command_argv"`
	ExitCode    int       `json:"exit_code"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
	DurationMS  int64     `json:"duration_ms"`
}

type Summary struct {
	Status          RunStatus       `json:"status"`
	CommandID       string          `json:"command_id"`
	Lane            string          `json:"lane"`
	Parser          string          `json:"parser"`
	CommandArgv     []string        `json:"command_argv"`
	ExitCode        int             `json:"exit_code"`
	StartedAt       time.Time       `json:"started_at"`
	EndedAt         time.Time       `json:"ended_at"`
	DurationMS      int64           `json:"duration_ms"`
	RawLog          string          `json:"raw_log"`
	RawLogSHA256    string          `json:"raw_log_sha256"`
	ExtractorStatus ExtractorStatus `json:"extractor_status"`
	FailureCount    int             `json:"failure_count"`
	WarningCount    int             `json:"warning_count"`
	Failures        []Failure       `json:"failures"`
	Warnings        []Warning       `json:"warnings"`
}

type Status struct {
	Status            RunStatus       `json:"status"`
	CommandID         string          `json:"command_id"`
	Lane              string          `json:"lane"`
	ExitCode          int             `json:"exit_code"`
	ExtractorStatus   ExtractorStatus `json:"extractor_status"`
	SummaryPath       string          `json:"summary_path"`
	SummarySHA256     string          `json:"summary_sha256"`
	RawLogPath        string          `json:"raw_log_path"`
	RawLogSHA256      string          `json:"raw_log_sha256"`
	FailureSignatures []string        `json:"failure_signatures"`
	WarningSignatures []string        `json:"warning_signatures"`
	UpdatedAt         time.Time       `json:"updated_at"`
	StatusHash        string          `json:"status_hash"`
}

type ArtifactPaths struct {
	BoundaryDir string
	BaseDir     string
	RawLogPath  string
	SummaryJSON string
	SummaryMD   string
	StatusJSON  string
	ExcerptsDir string
}

type RunOutput struct {
	Metadata        RunMetadata
	Status          RunStatus
	ExtractorStatus ExtractorStatus
	RawLogBytes     []byte
	Failures        []Failure
	Warnings        []Warning
}

type RuleTestResult struct {
	RuleID            string `json:"rule_id"`
	RawLogPath        string `json:"raw_log_path"`
	ExpectedStartLine int    `json:"expected_start_line"`
	ExpectedEndLine   int    `json:"expected_end_line"`
	ActualStartLine   int    `json:"actual_start_line"`
	ActualEndLine     int    `json:"actual_end_line"`
	FailureCount      int    `json:"failure_count"`
	Signature         string `json:"signature,omitempty"`
	Passed            bool   `json:"passed"`
}

type RuleProposal struct {
	Rule Rule   `json:"rule"`
	Path string `json:"path"`
}
