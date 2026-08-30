package lab

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CheckpointPublicationReport struct {
	SchemaVersion     int    `json:"schema_version"`
	Isolation         string `json:"isolation"`
	PublicationChecks int    `json:"publication_checks"`
	Certificate       string `json:"certificate"`
	CoreAudit         string `json:"core_audit"`
	Verdict           string `json:"verdict"`
}

func ValidateIsolatedPublicationCheckpoint(root string) (CheckpointPublicationReport, error) {
	report := CheckpointPublicationReport{SchemaVersion: 1, Isolation: "task-owned-temporary-copy", Verdict: "pass"}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	temporaryRoot, err := cloneCheckpointRepository(absoluteRoot)
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(temporaryRoot)
	if err := copyRuntimeRepository(absoluteRoot, temporaryRoot); err != nil {
		return report, err
	}
	if err := GenerateProvenance(temporaryRoot); err != nil {
		return report, err
	}
	publication, err := PublicationGate(temporaryRoot)
	if err != nil {
		return report, fmt.Errorf("隔離Publication Gateが失敗しました: %w", err)
	}
	if publication.Verdict != "pass" {
		return report, fmt.Errorf("隔離Publication Gateがpassではありません")
	}
	report.PublicationChecks = len(publication.Checks)
	if err := GenerateCertificate(temporaryRoot); err != nil {
		return report, err
	}
	if err := ValidateCertificate(temporaryRoot); err != nil {
		return report, err
	}
	report.Certificate = "pass"
	if err := ValidateLegacyV1Bundle(temporaryRoot); err != nil {
		return report, err
	}
	report.CoreAudit = "pass"
	return report, nil
}

func cloneCheckpointRepository(root string) (string, error) {
	temporaryRoot, err := os.MkdirTemp(filepath.Dir(root), ".atlas-lab-publication-checkpoint-")
	if err != nil {
		return "", err
	}
	if err := os.Remove(temporaryRoot); err != nil {
		return "", err
	}
	command := exec.Command("git", "clone", "--shared", "--no-hardlinks", "--quiet", root, temporaryRoot)
	if output, err := command.CombinedOutput(); err != nil {
		_ = os.RemoveAll(temporaryRoot)
		return "", fmt.Errorf("task-owned Publication checkpoint cloneを作成できません: %w: %s", err, output)
	}
	return temporaryRoot, nil
}
