package lab

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

const feDepthReferenceCommit = "deadad18b6588d2c907170a451c3b5cea5ea4192"
const feDepthReferenceDigest = "sha256:4f88b8bfd22a9b8262e4a1e8184e50cded0372dd82458fbbcc308b3647876d0d"

type PreviewArtifactLock struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type FEDepthReferenceLock struct {
	SchemaVersion   int          `json:"schema_version"`
	ID              string       `json:"id"`
	Repository      string       `json:"repository"`
	SourceURL       string       `json:"source_url"`
	License         string       `json:"license"`
	Commit          string       `json:"commit"`
	Path            string       `json:"path"`
	Digest          string       `json:"digest"`
	ExpectedStatus  string       `json:"expected_status"`
	ExpectedSummary DepthSummary `json:"expected_summary"`
}

type DepthSummary struct {
	Satisfied int `json:"satisfied"`
	Partial   int `json:"partial"`
	Missing   int `json:"missing"`
}

type feDepthReference struct {
	SchemaVersion int           `json:"schemaVersion"`
	ID            string        `json:"id"`
	Status        string        `json:"status"`
	Summary       DepthSummary  `json:"summary"`
	Axes          []feDepthAxis `json:"axes"`
}

type feDepthAxis struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type SubjectDepthManifest struct {
	SchemaVersion   int                  `json:"schema_version"`
	ID              string               `json:"id"`
	ReferenceID     string               `json:"reference_id"`
	ReferenceCommit string               `json:"reference_commit"`
	ReferenceDigest string               `json:"reference_digest"`
	Subjects        []SubjectDepthParity `json:"subjects"`
}

type SubjectDepthParity struct {
	SubjectID string             `json:"subject_id"`
	Status    string             `json:"status"`
	Summary   DepthSummary       `json:"summary"`
	Axes      []SubjectDepthAxis `json:"axes"`
}

type SubjectDepthAxis struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Proofs []DepthProofRef `json:"proofs"`
}

type DepthProofRef struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type IntegrationDepthManifest struct {
	SchemaVersion           int                     `json:"schema_version"`
	ID                      string                  `json:"id"`
	SourceCompositionID     string                  `json:"source_composition_id"`
	SourceCompositionDigest string                  `json:"source_composition_digest"`
	TargetCompositionIDs    []string                `json:"target_composition_ids"`
	Scope                   string                  `json:"scope"`
	Proofs                  []IntegrationDepthProof `json:"proofs"`
	Verdict                 string                  `json:"verdict"`
}

type IntegrationDepthProof struct {
	ID         string                    `json:"id"`
	Axis       string                    `json:"axis"`
	ScenarioID string                    `json:"scenario_id"`
	Profiles   []IntegrationProfileProof `json:"profiles"`
}

type IntegrationProfileProof struct {
	Profile string `json:"profile"`
	Path    string `json:"path"`
	Digest  string `json:"digest"`
}

func evaluateCompositionDepth(root string, composition PreviewComposition, result *DefinitiveGateResult) (bool, error) {
	reference, referenceStates, err := loadFEDepthReference(root, composition.DepthReferenceLock)
	if err != nil {
		return false, err
	}
	result.DepthReferenceStatus = reference.Status
	scenarioStats, err := validateFEScenarioContract(root, composition.ScenarioContractLock)
	if err != nil {
		return false, err
	}
	result.IntegratedScenariosPassed = scenarioStats.IntegrationPassed
	result.SurfacePatternRows = scenarioStats.Rows
	result.PatternSpecificRows = scenarioStats.PatternSpecificRows
	result.RuntimeIdentityRows = scenarioStats.PatternSpecificRuntimeRows
	result.PatternSpecificCaptureRows = scenarioStats.PatternSpecificCaptureRows
	result.SurfacePatternGaps = scenarioStats.PatternSpecificGaps
	result.AuthorityAtomicRows = scenarioStats.AuthorityAtomicRows
	result.SurfacePatternEligible = scenarioStats.CompletionEligibleRows
	if scenarioStats.PatternSpecificGaps > 0 || scenarioStats.CompletionEligibleRows != scenarioStats.Rows {
		addDefinitiveWarning(result, "surface-pattern-proof-gaps", "", fmt.Sprintf("Surface/Pattern %d row中gap=%d、completion eligible=%dです。", scenarioStats.Rows, scenarioStats.PatternSpecificGaps, scenarioStats.CompletionEligibleRows))
	}
	if scenarioStats.IntegrationPassed > 0 && scenarioStats.CompletionEligibleRows < scenarioStats.Rows {
		addDefinitiveWarning(result, "integrated-trace-not-component-proof", "", "統合Scenario Traceは各Surface/Pattern rowの固有Evidence、Runtime Identity、Atomic Authority Bindingを代替しません。")
	}
	if err := validateIntegrationDepthProofs(root, composition, reference, composition.IntegrationProofs); err != nil {
		return false, err
	}
	result.IntegrationProofsValid = true
	depthComplete, err := validateSubjectDepthParity(root, composition, reference, referenceStates, result)
	if err != nil {
		return false, err
	}
	result.DepthParityEligible = depthComplete
	if !depthComplete {
		addDefinitiveWarning(result, "integration-proof-cannot-promote-depth-gap", "", "Interop固有ProofがpassでもSubject Depth Parityの不足は解消されません。")
	}
	return depthComplete, nil
}

func ValidateDepthInheritance(root string) error {
	result, err := EvaluateDefinitiveComposition(root, "compositions/fixture-stage2-v2-definitive.preview.json", "2026-08-28T12:00:00Z")
	if err != nil {
		return err
	}
	if result.DepthReferenceStatus != "incomplete" || result.DepthParityEligible || !result.IntegrationProofsValid || result.DefinitiveEligible || result.EffectiveState != "incomplete" || result.IntegratedScenariosPassed != 10 || result.SurfacePatternRows != 850 || result.PatternSpecificRows != 429 || result.RuntimeIdentityRows != 170 || result.PatternSpecificCaptureRows != 259 || result.SurfacePatternGaps != 421 || result.AuthorityAtomicRows != 0 || result.SurfacePatternEligible != 0 {
		return fmt.Errorf("Subject Depth不足とInterop Proofの分離が保持されていません")
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
		return fmt.Errorf("各構成SubjectのDepth不足が保持されていません")
	}
	if !scenarioWarnings["surface-pattern-proof-gaps"] || !scenarioWarnings["integrated-trace-not-component-proof"] {
		return fmt.Errorf("統合Scenario成功とSurface/Pattern Proof不足の分離が保持されていません")
	}
	return nil
}

func loadFEDepthReference(root string, artifact PreviewArtifactLock) (feDepthReference, map[string]string, error) {
	var lock FEDepthReferenceLock
	if err := loadLockedJSON(root, artifact, &lock); err != nil {
		return feDepthReference{}, nil, err
	}
	if lock.SchemaVersion != 1 || lock.ID != "fe-depth-reference-v1" || lock.Repository != "frontend-behavior-atlas" || lock.SourceURL != "https://github.com/akaitigo/frontend-behavior-atlas/blob/"+feDepthReferenceCommit+"/FE_DEPTH_REFERENCE.json" || lock.License != "Apache-2.0" || lock.Commit != feDepthReferenceCommit || lock.Path != "FE_DEPTH_REFERENCE.json" || lock.Digest != feDepthReferenceDigest || lock.ExpectedStatus != "incomplete" || lock.ExpectedSummary != (DepthSummary{Satisfied: 1, Partial: 17, Missing: 0}) {
		return feDepthReference{}, nil, fmt.Errorf("FE Depth Reference Lockが確定値と一致しません")
	}
	repository := filepath.Join(root, "..", lock.Repository)
	data, err := exec.Command("git", "-C", repository, "show", lock.Commit+":"+lock.Path).Output()
	if err != nil {
		return feDepthReference{}, nil, fmt.Errorf("FE Depth Reference Git Objectを読めません: %w", err)
	}
	if DigestBytes(data) != lock.Digest {
		return feDepthReference{}, nil, fmt.Errorf("FE Depth Reference Digest不一致")
	}
	var reference feDepthReference
	if err := json.Unmarshal(data, &reference); err != nil {
		return feDepthReference{}, nil, err
	}
	if reference.SchemaVersion != 1 || reference.ID != lock.ID || reference.Status != lock.ExpectedStatus || reference.Summary != lock.ExpectedSummary || len(reference.Axes) != 18 {
		return feDepthReference{}, nil, fmt.Errorf("FE Depth ReferenceのStatus、Summaryまたは18軸契約が不一致です")
	}
	states, summary, err := depthAxisStates(reference.Axes)
	if err != nil || summary != reference.Summary {
		return feDepthReference{}, nil, fmt.Errorf("FE Depth ReferenceのAxis集計が不一致です")
	}
	return reference, states, nil
}

func validateSubjectDepthParity(root string, composition PreviewComposition, reference feDepthReference, referenceStates map[string]string, result *DefinitiveGateResult) (bool, error) {
	var manifest SubjectDepthManifest
	if err := loadLockedJSON(root, composition.SubjectDepthParity, &manifest); err != nil {
		return false, err
	}
	if manifest.SchemaVersion != 1 || manifest.ID != "fixture-subjects-depth-parity-preview" || manifest.ReferenceID != reference.ID || manifest.ReferenceCommit != feDepthReferenceCommit || manifest.ReferenceDigest != feDepthReferenceDigest {
		return false, fmt.Errorf("Subject Depth Parity ManifestのReference Lockが不一致です")
	}
	bySubject := map[string]SubjectDepthParity{}
	for _, subject := range manifest.Subjects {
		if bySubject[subject.SubjectID].SubjectID != "" {
			return false, fmt.Errorf("Subject Depth Parityが重複しています: %s", subject.SubjectID)
		}
		bySubject[subject.SubjectID] = subject
	}
	allSatisfied := true
	for _, subjectRef := range composition.Subjects {
		subject, ok := bySubject[subjectRef.SubjectID]
		if !ok {
			return false, fmt.Errorf("構成SubjectのDepth Parityがありません: %s", subjectRef.SubjectID)
		}
		states := map[string]string{}
		summary := DepthSummary{}
		for _, axis := range subject.Axes {
			if _, exists := states[axis.ID]; exists {
				return false, fmt.Errorf("Subject Depth Axisが重複しています: %s:%s", subject.SubjectID, axis.ID)
			}
			if _, exists := referenceStates[axis.ID]; !exists || depthRank(axis.Status) < 0 {
				return false, fmt.Errorf("Subject Depth AxisがFE確定18軸と一致しません: %s:%s", subject.SubjectID, axis.ID)
			}
			states[axis.ID] = axis.Status
			addDepthStatus(&summary, axis.Status)
			if axis.Status == "satisfied" {
				if len(axis.Proofs) == 0 {
					return false, fmt.Errorf("satisfied Depth AxisにProofがありません: %s:%s", subject.SubjectID, axis.ID)
				}
				for _, proof := range axis.Proofs {
					if err := validateDepthProof(root, proof); err != nil {
						return false, err
					}
				}
			}
		}
		subjectComplete := summary.Satisfied == len(referenceStates) && summary.Partial == 0 && summary.Missing == 0
		expectedStatus := "incomplete"
		if subjectComplete {
			expectedStatus = "complete"
		}
		if len(states) != len(referenceStates) || summary != subject.Summary || subject.Status != expectedStatus {
			return false, fmt.Errorf("Subject Depth Parityの18軸、SummaryまたはStatusが不一致です: %s", subject.SubjectID)
		}
		for axisID := range referenceStates {
			if states[axisID] == "" {
				return false, fmt.Errorf("Subject Depth Axisがありません: %s:%s", subject.SubjectID, axisID)
			}
		}
		if !subjectComplete {
			allSatisfied = false
			addDefinitiveWarning(result, "subject-depth-parity-incomplete", subjectRef.Name, fmt.Sprintf("18軸中satisfied=%d、partial=%d、missing=%dです。", summary.Satisfied, summary.Partial, summary.Missing))
		}
		delete(bySubject, subjectRef.SubjectID)
	}
	if len(bySubject) != 0 {
		return false, fmt.Errorf("Composition外のSubject Depth Parityがあります")
	}
	return allSatisfied, nil
}

func validateIntegrationDepthProofs(root string, composition PreviewComposition, reference feDepthReference, artifact PreviewArtifactLock) error {
	var manifest IntegrationDepthManifest
	if err := loadLockedJSON(root, artifact, &manifest); err != nil {
		return err
	}
	legacyDigest, err := DigestFile(resolve(root, "compositions/fixture-stage2.json"))
	if err != nil {
		return err
	}
	if manifest.SchemaVersion != 1 || manifest.SourceCompositionID != "fixture-http-stage2-v1" || manifest.SourceCompositionDigest != legacyDigest || !contains(manifest.TargetCompositionIDs, composition.ID) || manifest.Scope != "bounded-v1-fixture" || manifest.Verdict != "pass" || reference.Status != "incomplete" {
		return fmt.Errorf("Interop Depth Proof ManifestのScopeまたはSource Lockが不一致です")
	}
	covered := map[string]bool{}
	for _, proof := range manifest.Proofs {
		if proof.ID == "" || proof.Axis == "" || proof.ScenarioID == "" || covered[proof.Axis] {
			return fmt.Errorf("Interop Depth Proof IDまたはAxisが不正です")
		}
		covered[proof.Axis] = true
		profiles := map[string]bool{}
		for _, item := range proof.Profiles {
			if profiles[item.Profile] || (item.Profile != "local" && item.Profile != "container") {
				return fmt.Errorf("Interop Depth Proof Profileが不正です: %s", proof.ID)
			}
			profiles[item.Profile] = true
			if actual, err := DigestFile(resolve(root, item.Path)); err != nil || actual != item.Digest {
				return fmt.Errorf("Interop Depth Proof Digest不一致: %s:%s", proof.ID, item.Profile)
			}
			var report ScenarioReport
			if err := LoadJSON(resolve(root, item.Path), &report); err != nil {
				return err
			}
			if report.Verdict != "pass" || report.Profile != item.Profile || report.ScenarioID != proof.ScenarioID || report.CompositionDigest != manifest.SourceCompositionDigest || !contains(report.Axes, proof.Axis) {
				return fmt.Errorf("Interop Depth Proofが実Scenario Observableへ接続されていません: %s:%s", proof.ID, item.Profile)
			}
		}
		if !profiles["local"] || !profiles["container"] {
			return fmt.Errorf("Interop Depth Proofはlocal/containerが必須です: %s", proof.ID)
		}
	}
	if !sameSet(sortedKeys(covered), RequiredAxes) {
		return fmt.Errorf("Interop Depth Proofは10検証軸を過不足なく必要とします")
	}
	return nil
}

func loadLockedJSON(root string, artifact PreviewArtifactLock, value any) error {
	if artifact.Path == "" || artifact.Digest == "" {
		return fmt.Errorf("Depth Artifact Lockがありません")
	}
	actual, err := DigestFile(resolve(root, artifact.Path))
	if err != nil {
		return err
	}
	if actual != artifact.Digest {
		return fmt.Errorf("Depth Artifact Digest不一致: %s", artifact.Path)
	}
	return LoadJSON(resolve(root, artifact.Path), value)
}

func depthAxisStates(axes []feDepthAxis) (map[string]string, DepthSummary, error) {
	states := map[string]string{}
	summary := DepthSummary{}
	for _, axis := range axes {
		if axis.ID == "" || states[axis.ID] != "" || depthRank(axis.Status) < 0 {
			return nil, DepthSummary{}, fmt.Errorf("Depth Axisが不正です")
		}
		states[axis.ID] = axis.Status
		addDepthStatus(&summary, axis.Status)
	}
	return states, summary, nil
}

func addDepthStatus(summary *DepthSummary, status string) {
	switch status {
	case "satisfied":
		summary.Satisfied++
	case "partial":
		summary.Partial++
	case "missing":
		summary.Missing++
	}
}

func depthRank(status string) int {
	switch status {
	case "missing":
		return 0
	case "partial":
		return 1
	case "satisfied":
		return 2
	default:
		return -1
	}
}

func validateDepthProof(root string, proof DepthProofRef) error {
	actual, err := DigestFile(resolve(root, proof.Path))
	if err != nil || actual != proof.Digest {
		return fmt.Errorf("Subject Depth Proof Digest不一致: %s", proof.ID)
	}
	var record map[string]any
	if err := LoadJSON(resolve(root, proof.Path), &record); err != nil {
		return err
	}
	if record["verdict"] != "pass" {
		return fmt.Errorf("Subject Depth Proofがpassではありません: %s", proof.ID)
	}
	return nil
}
