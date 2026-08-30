package lab

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const feScenarioContractCommit = "deadad18b6588d2c907170a451c3b5cea5ea4192"
const feScenarioIndexDigest = "sha256:c70eeb43b85c4b4f8e57abacd4459fe8b01fa407ce355311f335dd5654c0f669"
const feIntegrationResultsDigest = "sha256:f16ea6113c829379e10b8def5baf4cf82ba6596ffa4146792315ed688da6985a"

type FEScenarioContractLock struct {
	SchemaVersion      int                     `json:"schema_version"`
	ID                 string                  `json:"id"`
	Repository         string                  `json:"repository"`
	Commit             string                  `json:"commit"`
	License            string                  `json:"license"`
	ScenarioIndex      PreviewArtifactLock     `json:"scenario_index"`
	IntegrationResults PreviewArtifactLock     `json:"integration_results"`
	Expected           FEScenarioContractStats `json:"expected"`
}

type FEScenarioContractStats struct {
	Patterns                   int `json:"patterns"`
	Scenarios                  int `json:"scenarios"`
	Rows                       int `json:"rows"`
	DedicatedArtifacts         int `json:"dedicated_artifacts"`
	PatternSpecificRows        int `json:"pattern_specific_rows"`
	PatternSpecificRuntimeRows int `json:"pattern_specific_runtime_rows"`
	PatternSpecificCaptureRows int `json:"pattern_specific_capture_rows"`
	PatternSpecificGaps        int `json:"pattern_specific_gaps"`
	IntegratedTraceRows        int `json:"integrated_trace_rows"`
	AuthorityAtomicRows        int `json:"authority_atomic_rows"`
	CompletionEligibleRows     int `json:"completion_eligible_rows"`
	IntegrationPassed          int `json:"integration_passed"`
}

type feScenarioIndex struct {
	Status           string                  `json:"status"`
	Denominator      string                  `json:"denominator"`
	Summary          FEScenarioContractStats `json:"summary"`
	Files            []feScenarioIndexFile   `json:"files"`
	CompletionLimits []string                `json:"completion_limits"`
}

type feScenarioIndexFile struct {
	ID        string `json:"id"`
	PatternID string `json:"pattern_id"`
	Scenario  string `json:"scenario"`
	Path      string `json:"path"`
	Digest    string `json:"digest"`
	Status    string `json:"status"`
}

type feScenarioProof struct {
	SchemaVersion       int                   `json:"schema_version"`
	ID                  string                `json:"id"`
	PatternID           string                `json:"pattern_id"`
	Scenario            string                `json:"scenario"`
	Status              string                `json:"status"`
	PatternEvidence     fePatternEvidence     `json:"pattern_evidence"`
	IntegratedReference feIntegratedReference `json:"integrated_reference"`
	Closure             feScenarioClosure     `json:"closure"`
	Gaps                []string              `json:"gaps"`
}

type fePatternEvidence struct {
	CaptureEnvironmentIdentity any   `json:"capture_environment_identity"`
	CaptureRecords             []any `json:"capture_records"`
	BenchmarkEnvironment       any   `json:"benchmark_environment"`
	BenchmarkRecords           []any `json:"benchmark_records"`
	CompatibilityEnvironment   any   `json:"compatibility_environment"`
	CompatibilityRecords       []any `json:"compatibility_records"`
}

type feIntegratedReference struct {
	Trace feTraceRef `json:"trace"`
}

type feTraceRef struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type feScenarioClosure struct {
	DedicatedRow            bool `json:"dedicated_row"`
	DedicatedArtifact       bool `json:"dedicated_artifact"`
	PatternSpecificEvidence bool `json:"pattern_specific_evidence"`
	RealRuntimeIdentity     bool `json:"real_runtime_identity"`
	IntegratedRuntimeTrace  bool `json:"integrated_runtime_trace"`
	AuthorityAtomicBehavior bool `json:"authority_atomic_behavior"`
	CompletionEligible      bool `json:"completion_eligible"`
}

type feIntegrationResults struct {
	Status           string              `json:"status"`
	Profile          string              `json:"profile"`
	Environment      map[string]any      `json:"environment"`
	Counts           feIntegrationCounts `json:"counts"`
	Tests            []feIntegrationTest `json:"tests"`
	CompletionLimits []string            `json:"completion_limits"`
}

type feIntegrationCounts struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Flaky   int `json:"flaky"`
	Skipped int `json:"skipped"`
}

type feIntegrationTest struct {
	Scenario    string     `json:"scenario"`
	FinalStatus string     `json:"final_status"`
	Trace       feTraceRef `json:"trace"`
}

var feScenarioValidation struct {
	sync.Once
	stats FEScenarioContractStats
	err   error
}

func validateFEScenarioContract(root, referenceRepositoryRoot string, artifact PreviewArtifactLock) (FEScenarioContractStats, error) {
	var lock FEScenarioContractLock
	if err := loadLockedJSON(root, artifact, &lock); err != nil {
		return FEScenarioContractStats{}, err
	}
	expected := FEScenarioContractStats{Patterns: 85, Scenarios: 10, Rows: 850, DedicatedArtifacts: 850, PatternSpecificRows: 429, PatternSpecificRuntimeRows: 170, PatternSpecificCaptureRows: 259, PatternSpecificGaps: 421, IntegratedTraceRows: 850, AuthorityAtomicRows: 0, CompletionEligibleRows: 0, IntegrationPassed: 10}
	if lock.SchemaVersion != 1 || lock.ID != "fe-scenario-contract-v1" || lock.Repository != "frontend-behavior-atlas" || lock.Commit != feScenarioContractCommit || lock.License != "Apache-2.0" || lock.ScenarioIndex.Path != "evidence/scenarios/index.json" || lock.ScenarioIndex.Digest != feScenarioIndexDigest || lock.IntegrationResults.Path != "artifacts/reference-system/results.json" || lock.IntegrationResults.Digest != feIntegrationResultsDigest || lock.Expected != expected {
		return FEScenarioContractStats{}, fmt.Errorf("FE Scenario Contract Lockが確定値と一致しません")
	}
	feScenarioValidation.Do(func() {
		feScenarioValidation.stats, feScenarioValidation.err = validateFEScenarioGitObject(referenceRepositoryRoot, lock)
	})
	return feScenarioValidation.stats, feScenarioValidation.err
}

func validateFEScenarioGitObject(referenceRepositoryRoot string, lock FEScenarioContractLock) (FEScenarioContractStats, error) {
	repository := filepath.Join(referenceRepositoryRoot, "..", lock.Repository)
	indexData, err := externalGitFile(repository, lock.Commit, lock.ScenarioIndex)
	if err != nil {
		return FEScenarioContractStats{}, err
	}
	resultsData, err := externalGitFile(repository, lock.Commit, lock.IntegrationResults)
	if err != nil {
		return FEScenarioContractStats{}, err
	}
	var index feScenarioIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return FEScenarioContractStats{}, err
	}
	expectedIndex := lock.Expected
	expectedIndex.IntegrationPassed = 0
	if index.Status != "incomplete-authority-atomic-and-runtime-closure" || index.Denominator != "85-current-domain-patterns-x-10-scenarios" || index.Summary != expectedIndex || len(index.Files) != lock.Expected.Rows || !hasText(index.CompletionLimits, "流用しない") {
		return FEScenarioContractStats{}, fmt.Errorf("FE Scenario Indexの850 row境界が不一致です")
	}
	var results feIntegrationResults
	if err := json.Unmarshal(resultsData, &results); err != nil {
		return FEScenarioContractStats{}, err
	}
	traceByScenario, err := validateFEIntegrationResults(results, lock.Expected.IntegrationPassed)
	if err != nil {
		return FEScenarioContractStats{}, err
	}
	proofs, err := archivedScenarioProofs(repository, lock.Commit)
	if err != nil {
		return FEScenarioContractStats{}, err
	}
	stats, err := validateFEScenarioRows(index.Files, proofs, traceByScenario)
	if err != nil {
		return FEScenarioContractStats{}, err
	}
	stats.IntegrationPassed = results.Counts.Passed
	if stats != lock.Expected {
		return FEScenarioContractStats{}, fmt.Errorf("FE Scenario Contract集計不一致: expected=%+v actual=%+v", lock.Expected, stats)
	}
	return stats, nil
}

func externalGitFile(repository, commit string, artifact PreviewArtifactLock) ([]byte, error) {
	data, err := exec.Command("git", "-C", repository, "show", commit+":"+artifact.Path).Output()
	if err != nil {
		return nil, fmt.Errorf("FE Git Objectを読めません: %s: %w", artifact.Path, err)
	}
	if DigestBytes(data) != artifact.Digest {
		return nil, fmt.Errorf("FE Git Object Digest不一致: %s", artifact.Path)
	}
	return data, nil
}

func validateFEIntegrationResults(results feIntegrationResults, expected int) (map[string]feTraceRef, error) {
	if results.Status != "passed" || results.Profile != "local-real-browser" || results.Counts != (feIntegrationCounts{Total: expected, Passed: expected}) || len(results.Tests) != expected || !nonEmptyString(results.Environment["browser_name"]) || !nonEmptyString(results.Environment["browser_version"]) || !nonEmptyString(results.Environment["platform"]) || !hasText(results.CompletionLimits, "does not replace per-Pattern runtime proof") {
		return nil, fmt.Errorf("FE統合Scenario 10件のRuntime結果が不正です")
	}
	traces := map[string]feTraceRef{}
	digests := map[string]bool{}
	for _, test := range results.Tests {
		if test.Scenario == "" || test.FinalStatus != "passed" || test.Trace.Path == "" || test.Trace.Digest == "" || traces[test.Scenario].Path != "" || digests[test.Trace.Digest] {
			return nil, fmt.Errorf("FE統合Scenario Traceが固有ではありません: %s", test.Scenario)
		}
		traces[test.Scenario] = test.Trace
		digests[test.Trace.Digest] = true
	}
	return traces, nil
}

func archivedScenarioProofs(repository, commit string) (map[string][]byte, error) {
	data, err := exec.Command("git", "-C", repository, "archive", "--format=tar", commit, "--", "evidence/scenarios/patterns").Output()
	if err != nil {
		return nil, fmt.Errorf("FE Scenario Proof archiveを読めません: %w", err)
	}
	reader := tar.NewReader(bytes.NewReader(data))
	proofs := map[string][]byte{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg || !strings.HasSuffix(header.Name, ".proof.json") {
			continue
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		proofs[filepath.ToSlash(header.Name)] = content
	}
	return proofs, nil
}

func validateFEScenarioRows(files []feScenarioIndexFile, proofs map[string][]byte, traceByScenario map[string]feTraceRef) (FEScenarioContractStats, error) {
	stats := FEScenarioContractStats{}
	patterns, scenarios := map[string]bool{}, map[string]bool{}
	ids, paths, digests := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, file := range files {
		if file.ID == "" || file.Path == "" || ids[file.ID] || paths[file.Path] || digests[file.Digest] {
			return stats, fmt.Errorf("FE個別Scenario rowのID、PathまたはDigestが重複しています")
		}
		data, ok := proofs[file.Path]
		if !ok || DigestBytes(data) != file.Digest {
			return stats, fmt.Errorf("FE個別Scenario Proof Digest不一致: %s", file.Path)
		}
		var proof feScenarioProof
		if err := json.Unmarshal(data, &proof); err != nil {
			return stats, err
		}
		if proof.SchemaVersion != 1 || proof.ID != file.ID || proof.PatternID != file.PatternID || proof.Scenario != file.Scenario || proof.Status != file.Status || !proof.Closure.DedicatedRow || !proof.Closure.DedicatedArtifact {
			return stats, fmt.Errorf("FE個別Scenario row契約不一致: %s", file.ID)
		}
		if expectedTrace := traceByScenario[proof.Scenario]; expectedTrace.Path == "" || proof.IntegratedReference.Trace != expectedTrace || !proof.Closure.IntegratedRuntimeTrace {
			return stats, fmt.Errorf("FE統合Trace接続不一致: %s", file.ID)
		}
		patternRecords := len(proof.PatternEvidence.CaptureRecords) + len(proof.PatternEvidence.BenchmarkRecords) + len(proof.PatternEvidence.CompatibilityRecords)
		if proof.Closure.PatternSpecificEvidence != (patternRecords > 0) {
			return stats, fmt.Errorf("FE Pattern固有Evidence判定不一致: %s", file.ID)
		}
		runtimeIdentity := proof.PatternEvidence.CaptureEnvironmentIdentity != nil || proof.PatternEvidence.BenchmarkEnvironment != nil || proof.PatternEvidence.CompatibilityEnvironment != nil
		if proof.Closure.RealRuntimeIdentity != runtimeIdentity {
			return stats, fmt.Errorf("FE Runtime Identity判定不一致: %s", file.ID)
		}
		expectedStatus := "bounded-runtime-proof"
		if !proof.Closure.PatternSpecificEvidence {
			expectedStatus = "pattern-specific-gap"
		} else if !proof.Closure.RealRuntimeIdentity {
			expectedStatus = "bounded-capture-proof"
		}
		if proof.Status != expectedStatus {
			return stats, fmt.Errorf("FE個別Scenario rowの降格状態が不一致です: %s", file.ID)
		}
		if (!proof.Closure.PatternSpecificEvidence || !proof.Closure.RealRuntimeIdentity || !proof.Closure.AuthorityAtomicBehavior) && !hasExplicitGaps(proof.Gaps) {
			return stats, fmt.Errorf("FE個別Scenario rowにProofまたは明示gapがありません: %s", file.ID)
		}
		if !proof.Closure.AuthorityAtomicBehavior && !hasText(proof.Gaps, "Authority") {
			return stats, fmt.Errorf("FE個別Scenario rowにAtomic Authority Binding gapがありません: %s", file.ID)
		}
		eligible := proof.Closure.PatternSpecificEvidence && proof.Closure.RealRuntimeIdentity && proof.Closure.AuthorityAtomicBehavior && len(proof.Gaps) == 0
		if proof.Closure.CompletionEligible != eligible {
			return stats, fmt.Errorf("FE Completion Eligible判定不一致: %s", file.ID)
		}
		ids[file.ID], paths[file.Path], digests[file.Digest] = true, true, true
		patterns[file.PatternID], scenarios[file.Scenario] = true, true
		stats.Rows++
		stats.DedicatedArtifacts++
		if proof.Closure.PatternSpecificEvidence {
			stats.PatternSpecificRows++
		} else {
			stats.PatternSpecificGaps++
		}
		if proof.Closure.RealRuntimeIdentity {
			stats.PatternSpecificRuntimeRows++
		}
		if len(proof.PatternEvidence.CaptureRecords) > 0 {
			stats.PatternSpecificCaptureRows++
		}
		if proof.Closure.IntegratedRuntimeTrace {
			stats.IntegratedTraceRows++
		}
		if proof.Closure.AuthorityAtomicBehavior {
			stats.AuthorityAtomicRows++
		}
		if proof.Closure.CompletionEligible {
			stats.CompletionEligibleRows++
		}
	}
	if len(proofs) != len(files) {
		return stats, fmt.Errorf("FE Scenario Index外のProofがあります")
	}
	stats.Patterns, stats.Scenarios = len(patterns), len(scenarios)
	return stats, nil
}

func hasText(items []string, fragment string) bool {
	for _, item := range items {
		if strings.Contains(item, fragment) {
			return true
		}
	}
	return false
}

func hasExplicitGaps(gaps []string) bool {
	if len(gaps) == 0 {
		return false
	}
	for _, gap := range gaps {
		if strings.TrimSpace(gap) == "" {
			return false
		}
	}
	return true
}

func nonEmptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}
