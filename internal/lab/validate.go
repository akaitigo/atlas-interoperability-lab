package lab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ValidatedComposition struct {
	Manifest Composition
	Path     string
	Digest   string
	Releases map[string]Release
}

func Preflight(root, compositionPath, profile string) (ValidatedComposition, error) {
	path := resolve(root, compositionPath)
	var composition Composition
	if err := LoadJSON(path, &composition); err != nil {
		return ValidatedComposition{}, err
	}
	if composition.SchemaVersion != 1 || composition.Stage != 2 || composition.ID == "" {
		return ValidatedComposition{}, fmt.Errorf("Compositionはschema_version=1、stage=2、非空IDが必須です")
	}
	if !sameSet(composition.Axes, RequiredAxes) {
		return ValidatedComposition{}, fmt.Errorf("Composition %s は10検証軸を過不足なく宣言する必要があります", composition.ID)
	}
	if profile != "" && !contains(composition.Profiles, profile) {
		return ValidatedComposition{}, fmt.Errorf("Profile %s はCompositionに許可されていません", profile)
	}
	if len(composition.Subjects) < 2 {
		return ValidatedComposition{}, fmt.Errorf("Compositionには2つ以上のSubject Releaseが必要です")
	}
	releases := map[string]Release{}
	for _, ref := range composition.Subjects {
		manifestPath := resolve(root, ref.ReleaseManifest)
		actual, err := DigestFile(manifestPath)
		if err != nil {
			return ValidatedComposition{}, err
		}
		if actual != ref.ReleaseDigest {
			return ValidatedComposition{}, fmt.Errorf("Subject %s のRelease Digest不一致: expected=%s actual=%s", ref.Name, ref.ReleaseDigest, actual)
		}
		var release Release
		if err := LoadJSON(manifestPath, &release); err != nil {
			return ValidatedComposition{}, err
		}
		if release.SchemaVersion != 1 || release.SubjectID != ref.SubjectID || release.Version != ref.Version {
			return ValidatedComposition{}, fmt.Errorf("Subject %s のID/Versionが固定参照と一致しません", ref.Name)
		}
		if release.Status != "complete" {
			return ValidatedComposition{}, fmt.Errorf("Subject %s は未完成Release(status=%s)のため拒否しました", ref.SubjectID, release.Status)
		}
		artifactPath := filepath.Clean(filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(release.Artifact.Path)))
		artifactDigest, err := DigestFile(artifactPath)
		if err != nil {
			return ValidatedComposition{}, err
		}
		if artifactDigest != release.Artifact.Digest {
			return ValidatedComposition{}, fmt.Errorf("Subject %s のArtifact Digest不一致", ref.SubjectID)
		}
		certificatePath := filepath.Clean(filepath.Join(filepath.Dir(manifestPath), filepath.FromSlash(release.Certificate)))
		certificateDigest, err := DigestFile(certificatePath)
		if err != nil {
			return ValidatedComposition{}, err
		}
		if certificateDigest != ref.CertificateDigest {
			return ValidatedComposition{}, fmt.Errorf("Subject %s のCertificate Digest不一致", ref.SubjectID)
		}
		var certificate SubjectCertificate
		if err := LoadJSON(certificatePath, &certificate); err != nil {
			return ValidatedComposition{}, err
		}
		if certificate.Status != "complete" || certificate.SubjectID != release.SubjectID || certificate.Version != release.Version || certificate.ArtifactDigest != release.Artifact.Digest {
			return ValidatedComposition{}, fmt.Errorf("Subject %s のCompletion CertificateがReleaseを証明していません", ref.SubjectID)
		}
		if release.Launch.Driver != "go-source" || release.Launch.Port < 1 {
			return ValidatedComposition{}, fmt.Errorf("Subject %s のLaunch契約は未対応です", ref.SubjectID)
		}
		release.Artifact.Path = artifactPath
		releases[ref.Name] = release
	}
	for _, scenarioPath := range composition.Scenarios {
		if _, err := ValidateScenario(root, scenarioPath, composition.Axes); err != nil {
			return ValidatedComposition{}, err
		}
	}
	if profile != "" {
		var environment Environment
		if err := LoadJSON(resolve(root, "environments/"+profile+".json"), &environment); err != nil {
			return ValidatedComposition{}, err
		}
		if environment.SchemaVersion != 1 || environment.Profile != profile {
			return ValidatedComposition{}, fmt.Errorf("Environment Profileが一致しません")
		}
	}
	digest, err := DigestFile(path)
	if err != nil {
		return ValidatedComposition{}, err
	}
	return ValidatedComposition{Manifest: composition, Path: path, Digest: digest, Releases: releases}, nil
}

func ValidateScenario(root, path string, allowedAxes []string) (Scenario, error) {
	var scenario Scenario
	resolved := resolve(root, path)
	if err := LoadJSON(resolved, &scenario); err != nil {
		return scenario, err
	}
	if scenario.SchemaVersion != 1 || scenario.ID == "" || len(scenario.Axes) == 0 || scenario.Oracle == "" {
		return scenario, fmt.Errorf("Scenario %s の必須項目が不足しています", path)
	}
	for _, axis := range scenario.Axes {
		if !contains(allowedAxes, axis) {
			return scenario, fmt.Errorf("Scenario %s が未知の検証軸 %s を参照しています", scenario.ID, axis)
		}
	}
	if _, err := os.Stat(resolve(root, scenario.Oracle)); err != nil {
		return scenario, fmt.Errorf("Scenario %s のOracleがありません: %w", scenario.ID, err)
	}
	actions := append(append(append([]Action{}, scenario.Phases.Setup...), scenario.Phases.Execute...), scenario.Phases.Verify...)
	actions = append(actions, scenario.Phases.Cleanup...)
	if len(actions) == 0 {
		return scenario, fmt.Errorf("Scenario %s にActionがありません", scenario.ID)
	}
	for _, action := range actions {
		if action.ID == "" || (action.Type != "http" && action.Type != "compare") {
			return scenario, fmt.Errorf("Scenario %s のAction契約が不正です", scenario.ID)
		}
	}
	return scenario, nil
}

func resolve(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(path)))
}

func ValidateRepository(root string) error {
	if err := ValidateRepositoryContract(root); err != nil {
		return err
	}
	if _, err := Preflight(root, "compositions/fixture-stage2.json", "local"); err != nil {
		return err
	}
	if err := ValidateCrossSubjectGraph(root); err != nil {
		return err
	}
	var rejected Composition
	if err := LoadJSON(resolve(root, "tests/fixtures/composition-incomplete.json"), &rejected); err != nil {
		return err
	}
	_, err := Preflight(root, "tests/fixtures/composition-incomplete.json", "local")
	if err == nil || !strings.Contains(err.Error(), "未完成Release") {
		return fmt.Errorf("未完成Subjectの拒否契約が実証されませんでした")
	}
	if _, err := NonRegressionGate(root); err != nil {
		return err
	}
	if err := ValidateDepthInheritance(root); err != nil {
		return err
	}
	return nil
}
