package lab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const compositionEvidenceGraphPath = "evidence/preview/composition-evidence-dependency.json"

type CompositionEvidenceDependencyGraph struct {
	SchemaVersion      int                                 `json:"schema_version"`
	AtlasID            string                              `json:"atlas_id"`
	CoreCommit         string                              `json:"core_commit"`
	GeneratedAt        string                              `json:"generated_at"`
	Status             string                              `json:"status"`
	Policy             CompositionEvidenceDependencyPolicy `json:"policy"`
	Inputs             []CompositionEvidenceInput          `json:"inputs"`
	Outputs            []CompositionEvidenceOutput         `json:"outputs"`
	Runs               []CompositionEvidenceRun            `json:"runs"`
	RequiredOutputs    []string                            `json:"required_outputs"`
	Structures         []CompositionEvidenceStructure      `json:"structures"`
	CompletionState    string                              `json:"completion_state"`
	Gaps               []string                            `json:"gaps"`
	DefinitiveEligible bool                                `json:"definitive_eligible"`
}

type CompositionEvidenceDependencyPolicy struct {
	TransitiveStaleness           bool `json:"transitive_staleness"`
	DigestOnlyClosureForbidden    bool `json:"digest_only_closure_forbidden"`
	ActualRerunRequired           bool `json:"actual_rerun_required"`
	MissingRerunTargetsFail       bool `json:"missing_rerun_targets_fail"`
	ProofStructureInvariant       bool `json:"proof_structure_invariant"`
	ClosurePlanStructureInvariant bool `json:"closure_plan_structure_invariant"`
}

type CompositionEvidenceInput struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Members        []string `json:"members"`
	BaselineDigest string   `json:"baseline_digest"`
	CurrentDigest  string   `json:"current_digest"`
	ObservedAt     string   `json:"observed_at"`
}

type CompositionEvidenceOutput struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	Digest    string   `json:"digest"`
	DependsOn []string `json:"depends_on"`
	Status    string   `json:"status"`
	RunID     string   `json:"run_id"`
}

type CompositionEvidenceBinding struct {
	InputID string `json:"input_id"`
	Digest  string `json:"digest"`
}

type CompositionEvidenceRun struct {
	ID              string                       `json:"id"`
	ExecutionKind   string                       `json:"execution_kind"`
	Command         string                       `json:"command"`
	StartedAt       string                       `json:"started_at"`
	CompletedAt     string                       `json:"completed_at"`
	Result          string                       `json:"result"`
	Attempts        int                          `json:"attempts"`
	RuntimeIdentity map[string]string            `json:"runtime_identity,omitempty"`
	InputBindings   []CompositionEvidenceBinding `json:"input_bindings"`
	OutputIDs       []string                     `json:"output_ids"`
}

type CompositionEvidenceStructure struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	Path           string `json:"path"`
	BaselineDigest string `json:"baseline_digest"`
}

type CompositionEvidenceAudit struct {
	SchemaVersion      int      `json:"schema_version"`
	CoreCommit         string   `json:"core_commit"`
	Inputs             int      `json:"inputs"`
	Outputs            int      `json:"outputs"`
	Runs               int      `json:"runs"`
	ChangedInputs      int      `json:"changed_inputs"`
	AffectedOutputs    int      `json:"affected_outputs"`
	CompletionState    string   `json:"completion_state"`
	Gaps               []string `json:"gaps"`
	DefinitiveEligible bool     `json:"definitive_eligible"`
	Verdict            string   `json:"verdict"`
}

func GenerateCompositionEvidenceClosure(root string) (CompositionEvidenceAudit, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if _, err := NonRegressionGate(absoluteRoot); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	pinnedRepositoryRoot := absoluteRoot
	if configured := os.Getenv("ATLAS_LAB_REFERENCE_ROOT"); configured != "" {
		pinnedRepositoryRoot, err = filepath.Abs(configured)
		if err != nil {
			return CompositionEvidenceAudit{}, err
		}
	}
	temporaryRoot, err := os.MkdirTemp("", "atlas-lab-composition-closure-")
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	defer os.RemoveAll(temporaryRoot)
	if err := copyRuntimeRepository(absoluteRoot, temporaryRoot); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if err := writeCompositionEvidenceStructures(temporaryRoot); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	subjectBinding, err := EvaluateSubjectBindingAdmission(temporaryRoot, pinnedRepositoryRoot)
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if err := WriteJSON(filepath.Join(temporaryRoot, "evidence", "preview", "subject-binding-admission.json"), subjectBinding); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	subjectBindingMatrix, err := RunSubjectBindingAdmissionMatrix(temporaryRoot, "tests/fixtures/subject-binding-admission.matrix.json")
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if err := WriteJSON(filepath.Join(temporaryRoot, "evidence", "preview", "subject-binding-admission.matrix.json"), subjectBindingMatrix); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	local, err := generateRuntimeBindingEvidence(temporaryRoot, "local", false)
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if err := WriteJSON(filepath.Join(temporaryRoot, "evidence", "preview", "runtime-binding", "local.binding.json"), local); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	container, err := generateRuntimeBindingEvidence(temporaryRoot, "container", false)
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if err := WriteJSON(filepath.Join(temporaryRoot, "evidence", "preview", "runtime-binding", "container.binding.json"), container); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	derivedStarted := time.Now().UTC()
	matrix, err := runCompositionCompatibilityMatrix(temporaryRoot, "tests/fixtures/composition-compatibility.matrix.json", false, pinnedRepositoryRoot)
	derivedCompleted := time.Now().UTC()
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if err := WriteJSON(filepath.Join(temporaryRoot, "evidence", "preview", "composition-compatibility.matrix.json"), matrix); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	graph, err := buildCompositionEvidenceGraph(temporaryRoot, local, container, derivedStarted, derivedCompleted)
	if err != nil {
		return CompositionEvidenceAudit{}, err
	}
	if err := WriteJSON(filepath.Join(temporaryRoot, filepath.FromSlash(compositionEvidenceGraphPath)), graph); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	audit, err := AuditCompositionEvidenceDependency(temporaryRoot, compositionEvidenceGraphPath)
	if err != nil {
		return audit, err
	}
	negativeMatrix, err := RunCompositionEvidenceDependencyMatrix(temporaryRoot, "tests/fixtures/composition-evidence-dependency.matrix.json")
	if err != nil {
		return audit, err
	}
	if err := WriteJSON(filepath.Join(temporaryRoot, "evidence", "preview", "composition-evidence-dependency.matrix.json"), negativeMatrix); err != nil {
		return audit, err
	}
	paths := append([]string{}, graph.RequiredOutputs...)
	paths = append(paths, compositionEvidenceGraphPath, "evidence/preview/composition-evidence-dependency.matrix.json")
	for _, path := range paths {
		info, err := os.Stat(filepath.Join(temporaryRoot, filepath.FromSlash(path)))
		if err != nil {
			return audit, err
		}
		if err := copyFile(filepath.Join(temporaryRoot, filepath.FromSlash(path)), filepath.Join(absoluteRoot, filepath.FromSlash(path)), info.Mode().Perm()); err != nil {
			return audit, err
		}
	}
	return AuditCompositionEvidenceDependency(absoluteRoot, compositionEvidenceGraphPath)
}

func writeCompositionEvidenceStructures(root string) error {
	proof := map[string]any{
		"schema_version": 1, "id": "fixture-stage2-runtime-proof-index", "authority_denominator": "interop-runtime-binding-only",
		"rows": []map[string]any{
			{"id": "local-runtime-binding", "profile": "local", "scenarios": []string{"normal", "rejection", "failure", "recovery", "compatibility"}, "binding": "evidence/preview/runtime-binding/local.binding.json", "gaps": []string{"subject-v2-certificate-atomic-binding-unavailable"}},
			{"id": "container-runtime-binding", "profile": "container", "scenarios": []string{"normal", "rejection", "failure", "recovery", "compatibility"}, "binding": "evidence/preview/runtime-binding/container.binding.json", "gaps": []string{"subject-v2-certificate-atomic-binding-unavailable"}},
		},
	}
	plan := map[string]any{
		"schema_version": 1, "id": "fixture-stage2-evidence-closure-preview", "core_commit": evidenceDependencyCoreCommit, "policy": "no-gap-aggregation",
		"tranches": []map[string]any{
			{"id": "runtime-binding", "state": "completed-with-explicit-gaps", "rows": []string{"local-runtime-binding", "container-runtime-binding"}},
			{"id": "actual-subject-binding-readiness", "state": "completed-with-explicit-rejections", "rows": []string{"rabbitmq-reference-atlas", "postgresql-reference-atlas", "zero-trust-reference-atlas"}},
			{"id": "subject-v2-atomic-binding", "state": "blocked-by-subject-authority", "rows": []string{"source-v2-certificate", "sink-v2-certificate"}},
		},
		"definitive_eligible": false,
	}
	if err := WriteJSON(filepath.Join(root, "evidence", "preview", "runtime-binding", "proof-index.json"), proof); err != nil {
		return err
	}
	return WriteJSON(filepath.Join(root, "evidence", "preview", "runtime-binding", "closure-plan.json"), plan)
}

func buildCompositionEvidenceGraph(root string, local, container RuntimeBindingEvidence, derivedStarted, derivedCompleted time.Time) (CompositionEvidenceDependencyGraph, error) {
	members, err := compositionEvidenceInputMembers(root)
	if err != nil {
		return CompositionEvidenceDependencyGraph{}, err
	}
	inputSpecs := []CompositionEvidenceInput{
		{ID: "repository-contract", Kind: "contract", Members: members["repository-contract"]},
		{ID: "composition-source", Kind: "source", Members: members["composition-source"]},
		{ID: "interop-harness", Kind: "harness", Members: members["interop-harness"]},
		{ID: "go-runtime", Kind: "runtime", Members: members["go-runtime"]},
		{ID: "local-profile", Kind: "profile", Members: members["local-profile"]},
		{ID: "container-profile", Kind: "profile", Members: members["container-profile"]},
	}
	observed := derivedStarted.UTC().Format(time.RFC3339)
	for index := range inputSpecs {
		digest, err := aggregateCompositionMembers(root, inputSpecs[index].Members)
		if err != nil {
			return CompositionEvidenceDependencyGraph{}, err
		}
		inputSpecs[index].BaselineDigest, inputSpecs[index].CurrentDigest, inputSpecs[index].ObservedAt = digest, digest, observed
	}
	inputDigest := map[string]string{}
	for _, input := range inputSpecs {
		inputDigest[input.ID] = input.CurrentDigest
	}
	localIDs, localOutputs, err := runtimeBindingOutputs(root, "local", []string{"repository-contract", "composition-source", "interop-harness", "go-runtime", "local-profile"}, "run-local-runtime-binding")
	if err != nil {
		return CompositionEvidenceDependencyGraph{}, err
	}
	containerIDs, containerOutputs, err := runtimeBindingOutputs(root, "container", []string{"repository-contract", "composition-source", "interop-harness", "go-runtime", "container-profile"}, "run-container-runtime-binding")
	if err != nil {
		return CompositionEvidenceDependencyGraph{}, err
	}
	outputs := append(localOutputs, containerOutputs...)
	derivedSpecs := []struct {
		id, kind, path string
		deps           []string
	}{
		{"composition-compatibility", "compatibility", "evidence/preview/composition-compatibility.matrix.json", []string{"local-binding", "container-binding", "composition-source", "interop-harness"}},
		{"actual-subject-binding-admission", "subject-admission", "evidence/preview/subject-binding-admission.json", []string{"composition-source", "interop-harness"}},
		{"actual-subject-binding-negative-matrix", "negative-fixture", "evidence/preview/subject-binding-admission.matrix.json", []string{"actual-subject-binding-admission", "composition-source", "interop-harness"}},
		{"runtime-proof-index", "scenario-proof", "evidence/preview/runtime-binding/proof-index.json", []string{"local-binding", "container-binding", "composition-source"}},
		{"evidence-closure-plan", "closure-plan", "evidence/preview/runtime-binding/closure-plan.json", []string{"runtime-proof-index", "composition-source"}},
	}
	derivedIDs := []string{}
	for _, spec := range derivedSpecs {
		digest, err := DigestFile(filepath.Join(root, filepath.FromSlash(spec.path)))
		if err != nil {
			return CompositionEvidenceDependencyGraph{}, err
		}
		outputs = append(outputs, CompositionEvidenceOutput{ID: spec.id, Kind: spec.kind, Path: spec.path, Digest: digest, DependsOn: spec.deps, Status: "current", RunID: "run-composition-closure"})
		derivedIDs = append(derivedIDs, spec.id)
	}
	allPaths := []string{}
	for _, output := range outputs {
		allPaths = append(allPaths, output.Path)
	}
	sort.Strings(allPaths)
	localStart, _ := time.Parse(time.RFC3339, local.ExecutionStartedAt)
	localComplete, _ := time.Parse(time.RFC3339, local.ExecutionCompletedAt)
	containerStart, _ := time.Parse(time.RFC3339, container.ExecutionStartedAt)
	containerComplete, _ := time.Parse(time.RFC3339, container.ExecutionCompletedAt)
	runs := []CompositionEvidenceRun{
		compositionRuntimeRun("run-local-runtime-binding", "go run ./cmd/atlas-lab runtime-binding --profile local", localStart, localComplete, local.Platform, local.Executable.RuntimeBinaryDigest, []string{"repository-contract", "composition-source", "interop-harness", "go-runtime", "local-profile"}, inputDigest, localIDs),
		compositionRuntimeRun("run-container-runtime-binding", "go run ./cmd/atlas-lab runtime-binding --profile container", containerStart, containerComplete, container.Platform, container.Executable.RuntimeBinaryDigest, []string{"repository-contract", "composition-source", "interop-harness", "go-runtime", "container-profile"}, inputDigest, containerIDs),
		{ID: "run-composition-closure", ExecutionKind: "derived", Command: "go run ./cmd/atlas-lab composition-evidence-closure", StartedAt: derivedStarted.UTC().Format(time.RFC3339), CompletedAt: derivedCompleted.UTC().Format(time.RFC3339), Result: "passed", Attempts: 1, InputBindings: compositionBindings([]string{"repository-contract", "composition-source", "interop-harness", "go-runtime", "local-profile", "container-profile"}, inputDigest), OutputIDs: derivedIDs},
	}
	proofDigest, _ := DigestFile(filepath.Join(root, "evidence", "preview", "runtime-binding", "proof-index.json"))
	planDigest, _ := DigestFile(filepath.Join(root, "evidence", "preview", "runtime-binding", "closure-plan.json"))
	return CompositionEvidenceDependencyGraph{
		SchemaVersion: 1, AtlasID: "atlas-interoperability-lab", CoreCommit: evidenceDependencyCoreCommit, GeneratedAt: derivedCompleted.UTC().Format(time.RFC3339), Status: "current",
		Policy: CompositionEvidenceDependencyPolicy{true, true, true, true, true, true}, Inputs: inputSpecs, Outputs: outputs, Runs: runs, RequiredOutputs: allPaths,
		Structures:      []CompositionEvidenceStructure{{ID: "runtime-proof-structure", Kind: "scenario-proof-index", Path: "evidence/preview/runtime-binding/proof-index.json", BaselineDigest: proofDigest}, {ID: "closure-plan-structure", Kind: "scenario-closure-plan", Path: "evidence/preview/runtime-binding/closure-plan.json", BaselineDigest: planDigest}},
		CompletionState: "incomplete", Gaps: []string{"subject-depth-parity-incomplete", "subject-v2-certificate-atomic-binding-unavailable", "surface-pattern-proof-gaps"}, DefinitiveEligible: false,
	}, nil
}

func compositionEvidenceInputMembers(root string) (map[string][]string, error) {
	harness := []string{"cmd/atlas-lab/main.go", "graphs/fixture-stage2.claim-evidence.json", "scenarios/normal.json", "scenarios/rejection.json", "scenarios/failure.json", "scenarios/recovery.json", "scenarios/compatibility.json", "oracles/http-json-v1.json", "oracles/failure-recovery-v1.json", "oracles/security-boundary-v1.json", "tests/fixtures/composition-compatibility.matrix.json", "tests/fixtures/composition-evidence-dependency.matrix.json", "tests/fixtures/subject-binding-admission.matrix.json"}
	matches, err := filepath.Glob(filepath.Join(root, "internal", "lab", "*.go"))
	if err != nil {
		return nil, err
	}
	for _, match := range matches {
		if strings.HasSuffix(match, "_test.go") {
			continue
		}
		relative, err := filepath.Rel(root, match)
		if err != nil {
			return nil, err
		}
		harness = append(harness, filepath.ToSlash(relative))
	}
	sort.Strings(harness)
	return map[string][]string{
		"repository-contract": {"repo.yaml"},
		"composition-source":  {"compatibility/evidence-dependency-core.lock.json", "compatibility/subject-binding-candidates.lock.json", "schemas/subject-binding-admission.schema.json", "compositions/fixture-stage2.json", "compositions/fixture-stage2-v2-definitive.preview.json", "migrations/runtime-binding-executable-attestation.json", "attestations/fe-upstream.local-report.json", "attestations/fe-upstream.attestation.json", "attestations/fe-upstream.attestation.json.sig", "attestations/fe-upstream.allowed-signers", "fixtures/subjects/fixture-http-source/release.json", "fixtures/subjects/fixture-http-source/completion-certificate.json", "fixtures/subjects/fixture-http-sink/release.json", "fixtures/subjects/fixture-http-sink/completion-certificate.json", "cmd/fixture-subject/main.go"},
		"interop-harness":     harness,
		"go-runtime":          {"go.mod"},
		"local-profile":       {"environments/local.json"},
		"container-profile":   {"environments/container.json", "environments/Dockerfile.fixture"},
	}, nil
}

func runtimeBindingOutputs(root, profile string, dependencies []string, runID string) ([]string, []CompositionEvidenceOutput, error) {
	paths := []string{"evidence/preview/runtime-binding/" + profile + ".binding.json"}
	for _, name := range []string{"summary.json", "normal.json", "rejection.json", "failure.json", "recovery.json", "compatibility.json", "cleanup.receipt.json"} {
		paths = append(paths, "evidence/preview/runtime-binding/"+profile+"/"+name)
	}
	ids, outputs := []string{}, []CompositionEvidenceOutput{}
	for index, path := range paths {
		id := profile + "-" + strings.TrimSuffix(strings.ReplaceAll(filepath.Base(path), ".", "-"), "-json")
		if index == 0 {
			id = profile + "-binding"
		}
		digest, err := DigestFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, nil, err
		}
		outputs = append(outputs, CompositionEvidenceOutput{ID: id, Kind: "runtime-evidence", Path: path, Digest: digest, DependsOn: dependencies, Status: "current", RunID: runID})
		ids = append(ids, id)
	}
	return ids, outputs, nil
}

func compositionRuntimeRun(id, command string, started, completed time.Time, platform RuntimeBindingPlatform, binaryDigest string, inputs []string, digests map[string]string, outputs []string) CompositionEvidenceRun {
	identity := map[string]string{"kind": platform.Kind, "os": platform.OS, "architecture": platform.Architecture, "go_version": platform.GoVersion, "runtime_binary_digest": binaryDigest}
	if platform.ContainerRuntime != "" {
		identity["container_runtime"] = platform.ContainerRuntime
	}
	return CompositionEvidenceRun{ID: id, ExecutionKind: "runtime", Command: command, StartedAt: started.UTC().Format(time.RFC3339), CompletedAt: completed.UTC().Format(time.RFC3339), Result: "passed", Attempts: 1, RuntimeIdentity: identity, InputBindings: compositionBindings(inputs, digests), OutputIDs: outputs}
}

func compositionBindings(ids []string, digests map[string]string) []CompositionEvidenceBinding {
	result := []CompositionEvidenceBinding{}
	for _, id := range ids {
		result = append(result, CompositionEvidenceBinding{InputID: id, Digest: digests[id]})
	}
	return result
}

func aggregateCompositionMembers(root string, members []string) (string, error) {
	items := []map[string]string{}
	seen := map[string]bool{}
	for _, path := range members {
		if path == "" || filepath.IsAbs(path) || strings.HasPrefix(filepath.Clean(path), "..") || seen[path] {
			return "", fmt.Errorf("Evidence Dependency input memberが不正です: %s", path)
		}
		seen[path] = true
		digest, err := DigestFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return "", err
		}
		items = append(items, map[string]string{"path": path, "digest": digest})
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["path"] < items[j]["path"] })
	data, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return DigestBytes(data), nil
}
