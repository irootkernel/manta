# Manta Requirements Test Matrix

Status: Complete for the current checked requirements
Scope: Primary executable or documentary evidence for each completed requirement in `requirements-specs.md`

This matrix records the primary evidence for every requirement marked complete. The audit regression suite fails when a completed requirement is missing, duplicated, has no evidence, or when the matrix references an unknown or incomplete requirement.

| Requirement | Primary evidence |
|---|---|
| `MANTA-REQ-RQCLI-001` | `TestDocumentedCLIWorkflowAgainstFreshFixture`; `TestMakeInstallTargetsAndResolver` |
| `MANTA-REQ-RQCLI-002` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `MANTA-REQ-RQCLI-003` | `TestAdHocRunWithoutConfig`; `TestBinaryTagsSelectRulesByAllTags`; `TestBinaryTagInterfacesFailBeforeExecution`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `MANTA-REQ-RQCLI-004` | `TestSummarizeRawLogUsesConfigRedaction`; `TestRunAndSummarizeSelectRulesByAllTags`; `TestBinaryTagsSelectRulesByAllTags`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `MANTA-REQ-RQCLI-005` | `TestConfiguredRunAndExcerpt`; `TestExcerptRejectsUnsafeReferences` |
| `MANTA-REQ-RQCLI-006` | `TestTimeoutPreservesPartialArtifacts`; `TestBinaryExtractionContracts` |
| `MANTA-REQ-RQCFG-001` | `TestConfiguredRunAndExcerpt`; `TestAdHocRunWithoutConfig` |
| `MANTA-REQ-RQCFG-002` | `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `MANTA-REQ-RQCFG-003` | `TestValidateAcceptsImplementedParsers`; `TestLoadCanonicalizesTags`; `TestValidateRejectsMissingAndUnsafeTags`; `TestBinaryTagsSelectRulesByAllTags` |
| `MANTA-REQ-RQCFG-004` | `TestSummarizeRawLogUsesConfigRedaction` |
| `MANTA-REQ-RQCFG-005` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `MANTA-REQ-RQCFG-006` | `TestLoadRejectsUnknownFieldsAndMultipleDocuments`; `TestBinaryRejectsUnknownConfigFields`; `TestBinaryTagInterfacesFailBeforeExecution` |
| `MANTA-REQ-RQRUN-001` | `TestConfiguredRunAndExcerpt`; `TestAdHocRunWithoutConfig` |
| `MANTA-REQ-RQRUN-002` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `MANTA-REQ-RQRUN-003` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `MANTA-REQ-RQRUN-004` | `TestConfiguredRunAndExcerpt`; `TestBinaryConfiguredRunAndExcerpt` |
| `MANTA-REQ-RQRUN-005` | `TestExecuteTimeout`; `TestTimeoutPreservesPartialArtifacts` |
| `MANTA-REQ-RQRUN-006` | `TestExecuteForwardsTerminationAndNormalizesResult`; `TestBinaryPreservesInterruptedEvidence` |
| `MANTA-REQ-RQART-001` | `TestRunIDArtifactLayout`; `TestBinaryArtifactContainment` |
| `MANTA-REQ-RQART-002` | `TestArtifactOutputDirectories`; `TestBinaryStandaloneCollisionResistance` |
| `MANTA-REQ-RQART-003` | `TestConfiguredRunAndExcerpt`; `TestOversizedSummarizeUsesBoundedExtraction`; `TestNoisyRunsWriteBoundedTerminalArtifacts`; `TestWriteSummaryJSONIncludesFalseTruncationFields` |
| `MANTA-REQ-RQART-004` | `TestWriteSummaryMarkdownMatchesDocumentedShape`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `MANTA-REQ-RQART-005` | `TestConfiguredRunAndExcerpt`; `TestBinaryJSONRedactsCommandMetadata` |
| `MANTA-REQ-RQART-006` | `TestConfiguredRunAndExcerpt`; `TestExcerptSymlinkContainment` |
| `MANTA-REQ-RQART-007` | `TestArtifactOutputDirectories`; `TestRunIDArtifactLayout` |
| `MANTA-REQ-RQEXT-001` | `TestProcessGenericFailureProducesPreciseSpan` |
| `MANTA-REQ-RQEXT-002` | `TestValidateAcceptsImplementedParsers`; `TestProcessVitestFixture`; `TestProcessPytestFixture`; `TestProcessGoTestFixture`; `TestProcessPlaywrightFixture`; `TestConfiguredRunsUseSpecializedParsers`; `TestBinaryExtractionContracts` |
| `MANTA-REQ-RQEXT-003` | `TestProcessGenericFailureProducesPreciseSpan`; `TestProcessRulesBoundsUnvalidatedContext` |
| `MANTA-REQ-RQEXT-004` | `TestProcessGenericFailureProducesPreciseSpan`; `TestProcessVitestFixture`; `TestProcessPytestFixture`; `TestProcessGoTestFixture`; `TestProcessPlaywrightFixture`; `TestConfiguredRunsUseSpecializedParsers`; `TestBinaryExtractionContracts` |
| `MANTA-REQ-RQEXT-005` | `TestProcessExtractorStatusContract`; `TestNoisyRunsWriteBoundedTerminalArtifacts` |
| `MANTA-REQ-RQEXT-006` | `TestProcessExtractorStatusContract`; `TestBinaryExtractionContracts` |
| `MANTA-REQ-RQEXT-007` | `TestMaterializeArtifactsExtractionErrorContract`; `TestBinaryExtractionContracts` |
| `MANTA-REQ-RQRUL-001` | `TestRulesLifecycleCommands`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `MANTA-REQ-RQRUL-002` | `TestCreateSearchAndDeleteRule`; `TestRulesLifecycleCommands` |
| `MANTA-REQ-RQRUL-003` | `TestValidateStoredRuleRejectsInvalidContextAndStatus`; `TestCreateSearchAndDeleteRule` |
| `MANTA-REQ-RQRUL-004` | `TestCreateSearchAndDeleteRule`; `TestRulesLifecycleCommands` |
| `MANTA-REQ-RQRUL-005` | `TestTestRuleMatchesExpectedSpan`; `TestRuleMatchesCRLFLineEndings` |
| `MANTA-REQ-RQRUL-006` | `TestRuleDetectsOvermatch`; `TestBinaryRejectsOversizedRuleContext` |
| `MANTA-REQ-RQRUL-007` | `TestProposeWritesRunLocalProposal`; `TestProposePreservesMeaningfulLineWhitespace` |
| `MANTA-REQ-RQRUL-008` | `TestLoadApplicableRequiresAllRuleTags`; `TestRunAndSummarizeSelectRulesByAllTags`; `TestBinaryTagsSelectRulesByAllTags` |
| `MANTA-REQ-RQSEC-001` | `TestRedactSummaryCoversSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `MANTA-REQ-RQSEC-002` | `TestConfiguredRunRedactsSurfacedMetadata`; `TestBinaryJSONRedactsCommandMetadata` |
| `MANTA-REQ-RQSEC-003` | `TestBinaryRejectsUnknownConfigFields`; `TestBinaryRejectsOversizedRuleContext` |
| `MANTA-REQ-RQSEC-004` | `TestWriteSummaryJSONFailsWhenTooLarge`; `TestProcessOversizedLogUsesBoundedTail`; `TestProcessPytestDetailScanIsBounded`; `TestProcessRulesRejectsOversizedInput`; `TestTestRuleBoundsFixtureBeforeExtraction`; `TestBoundSummaryEvidenceCapsRecordsAndKeepsCountsAligned`; `TestBoundSummaryEvidenceUsesRenderedByteBudget`; `TestBoundSummaryEvidenceUsesRemainingBudgetForWarnings`; `TestNoisyRunsWriteBoundedTerminalArtifacts`; `TestLoadEnforcesInputSizeLimit`; `TestLoadAllEnforcesRuleFileSizeLimit`; `TestRuleSourceFilesEnforceInputSizeLimit`; `TestProposeEnforcesRawLogInputSizeLimit`; `TestBinaryEnforcesRuleAndConfigInputSizeLimits` |
| `MANTA-REQ-RQSEC-005` | `TestProcessExtractorStatusContract`; `TestBinaryExtractionContracts` |
| `MANTA-REQ-RQWAT-001` | `TestConfiguredRunAndExcerpt`; status-hash assertions in CLI and binary tests; `TestNoisyRunsWriteBoundedTerminalArtifacts` |
| `MANTA-REQ-RQWAT-002` | `ComputeStatusHash`; status-hash assertions in CLI and binary tests |
| `MANTA-REQ-RQWAT-003` | `TestBinaryJSONRedactsCommandMetadata`; `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `MANTA-REQ-RQDOC-001` | authoritative documents listed in `AGENTS.md` and `README.md` |
| `MANTA-REQ-RQDOC-002` | `TestDocumentedCLIWorkflowAgainstFreshFixture` |
| `MANTA-REQ-RQDOC-003` | parser fixtures under `internal/extract/testdata`; `TestTestRuleMatchesExpectedSpan` |
| `MANTA-REQ-RQDOC-004` | release-readiness checklist in `implementation-note.md`; `make test` |
| `MANTA-REQ-RQHAR-001` | `TestBinaryArtifactContainment`; path, artifact, and rule symlink tests |
| `MANTA-REQ-RQHAR-002` | `TestBinaryPreservesInterruptedEvidence`; Unix runner signal tests |
| `MANTA-REQ-RQHAR-003` | `TestBinaryStandaloneCollisionResistance`; concurrent artifact allocation tests |
| `MANTA-REQ-RQHAR-004` | `TestBinaryJSONRedactsCommandMetadata`; CLI redaction integration tests |
| `MANTA-REQ-RQHAR-005` | `TestBinaryExtractionContracts`; CLI extraction contract tests |
| `MANTA-REQ-RQHAR-006` | `TestDocumentedCLIWorkflowAgainstFreshFixture`; toolchain script E2E tests |
| `MANTA-REQ-RQHAR-007` | `make test`; focused binary containment, signal, collision, extraction, install, and workflow E2E tests |

The matrix is traceability evidence, not acceptance authority. Command exit status and the artifact contracts remain authoritative for individual runs.
