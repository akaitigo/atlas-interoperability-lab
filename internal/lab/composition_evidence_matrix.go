package lab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CompositionEvidenceMatrix struct {
	SchemaVersion int                             `json:"schema_version"`
	Graph         string                          `json:"graph"`
	Cases         []CompositionEvidenceMatrixCase `json:"cases"`
}

type CompositionEvidenceMatrixCase struct {
	ID            string `json:"id"`
	Mutation      string `json:"mutation"`
	ExpectedState string `json:"expected_state"`
	ExpectedError string `json:"expected_error,omitempty"`
}

type CompositionEvidenceMatrixResult struct {
	SchemaVersion int                                   `json:"schema_version"`
	CoreCommit    string                                `json:"core_commit"`
	Results       []CompositionEvidenceMatrixCaseResult `json:"results"`
	Verdict       string                                `json:"verdict"`
}

type CompositionEvidenceMatrixCaseResult struct {
	ID             string `json:"id"`
	Mutation       string `json:"mutation"`
	State          string `json:"state"`
	Classification string `json:"classification"`
	Verdict        string `json:"verdict"`
}

func RunCompositionEvidenceDependencyMatrix(root, matrixPath string) (CompositionEvidenceMatrixResult, error) {
	var matrix CompositionEvidenceMatrix
	if err := LoadJSON(filepath.Join(root, filepath.FromSlash(matrixPath)), &matrix); err != nil {
		return CompositionEvidenceMatrixResult{}, err
	}
	if matrix.SchemaVersion != 1 || matrix.Graph != compositionEvidenceGraphPath || len(matrix.Cases) != 10 {
		return CompositionEvidenceMatrixResult{}, fmt.Errorf("Composition Evidence Dependency Matrix契約が不正です")
	}
	report := CompositionEvidenceMatrixResult{SchemaVersion: 1, CoreCommit: evidenceDependencyCoreCommit, Results: []CompositionEvidenceMatrixCaseResult{}, Verdict: "pass"}
	seen := map[string]bool{}
	for _, testCase := range matrix.Cases {
		if testCase.ID == "" || seen[testCase.ID] || (testCase.ExpectedState != "pass" && testCase.ExpectedState != "reject") {
			return report, fmt.Errorf("Composition Evidence Dependency caseが不正です: %s", testCase.ID)
		}
		seen[testCase.ID] = true
		caseRoot, err := copyCompositionEvidenceFixture(root, matrix.Graph)
		if err != nil {
			return report, err
		}
		if err := applyCompositionEvidenceMutation(caseRoot, matrix.Graph, testCase.Mutation); err != nil {
			os.RemoveAll(caseRoot)
			return report, err
		}
		_, auditErr := AuditCompositionEvidenceDependency(caseRoot, matrix.Graph)
		state := "pass"
		if auditErr != nil {
			state = "reject"
		}
		verdict := "pass"
		if state != testCase.ExpectedState || (testCase.ExpectedError != "" && (auditErr == nil || !strings.Contains(auditErr.Error(), testCase.ExpectedError))) {
			verdict, report.Verdict = "fail", "fail"
		}
		report.Results = append(report.Results, CompositionEvidenceMatrixCaseResult{ID: testCase.ID, Mutation: testCase.Mutation, State: state, Classification: testCase.Mutation, Verdict: verdict})
		os.RemoveAll(caseRoot)
	}
	if report.Verdict != "pass" {
		return report, fmt.Errorf("Composition Evidence Dependency Matrixがfailです")
	}
	return report, nil
}

func ValidateCompositionEvidenceClosure(root string) error {
	audit, err := AuditCompositionEvidenceDependency(root, compositionEvidenceGraphPath)
	if err != nil {
		return err
	}
	if audit.Verdict != "pass" || audit.CompletionState != "incomplete" || audit.DefinitiveEligible || !containsAll(audit.Gaps, []string{"process-executable-attestation-unavailable", "subject-depth-parity-incomplete", "subject-v2-certificate-atomic-binding-unavailable", "surface-pattern-proof-gaps"}) {
		return fmt.Errorf("Composition Evidence closureが未完Gapを保持していません")
	}
	matrix, err := RunCompositionEvidenceDependencyMatrix(root, "tests/fixtures/composition-evidence-dependency.matrix.json")
	if err != nil {
		return err
	}
	if matrix.Verdict != "pass" || len(matrix.Results) != 10 {
		return fmt.Errorf("Composition Evidence Dependency negative fixtureが不足しています")
	}
	return nil
}

func copyCompositionEvidenceFixture(root, graphPath string) (string, error) {
	var graph CompositionEvidenceDependencyGraph
	if err := LoadJSON(filepath.Join(root, filepath.FromSlash(graphPath)), &graph); err != nil {
		return "", err
	}
	temporaryRoot, err := os.MkdirTemp("", "atlas-lab-composition-evidence-matrix-")
	if err != nil {
		return "", err
	}
	paths := []string{graphPath}
	for _, input := range graph.Inputs {
		paths = append(paths, input.Members...)
	}
	paths = append(paths, graph.RequiredOutputs...)
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			os.RemoveAll(temporaryRoot)
			return "", err
		}
		if err := copyFile(filepath.Join(root, filepath.FromSlash(path)), filepath.Join(temporaryRoot, filepath.FromSlash(path)), info.Mode().Perm()); err != nil {
			os.RemoveAll(temporaryRoot)
			return "", err
		}
	}
	return temporaryRoot, nil
}

func applyCompositionEvidenceMutation(root, graphPath, mutation string) error {
	if mutation == "none" {
		return nil
	}
	path := filepath.Join(root, filepath.FromSlash(graphPath))
	var graph CompositionEvidenceDependencyGraph
	if err := LoadJSON(path, &graph); err != nil {
		return err
	}
	switch mutation {
	case "stale-status":
		graph.Status = "stale"
	case "digest-only-closure":
		input := &graph.Inputs[1]
		member := filepath.Join(root, filepath.FromSlash(input.Members[0]))
		file, err := os.OpenFile(member, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString("\nchanged without rerun\n")
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
		digest, err := aggregateCompositionMembers(root, input.Members)
		if err != nil {
			return err
		}
		input.CurrentDigest, input.ObservedAt = digest, "2099-01-01T00:00:00Z"
		for runIndex := range graph.Runs {
			for bindingIndex := range graph.Runs[runIndex].InputBindings {
				if graph.Runs[runIndex].InputBindings[bindingIndex].InputID == input.ID {
					graph.Runs[runIndex].InputBindings[bindingIndex].Digest = digest
				}
			}
		}
	case "missing-local-rerun-target":
		removeRunOutput(&graph, "run-local-runtime-binding", "local-normal")
	case "missing-container-rerun-target":
		removeRunOutput(&graph, "run-container-runtime-binding", "container-normal")
	case "output-withdrawal":
		withdrawCompositionOutput(&graph, "local-failure")
	case "proof-structure-shrink":
		if err := shrinkCompositionStructure(root, &graph, "scenario-proof-index", "runtime-proof-index", "\n"); err != nil {
			return err
		}
	case "closure-structure-shrink":
		if err := shrinkCompositionStructure(root, &graph, "scenario-closure-plan", "evidence-closure-plan", "\n"); err != nil {
			return err
		}
	case "definitive-promotion":
		graph.CompletionState, graph.DefinitiveEligible, graph.Gaps = "definitive-complete", true, []string{}
	case "runtime-identity-drift":
		graph.Runs[0].RuntimeIdentity["runtime_binary_digest"] = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	default:
		return fmt.Errorf("未知のComposition Evidence mutationです: %s", mutation)
	}
	return WriteJSON(path, graph)
}

func removeRunOutput(graph *CompositionEvidenceDependencyGraph, runID, outputID string) {
	for index := range graph.Runs {
		if graph.Runs[index].ID == runID {
			result := []string{}
			for _, id := range graph.Runs[index].OutputIDs {
				if id != outputID {
					result = append(result, id)
				}
			}
			graph.Runs[index].OutputIDs = result
		}
	}
}

func withdrawCompositionOutput(graph *CompositionEvidenceDependencyGraph, outputID string) {
	outputs := []CompositionEvidenceOutput{}
	removedPath := ""
	for _, output := range graph.Outputs {
		if output.ID == outputID {
			removedPath = output.Path
			continue
		}
		outputs = append(outputs, output)
	}
	graph.Outputs = outputs
	required := []string{}
	for _, path := range graph.RequiredOutputs {
		if path != removedPath {
			required = append(required, path)
		}
	}
	graph.RequiredOutputs = required
	for index := range graph.Runs {
		removeRunOutput(graph, graph.Runs[index].ID, outputID)
	}
}

func shrinkCompositionStructure(root string, graph *CompositionEvidenceDependencyGraph, kind, outputID, suffix string) error {
	structurePath := ""
	for _, structure := range graph.Structures {
		if structure.Kind == kind {
			structurePath = structure.Path
		}
	}
	if structurePath == "" {
		return fmt.Errorf("structure fixtureがありません: %s", kind)
	}
	full := filepath.Join(root, filepath.FromSlash(structurePath))
	file, err := os.OpenFile(full, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(suffix)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	digest, err := DigestFile(full)
	if err != nil {
		return err
	}
	for index := range graph.Outputs {
		if graph.Outputs[index].ID == outputID {
			graph.Outputs[index].Digest = digest
		}
	}
	return nil
}
