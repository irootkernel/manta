# KAT Requirements Test Matrix

Status: Complete for the current checked requirements
Scope: Primary executable or documentary evidence for each completed requirement in `requirements-specs.md`

This matrix records the primary evidence for every requirement marked complete. The audit regression suite fails when a completed requirement is missing, duplicated, has no evidence, or when the matrix references an unknown or incomplete requirement.

| Requirement | Primary evidence |
|---|---|
| `KAT-REQ-RQCLI-001` | `TestDocumentedCLIWorkflowAgainstFreshFixture`; `TestMakeInstallTargetsAndResolver` |
| `KAT-REQ-RQCLI-002` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `KAT-REQ-RQCLI-003` | `TestAdHocRunWithoutConfig`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `KAT-REQ-RQCLI-004` | `TestSummarizeRawLogUsesConfigRedaction`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `KAT-REQ-RQCLI-005` | `TestConfiguredRunAndExcerpt`; `TestExcerptRejectsUnsafeReferences` |
| `KAT-REQ-RQCLI-006` | `TestTimeoutPreservesPartialArtifacts`; `TestBinaryExtractionContracts` |
| `KAT-REQ-RQCFG-001` | `TestConfiguredRunAndExcerpt`; `TestAdHocRunWithoutConfig` |
| `KAT-REQ-RQCFG-002` | `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `KAT-REQ-RQCFG-003` | `TestValidateAcceptsImplementedParsers`; `TestConfiguredRunAndExcerpt` |
| `KAT-REQ-RQCFG-004` | `TestSummarizeRawLogUsesConfigRedaction` |
| `KAT-REQ-RQCFG-005` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `KAT-REQ-RQCFG-006` | `TestLoadRejectsUnknownFieldsAndMultipleDocuments`; `TestBinaryRejectsUnknownConfigFields` |
| `KAT-REQ-RQRUN-001` | `TestConfiguredRunAndExcerpt`; `TestAdHocRunWithoutConfig` |
| `KAT-REQ-RQRUN-002` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `KAT-REQ-RQRUN-003` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `KAT-REQ-RQRUN-004` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `KAT-REQ-RQRUN-005` | `TestExecuteTimeout`; `TestTimeoutPreservesPartialArtifacts` |
| `KAT-REQ-RQRUN-006` | `TestExecuteForwardsTerminationAndNormalizesResult`; `TestBinaryPreservesInterruptedEvidence` |
| `KAT-REQ-RQART-001` | `TestKkachiArtifactLayout`; `TestBinaryArtifactContainment` |
| `KAT-REQ-RQART-002` | `TestArtifactOutputDirectories`; `TestBinaryStandaloneCollisionResistance` |
| `KAT-REQ-RQART-003` | `TestConfiguredRunAndExcerpt`; `TestSummarizeInternalErrorMaterializesArtifacts` |
| `KAT-REQ-RQART-004` | `TestWriteSummaryMarkdownMatchesDocumentedShape`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `KAT-REQ-RQART-005` | `TestConfiguredRunAndExcerpt`; `TestBinaryJSONRedactsCommandMetadata` |
| `KAT-REQ-RQART-006` | `TestConfiguredRunAndExcerpt`; `TestExcerptSymlinkContainment` |
| `KAT-REQ-RQART-007` | `TestArtifactOutputDirectories`; `TestKkachiArtifactLayout` |
| `KAT-REQ-RQEXT-001` | `TestProcessGenericFailureProducesPreciseSpan` |
| `KAT-REQ-RQEXT-002` | `TestValidateAcceptsImplementedParsers`; specialized parser fixture tests |
| `KAT-REQ-RQEXT-003` | `TestProcessGenericFailureProducesPreciseSpan`; `TestProcessRulesBoundsUnvalidatedContext` |
| `KAT-REQ-RQEXT-004` | `TestProcessGenericFailureProducesPreciseSpan`; specialized parser fixture tests |
| `KAT-REQ-RQEXT-005` | `TestProcessExtractorStatusContract` |
| `KAT-REQ-RQEXT-006` | `TestProcessExtractorStatusContract`; `TestBinaryExtractionContracts` |
| `KAT-REQ-RQEXT-007` | `TestMaterializeArtifactsExtractionErrorRetainsNonPassRunState`; `TestBinaryExtractionContracts` |
| `KAT-REQ-RQRUL-001` | `TestRulesLifecycleCommands`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `KAT-REQ-RQRUL-002` | `TestCreateSearchAndDeleteRule`; `TestRulesLifecycleCommands` |
| `KAT-REQ-RQRUL-003` | `TestValidateStoredRuleRejectsInvalidContextAndStatus`; `TestCreateSearchAndDeleteRule` |
| `KAT-REQ-RQRUL-004` | `TestCreateSearchAndDeleteRule`; `TestRulesLifecycleCommands` |
| `KAT-REQ-RQRUL-005` | `TestTestRuleMatchesExpectedSpan`; `TestRuleMatchesCRLFLineEndings` |
| `KAT-REQ-RQRUL-006` | `TestRuleDetectsOvermatch`; `TestBinaryRejectsOversizedRuleContext` |
| `KAT-REQ-RQRUL-007` | `TestProposeWritesRunLocalProposal`; `TestProposePreservesMeaningfulLineWhitespace` |
| `KAT-REQ-RQSEC-001` | `TestRedactSummaryCoversSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `KAT-REQ-RQSEC-002` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `KAT-REQ-RQSEC-003` | `TestBinaryRejectsUnknownConfigFields`; `TestBinaryRejectsOversizedRuleContext` |
| `KAT-REQ-RQSEC-004` | `TestWriteSummaryJSONFailsWhenTooLarge`; `TestProcessRulesBoundsUnvalidatedContext` |
| `KAT-REQ-RQSEC-005` | `TestProcessExtractorStatusContract`; `TestBinaryExtractionContracts` |
| `KAT-REQ-RQWAT-001` | `TestConfiguredRunAndExcerpt`; status-hash assertions in CLI and binary tests |
| `KAT-REQ-RQWAT-002` | `ComputeStatusHash`; status-hash assertions in CLI and binary tests |
| `KAT-REQ-RQWAT-003` | `TestBinaryJSONRedactsCommandMetadata`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `KAT-REQ-GAJAE-009` | `architecture.md` GAJAE contract; `TestBinaryConfiguredRunAndExcerpt` artifact compatibility |
| `KAT-REQ-RQDOC-001` | authoritative documents listed in `AGENTS.md` and `README.md` |
| `KAT-REQ-RQDOC-002` | `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `KAT-REQ-RQDOC-003` | parser fixtures under `internal/extract/testdata`; `TestTestRuleMatchesExpectedSpan` |
| `KAT-REQ-RQDOC-004` | release-readiness checklist in `implementation-note.md`; `make test` |
| `KAT-REQ-RQHAR-001` | `TestBinaryArtifactContainment`; path, artifact, and rule symlink tests |
| `KAT-REQ-RQHAR-002` | `TestBinaryPreservesInterruptedEvidence`; Unix runner signal tests |
| `KAT-REQ-RQHAR-003` | `TestBinaryStandaloneCollisionResistance`; concurrent artifact allocation tests |
| `KAT-REQ-RQHAR-004` | `TestBinaryJSONRedactsCommandMetadata`; CLI redaction integration tests |
| `KAT-REQ-RQHAR-005` | `TestBinaryExtractionContracts`; CLI extraction contract tests |
| `KAT-REQ-RQHAR-006` | `TestDocumentedCLIWorkflowAgainstFreshFixture`; toolchain script E2E tests |
| `KAT-REQ-RQHAR-007` | `make test`; focused binary containment, signal, collision, extraction, install, and workflow E2E tests |

The matrix is traceability evidence, not acceptance authority. Command exit status and the artifact contracts remain authoritative for individual runs.
