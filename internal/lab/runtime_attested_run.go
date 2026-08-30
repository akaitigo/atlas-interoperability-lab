package lab

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func runWithRuntimeAttestation(root, compositionPath, profile string) (RunSummary, []RuntimeExecutableAttestation, error) {
	validated, err := Preflight(root, compositionPath, profile)
	if err != nil {
		return RunSummary{}, nil, err
	}
	token, err := credential()
	if err != nil {
		return RunSummary{}, nil, err
	}
	admin, err := credential()
	if err != nil {
		return RunSummary{}, nil, err
	}
	subjectRuntime, err := startRuntime(root, profile, validated, token, admin)
	if err != nil {
		return RunSummary{}, nil, err
	}
	attestations, err := attestRunningExecutables(subjectRuntime, validated)
	if err != nil {
		subjectRuntime.Cleanup()
		return RunSummary{}, nil, err
	}
	variables := map[string]any{"token.valid": token, "token.admin": admin, "token.invalid": "invalid-fixture-token"}
	for name, url := range subjectRuntime.URLs() {
		variables[name+".url"] = url
	}
	reportPaths := []string{}
	reports := []ScenarioReport{}
	var runErr error
	for _, scenarioPath := range validated.Manifest.Scenarios {
		scenario, loadErr := ValidateScenario(root, scenarioPath, validated.Manifest.Axes)
		if loadErr != nil {
			runErr = loadErr
			break
		}
		scenarioDigest, _ := DigestFile(resolve(root, scenarioPath))
		report := executeScenario(scenario, profile, validated.Digest, scenarioDigest, subjectRuntime.URLs(), variables)
		path := filepath.Join(root, "evidence", "runs", profile, scenario.ID+".json")
		if err := WriteJSON(path, report); err != nil {
			runErr = err
			break
		}
		reportPaths = append(reportPaths, path)
		reports = append(reports, report)
		if report.Verdict != "pass" && runErr == nil {
			runErr = fmt.Errorf("Scenario %s が失敗しました", scenario.ID)
		}
	}
	receipt := subjectRuntime.Cleanup()
	receiptPath := filepath.Join(root, "cleanup", profile+".receipt.json")
	if err := WriteJSON(receiptPath, receipt); err != nil && runErr == nil {
		runErr = err
	}
	if receipt.Verdict != "pass" && runErr == nil {
		runErr = fmt.Errorf("%s ProfileのCleanupが不完全です", profile)
	}
	evidenceDigest, digestErr := DigestSet(reportPaths)
	if digestErr != nil && runErr == nil {
		runErr = digestErr
	}
	verdict := "pass"
	if runErr != nil {
		verdict = "fail"
	}
	summary := RunSummary{Profile: profile, CompositionDigest: validated.Digest, ScenarioReports: relativePaths(root, reportPaths), EvidenceSetDigest: evidenceDigest, Verdict: verdict, CleanupReceipt: filepath.ToSlash(mustRelative(root, receiptPath))}
	summaryPath := filepath.Join(root, "evidence", "runs", profile, "summary.json")
	if err := WriteJSON(summaryPath, summary); err != nil {
		return summary, attestations, err
	}
	if err := writeCoreEvidence(root, profile, validated, summaryPath, verdict); err != nil {
		return summary, attestations, err
	}
	return summary, attestations, runErr
}

func attestRunningExecutables(subjectRuntime subjectRuntime, validated ValidatedComposition) ([]RuntimeExecutableAttestation, error) {
	switch active := subjectRuntime.(type) {
	case *localRuntime:
		ordered := orderedSubjects(validated.Manifest.Subjects)
		if len(active.commands) != len(ordered) {
			return nil, fmt.Errorf("local executable attestationのSubject数が不一致です")
		}
		attestations := make([]RuntimeExecutableAttestation, 0, len(ordered))
		for index, command := range active.commands {
			if command.Process == nil {
				return nil, fmt.Errorf("local processが起動していません: %s", ordered[index].Name)
			}
			path, method, err := runningProcessExecutable(command.Process.Pid)
			if err != nil {
				return nil, err
			}
			digest, err := DigestFile(path)
			if err != nil {
				return nil, err
			}
			attestations = append(attestations, RuntimeExecutableAttestation{SubjectName: ordered[index].Name, CaptureMethod: method, ObservedDigest: digest, ObservedAt: time.Now().UTC().Format(time.RFC3339)})
		}
		return attestations, nil
	case *containerRuntime:
		attestations := make([]RuntimeExecutableAttestation, 0, len(active.names))
		for _, name := range active.names {
			temporaryRoot, err := os.MkdirTemp("", "atlas-lab-live-container-executable-")
			if err != nil {
				return nil, err
			}
			temporaryExecutable := filepath.Join(temporaryRoot, "fixture-subject")
			_, copyErr := commandOutput("docker", "cp", name+":/fixture-subject", temporaryExecutable)
			if copyErr != nil {
				_ = os.RemoveAll(temporaryRoot)
				return nil, copyErr
			}
			digest, digestErr := DigestFile(temporaryExecutable)
			_ = os.RemoveAll(temporaryRoot)
			if digestErr != nil {
				return nil, digestErr
			}
			attestations = append(attestations, RuntimeExecutableAttestation{SubjectName: strings.TrimPrefix(name, active.runID+"-"), CaptureMethod: "docker-cp-live-container", ObservedDigest: digest, ObservedAt: time.Now().UTC().Format(time.RFC3339)})
		}
		return attestations, nil
	default:
		return nil, fmt.Errorf("Runtime executable attestationに未対応のDriverです")
	}
}

func runningProcessExecutable(pid int) (string, string, error) {
	if runtime.GOOS == "linux" {
		path, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
		return path, "procfs-live-process", err
	}
	if runtime.GOOS == "darwin" {
		output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
		if err != nil {
			return "", "", err
		}
		path := strings.TrimSpace(string(output))
		if !filepath.IsAbs(path) {
			return "", "", fmt.Errorf("psが絶対executable pathを返しませんでした")
		}
		return path, "darwin-ps-live-process", nil
	}
	return "", "", fmt.Errorf("live process executable観測に未対応のOSです: %s", runtime.GOOS)
}
