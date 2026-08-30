package lab

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type RuntimeBindingEvidence struct {
	SchemaVersion      int                      `json:"schema_version"`
	ID                 string                   `json:"id"`
	CoreCommit         string                   `json:"core_commit"`
	CompositionID      string                   `json:"composition_id"`
	CompositionDigest  string                   `json:"composition_digest"`
	Profile            string                   `json:"profile"`
	ObservedAt         string                   `json:"observed_at"`
	Platform           RuntimeBindingPlatform   `json:"platform"`
	Executable         RuntimeExecutableBinding `json:"executable"`
	Subjects           []RuntimeSubjectBinding  `json:"subjects"`
	RuntimeEvidence    RuntimeEvidenceBinding   `json:"runtime_evidence"`
	BindingState       string                   `json:"binding_state"`
	Gaps               []string                 `json:"gaps"`
	DefinitiveEligible bool                     `json:"definitive_eligible"`
	Verdict            string                   `json:"verdict"`
}

type RuntimeBindingPlatform struct {
	Kind             string `json:"kind"`
	OS               string `json:"os"`
	Architecture     string `json:"architecture"`
	GoVersion        string `json:"go_version"`
	ContainerRuntime string `json:"container_runtime,omitempty"`
}

type RuntimeExecutableBinding struct {
	SourceArtifactDigest string `json:"source_artifact_digest"`
	RuntimeBinaryDigest  string `json:"runtime_binary_digest"`
	BuildRecipe          string `json:"build_recipe"`
	BindingMethod        string `json:"binding_method"`
}

type RuntimeSubjectBinding struct {
	Name              string `json:"name"`
	SubjectID         string `json:"subject_id"`
	Version           string `json:"version"`
	ReleaseDigest     string `json:"release_digest"`
	CertificateDigest string `json:"certificate_digest"`
	ArtifactDigest    string `json:"artifact_digest"`
	LaunchDriver      string `json:"launch_driver"`
}

type RuntimeEvidenceBinding struct {
	Summary        PreviewArtifactLock   `json:"summary"`
	Scenarios      []PreviewArtifactLock `json:"scenarios"`
	Cleanup        PreviewArtifactLock   `json:"cleanup"`
	ScenarioCount  int                   `json:"scenario_count"`
	ExecutionState string                `json:"execution_state"`
	CleanupState   string                `json:"cleanup_state"`
}

func GenerateRuntimeBindingEvidence(root, profile string) (RuntimeBindingEvidence, error) {
	if profile != "local" && profile != "container" {
		return RuntimeBindingEvidence{}, fmt.Errorf("Runtime Bindingはlocalまたはcontainer Profileが必須です")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return RuntimeBindingEvidence{}, err
	}
	root = absoluteRoot
	if _, err := NonRegressionGate(root); err != nil {
		return RuntimeBindingEvidence{}, err
	}
	temporaryRoot, err := os.MkdirTemp("", "atlas-lab-runtime-binding-")
	if err != nil {
		return RuntimeBindingEvidence{}, err
	}
	defer os.RemoveAll(temporaryRoot)
	if err := copyRuntimeRepository(root, temporaryRoot); err != nil {
		return RuntimeBindingEvidence{}, err
	}
	validated, err := Preflight(temporaryRoot, "compositions/fixture-stage2.json", profile)
	if err != nil {
		return RuntimeBindingEvidence{}, err
	}
	platform, binaryDigest, recipe, err := buildRuntimeProbe(temporaryRoot, validated, profile)
	if err != nil {
		return RuntimeBindingEvidence{}, err
	}
	summary, runErr := Run(temporaryRoot, "compositions/fixture-stage2.json", profile)
	if runErr != nil {
		return RuntimeBindingEvidence{}, fmt.Errorf("隔離%s Runtime実行失敗: %w", profile, runErr)
	}
	if summary.Verdict != "pass" {
		return RuntimeBindingEvidence{}, fmt.Errorf("隔離%s Runtime Evidenceがpassではありません", profile)
	}

	destination := filepath.Join(root, "evidence", "preview", "runtime-binding", profile)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return RuntimeBindingEvidence{}, err
	}
	summaryLock, err := copyRuntimeArtifact(temporaryRoot, root, filepath.Join("evidence", "runs", profile, "summary.json"), filepath.Join("evidence", "preview", "runtime-binding", profile, "summary.json"))
	if err != nil {
		return RuntimeBindingEvidence{}, err
	}
	scenarioLocks := []PreviewArtifactLock{}
	for _, scenario := range validated.Manifest.Scenarios {
		id := strings.TrimSuffix(filepath.Base(scenario), filepath.Ext(scenario))
		lock, err := copyRuntimeArtifact(temporaryRoot, root, filepath.Join("evidence", "runs", profile, id+".json"), filepath.Join("evidence", "preview", "runtime-binding", profile, id+".json"))
		if err != nil {
			return RuntimeBindingEvidence{}, err
		}
		scenarioLocks = append(scenarioLocks, lock)
	}
	cleanupLock, err := copyRuntimeArtifact(temporaryRoot, root, filepath.Join("cleanup", profile+".receipt.json"), filepath.Join("evidence", "preview", "runtime-binding", profile, "cleanup.receipt.json"))
	if err != nil {
		return RuntimeBindingEvidence{}, err
	}
	subjects := []RuntimeSubjectBinding{}
	artifactDigest := ""
	for _, ref := range validated.Manifest.Subjects {
		release := validated.Releases[ref.Name]
		if artifactDigest == "" {
			artifactDigest = release.Artifact.Digest
		}
		if artifactDigest != release.Artifact.Digest {
			return RuntimeBindingEvidence{}, fmt.Errorf("Fixture SubjectのRuntime Artifactが単一buildへ束縛できません")
		}
		subjects = append(subjects, RuntimeSubjectBinding{Name: ref.Name, SubjectID: ref.SubjectID, Version: ref.Version, ReleaseDigest: ref.ReleaseDigest, CertificateDigest: ref.CertificateDigest, ArtifactDigest: release.Artifact.Digest, LaunchDriver: release.Launch.Driver})
	}
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].Name < subjects[j].Name })
	evidence := RuntimeBindingEvidence{
		SchemaVersion: 1, ID: "fixture-stage2-" + profile + "-runtime-binding", CoreCommit: evidenceDependencyCoreCommit,
		CompositionID: validated.Manifest.ID, CompositionDigest: validated.Digest, Profile: profile, ObservedAt: time.Now().UTC().Format(time.RFC3339),
		Platform: platform, Executable: RuntimeExecutableBinding{SourceArtifactDigest: artifactDigest, RuntimeBinaryDigest: binaryDigest, BuildRecipe: recipe, BindingMethod: "sealed-runner-reproducible-build-recipe"}, Subjects: subjects,
		RuntimeEvidence: RuntimeEvidenceBinding{Summary: summaryLock, Scenarios: scenarioLocks, Cleanup: cleanupLock, ScenarioCount: len(scenarioLocks), ExecutionState: "pass", CleanupState: "pass"},
		BindingState:    "runtime-recipe-observed-with-explicit-gaps", Gaps: []string{"process-executable-attestation-unavailable", "subject-v2-certificate-atomic-binding-unavailable"}, DefinitiveEligible: false, Verdict: "pass",
	}
	if err := ValidateRuntimeBindingEvidence(root, evidence); err != nil {
		return RuntimeBindingEvidence{}, err
	}
	return evidence, nil
}

func ValidateRuntimeBindingEvidence(root string, evidence RuntimeBindingEvidence) error {
	validated, err := Preflight(root, "compositions/fixture-stage2.json", evidence.Profile)
	if err != nil {
		return err
	}
	if evidence.SchemaVersion != 1 || evidence.ID != "fixture-stage2-"+evidence.Profile+"-runtime-binding" || evidence.CoreCommit != evidenceDependencyCoreCommit || evidence.CompositionID != validated.Manifest.ID || evidence.CompositionDigest != validated.Digest || evidence.Verdict != "pass" || evidence.DefinitiveEligible || evidence.BindingState != "runtime-recipe-observed-with-explicit-gaps" || !sameSet(evidence.Gaps, []string{"process-executable-attestation-unavailable", "subject-v2-certificate-atomic-binding-unavailable"}) {
		return fmt.Errorf("Runtime Binding Evidenceの状態契約が不正です")
	}
	if _, err := time.Parse(time.RFC3339, evidence.ObservedAt); err != nil {
		return fmt.Errorf("Runtime Binding Evidenceの観測時刻が不正です: %w", err)
	}
	if evidence.Executable.BindingMethod != "sealed-runner-reproducible-build-recipe" || evidence.Executable.BuildRecipe == "" || !validSHA256Digest(evidence.Executable.RuntimeBinaryDigest) || !validSHA256Digest(evidence.Executable.SourceArtifactDigest) {
		return fmt.Errorf("Runtime executableのrecipe／digest契約が不正です")
	}
	if evidence.Profile == "local" {
		if evidence.Platform.Kind != "local-process" || evidence.Platform.OS == "" || evidence.Platform.Architecture == "" || evidence.Platform.GoVersion == "" || evidence.Platform.ContainerRuntime != "" {
			return fmt.Errorf("local Runtime Platform契約が不正です")
		}
	} else if evidence.Profile == "container" {
		if evidence.Platform.Kind != "docker-container" || evidence.Platform.OS != "linux" || evidence.Platform.Architecture == "" || evidence.Platform.GoVersion == "" || !strings.HasPrefix(evidence.Platform.ContainerRuntime, "docker/") {
			return fmt.Errorf("container Runtime Platform契約が不正です")
		}
	}
	if evidence.RuntimeEvidence.ExecutionState != "pass" || evidence.RuntimeEvidence.CleanupState != "pass" || evidence.RuntimeEvidence.ScenarioCount != len(validated.Manifest.Scenarios) || len(evidence.RuntimeEvidence.Scenarios) != len(validated.Manifest.Scenarios) {
		return fmt.Errorf("Runtime／Cleanup Evidenceが全Scenarioを閉じていません")
	}
	locks := []PreviewArtifactLock{evidence.RuntimeEvidence.Summary, evidence.RuntimeEvidence.Cleanup}
	locks = append(locks, evidence.RuntimeEvidence.Scenarios...)
	for _, lock := range locks {
		digest, err := DigestFile(resolve(root, lock.Path))
		if err != nil || digest != lock.Digest {
			return fmt.Errorf("Runtime Binding Artifact Digest不一致: %s", lock.Path)
		}
	}
	var summary RunSummary
	if err := LoadJSON(resolve(root, evidence.RuntimeEvidence.Summary.Path), &summary); err != nil {
		return fmt.Errorf("Runtime Summaryを検証できません: %w", err)
	}
	if summary.Profile != evidence.Profile || summary.CompositionDigest != evidence.CompositionDigest || summary.Verdict != "pass" || len(summary.ScenarioReports) != len(validated.Manifest.Scenarios) {
		return fmt.Errorf("Runtime Summaryの実行結果が不正です")
	}
	var cleanup CleanupReceipt
	if err := LoadJSON(resolve(root, evidence.RuntimeEvidence.Cleanup.Path), &cleanup); err != nil {
		return fmt.Errorf("Cleanup Receiptを検証できません: %w", err)
	}
	if cleanup.SchemaVersion != 1 || cleanup.Profile != evidence.Profile || cleanup.CompositionID != evidence.CompositionID || cleanup.Processes != 0 || cleanup.Containers != 0 || cleanup.Networks != 0 || cleanup.Images != 0 || cleanup.CredentialsPersisted || cleanup.Verdict != "pass" {
		return fmt.Errorf("Cleanup Receiptが完全Cleanupを証明していません")
	}
	seenScenarios := map[string]bool{}
	for _, lock := range evidence.RuntimeEvidence.Scenarios {
		var scenario ScenarioReport
		if err := LoadJSON(resolve(root, lock.Path), &scenario); err != nil {
			return fmt.Errorf("Scenario Reportを検証できません: %w", err)
		}
		if scenario.Profile != evidence.Profile || scenario.CompositionDigest != evidence.CompositionDigest || scenario.Verdict != "pass" || seenScenarios[scenario.ScenarioID] {
			return fmt.Errorf("Scenario Reportの実行結果が不正です: %s", scenario.ScenarioID)
		}
		seenScenarios[scenario.ScenarioID] = true
	}
	for _, scenarioPath := range validated.Manifest.Scenarios {
		expectedID := strings.TrimSuffix(filepath.Base(scenarioPath), filepath.Ext(scenarioPath))
		if !seenScenarios[expectedID] {
			return fmt.Errorf("Runtime Bindingに必須Scenarioがありません: %s", expectedID)
		}
	}
	byName := map[string]RuntimeSubjectBinding{}
	for _, subject := range evidence.Subjects {
		if subject.Name == "" || byName[subject.Name].Name != "" {
			return fmt.Errorf("Runtime BindingのSubject識別子が重複しています: %s", subject.Name)
		}
		byName[subject.Name] = subject
	}
	if len(byName) != len(validated.Manifest.Subjects) {
		return fmt.Errorf("Runtime BindingのSubject数がCompositionと一致しません")
	}
	for _, ref := range validated.Manifest.Subjects {
		release := validated.Releases[ref.Name]
		actual := byName[ref.Name]
		if actual.SubjectID != ref.SubjectID || actual.Version != ref.Version || actual.ReleaseDigest != ref.ReleaseDigest || actual.CertificateDigest != ref.CertificateDigest || actual.ArtifactDigest != release.Artifact.Digest || actual.LaunchDriver != release.Launch.Driver {
			return fmt.Errorf("Runtime BindingのSubject Lock不一致: %s", ref.Name)
		}
		if evidence.Executable.SourceArtifactDigest != actual.ArtifactDigest {
			return fmt.Errorf("Runtime executableのsource artifactがSubject Releaseと一致しません: %s", ref.Name)
		}
	}
	return nil
}

func ValidateSavedRuntimeBindingEvidence(root, profile string) error {
	if profile != "local" && profile != "container" {
		return fmt.Errorf("保存Runtime Bindingはlocalまたはcontainer Profileが必須です")
	}
	var evidence RuntimeBindingEvidence
	path := filepath.Join(root, "evidence", "preview", "runtime-binding", profile+".binding.json")
	if err := LoadJSON(path, &evidence); err != nil {
		return err
	}
	if evidence.Profile != profile {
		return fmt.Errorf("保存Runtime BindingのProfileが不一致です: %s", profile)
	}
	return ValidateRuntimeBindingEvidence(root, evidence)
}

func validSHA256Digest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	for _, character := range strings.TrimPrefix(value, "sha256:") {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func buildRuntimeProbe(root string, validated ValidatedComposition, profile string) (RuntimeBindingPlatform, string, string, error) {
	artifact := validated.Releases[validated.Manifest.Subjects[0].Name].Artifact.Path
	output := filepath.Join(root, ".lab", "runtime-binding", profile, "fixture-subject")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return RuntimeBindingPlatform{}, "", "", err
	}
	platform := RuntimeBindingPlatform{GoVersion: runtime.Version()}
	recipe := "go build -trimpath"
	command := exec.Command("go", "build", "-trimpath", "-o", output, artifact)
	command.Dir = root
	command.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, ".cache", "go-build-probe"))
	if profile == "local" {
		platform.Kind, platform.OS, platform.Architecture = "local-process", runtime.GOOS, runtime.GOARCH
	} else {
		architectureOutput, err := commandOutput("docker", "info", "--format", "{{.Architecture}}")
		if err != nil {
			return RuntimeBindingPlatform{}, "", "", err
		}
		architecture := normalizeArch(strings.TrimSpace(architectureOutput))
		version, err := commandOutput("docker", "info", "--format", "{{.ServerVersion}}")
		if err != nil {
			return RuntimeBindingPlatform{}, "", "", err
		}
		platform.Kind, platform.OS, platform.Architecture, platform.ContainerRuntime = "docker-container", "linux", architecture, "docker/"+strings.TrimSpace(version)
		recipe = "CGO_ENABLED=0 GOOS=linux GOARCH=" + architecture + " go build -trimpath"
		command.Env = append(command.Env, "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+architecture)
	}
	if outputBytes, err := command.CombinedOutput(); err != nil {
		return RuntimeBindingPlatform{}, "", "", fmt.Errorf("Runtime probe build失敗: %w: %s", err, strings.TrimSpace(string(outputBytes)))
	}
	digest, err := DigestFile(output)
	return platform, digest, recipe, err
}

func copyRuntimeRepository(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if info.IsDir() && (parts[0] == ".git" || parts[0] == ".cache" || parts[0] == ".lab" || filepath.ToSlash(relative) == "evidence/preview/runtime-binding") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(destination, relative), info.Mode().Perm())
		}
		return copyFile(path, filepath.Join(destination, relative), info.Mode().Perm())
	})
}

func copyRuntimeArtifact(temporaryRoot, root, sourceRelative, destinationRelative string) (PreviewArtifactLock, error) {
	source := filepath.Join(temporaryRoot, sourceRelative)
	destination := filepath.Join(root, destinationRelative)
	if err := copyFile(source, destination, 0o644); err != nil {
		return PreviewArtifactLock{}, err
	}
	digest, err := DigestFile(destination)
	return PreviewArtifactLock{Path: filepath.ToSlash(destinationRelative), Digest: digest}, err
}

func copyFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
