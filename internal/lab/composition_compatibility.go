package lab

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type CompositionCompatibilityMatrix struct {
	SchemaVersion      int                                  `json:"schema_version"`
	CoreLock           string                               `json:"core_lock"`
	Composition        string                               `json:"composition"`
	ClaimEvidenceGraph string                               `json:"claim_evidence_graph"`
	Consumers          []string                             `json:"consumers"`
	Subjects           []string                             `json:"subjects"`
	Cases              []CompositionCompatibilityMatrixCase `json:"cases"`
}

type CompositionCompatibilityMatrixCase struct {
	ID                     string            `json:"id"`
	Mutations              map[string]string `json:"mutations"`
	GraphMutation          string            `json:"graph_mutation"`
	ExpectedComposition    string            `json:"expected_composition"`
	ExpectedClaimGraph     string            `json:"expected_claim_graph"`
	ExpectedFailedSubjects []string          `json:"expected_failed_subjects"`
}

type CompositionCompatibilityResult struct {
	SchemaVersion int                                      `json:"schema_version"`
	CoreCommit    string                                   `json:"core_commit"`
	CompositionID string                                   `json:"composition_id"`
	Results       []CompositionCompatibilityConsumerResult `json:"results"`
	Verdict       string                                   `json:"verdict"`
}

type CompositionCompatibilityConsumerResult struct {
	Consumer            string                                  `json:"consumer"`
	CaseID              string                                  `json:"case_id"`
	Subjects            []CompositionCompatibilitySubjectResult `json:"subjects"`
	ClaimGraphState     string                                  `json:"claim_graph_state"`
	CompatibilityState  string                                  `json:"compatibility_state"`
	FailedSubjects      []string                                `json:"failed_subjects"`
	DefinitiveEligible  bool                                    `json:"definitive_eligible"`
	InheritedGaps       []string                                `json:"inherited_gaps"`
	ConsumerIndependent bool                                    `json:"consumer_independent"`
	Verdict             string                                  `json:"verdict"`
}

type CompositionCompatibilitySubjectResult struct {
	Name                 string `json:"name"`
	CompositionSubjectID string `json:"composition_subject_id"`
	ProbeIdentity        string `json:"probe_identity"`
	BindingState         string `json:"binding_state"`
	Mutation             string `json:"mutation"`
	GateState            string `json:"gate_state"`
	CertificateState     string `json:"certificate_state"`
	Classification       string `json:"classification"`
}

func RunCompositionCompatibilityMatrix(root, matrixPath string) (CompositionCompatibilityResult, error) {
	return runCompositionCompatibilityMatrix(root, matrixPath, true, root)
}

func runCompositionCompatibilityMatrix(root, matrixPath string, requireNonRegression bool, pinnedRepositoryRoot string) (CompositionCompatibilityResult, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return CompositionCompatibilityResult{}, err
	}
	root = absoluteRoot
	if requireNonRegression {
		if _, err := NonRegressionGate(root); err != nil {
			return CompositionCompatibilityResult{}, err
		}
	}
	if err := ValidateSavedRuntimeBindingEvidence(root, "local"); err != nil {
		return CompositionCompatibilityResult{}, fmt.Errorf("local Runtime Binding未閉鎖: %w", err)
	}
	if err := ValidateSavedRuntimeBindingEvidence(root, "container"); err != nil {
		return CompositionCompatibilityResult{}, fmt.Errorf("container Runtime Binding未閉鎖: %w", err)
	}
	var matrix CompositionCompatibilityMatrix
	if err := LoadJSON(resolve(root, matrixPath), &matrix); err != nil {
		return CompositionCompatibilityResult{}, err
	}
	var lock EvidenceDependencyCoreLock
	if err := LoadJSON(resolve(root, matrix.CoreLock), &lock); err != nil {
		return CompositionCompatibilityResult{}, err
	}
	var composition PreviewComposition
	if err := LoadJSON(resolve(root, matrix.Composition), &composition); err != nil {
		return CompositionCompatibilityResult{}, err
	}
	if err := validateCompositionCompatibilityContract(matrix, composition, lock); err != nil {
		return CompositionCompatibilityResult{}, err
	}
	var depth DefinitiveGateResult
	if requireNonRegression {
		depth, err = EvaluateDefinitiveComposition(root, matrix.Composition, "2026-08-28T12:00:00Z")
	} else {
		depth, err = evaluateDefinitiveCompositionWithPinnedRoot(root, pinnedRepositoryRoot, matrix.Composition, "2026-08-28T12:00:00Z", nil)
	}
	if err != nil {
		return CompositionCompatibilityResult{}, err
	}
	inheritedGaps := warningCodes(depth.Warnings)
	inheritedGaps = append(inheritedGaps, "subject-probe-atomic-binding-gap")
	sort.Strings(inheritedGaps)
	requiredGaps := []string{"core-v2-draft", "integrated-trace-not-component-proof", "subject-depth-parity-incomplete", "subject-probe-atomic-binding-gap", "surface-pattern-proof-gaps"}
	if depth.DefinitiveEligible || depth.EffectiveState != "incomplete" || !containsAll(inheritedGaps, requiredGaps) {
		return CompositionCompatibilityResult{}, fmt.Errorf("Compositionの未完Gap継承が保持されていません")
	}

	coreRoot, cleanupCore, err := extractPinnedCore(pinnedRepositoryRoot, lock.Repository, lock.Commit)
	if err != nil {
		return CompositionCompatibilityResult{}, err
	}
	defer cleanupCore()
	binary := filepath.Join(coreRoot, "atlas-interop-core")
	build := exec.Command("go", "build", "-o", binary, "./cmd/atlas")
	build.Dir = coreRoot
	build.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, ".cache", "go-build"))
	if output, err := build.CombinedOutput(); err != nil {
		return CompositionCompatibilityResult{}, fmt.Errorf("固定Core CLIをbuildできません: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fixtureRoot, err := exportPinnedCoreFixture(root, coreRoot)
	if err != nil {
		return CompositionCompatibilityResult{}, err
	}
	defer os.RemoveAll(fixtureRoot)

	byName := map[string]PreviewSubjectRef{}
	for _, subject := range composition.Subjects {
		byName[subject.Name] = subject
	}
	report := CompositionCompatibilityResult{SchemaVersion: 1, CoreCommit: lock.Commit, CompositionID: composition.ID, Results: []CompositionCompatibilityConsumerResult{}, Verdict: "pass"}
	consumerBaselines := map[string]string{}
	for _, testCase := range matrix.Cases {
		for _, consumer := range matrix.Consumers {
			result := CompositionCompatibilityConsumerResult{Consumer: consumer, CaseID: testCase.ID, Subjects: []CompositionCompatibilitySubjectResult{}, FailedSubjects: []string{}, DefinitiveEligible: false, InheritedGaps: inheritedGaps, ConsumerIndependent: true, Verdict: "pass"}
			for _, name := range matrix.Subjects {
				mutation := testCase.Mutations[name]
				caseRoot := filepath.Join(fixtureRoot, "composition-cases", consumer, testCase.ID, name)
				if err := os.CopyFS(caseRoot, os.DirFS(filepath.Join(fixtureRoot, "baseline"))); err != nil {
					return report, err
				}
				if err := applyEvidenceDependencyMutation(caseRoot, mutation); err != nil {
					return report, fmt.Errorf("Fixture %s/%s: %w", testCase.ID, name, err)
				}
				gateOutput, gateErr := runCoreBinary(binary, "audit", caseRoot, "--gate", "evidence-dependency")
				certificateOutput, certificateErr := runCoreBinary(binary, "certificate", "verify-definitive", caseRoot)
				gateState, certificateState := commandState(gateErr), commandState(certificateErr)
				classification := "current-closure"
				if mutation != "none" {
					classification = mutation
				}
				if err := validateSubjectProbeOutput(mutation, gateState, certificateState, gateOutput, certificateOutput); err != nil {
					result.Verdict, report.Verdict = "fail", "fail"
				}
				if gateState == "reject" || certificateState == "reject" {
					result.FailedSubjects = append(result.FailedSubjects, name)
				}
				result.Subjects = append(result.Subjects, CompositionCompatibilitySubjectResult{Name: name, CompositionSubjectID: byName[name].SubjectID, ProbeIdentity: "core-generated-definitive-fixture-" + name, BindingState: "explicit-gap", Mutation: mutation, GateState: gateState, CertificateState: certificateState, Classification: classification})
			}
			graphState, graphErr := runClaimGraphFixture(root, matrix.ClaimEvidenceGraph, testCase.GraphMutation)
			result.ClaimGraphState = graphState
			result.CompatibilityState = "incomplete"
			if len(result.FailedSubjects) > 0 || graphErr != nil {
				result.CompatibilityState = "reject"
			}
			sort.Strings(result.FailedSubjects)
			expectedFailed := append([]string{}, testCase.ExpectedFailedSubjects...)
			sort.Strings(expectedFailed)
			if result.CompatibilityState != testCase.ExpectedComposition || result.ClaimGraphState != testCase.ExpectedClaimGraph || !sameSet(result.FailedSubjects, expectedFailed) || result.DefinitiveEligible || !containsAll(result.InheritedGaps, requiredGaps) {
				result.Verdict, report.Verdict = "fail", "fail"
			}
			signature := compositionCompatibilitySignature(result)
			if previous, ok := consumerBaselines[testCase.ID]; ok && previous != signature {
				result.ConsumerIndependent = false
				result.Verdict, report.Verdict = "fail", "fail"
			}
			consumerBaselines[testCase.ID] = signature
			report.Results = append(report.Results, result)
		}
	}
	if report.Verdict != "pass" {
		return report, fmt.Errorf("Composition互換性Matrixがfailです")
	}
	return report, nil
}

func validateCompositionCompatibilityContract(matrix CompositionCompatibilityMatrix, composition PreviewComposition, lock EvidenceDependencyCoreLock) error {
	if matrix.SchemaVersion != 1 || matrix.CoreLock != "compatibility/evidence-dependency-core.lock.json" || matrix.Composition != "compositions/fixture-stage2-v2-definitive.preview.json" || matrix.ClaimEvidenceGraph != "graphs/fixture-stage2.claim-evidence.json" || !sameSet(matrix.Consumers, []string{"codex", "claude-code", "generic-cli"}) || !sameSet(matrix.Subjects, []string{"source", "sink"}) || len(matrix.Cases) != 9 {
		return fmt.Errorf("Composition互換性Matrix契約が不正です")
	}
	if err := validateEvidenceDependencyCoreLock(lock); err != nil {
		return err
	}
	compositionNames := []string{}
	for _, subject := range composition.Subjects {
		compositionNames = append(compositionNames, subject.Name)
	}
	if composition.CoreContract.BaseCommit != evidenceDependencyCoreCommit || !sameSet(compositionNames, matrix.Subjects) {
		return fmt.Errorf("Composition SubjectまたはCore LockがMatrixと一致しません")
	}
	seen := map[string]bool{}
	for _, testCase := range matrix.Cases {
		if testCase.ID == "" || seen[testCase.ID] || (testCase.ExpectedComposition != "incomplete" && testCase.ExpectedComposition != "reject") || (testCase.ExpectedClaimGraph != "pass" && testCase.ExpectedClaimGraph != "reject") || (testCase.GraphMutation != "none" && testCase.GraphMutation != "remove-subject-link") {
			return fmt.Errorf("Composition互換性caseが不正です: %s", testCase.ID)
		}
		seen[testCase.ID] = true
		for _, name := range matrix.Subjects {
			if !validEvidenceDependencyMutation(testCase.Mutations[name]) {
				return fmt.Errorf("Composition互換性mutationが不正です: %s/%s", testCase.ID, name)
			}
		}
	}
	return nil
}

func validEvidenceDependencyMutation(value string) bool {
	return contains([]string{"none", "stale-status", "digest-only-closure", "missing-rerun-target", "output-withdrawal", "proof-structure-shrink", "closure-structure-shrink"}, value)
}

func validateSubjectProbeOutput(mutation, gateState, certificateState string, gateOutput, certificateOutput []byte) error {
	if mutation == "none" {
		if gateState != "pass" || certificateState != "pass" || !strings.Contains(string(gateOutput), "digest_only_closure=false") || !strings.Contains(string(certificateOutput), "Subject Definitive Certificate") {
			return fmt.Errorf("current closure probeがpassしません")
		}
		return nil
	}
	expectedErrors := map[string]string{"stale-status": "Evidence dependency graphはstaleです", "digest-only-closure": "digest書換えだけではClosureできません", "missing-rerun-target": "rerun対象からoutputが漏れています", "output-withdrawal": "graphから欠落", "proof-structure-shrink": "構造が変化", "closure-structure-shrink": "構造が変化"}
	if gateState != "reject" || certificateState != "reject" || !strings.Contains(string(gateOutput), expectedErrors[mutation]) || len(certificateOutput) == 0 {
		return fmt.Errorf("negative Subject probeが期待どおり拒否されません: %s", mutation)
	}
	return nil
}

func runClaimGraphFixture(root, graphPath, mutation string) (string, error) {
	if mutation == "none" {
		if err := validateCrossSubjectGraphFile(root, resolve(root, graphPath)); err != nil {
			return "reject", err
		}
		return "pass", nil
	}
	temporaryRoot, err := os.MkdirTemp("", "atlas-lab-claim-graph-")
	if err != nil {
		return "reject", err
	}
	defer os.RemoveAll(temporaryRoot)
	var graph map[string]any
	if err := LoadJSON(resolve(root, graphPath), &graph); err != nil {
		return "reject", err
	}
	links := anyValues(graph["links"])
	link, _ := links[0].(map[string]any)
	subjects := anyValues(link["subject_names"])
	if len(subjects) < 2 {
		return "reject", fmt.Errorf("Claim link fixtureのSubjectが不足しています")
	}
	link["subject_names"] = subjects[:len(subjects)-1]
	path := filepath.Join(temporaryRoot, "claim-evidence.json")
	if err := WriteJSON(path, graph); err != nil {
		return "reject", err
	}
	err = validateCrossSubjectGraphFile(root, path)
	if err == nil || !strings.Contains(err.Error(), "全Subjectを横断") {
		return "pass", fmt.Errorf("Claim/Evidence link欠落が拒否されません")
	}
	return "reject", err
}

func warningCodes(warnings []DefinitiveWarning) []string {
	codes := []string{}
	for _, warning := range warnings {
		if !contains(codes, warning.Code) {
			codes = append(codes, warning.Code)
		}
	}
	sort.Strings(codes)
	return codes
}

func containsAll(actual, expected []string) bool {
	for _, item := range expected {
		if !contains(actual, item) {
			return false
		}
	}
	return true
}

func compositionCompatibilitySignature(result CompositionCompatibilityConsumerResult) string {
	parts := []string{result.ClaimGraphState, result.CompatibilityState, strings.Join(result.FailedSubjects, ",")}
	for _, subject := range result.Subjects {
		parts = append(parts, subject.Name+":"+subject.BindingState+":"+subject.Classification+":"+subject.GateState+":"+subject.CertificateState)
	}
	return strings.Join(parts, "|")
}

func PreviewPublicationGate(root string) (SelfAuditReport, error) {
	report := SelfAuditReport{SchemaVersion: 1, Audit: "definitive-v2-preview-publication", Checks: []SelfAuditCheck{}, Verdict: "pass"}
	add := func(name string, err error) {
		report.add(name, err)
	}
	_, v1Err := PublicationGate(root)
	add("v1-publication-baseline", v1Err)
	add("depth-gap-inheritance", ValidateDepthInheritance(root))
	add("runtime-binding-local", ValidateSavedRuntimeBindingEvidence(root, "local"))
	add("runtime-binding-container", ValidateSavedRuntimeBindingEvidence(root, "container"))
	add("composition-evidence-dependency-closure", ValidateCompositionEvidenceClosure(root))
	subjectBinding, subjectBindingErr := EvaluateSubjectBindingAdmission(root, root)
	if subjectBindingErr == nil && (subjectBinding.Verdict != "pass" || subjectBinding.CompletionState != "incomplete" || subjectBinding.DefinitiveEligible || len(subjectBinding.Candidates) != 3) {
		subjectBindingErr = fmt.Errorf("Actual Subject binding admissionが未完成候補を保持していません")
	}
	add("actual-subject-binding-admission", subjectBindingErr)
	matrix, matrixErr := RunCompositionCompatibilityMatrix(root, "tests/fixtures/composition-compatibility.matrix.json")
	add("multi-subject-composition-compatibility", matrixErr)
	if matrixErr == nil {
		honestIncomplete := 0
		var noGapAggregationErr error
		for _, result := range matrix.Results {
			if result.CompatibilityState == "incomplete" {
				honestIncomplete++
			}
			if result.DefinitiveEligible {
				noGapAggregationErr = fmt.Errorf("Gapを集約で隠してDefinitiveへ昇格しました")
				break
			}
		}
		if honestIncomplete != 3 && noGapAggregationErr == nil {
			noGapAggregationErr = fmt.Errorf("consumer別のhonest incomplete fixtureが不足しています")
		}
		add("no-gap-aggregation", noGapAggregationErr)
	}
	var router map[string]any
	routerErr := LoadJSON(filepath.Join(root, "evals", "preview", "interoperability-router.definitive-v2-preview.json"), &router)
	if routerErr == nil && router["verdict"] != "pass" {
		routerErr = fmt.Errorf("Router v2 Evalがpassではありません")
	}
	add("router-v2-eval", routerErr)
	add("neutral-language", ValidateNeutralLanguage(root))
	if report.Verdict != "pass" {
		return report, fmt.Errorf("Preview Publication Gateがfailです")
	}
	return report, nil
}
