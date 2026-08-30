package lab

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNonRegressionGateAcceptsSuperset(t *testing.T) {
	report, err := NonRegressionGate("../..")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestProvenReplacementPreservesOrExceedsIntegrationProof(t *testing.T) {
	root := "../.."
	repository := &overlayRepository{root: root, overrides: map[string][]byte{}, missing: map[string]bool{}}
	oldScenario, err := os.ReadFile(resolve(root, "scenarios/failure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var replacementScenario map[string]any
	if err := json.Unmarshal(oldScenario, &replacementScenario); err != nil {
		t.Fatal(err)
	}
	replacementScenario["id"] = "failure-v2"
	repository.overrides["scenarios/failure-v2.json"] = mustJSON(t, replacementScenario)
	repository.missing["scenarios/failure.json"] = true
	if err := mutateJSON(repository, "compositions/fixture-stage2.json", func(document map[string]any) {
		scenarios, _ := document["scenarios"].([]any)
		for index, scenario := range scenarios {
			if scenario == "scenarios/failure.json" {
				scenarios[index] = "scenarios/failure-v2.json"
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	proofs := []EvidenceRef{}
	for index, profile := range []string{"local", "container"} {
		sequence := index + 1
		id := "replacement.integration." + string(rune('0'+sequence))
		path := "evidence/preview/replacement-integration-" + string(rune('0'+sequence)) + ".json"
		data := mustJSON(t, map[string]any{"schema_version": 1, "id": id, "kind": "test-report", "profile": profile, "verdict": "pass"})
		repository.overrides[path] = data
		proofs = append(proofs, EvidenceRef{ID: id, Path: path, Digest: DigestBytes(data), Profile: profile})
	}
	migrationPath := "evidence/preview/replacement-migration.json"
	migrationData := mustJSON(t, map[string]any{"schema_version": 1, "id": "replacement.migration.1", "kind": "migration", "profile": "local", "verdict": "pass"})
	repository.overrides[migrationPath] = migrationData
	mapping := NonRegressionMigration{
		SchemaVersion: 1,
		BaselineID:    "interop-v1-non-regression",
		Mappings: []NonRegressionMapping{{
			OldID: "scenario:failure", OldPath: "scenarios/failure.json",
			ReplacementIDs: []string{"failure-v2"}, ReplacementPaths: []string{"scenarios/failure-v2.json"},
			IntegrationProofs: proofs,
			MigrationEvidence: []EvidenceRef{{ID: "replacement.migration.1", Path: migrationPath, Digest: DigestBytes(migrationData), Profile: "local"}},
		}},
	}
	repository.overrides["migrations/interop-v1-non-regression.json"] = mustJSON(t, mapping)
	report, err := nonRegressionGate(root, repository.read)
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func TestNonRegressionMutationMatrixRejectsEveryRegression(t *testing.T) {
	report, err := RunNonRegressionMutationMatrix("../..", "tests/fixtures/non-regression.matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "pass" || len(report.Cases) != 27 {
		t.Fatalf("unexpected matrix: %#v", report)
	}
}

func TestNeutralLanguageAllowsTechnicalNamespaceReferences(t *testing.T) {
	for _, line := range []string{
		"https://github.com/akaitigo/atlas-interoperability-lab",
		"github: akaitigo",
		"Copyright 2026 akaitigo",
		"go install github.com/akaitigo/atlas-interoperability-lab/cmd/atlas-lab@v1.0.0",
	} {
		if !technicalAuthorReference(line) {
			t.Fatalf("technical reference rejected: %s", line)
		}
	}
}

func TestScenarioSuccessCannotPromoteIncompleteSubjectDepth(t *testing.T) {
	result, err := EvaluateDefinitiveComposition("../..", "compositions/fixture-stage2-v2-definitive.preview.json", "2026-08-28T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if result.EffectiveState != "incomplete" || result.DefinitiveEligible || result.DepthParityEligible || !result.IntegrationProofsValid || result.DepthReferenceStatus != "incomplete" || result.IntegratedScenariosPassed != 10 || result.SurfacePatternRows != 850 || result.PatternSpecificRows != 429 || result.RuntimeIdentityRows != 170 || result.PatternSpecificCaptureRows != 259 || result.SurfacePatternGaps != 421 || result.AuthorityAtomicRows != 0 || result.SurfacePatternEligible != 0 {
		t.Fatalf("Depth不足をScenario成功で昇格しました: %#v", result)
	}
	depthWarnings := 0
	scenarioWarnings := map[string]bool{}
	for _, warning := range result.Warnings {
		if warning.Code == "subject-depth-parity-incomplete" {
			depthWarnings++
		}
		if warning.Code == "surface-pattern-proof-gaps" || warning.Code == "integrated-trace-not-component-proof" {
			scenarioWarnings[warning.Code] = true
		}
	}
	if depthWarnings != 2 {
		t.Fatalf("各構成SubjectのDepth不足が保持されていません: %#v", result.Warnings)
	}
	if !scenarioWarnings["surface-pattern-proof-gaps"] || !scenarioWarnings["integrated-trace-not-component-proof"] {
		t.Fatalf("統合Traceを個別Proofへ流用しない境界がありません: %#v", result.Warnings)
	}
}
