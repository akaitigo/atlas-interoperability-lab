package lab

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type GateReport struct {
	SchemaVersion int      `json:"schema_version"`
	Gate          string   `json:"gate"`
	Checks        []string `json:"checks"`
	Verdict       string   `json:"verdict"`
}

func PublicationGate(root string) (GateReport, error) {
	report := GateReport{SchemaVersion: 1, Gate: "publication", Verdict: "pass"}
	required := []string{"LICENSE", "NOTICE", "SECURITY.md", "repo.yaml", runtimeBindingMigrationPath, fePublicReportPath, fePublicAttestationPath, fePublicSignaturePath, feAllowedSignersPath, "third_party/manifest.yaml", "sbom.spdx.json", "sources.lock.yaml", "provenance.yaml", "graphs/fixture-stage2.claim-evidence.json", "evidence/records/stage2.local.evidence.json", "evidence/records/stage2.container.evidence.json", "evals/skill/interoperability-router.skill-eval.json", "cleanup/local.receipt.json", "cleanup/container.receipt.json"}
	for _, relative := range required {
		info, err := os.Stat(filepath.Join(root, relative))
		if err != nil || info.Size() == 0 {
			return failGate(report, "必須Publication成果物がありません: "+relative)
		}
		report.Checks = append(report.Checks, "present:"+relative)
	}
	if err := ValidateRepositoryContract(root); err != nil {
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "repository-contract")
	if err := ValidateRuntimeBindingMigration(root); err != nil {
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "runtime-binding-attestation-migration")
	if publicCI, err := ValidatePublicCIGate(root); err != nil || publicCI.Verdict != "pass" {
		if err == nil {
			err = fmt.Errorf("public CI upstream separationがpassではありません")
		}
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "public-ci-upstream-separation")
	if err := verifyCoreLock(root); err != nil {
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "core-lock")
	if _, err := Preflight(root, "compositions/fixture-stage2.json", "local"); err != nil {
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "release-locks")
	if err := ValidateCrossSubjectGraph(root); err != nil {
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "cross-subject-claim-evidence-graph")
	// v1 Publication Report自体は不変に保つが、判定前に上位の
	// Non-Regression Gateを必ず通す。
	if _, err := NonRegressionGate(root); err != nil {
		return failGate(report, err.Error())
	}
	for _, profile := range []string{"local", "container"} {
		var summary RunSummary
		if err := LoadJSON(filepath.Join(root, "evidence", "runs", profile, "summary.json"), &summary); err != nil {
			return failGate(report, err.Error())
		}
		if summary.Verdict != "pass" {
			return failGate(report, profile+" Evidenceがpassではありません")
		}
		var receipt CleanupReceipt
		if err := LoadJSON(filepath.Join(root, "cleanup", profile+".receipt.json"), &receipt); err != nil {
			return failGate(report, err.Error())
		}
		if receipt.Verdict != "pass" || receipt.CredentialsPersisted {
			return failGate(report, profile+" Cleanupが不完全です")
		}
		report.Checks = append(report.Checks, "e2e:"+profile, "cleanup:"+profile)
	}
	var skillEval map[string]any
	if err := LoadJSON(filepath.Join(root, "evals", "skill", "interoperability-router.skill-eval.json"), &skillEval); err != nil {
		return failGate(report, err.Error())
	}
	cases, ok := skillEval["cases"].([]any)
	if !ok || len(cases) != 8 {
		return failGate(report, "Router Skill Evalの必須Categoryが不足しています")
	}
	for _, raw := range cases {
		item, _ := raw.(map[string]any)
		if item["result"] != "pass" {
			return failGate(report, "Router Skill Evalがpassではありません")
		}
	}
	report.Checks = append(report.Checks, "skill-eval")
	if err := scanSecrets(root); err != nil {
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "secret-scan")
	if err := validateRights(root); err != nil {
		return failGate(report, err.Error())
	}
	report.Checks = append(report.Checks, "rights", "sbom", "provenance")
	sort.Strings(report.Checks)
	return report, nil
}

func failGate(report GateReport, message string) (GateReport, error) {
	report.Verdict = "fail"
	return report, fmt.Errorf("Publication Gate拒否: %s", message)
}

func verifyCoreLock(root string) error {
	var composition Composition
	if err := LoadJSON(filepath.Join(root, "compositions", "fixture-stage2.json"), &composition); err != nil {
		return err
	}
	core := filepath.Join(root, "..", composition.CoreContract.Repository)
	if err := verifyPinnedCoreCommit(root, composition.CoreContract.Repository, composition.CoreContract.Commit); err != nil {
		return err
	}
	output, err := exec.Command("git", "-C", core, "show", composition.CoreContract.Commit+":VERSION").Output()
	if err != nil {
		return fmt.Errorf("固定Core VERSIONを検証できません: %w", err)
	}
	if strings.TrimSpace(string(output)) != composition.CoreContract.PolicyVersion {
		return fmt.Errorf("固定Core Policy Version不一致: expected=%s actual=%s", composition.CoreContract.PolicyVersion, strings.TrimSpace(string(output)))
	}
	return nil
}

var secretPatterns = []*regexp.Regexp{regexp.MustCompile(`BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY`), regexp.MustCompile(`AKIA[0-9A-Z]{16}`), regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`), regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{20,}`), regexp.MustCompile(`(?i)"(password|secret|api_key|access_token)"\s*:\s*"[^"$]{8,}"`)}

func scanSecrets(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		if info.IsDir() {
			if relative == ".git" || relative == ".cache" || relative == ".lab" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 2<<20 {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			for _, pattern := range secretPatterns {
				if pattern.MatchString(text) {
					return fmt.Errorf("秘密候補を検出: %s:%d", relative, line)
				}
			}
		}
		return scanner.Err()
	})
}

func validateRights(root string) error {
	data, err := os.ReadFile(filepath.Join(root, "third_party", "manifest.yaml"))
	if err != nil {
		return err
	}
	text := string(data)
	if strings.Contains(text, "unknown") || !strings.Contains(text, "redistribution: allowed") {
		return fmt.Errorf("第三者権利Manifestが公開可能状態ではありません")
	}
	var sbom map[string]any
	if err := LoadJSON(filepath.Join(root, "sbom.spdx.json"), &sbom); err != nil {
		return err
	}
	if sbom["spdxVersion"] != "SPDX-2.3" || sbom["dataLicense"] != "CC0-1.0" {
		return fmt.Errorf("SBOM契約不適合")
	}
	return validateProvenance(root)
}

func GenerateProvenance(root string) error {
	artifacts := []map[string]any{}
	for _, item := range []struct{ Path, Kind, License, Source, GeneratedBy string }{
		{"graphs/fixture-stage2.claim-evidence.json", "generated", "Apache-2.0", "reference-atlas-core.architecture", "atlas-lab graph contract v1"},
		{"evidence/runs/local/summary.json", "test-report", "Apache-2.0", "fixture-subject.source", "atlas-lab runner v1"},
		{"evidence/runs/container/summary.json", "test-report", "Apache-2.0", "fixture-subject.source", "atlas-lab runner v1"},
		{"evals/skill/interoperability-router.skill-eval.json", "skill-eval", "Apache-2.0", "reference-atlas-core.architecture", "python3 evals/run.py"},
		{"sbom.spdx.json", "sbom", "CC0-1.0", "reference-atlas-core.architecture", "atlas-lab publication v1"},
	} {
		digest, err := DigestFile(filepath.Join(root, filepath.FromSlash(item.Path)))
		if err != nil {
			return err
		}
		artifacts = append(artifacts, map[string]any{"path": item.Path, "digest": digest, "kind": item.Kind, "license": item.License, "source_ids": []string{item.Source}, "generated_by": item.GeneratedBy})
	}
	return WriteJSON(filepath.Join(root, "provenance.yaml"), map[string]any{"schema_version": 1, "atlas_id": "atlas-interoperability-lab", "generated_at": "2026-08-28T00:00:00+09:00", "artifacts": artifacts})
}

func validateProvenance(root string) error {
	var doc map[string]any
	if err := LoadJSON(filepath.Join(root, "provenance.yaml"), &doc); err != nil {
		return err
	}
	artifacts, _ := doc["artifacts"].([]any)
	for _, raw := range artifacts {
		item, _ := raw.(map[string]any)
		path, _ := item["path"].(string)
		digest, _ := item["digest"].(string)
		actual, err := DigestFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if actual != digest {
			return fmt.Errorf("Provenance Digest不一致: %s", path)
		}
	}
	return nil
}

func GenerateCertificate(root string) error {
	report, err := PublicationGate(root)
	if err != nil {
		return err
	}
	if err := WriteJSON(filepath.Join(root, "evidence", "publication-gate.json"), report); err != nil {
		return err
	}
	commitOutput, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("Certificate生成前にSource Commitが必要です: %w", err)
	}
	commit := strings.TrimSpace(string(commitOutput))
	output, err := runPinnedV1Core(root, "certificate", "generate", root, "--issued-at", time.Now().UTC().Format(time.RFC3339), "--commit", commit)
	if err != nil {
		return fmt.Errorf("Core Certificate生成失敗: %w: %s", err, output)
	}
	return nil
}

func ValidateCertificate(root string) error {
	output, err := runPinnedV1Core(root, "certificate", "verify", root)
	if err != nil {
		return fmt.Errorf("Core Certificate検証失敗: %w: %s", err, output)
	}
	return nil
}
