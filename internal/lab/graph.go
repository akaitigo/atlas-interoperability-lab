package lab

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type ClaimEvidenceGraph struct {
	SchemaVersion int            `json:"schema_version"`
	CompositionID string         `json:"composition_id"`
	Subjects      []GraphSubject `json:"subjects"`
	Links         []GraphLink    `json:"links"`
}

type GraphSubject struct {
	Name              string `json:"name"`
	SubjectID         string `json:"subject_id"`
	ReleaseDigest     string `json:"release_digest"`
	CertificateDigest string `json:"certificate_digest"`
}

type GraphLink struct {
	Axis         string   `json:"axis"`
	ClaimID      string   `json:"claim_id"`
	SubjectNames []string `json:"subject_names"`
	ScenarioIDs  []string `json:"scenario_ids"`
	EvidenceIDs  []string `json:"evidence_ids"`
}

func ValidateCrossSubjectGraph(root string) error {
	return validateCrossSubjectGraphFile(root, filepath.Join(root, "graphs", "fixture-stage2.claim-evidence.json"))
}

func validateCrossSubjectGraphFile(root, graphPath string) error {
	validated, err := Preflight(root, "compositions/fixture-stage2.json", "local")
	if err != nil {
		return err
	}
	var graph ClaimEvidenceGraph
	if err := LoadJSON(graphPath, &graph); err != nil {
		return err
	}
	if graph.SchemaVersion != 1 || graph.CompositionID != validated.Manifest.ID {
		return fmt.Errorf("Claim/Evidence GraphのComposition IDが一致しません")
	}
	compositionSubjects := map[string]SubjectRef{}
	for _, subject := range validated.Manifest.Subjects {
		compositionSubjects[subject.Name] = subject
	}
	graphSubjects := map[string]GraphSubject{}
	for _, subject := range graph.Subjects {
		if _, duplicate := graphSubjects[subject.Name]; duplicate {
			return fmt.Errorf("Claim/Evidence GraphのSubjectが重複しています: %s", subject.Name)
		}
		ref, ok := compositionSubjects[subject.Name]
		if !ok || ref.SubjectID != subject.SubjectID || ref.ReleaseDigest != subject.ReleaseDigest || ref.CertificateDigest != subject.CertificateDigest {
			return fmt.Errorf("Claim/Evidence GraphのSubject LockがCompositionと一致しません: %s", subject.Name)
		}
		graphSubjects[subject.Name] = subject
	}
	if len(graphSubjects) != len(compositionSubjects) {
		return fmt.Errorf("Claim/Evidence Graphに全Subjectがありません")
	}
	scenarios := map[string]Scenario{}
	for _, scenarioPath := range validated.Manifest.Scenarios {
		scenario, err := ValidateScenario(root, scenarioPath, validated.Manifest.Axes)
		if err != nil {
			return err
		}
		scenarios[scenario.ID] = scenario
	}
	evidence := map[string]map[string]any{}
	for _, profile := range []string{"local", "container"} {
		path := filepath.Join(root, "evidence", "records", "stage2."+profile+".evidence.json")
		var record map[string]any
		if err := LoadJSON(path, &record); err != nil {
			return err
		}
		id, _ := record["id"].(string)
		if record["verdict"] != "pass" {
			return fmt.Errorf("Graph Evidenceがpassではありません: %s", id)
		}
		artifact, _ := record["artifact"].(map[string]any)
		uri, _ := artifact["uri"].(string)
		expectedDigest, _ := artifact["digest"].(string)
		actualDigest, err := DigestFile(filepath.Join(root, filepath.FromSlash(uri)))
		if err != nil || actualDigest != expectedDigest {
			return fmt.Errorf("Graph Evidence ArtifactのDigest不一致: %s", id)
		}
		evidence[id] = record
	}
	seenAxes := []string{}
	for _, link := range graph.Links {
		if contains(seenAxes, link.Axis) {
			return fmt.Errorf("Claim/Evidence GraphのAxisが重複しています: %s", link.Axis)
		}
		seenAxes = append(seenAxes, link.Axis)
		if !sameSet(link.SubjectNames, sortedKeys(graphSubjects)) {
			return fmt.Errorf("Claim %sが全Subjectを横断していません", link.ClaimID)
		}
		var claim map[string]any
		if err := LoadJSON(filepath.Join(root, "claims", link.ClaimID+".claim.json"), &claim); err != nil {
			return err
		}
		if claim["id"] != link.ClaimID || claim["status"] != "accepted" {
			return fmt.Errorf("Graph Claimがacceptedではありません: %s", link.ClaimID)
		}
		for _, scenarioID := range link.ScenarioIDs {
			scenario, ok := scenarios[scenarioID]
			if !ok || !contains(scenario.Axes, link.Axis) {
				return fmt.Errorf("Claim %sがAxisを検証するScenarioへ接続されていません: %s", link.ClaimID, scenarioID)
			}
		}
		for _, evidenceID := range link.EvidenceIDs {
			record, ok := evidence[evidenceID]
			if !ok || !containsAny(record["claim_ids"], link.ClaimID) {
				return fmt.Errorf("Claim %sがEvidence %sへ接続されていません", link.ClaimID, evidenceID)
			}
			if err := evidenceContainsScenarios(root, record, link.ScenarioIDs); err != nil {
				return fmt.Errorf("Claim %s: %w", link.ClaimID, err)
			}
		}
	}
	if !sameSet(seenAxes, RequiredAxes) {
		return fmt.Errorf("Claim/Evidence Graphが10検証軸を閉じていません")
	}
	return nil
}

func evidenceContainsScenarios(root string, record map[string]any, scenarioIDs []string) error {
	artifact, _ := record["artifact"].(map[string]any)
	uri, _ := artifact["uri"].(string)
	var summary RunSummary
	if err := LoadJSON(filepath.Join(root, filepath.FromSlash(uri)), &summary); err != nil {
		return err
	}
	reports := map[string]bool{}
	for _, report := range summary.ScenarioReports {
		base := filepath.Base(report)
		reports[strings.TrimSuffix(base, filepath.Ext(base))] = true
	}
	for _, scenarioID := range scenarioIDs {
		if !reports[scenarioID] {
			return fmt.Errorf("EvidenceにScenarioがありません: %s", scenarioID)
		}
	}
	return nil
}

func containsAny(raw any, expected string) bool {
	items, _ := raw.([]any)
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
