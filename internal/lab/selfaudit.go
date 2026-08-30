package lab

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type SelfAuditReport struct {
	SchemaVersion int              `json:"schema_version"`
	Audit         string           `json:"audit"`
	Checks        []SelfAuditCheck `json:"checks"`
	Verdict       string           `json:"verdict"`
}

type SelfAuditCheck struct {
	Name    string `json:"name"`
	Verdict string `json:"verdict"`
	Detail  string `json:"detail"`
}

func SelfAudit(root string, allowDirty bool) SelfAuditReport {
	report := SelfAuditReport{SchemaVersion: 1, Audit: "atlas-interoperability-lab-v1", Checks: []SelfAuditCheck{}, Verdict: "pass"}
	report.add("repository-contract", ValidateRepository(root))
	_, publicationErr := PublicationGate(root)
	report.add("publication-gate", publicationErr)
	report.add("completion-certificate", ValidateCertificate(root))
	report.add("core-audit", runCoreAudit(root))
	report.add("dco", validateDCO(root))
	if allowDirty {
		report.Checks = append(report.Checks, SelfAuditCheck{Name: "worktree-clean", Verdict: "skipped", Detail: "--allow-dirtyにより実装中の作業ツリー検査を省略"})
	} else {
		report.add("worktree-clean", validateCleanWorktree(root))
	}
	return report
}

func (report *SelfAuditReport) add(name string, err error) {
	check := SelfAuditCheck{Name: name, Verdict: "pass", Detail: "検証済み"}
	if err != nil {
		check.Verdict = "fail"
		check.Detail = err.Error()
		report.Verdict = "fail"
	}
	report.Checks = append(report.Checks, check)
}

func runCoreAudit(root string) error {
	output, err := runPinnedV1Core(root, "audit", root)
	if err != nil {
		return fmt.Errorf("Core audit失敗: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

var dcoTrailer = regexp.MustCompile(`(?m)^Signed-off-by: .+ <[^<>[:space:]]+>$`)

func validateDCO(root string) error {
	output, err := exec.Command("git", "-C", root, "log", "--format=%H%x00%B%x00").Output()
	if err != nil {
		return fmt.Errorf("Git履歴を検証できません: %w", err)
	}
	parts := strings.Split(string(output), "\x00")
	missing := []string{}
	for i := 0; i+1 < len(parts); i += 2 {
		commit := strings.TrimSpace(parts[i])
		message := parts[i+1]
		if commit != "" && !dcoTrailer.MatchString(message) {
			missing = append(missing, commit)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("Signed-off-byがないCommit: %s", strings.Join(missing, ","))
	}
	return nil
}

func validateCleanWorktree(root string) error {
	output, err := exec.Command("git", "-C", root, "status", "--porcelain=v1", "--untracked-files=all").Output()
	if err != nil {
		return fmt.Errorf("作業ツリーを検証できません: %w", err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("作業ツリーに未Commit差分があります")
	}
	return nil
}
