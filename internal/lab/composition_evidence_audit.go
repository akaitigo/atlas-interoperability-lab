package lab

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func AuditCompositionEvidenceDependency(root, relative string) (CompositionEvidenceAudit, error) {
	var graph CompositionEvidenceDependencyGraph
	if err := LoadJSON(filepath.Join(root, filepath.FromSlash(relative)), &graph); err != nil {
		return CompositionEvidenceAudit{}, err
	}
	audit := CompositionEvidenceAudit{SchemaVersion: 1, CoreCommit: graph.CoreCommit, Inputs: len(graph.Inputs), Outputs: len(graph.Outputs), Runs: len(graph.Runs), CompletionState: graph.CompletionState, Gaps: graph.Gaps, DefinitiveEligible: graph.DefinitiveEligible, Verdict: "pass"}
	fail := func(format string, args ...any) (CompositionEvidenceAudit, error) {
		audit.Verdict = "reject"
		return audit, fmt.Errorf(format, args...)
	}
	if graph.SchemaVersion != 1 || graph.AtlasID != "atlas-interoperability-lab" || graph.CoreCommit != evidenceDependencyCoreCommit || graph.Status != "current" {
		return fail("Composition Evidence Dependency GraphはcurrentなCore正式main契約ではありません")
	}
	generatedAt, generatedErr := time.Parse(time.RFC3339, graph.GeneratedAt)
	if generatedErr != nil || len(graph.Inputs) != 6 || len(graph.Outputs) != 21 || len(graph.Runs) != 3 {
		return fail("Composition Evidence Dependency Graphの時刻またはclosure分母が不正です")
	}
	if !graph.Policy.TransitiveStaleness || !graph.Policy.DigestOnlyClosureForbidden || !graph.Policy.ActualRerunRequired || !graph.Policy.MissingRerunTargetsFail || !graph.Policy.ProofStructureInvariant || !graph.Policy.ClosurePlanStructureInvariant {
		return fail("Core Evidence Dependency portable predicateが縮小されています")
	}
	requiredGaps := []string{"subject-depth-parity-incomplete", "subject-v2-certificate-atomic-binding-unavailable", "surface-pattern-proof-gaps"}
	if graph.CompletionState != "incomplete" || graph.DefinitiveEligible || !sameSet(graph.Gaps, requiredGaps) {
		return fail("Compositionの未完GapがEvidence Dependency Graphで隠されています")
	}
	inputs := map[string]CompositionEvidenceInput{}
	changed := map[string]time.Time{}
	expectedMembers, expectedMembersErr := compositionEvidenceInputMembers(root)
	if expectedMembersErr != nil {
		return fail("Lab固有input memberを列挙できません: %v", expectedMembersErr)
	}
	for _, input := range graph.Inputs {
		if input.ID == "" || inputs[input.ID].ID != "" || !contains([]string{"contract", "source", "harness", "runtime", "profile"}, input.Kind) {
			return fail("Evidence Dependency inputが空、重複、または未知kindです: %s", input.ID)
		}
		actual, err := aggregateCompositionMembers(root, input.Members)
		if err != nil || actual != input.CurrentDigest {
			return fail("Evidence Dependency input digestが実体と一致しません: %s", input.ID)
		}
		observed, err := time.Parse(time.RFC3339, input.ObservedAt)
		if err != nil {
			return fail("Evidence Dependency input observed_atが不正です: %s", input.ID)
		}
		if input.BaselineDigest != input.CurrentDigest {
			changed[input.ID] = observed
		}
		if !sameSet(input.Members, expectedMembers[input.ID]) {
			return fail("Evidence Dependency input member closureが縮小されています: %s", input.ID)
		}
		inputs[input.ID] = input
	}
	if !sameSet(mapKeysInput(inputs), []string{"repository-contract", "composition-source", "interop-harness", "go-runtime", "local-profile", "container-profile"}) {
		return fail("Lab固有input closureが不足しています")
	}
	outputs, outputPaths := map[string]CompositionEvidenceOutput{}, map[string]string{}
	for _, output := range graph.Outputs {
		if output.ID == "" || outputs[output.ID].ID != "" || output.Path == "" || outputPaths[output.Path] != "" || output.Status != "current" {
			return fail("Evidence Dependency outputが空、重複、またはstaleです: %s", output.ID)
		}
		digest, err := DigestFile(filepath.Join(root, filepath.FromSlash(output.Path)))
		if err != nil || digest != output.Digest {
			return fail("Evidence Dependency output digestが実体と一致しません: %s", output.ID)
		}
		outputs[output.ID], outputPaths[output.Path] = output, output.ID
	}
	expectedPaths := expectedCompositionEvidenceOutputs()
	if !sameSet(graph.RequiredOutputs, expectedPaths) {
		return fail("required output closureが縮小または差替えられています")
	}
	for _, path := range expectedPaths {
		if outputPaths[path] == "" {
			return fail("既知EvidenceがGraph外へ退避されています: %s", path)
		}
	}
	for id, output := range outputs {
		for _, dependency := range output.DependsOn {
			if inputs[dependency].ID == "" && outputs[dependency].ID == "" {
				return fail("Evidence Dependency edgeが未知nodeを参照しています: %s -> %s", id, dependency)
			}
		}
	}
	if err := rejectCompositionDependencyCycles(outputs); err != nil {
		return fail("%v", err)
	}
	runs := map[string]CompositionEvidenceRun{}
	for _, run := range graph.Runs {
		started, startErr := time.Parse(time.RFC3339, run.StartedAt)
		completed, completedErr := time.Parse(time.RFC3339, run.CompletedAt)
		if run.ID == "" || runs[run.ID].ID != "" || startErr != nil || completedErr != nil || completed.Before(started) || run.Result != "passed" || run.Attempts != 1 {
			return fail("Evidence rerun契約が不正です: %s", run.ID)
		}
		if run.ExecutionKind != "derived" && len(run.RuntimeIdentity) == 0 {
			return fail("実Runtime rerunにruntime identityがありません: %s", run.ID)
		}
		runs[run.ID] = run
	}
	if !sameSet(mapKeysRun(runs), []string{"run-local-runtime-binding", "run-container-runtime-binding", "run-composition-closure"}) {
		return fail("Evidence rerun集合が縮小または差替えられています")
	}
	var localBinding, containerBinding RuntimeBindingEvidence
	if err := LoadJSON(filepath.Join(root, "evidence", "preview", "runtime-binding", "local.binding.json"), &localBinding); err != nil {
		return fail("local Runtime Bindingを再読込できません: %v", err)
	}
	if err := LoadJSON(filepath.Join(root, "evidence", "preview", "runtime-binding", "container.binding.json"), &containerBinding); err != nil {
		return fail("container Runtime Bindingを再読込できません: %v", err)
	}
	if err := validateCompositionRuntimeRun(runs["run-local-runtime-binding"], localBinding); err != nil {
		return fail("local Runtime runがBindingと一致しません: %v", err)
	}
	if err := validateCompositionRuntimeRun(runs["run-container-runtime-binding"], containerBinding); err != nil {
		return fail("container Runtime runがBindingと一致しません: %v", err)
	}
	derivedRun := runs["run-composition-closure"]
	derivedStarted, _ := time.Parse(time.RFC3339, derivedRun.StartedAt)
	derivedCompleted, _ := time.Parse(time.RFC3339, derivedRun.CompletedAt)
	localCompleted, _ := time.Parse(time.RFC3339, localBinding.ExecutionCompletedAt)
	containerCompleted, _ := time.Parse(time.RFC3339, containerBinding.ExecutionCompletedAt)
	if derivedRun.ExecutionKind != "derived" || derivedStarted.Before(localCompleted) || derivedStarted.Before(containerCompleted) || !generatedAt.Equal(derivedCompleted) {
		return fail("Composition derived runの実行順序またはgenerated_atが不正です")
	}
	affected := 0
	for id, output := range outputs {
		ancestors, err := compositionInputAncestors(id, outputs, inputs, map[string]bool{})
		if err != nil || len(ancestors) == 0 {
			return fail("Evidence outputがinput closureへ到達しません: %s", id)
		}
		run := runs[output.RunID]
		if run.ID == "" || !contains(run.OutputIDs, id) {
			return fail("Evidence rerun対象からoutputが漏れています: run=%s output=%s", output.RunID, id)
		}
		bindings := map[string]string{}
		for _, binding := range run.InputBindings {
			if inputs[binding.InputID].ID == "" || bindings[binding.InputID] != "" {
				return fail("Evidence rerun input bindingが未知または重複しています: %s", binding.InputID)
			}
			bindings[binding.InputID] = binding.Digest
		}
		started, _ := time.Parse(time.RFC3339, run.StartedAt)
		isAffected := false
		for inputID := range ancestors {
			if bindings[inputID] != inputs[inputID].CurrentDigest {
				return fail("Evidence rerunが現在のinput digestへ結ばれていません: run=%s input=%s", run.ID, inputID)
			}
			if observed, ok := changed[inputID]; ok {
				isAffected = true
				if started.Before(observed) {
					return fail("digest書換えだけではClosureできません: input=%s output=%s", inputID, id)
				}
			}
		}
		if isAffected {
			affected++
		}
	}
	structureKinds := map[string]bool{}
	for _, structure := range graph.Structures {
		if structureKinds[structure.Kind] {
			return fail("Evidence structure baselineが重複しています: %s", structure.Kind)
		}
		digest, err := DigestFile(filepath.Join(root, filepath.FromSlash(structure.Path)))
		if err != nil || digest != structure.BaselineDigest {
			return fail("既存Proof/Closure Planの構造が変化しています: %s", structure.Kind)
		}
		structureKinds[structure.Kind] = true
	}
	if !structureKinds["scenario-proof-index"] || !structureKinds["scenario-closure-plan"] {
		return fail("Proof/Closure Plan構造baselineが不足しています")
	}
	audit.ChangedInputs, audit.AffectedOutputs = len(changed), affected
	return audit, nil
}

func expectedCompositionEvidenceOutputs() []string {
	paths := []string{}
	for _, profile := range []string{"local", "container"} {
		paths = append(paths, "evidence/preview/runtime-binding/"+profile+".binding.json")
		for _, name := range []string{"summary.json", "normal.json", "rejection.json", "failure.json", "recovery.json", "compatibility.json", "cleanup.receipt.json"} {
			paths = append(paths, "evidence/preview/runtime-binding/"+profile+"/"+name)
		}
	}
	paths = append(paths, "evidence/preview/composition-compatibility.matrix.json", "evidence/preview/subject-binding-admission.json", "evidence/preview/subject-binding-admission.matrix.json", "evidence/preview/runtime-binding/proof-index.json", "evidence/preview/runtime-binding/closure-plan.json")
	sort.Strings(paths)
	return paths
}

func mapKeysInput(values map[string]CompositionEvidenceInput) []string {
	result := []string{}
	for key := range values {
		result = append(result, key)
	}
	return result
}

func mapKeysRun(values map[string]CompositionEvidenceRun) []string {
	result := []string{}
	for key := range values {
		result = append(result, key)
	}
	return result
}

func validateCompositionRuntimeRun(run CompositionEvidenceRun, binding RuntimeBindingEvidence) error {
	if run.ExecutionKind != "runtime" || run.StartedAt != binding.ExecutionStartedAt || run.CompletedAt != binding.ExecutionCompletedAt {
		return fmt.Errorf("execution window不一致")
	}
	expected := map[string]string{"kind": binding.Platform.Kind, "os": binding.Platform.OS, "architecture": binding.Platform.Architecture, "go_version": binding.Platform.GoVersion, "runtime_binary_digest": binding.Executable.RuntimeBinaryDigest}
	if binding.Platform.ContainerRuntime != "" {
		expected["container_runtime"] = binding.Platform.ContainerRuntime
	}
	if len(run.RuntimeIdentity) != len(expected) {
		return fmt.Errorf("runtime identity field数不一致")
	}
	for key, value := range expected {
		if run.RuntimeIdentity[key] != value {
			return fmt.Errorf("runtime identity不一致: %s", key)
		}
	}
	return nil
}

func rejectCompositionDependencyCycles(outputs map[string]CompositionEvidenceOutput) error {
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("Evidence Dependency Graphにcycleがあります: %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, dependency := range outputs[id].DependsOn {
			if outputs[dependency].ID != "" {
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		state[id] = 2
		return nil
	}
	for id := range outputs {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func compositionInputAncestors(id string, outputs map[string]CompositionEvidenceOutput, inputs map[string]CompositionEvidenceInput, visiting map[string]bool) (map[string]bool, error) {
	if visiting[id] {
		return nil, fmt.Errorf("cycle: %s", id)
	}
	visiting[id] = true
	result := map[string]bool{}
	for _, dependency := range outputs[id].DependsOn {
		if inputs[dependency].ID != "" {
			result[dependency] = true
			continue
		}
		child, err := compositionInputAncestors(dependency, outputs, inputs, visiting)
		if err != nil {
			return nil, err
		}
		for input := range child {
			result[input] = true
		}
	}
	delete(visiting, id)
	return result, nil
}

func validateCompositionEvidenceGraphStructure(graph CompositionEvidenceDependencyGraph) error {
	if strings.TrimSpace(graph.GeneratedAt) == "" {
		return fmt.Errorf("generated_atがありません")
	}
	return nil
}
