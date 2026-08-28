package lab

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const evidenceDependencyCoreCommit = "072d7ca77981f51754e824d70c6d4ecd55ea67e5"

type EvidenceDependencyCoreLock struct {
	SchemaVersion int                        `json:"schema_version"`
	ID            string                     `json:"id"`
	Repository    string                     `json:"repository"`
	Commit        string                     `json:"commit"`
	Version       string                     `json:"version"`
	License       string                     `json:"license"`
	Status        string                     `json:"status"`
	Commands      EvidenceDependencyCommands `json:"commands"`
	Thresholds    PortableThresholdPolicy    `json:"threshold_policy"`
}

type EvidenceDependencyCommands struct {
	Gate        string `json:"gate"`
	Certificate string `json:"certificate"`
}

type PortableThresholdPolicy struct {
	SubjectCounts   string   `json:"subject_counts"`
	SubjectProfiles string   `json:"subject_profiles"`
	Predicates      []string `json:"shared_predicates"`
}

type EvidenceDependencyMatrix struct {
	SchemaVersion int                            `json:"schema_version"`
	CoreLock      string                         `json:"core_lock"`
	Consumers     []string                       `json:"consumers"`
	Cases         []EvidenceDependencyMatrixCase `json:"cases"`
}

type EvidenceDependencyMatrixCase struct {
	ID                  string `json:"id"`
	Mutation            string `json:"mutation"`
	ExpectedGate        string `json:"expected_gate"`
	ExpectedCertificate string `json:"expected_certificate"`
	ExpectedError       string `json:"expected_error,omitempty"`
}

type EvidenceDependencyMatrixResult struct {
	SchemaVersion int                                    `json:"schema_version"`
	CoreCommit    string                                 `json:"core_commit"`
	CoreStatus    string                                 `json:"core_status"`
	Thresholds    PortableThresholdPolicy                `json:"threshold_policy"`
	Results       []EvidenceDependencyConsumerCaseResult `json:"results"`
	Verdict       string                                 `json:"verdict"`
}

type EvidenceDependencyConsumerCaseResult struct {
	Consumer            string `json:"consumer"`
	CaseID              string `json:"case_id"`
	GateState           string `json:"gate_state"`
	CertificateState    string `json:"certificate_state"`
	Classification      string `json:"classification"`
	ConsumerIndependent bool   `json:"consumer_independent"`
	Verdict             string `json:"verdict"`
}

func RunEvidenceDependencyConsumerMatrix(root, matrixPath string) (EvidenceDependencyMatrixResult, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return EvidenceDependencyMatrixResult{}, err
	}
	root = absoluteRoot
	if _, err := NonRegressionGate(root); err != nil {
		return EvidenceDependencyMatrixResult{}, err
	}
	var matrix EvidenceDependencyMatrix
	if err := LoadJSON(resolve(root, matrixPath), &matrix); err != nil {
		return EvidenceDependencyMatrixResult{}, err
	}
	var lock EvidenceDependencyCoreLock
	if err := LoadJSON(resolve(root, matrix.CoreLock), &lock); err != nil {
		return EvidenceDependencyMatrixResult{}, err
	}
	if err := validateEvidenceDependencyMatrixContract(matrix, lock); err != nil {
		return EvidenceDependencyMatrixResult{}, err
	}
	coreRoot, cleanupCore, err := extractPinnedCore(root, lock.Repository, lock.Commit)
	if err != nil {
		return EvidenceDependencyMatrixResult{}, err
	}
	defer cleanupCore()
	binary := filepath.Join(coreRoot, "atlas-interop-core")
	build := exec.Command("go", "build", "-o", binary, "./cmd/atlas")
	build.Dir = coreRoot
	build.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, ".cache", "go-build"))
	if output, err := build.CombinedOutput(); err != nil {
		return EvidenceDependencyMatrixResult{}, fmt.Errorf("固定Core CLIをbuildできません: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fixtureRoot, err := exportPinnedCoreFixture(root, coreRoot)
	if err != nil {
		return EvidenceDependencyMatrixResult{}, err
	}
	defer os.RemoveAll(fixtureRoot)

	report := EvidenceDependencyMatrixResult{SchemaVersion: 1, CoreCommit: lock.Commit, CoreStatus: lock.Status, Thresholds: lock.Thresholds, Results: []EvidenceDependencyConsumerCaseResult{}, Verdict: "pass"}
	classifications := map[string]string{}
	for _, testCase := range matrix.Cases {
		for _, consumer := range matrix.Consumers {
			caseRoot := filepath.Join(fixtureRoot, "cases", consumer, testCase.ID)
			if err := os.CopyFS(caseRoot, os.DirFS(filepath.Join(fixtureRoot, "baseline"))); err != nil {
				return report, err
			}
			if err := applyEvidenceDependencyMutation(caseRoot, testCase.Mutation); err != nil {
				return report, fmt.Errorf("Fixture %s: %w", testCase.ID, err)
			}
			gateOutput, gateErr := runCoreBinary(binary, "audit", caseRoot, "--gate", "evidence-dependency")
			certificateOutput, certificateErr := runCoreBinary(binary, "certificate", "verify-definitive", caseRoot)
			gateState := commandState(gateErr)
			certificateState := commandState(certificateErr)
			classification := "current-closure"
			if testCase.ExpectedError != "" {
				classification = testCase.Mutation
			}
			verdict := "pass"
			if gateState != testCase.ExpectedGate || certificateState != testCase.ExpectedCertificate || (testCase.ExpectedError != "" && !strings.Contains(string(gateOutput), testCase.ExpectedError)) || (testCase.ExpectedGate == "pass" && !strings.Contains(string(gateOutput), "digest_only_closure=false")) || (testCase.ExpectedCertificate == "pass" && !strings.Contains(string(certificateOutput), "Subject Definitive Certificate")) {
				verdict = "fail"
				report.Verdict = "fail"
			}
			if certificateErr == nil && testCase.ExpectedCertificate == "reject" {
				verdict = "fail"
				report.Verdict = "fail"
			}
			if certificateErr != nil && testCase.ExpectedCertificate == "reject" && len(certificateOutput) == 0 {
				verdict = "fail"
				report.Verdict = "fail"
			}
			consumerIndependent := true
			if previous, ok := classifications[testCase.ID]; ok && previous != classification+":"+gateState+":"+certificateState {
				consumerIndependent = false
				verdict = "fail"
				report.Verdict = "fail"
			}
			classifications[testCase.ID] = classification + ":" + gateState + ":" + certificateState
			report.Results = append(report.Results, EvidenceDependencyConsumerCaseResult{Consumer: consumer, CaseID: testCase.ID, GateState: gateState, CertificateState: certificateState, Classification: classification, ConsumerIndependent: consumerIndependent, Verdict: verdict})
		}
	}
	if report.Verdict != "pass" {
		return report, fmt.Errorf("Evidence Dependency consumer互換性Matrixがfailです")
	}
	return report, nil
}

func validateEvidenceDependencyMatrixContract(matrix EvidenceDependencyMatrix, lock EvidenceDependencyCoreLock) error {
	expectedPredicates := []string{"transitive-staleness", "actual-rerun", "complete-output-closure", "proof-structure-invariant", "closure-plan-structure-invariant"}
	if matrix.SchemaVersion != 1 || matrix.CoreLock != "compatibility/evidence-dependency-core.lock.json" || !sameSet(matrix.Consumers, []string{"codex", "claude-code", "generic-cli"}) || len(matrix.Cases) != 7 {
		return fmt.Errorf("Evidence Dependency consumer Matrix契約が不正です")
	}
	if lock.SchemaVersion != 1 || lock.ID != "core-evidence-dependency-v1" || lock.Repository != "reference-atlas-core" || lock.Commit != evidenceDependencyCoreCommit || lock.Version != "1.1.0" || lock.License != "Apache-2.0" || lock.Status != "main-ci-confirmed" || lock.Commands.Gate != "atlas audit <subject-root> --gate evidence-dependency" || lock.Commands.Certificate != "atlas certificate verify-definitive <subject-root>" || lock.Thresholds.SubjectCounts != "subject-defined" || lock.Thresholds.SubjectProfiles != "subject-defined" || !sameSet(lock.Thresholds.Predicates, expectedPredicates) {
		return fmt.Errorf("Evidence Dependency Core Lockが確定値と一致しません")
	}
	seen := map[string]bool{}
	for _, testCase := range matrix.Cases {
		if testCase.ID == "" || seen[testCase.ID] || (testCase.ExpectedGate != "pass" && testCase.ExpectedGate != "reject") || (testCase.ExpectedCertificate != "pass" && testCase.ExpectedCertificate != "reject") {
			return fmt.Errorf("Evidence Dependency Matrix caseが不正です: %s", testCase.ID)
		}
		seen[testCase.ID] = true
	}
	return nil
}

const fixtureExportTest = `package validate

import (
    "os"
    "testing"
)

func TestExportInteropDefinitiveFixture(t *testing.T) {
    destination := os.Getenv("ATLAS_INTEROP_FIXTURE_DIR")
    if destination == "" {
        t.Fatal("ATLAS_INTEROP_FIXTURE_DIR is required")
    }
    source := createDefinitiveRepositoryFixture(t)
    if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
        t.Fatal(err)
    }
}
`

func exportPinnedCoreFixture(root, coreRoot string) (string, error) {
	fixtureRoot, err := os.MkdirTemp("", "atlas-lab-evidence-dependency-")
	if err != nil {
		return "", err
	}
	baseline := filepath.Join(fixtureRoot, "baseline")
	bridge := filepath.Join(coreRoot, "internal", "validate", "interop_fixture_export_test.go")
	if err := os.WriteFile(bridge, []byte(fixtureExportTest), 0o644); err != nil {
		_ = os.RemoveAll(fixtureRoot)
		return "", err
	}
	command := exec.Command("go", "test", "./internal/validate", "-run", "^TestExportInteropDefinitiveFixture$", "-count=1")
	command.Dir = coreRoot
	command.Env = append(os.Environ(), "ATLAS_INTEROP_FIXTURE_DIR="+baseline, "GOCACHE="+filepath.Join(root, ".cache", "go-build"))
	if output, err := command.CombinedOutput(); err != nil {
		_ = os.RemoveAll(fixtureRoot)
		return "", fmt.Errorf("固定Core Fixtureを生成できません: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return fixtureRoot, nil
}

func runCoreBinary(binary string, args ...string) ([]byte, error) {
	return exec.Command(binary, args...).CombinedOutput()
}

func commandState(err error) string {
	if err == nil {
		return "pass"
	}
	return "reject"
}

func applyEvidenceDependencyMutation(root, mutation string) error {
	if mutation == "none" {
		return nil
	}
	graphPath := filepath.Join(root, "evidence", "dependency-graph.json")
	graph, err := readLooseJSON(graphPath)
	if err != nil {
		return err
	}
	switch mutation {
	case "stale-status":
		graph["status"] = "stale"
	case "digest-only-closure":
		if err := mutateDigestOnlyClosure(root, graph); err != nil {
			return err
		}
	case "missing-rerun-target":
		run := firstMap(graph["runs"])
		ids := anyValues(run["output_ids"])
		if len(ids) < 2 {
			return fmt.Errorf("rerun output fixtureが不足しています")
		}
		run["output_ids"] = ids[1:]
	case "output-withdrawal":
		if err := mutateOutputWithdrawal(graph); err != nil {
			return err
		}
	case "proof-structure-shrink":
		if err := shrinkProofStructure(root, graph); err != nil {
			return err
		}
	case "closure-structure-shrink":
		if err := shrinkClosureStructure(root, graph); err != nil {
			return err
		}
	default:
		return fmt.Errorf("未知のEvidence Dependency mutationです: %s", mutation)
	}
	return WriteJSON(graphPath, graph)
}

func mutateDigestOnlyClosure(root string, graph map[string]any) error {
	inputs := anyValues(graph["inputs"])
	var input map[string]any
	for _, raw := range inputs {
		candidate, _ := raw.(map[string]any)
		if candidate["kind"] == "source" {
			input = candidate
			break
		}
	}
	if input == nil {
		return fmt.Errorf("source input fixtureがありません")
	}
	members := anyValues(input["members"])
	member, _ := members[0].(string)
	path := filepath.Join(root, filepath.FromSlash(member))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, []byte("\nchanged input\n")...), 0o644); err != nil {
		return err
	}
	digest, err := aggregateFixtureMembers(root, members)
	if err != nil {
		return err
	}
	input["current_digest"] = digest
	input["observed_at"] = "2026-08-28T02:00:00Z"
	for _, runRaw := range anyValues(graph["runs"]) {
		run, _ := runRaw.(map[string]any)
		for _, bindingRaw := range anyValues(run["input_bindings"]) {
			binding, _ := bindingRaw.(map[string]any)
			if binding["input_id"] == input["id"] {
				binding["digest"] = digest
			}
		}
	}
	return nil
}

func aggregateFixtureMembers(root string, members []any) (string, error) {
	items := []map[string]string{}
	for _, raw := range members {
		path, _ := raw.(string)
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

func mutateOutputWithdrawal(graph map[string]any) error {
	outputs := anyValues(graph["outputs"])
	removeAt := -1
	var removed map[string]any
	for index, raw := range outputs {
		output, _ := raw.(map[string]any)
		if output["kind"] == "scenario-proof" {
			removeAt, removed = index, output
			break
		}
	}
	if removeAt < 0 {
		return fmt.Errorf("scenario-proof output fixtureがありません")
	}
	graph["outputs"] = append(outputs[:removeAt], outputs[removeAt+1:]...)
	path, _ := removed["path"].(string)
	id, _ := removed["id"].(string)
	graph["required_outputs"] = withoutString(anyValues(graph["required_outputs"]), path)
	for _, runRaw := range anyValues(graph["runs"]) {
		run, _ := runRaw.(map[string]any)
		run["output_ids"] = withoutString(anyValues(run["output_ids"]), id)
	}
	return nil
}

func shrinkProofStructure(root string, graph map[string]any) error {
	path := filepath.Join(root, "evidence", "scenarios", "index.json")
	document, err := readLooseJSON(path)
	if err != nil {
		return err
	}
	files := anyValues(document["files"])
	if len(files) < 2 {
		return fmt.Errorf("Scenario Proof row fixtureが不足しています")
	}
	document["files"] = files[:len(files)-1]
	if err := WriteJSON(path, document); err != nil {
		return err
	}
	return refreshGraphOutputDigest(graph, "evidence/scenarios/index.json", path)
}

func shrinkClosureStructure(root string, graph map[string]any) error {
	path := filepath.Join(root, "evidence", "scenarios", "closure-plan.json")
	document, err := readLooseJSON(path)
	if err != nil {
		return err
	}
	rows := anyValues(document["rows"])
	if len(rows) > 0 {
		document["rows"] = rows[:len(rows)-1]
	} else {
		policy, _ := document["policy"].(map[string]any)
		riskOrder := anyValues(policy["risk_order"])
		if len(riskOrder) < 2 {
			return fmt.Errorf("Closure Plan risk order fixtureが不足しています")
		}
		policy["risk_order"] = riskOrder[:len(riskOrder)-1]
	}
	if err := WriteJSON(path, document); err != nil {
		return err
	}
	return refreshGraphOutputDigest(graph, "evidence/scenarios/closure-plan.json", path)
}

func refreshGraphOutputDigest(graph map[string]any, relative, path string) error {
	digest, err := DigestFile(path)
	if err != nil {
		return err
	}
	for _, raw := range anyValues(graph["outputs"]) {
		output, _ := raw.(map[string]any)
		if output["path"] == relative {
			output["digest"] = digest
			return nil
		}
	}
	return fmt.Errorf("Graph outputがありません: %s", relative)
}

func readLooseJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func anyValues(value any) []any {
	items, _ := value.([]any)
	return items
}

func firstMap(value any) map[string]any {
	items := anyValues(value)
	if len(items) == 0 {
		return nil
	}
	item, _ := items[0].(map[string]any)
	return item
}

func withoutString(values []any, remove string) []any {
	kept := []any{}
	for _, value := range values {
		if text, _ := value.(string); text != remove {
			kept = append(kept, value)
		}
	}
	return kept
}
