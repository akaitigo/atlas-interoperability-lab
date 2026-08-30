package lab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	fePublicReportPath      = "attestations/fe-upstream.local-report.json"
	fePublicAttestationPath = "attestations/fe-upstream.attestation.json"
	fePublicSignaturePath   = "attestations/fe-upstream.attestation.json.sig"
	feAllowedSignersPath    = "attestations/fe-upstream.allowed-signers"
	feSignaturePrincipal    = "84075783+akaitigo@users.noreply.github.com"
	feSignatureNamespace    = "atlas-interoperability-lab-fe-upstream"
)

type FEPublicReport struct {
	SchemaVersion int                   `json:"schema_version"`
	ID            string                `json:"id"`
	Repository    string                `json:"repository"`
	Commit        string                `json:"commit"`
	Verification  string                `json:"verification"`
	SourceObjects []PreviewArtifactLock `json:"source_objects"`
	TrackedInputs []PreviewArtifactLock `json:"tracked_inputs"`
	Verdict       string                `json:"verdict"`
}

type FEPublicBoundary struct {
	UpstreamLiveVerification string `json:"upstream_live_verification"`
	LocalGateRequired        bool   `json:"local_gate_required"`
	CompletionEffect         string `json:"completion_effect"`
	DistributionGapEffect    string `json:"distribution_gap_effect"`
	AuthorizationEffect      string `json:"authorization_effect"`
}

type FEPublicAttestation struct {
	SchemaVersion      int                 `json:"schema_version"`
	ID                 string              `json:"id"`
	SourceCommit       string              `json:"source_commit"`
	Report             PreviewArtifactLock `json:"report"`
	RepositoryContract PreviewArtifactLock `json:"repository_contract"`
	Boundary           FEPublicBoundary    `json:"boundary"`
}

type PublicCIGateReport struct {
	SchemaVersion int              `json:"schema_version"`
	Gate          string           `json:"gate"`
	Checks        []string         `json:"checks"`
	NegativeCases int              `json:"negative_cases"`
	Boundary      FEPublicBoundary `json:"boundary"`
	Verdict       string           `json:"verdict"`
}

var fePublicTrackedInputs = []string{
	".github/workflows/ci.yml",
	"internal/lab/depth.go",
	"internal/lab/scenario_contract.go",
	"internal/lab/public_ci.go",
	"internal/lab/nonregression.go",
	"internal/lab/nonregression_test.go",
	"cmd/atlas-lab/main.go",
	"attestations/fe-upstream.allowed-signers",
	"scripts/check_dco_range.py",
	"depth/fe-depth-reference.lock.json",
	"depth/fe-scenario-contract.lock.json",
	"depth/fixture-subjects.depth-parity.preview.json",
	"depth/fixture-stage2.integration-proofs.preview.json",
	"evidence/preview/depth-parity.result.json",
	"compositions/fixture-stage2-v2-bounded.preview.json",
	"compositions/fixture-stage2-v2-definitive.preview.json",
	"compatibility/subject-binding-candidates.lock.json",
	"schemas/subject-binding-admission.schema.json",
	"tests/fixtures/subject-binding-admission.matrix.json",
	"internal/lab/subject_binding_admission.go",
	"internal/lab/subject_binding_admission_test.go",
}

var fePublicRequiredGateCommands = []string{"go test ./...", "go run ./cmd/atlas-lab depth-parity", "go run ./cmd/atlas-lab definitive-matrix", "go run ./cmd/atlas-lab definitive-migrate", "go run ./cmd/atlas-lab composition-compatibility-matrix", "go run ./cmd/atlas-lab subject-binding-admission", "go run ./cmd/atlas-lab definitive-preview-audit", "go run ./cmd/atlas-lab preview-publication-gate"}

var fePublicIsolatedGateBindings = []struct {
	Step string
	Root string
}{
	{Step: "Local E2E", Root: "/tmp/atlas-lab-e2e-local"},
	{Step: "Container E2E", Root: "/tmp/atlas-lab-e2e-container"},
	{Step: "Runtime binding evidence", Root: "/tmp/atlas-lab-runtime-binding"},
	{Step: "Failure diagnostics", Root: "/tmp/atlas-lab-diagnostics"},
}

func GenerateFEPublicReport(root string) error {
	var composition PreviewComposition
	if err := LoadJSON(resolve(root, "compositions/fixture-stage2-v2-definitive.preview.json"), &composition); err != nil {
		return err
	}
	if _, _, err := loadFEDepthReference(root, root, composition.DepthReferenceLock); err != nil {
		return fmt.Errorf("local実FE Depth Git object検証が失敗しました: %w", err)
	}
	if _, err := validateFEScenarioContract(root, root, composition.ScenarioContractLock); err != nil {
		return fmt.Errorf("local実FE Scenario Git object検証が失敗しました: %w", err)
	}
	tracked := make([]PreviewArtifactLock, 0, len(fePublicTrackedInputs))
	for _, path := range fePublicTrackedInputs {
		digest, err := DigestFile(resolve(root, path))
		if err != nil {
			return err
		}
		tracked = append(tracked, PreviewArtifactLock{Path: path, Digest: digest})
	}
	report := FEPublicReport{SchemaVersion: 1, ID: "fe-upstream-local-verification-v1", Repository: "frontend-behavior-atlas", Commit: feDepthReferenceCommit, Verification: "live-git-object-byte-digest", SourceObjects: []PreviewArtifactLock{{Path: "FE_DEPTH_REFERENCE.json", Digest: feDepthReferenceDigest}, {Path: "evidence/scenarios/index.json", Digest: feScenarioIndexDigest}, {Path: "artifacts/reference-system/results.json", Digest: feIntegrationResultsDigest}}, TrackedInputs: tracked, Verdict: "pass"}
	if err := WriteJSON(resolve(root, fePublicReportPath), report); err != nil {
		return err
	}
	reportDigest, err := DigestFile(resolve(root, fePublicReportPath))
	if err != nil {
		return err
	}
	repoDigest, err := DigestFile(resolve(root, "repo.yaml"))
	if err != nil {
		return err
	}
	attestation := FEPublicAttestation{SchemaVersion: 1, ID: "fe-upstream-public-ci-boundary-v1", SourceCommit: feDepthReferenceCommit, Report: PreviewArtifactLock{Path: fePublicReportPath, Digest: reportDigest}, RepositoryContract: PreviewArtifactLock{Path: "repo.yaml", Digest: repoDigest}, Boundary: expectedFEPublicBoundary()}
	return WriteJSON(resolve(root, fePublicAttestationPath), attestation)
}

func ValidatePublicCIGate(root string) (PublicCIGateReport, error) {
	reader := func(path string) ([]byte, error) { return os.ReadFile(resolve(root, path)) }
	boundary, err := validateFEPublicAttestation(reader)
	report := PublicCIGateReport{SchemaVersion: 1, Gate: "public-ci-upstream-separation", Boundary: boundary, Verdict: "pass"}
	if err != nil {
		report.Verdict = "fail"
		return report, err
	}
	report.Checks = []string{"ssh-signature", "source-commit-path-digests", "tracked-lock-preview-report-repository-digests", "local-required-no-completion-effect", "private-clone-absent"}
	if err := validatePublicCINegatives(reader); err != nil {
		report.Verdict = "fail"
		return report, err
	}
	subjectBinding, err := EvaluateSubjectBindingAdmission(root, root)
	if err != nil || subjectBinding.Verdict != "pass" || subjectBinding.CompletionState != "incomplete" || subjectBinding.DefinitiveEligible {
		if err == nil {
			err = fmt.Errorf("Actual Subject binding public境界が正直なincompleteではありません")
		}
		report.Verdict = "fail"
		return report, err
	}
	report.Checks = append(report.Checks, "actual-subject-rejection-metadata-no-completion-effect")
	report.NegativeCases = 19 + len(fePublicRequiredGateCommands)
	return report, nil
}

func validateFEPublicAttestation(reader repositoryReader) (FEPublicBoundary, error) {
	data, err := reader(fePublicAttestationPath)
	if err != nil {
		return FEPublicBoundary{}, err
	}
	if err := verifySSHSignature(reader, data); err != nil {
		return FEPublicBoundary{}, fmt.Errorf("public attestation署名が不正です: %w", err)
	}
	var attestation FEPublicAttestation
	if err := json.Unmarshal(data, &attestation); err != nil {
		return FEPublicBoundary{}, err
	}
	expectedBoundary := expectedFEPublicBoundary()
	if attestation.SchemaVersion != 1 || attestation.ID != "fe-upstream-public-ci-boundary-v1" || attestation.SourceCommit != feDepthReferenceCommit || attestation.Report.Path != fePublicReportPath || attestation.RepositoryContract.Path != "repo.yaml" || attestation.Boundary != expectedBoundary {
		return attestation.Boundary, fmt.Errorf("public attestation境界が不正です")
	}
	reportData, err := reader(attestation.Report.Path)
	if err != nil || DigestBytes(reportData) != attestation.Report.Digest {
		return attestation.Boundary, fmt.Errorf("local verification report digestが不正です")
	}
	repoData, err := reader("repo.yaml")
	if err != nil || DigestBytes(repoData) != attestation.RepositoryContract.Digest {
		return attestation.Boundary, fmt.Errorf("repo.yaml digestが不正です")
	}
	var report FEPublicReport
	if err := json.Unmarshal(reportData, &report); err != nil {
		return attestation.Boundary, err
	}
	expectedObjects := []PreviewArtifactLock{{Path: "FE_DEPTH_REFERENCE.json", Digest: feDepthReferenceDigest}, {Path: "evidence/scenarios/index.json", Digest: feScenarioIndexDigest}, {Path: "artifacts/reference-system/results.json", Digest: feIntegrationResultsDigest}}
	if report.SchemaVersion != 1 || report.ID != "fe-upstream-local-verification-v1" || report.Repository != "frontend-behavior-atlas" || report.Commit != feDepthReferenceCommit || report.Verification != "live-git-object-byte-digest" || report.Verdict != "pass" || !sameArtifactLocks(report.SourceObjects, expectedObjects) || len(report.TrackedInputs) != len(fePublicTrackedInputs) {
		return attestation.Boundary, fmt.Errorf("local verification report契約が不正です")
	}
	for _, lock := range report.TrackedInputs {
		current, err := reader(lock.Path)
		if err != nil || DigestBytes(current) != lock.Digest || !contains(fePublicTrackedInputs, lock.Path) {
			return attestation.Boundary, fmt.Errorf("tracked lock/preview digestが不正です: %s", lock.Path)
		}
	}
	workflow, err := reader(".github/workflows/ci.yml")
	if err != nil {
		return attestation.Boundary, err
	}
	if err := validatePublicCIWorkflow(string(workflow)); err != nil {
		return attestation.Boundary, err
	}
	return attestation.Boundary, nil
}

func validatePublicCIWorkflow(text string) error {
	if strings.Contains(text, "git clone --no-checkout https://github.com/akaitigo/frontend-behavior-atlas.git") || !strings.Contains(text, "go run ./cmd/atlas-lab public-ci-gate") {
		return fmt.Errorf("public CIへprivate FE cloneが再導入されたかGate入口がありません")
	}
	requiredWorkflowFragments := append([]string{"actions/checkout@11d5960a326750d5838078e36cf38b85af677262", "actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff", "fetch-depth: 0", "ATLAS_LAB_PUBLIC_CI_ATTESTATION: \"1\"", "ATLAS_LAB_REFERENCE_ROOT: ${{ github.workspace }}", "checkout --detach 072d7ca77981f51754e824d70c6d4ecd55ea67e5", "rev-parse HEAD", "scripts/check_dco_range.py --self-test", "scripts/check_dco_range.py --base", "BASE_SHA:", "HEAD_SHA:"}, fePublicRequiredGateCommands...)
	for _, fragment := range requiredWorkflowFragments {
		if !strings.Contains(text, fragment) {
			return fmt.Errorf("public CI供給網またはDCO range Gateが不足しています: %s", fragment)
		}
	}
	seenRoots := map[string]bool{}
	for _, binding := range fePublicIsolatedGateBindings {
		if seenRoots[binding.Root] {
			return fmt.Errorf("public CI Gateの隔離rootが重複しています: %s", binding.Root)
		}
		seenRoots[binding.Root] = true
		archiveFragment := "git archive HEAD | tar -x -C " + binding.Root
		bindingFragment := "- name: " + binding.Step + "\n        working-directory: " + binding.Root
		if !strings.Contains(text, archiveFragment) || !strings.Contains(text, bindingFragment) {
			return fmt.Errorf("public CI Gateの独立HEAD copy bindingが不足しています: %s", binding.Step)
		}
	}
	if strings.Contains(text, "uses: actions/checkout@v") || strings.Contains(text, "uses: actions/setup-go@v") || strings.Contains(text, "checkout -B main") || strings.Contains(text, "git log --format='%H%x00%B%x00' |") {
		return fmt.Errorf("mutable Action、Core branch上書き、または全履歴DCO検査は禁止です")
	}
	return nil
}

func validatePublicCINegatives(reader repositoryReader) error {
	paths := []string{fePublicSignaturePath, fePublicReportPath, "depth/fe-depth-reference.lock.json", "depth/fixture-subjects.depth-parity.preview.json", fePublicAttestationPath, ".github/workflows/ci.yml", "repo.yaml", "internal/lab/public_ci.go", "internal/lab/nonregression.go", "internal/lab/nonregression_test.go", "cmd/atlas-lab/main.go", feAllowedSignersPath, "scripts/check_dco_range.py", "compatibility/subject-binding-candidates.lock.json", "schemas/subject-binding-admission.schema.json", "tests/fixtures/subject-binding-admission.matrix.json", "internal/lab/subject_binding_admission.go", "internal/lab/subject_binding_admission_test.go"}
	for _, path := range paths {
		overlay := func(candidate string) ([]byte, error) {
			data, err := reader(candidate)
			if err != nil {
				return nil, err
			}
			clone := append([]byte{}, data...)
			if candidate == path {
				switch path {
				case fePublicSignaturePath:
					clone = []byte("tampered-signature\n")
				case ".github/workflows/ci.yml":
					clone = append(clone, []byte("\n# git clone --no-checkout https://github.com/akaitigo/frontend-behavior-atlas.git ../frontend-behavior-atlas\n")...)
				default:
					clone = append(clone, '\n')
				}
			}
			return clone, nil
		}
		if _, err := validateFEPublicAttestation(overlay); err == nil {
			return fmt.Errorf("public CI negative fixtureが受理されました: %s", path)
		}
	}
	for _, command := range fePublicRequiredGateCommands {
		overlay := func(candidate string) ([]byte, error) {
			data, err := reader(candidate)
			if err != nil {
				return nil, err
			}
			if candidate == ".github/workflows/ci.yml" {
				return []byte(strings.Replace(string(data), "          "+command+"\n", "", 1)), nil
			}
			return data, nil
		}
		if _, err := validateFEPublicAttestation(overlay); err == nil {
			return fmt.Errorf("public CI Gate削除fixtureが受理されました: %s", command)
		}
	}
	workflow, err := reader(".github/workflows/ci.yml")
	if err != nil {
		return err
	}
	merged := strings.Replace(string(workflow), "working-directory: /tmp/atlas-lab-e2e-container", "working-directory: /tmp/atlas-lab-e2e-local", 1)
	if err := validatePublicCIWorkflow(merged); err == nil {
		return fmt.Errorf("public CI Gate共有copy再統合fixtureが受理されました")
	}
	return nil
}

func verifySSHSignature(reader repositoryReader, payload []byte) error {
	temporaryRoot, err := os.MkdirTemp("", "atlas-lab-public-ci-signature-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporaryRoot)
	allowed, err := reader(feAllowedSignersPath)
	if err != nil {
		return err
	}
	signature, err := reader(fePublicSignaturePath)
	if err != nil {
		return err
	}
	allowedPath := filepath.Join(temporaryRoot, "allowed_signers")
	signaturePath := filepath.Join(temporaryRoot, "attestation.sig")
	if err := os.WriteFile(allowedPath, allowed, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(signaturePath, signature, 0o600); err != nil {
		return err
	}
	command := exec.Command("ssh-keygen", "-Y", "verify", "-f", allowedPath, "-I", feSignaturePrincipal, "-n", feSignatureNamespace, "-s", signaturePath)
	command.Stdin = bytes.NewReader(payload)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("ssh-keygen verify: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func expectedFEPublicBoundary() FEPublicBoundary {
	return FEPublicBoundary{UpstreamLiveVerification: "unavailable", LocalGateRequired: true, CompletionEffect: "none", DistributionGapEffect: "none", AuthorizationEffect: "none"}
}

func sameArtifactLocks(left, right []PreviewArtifactLock) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
