package lab

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type subjectRuntime interface {
	URLs() map[string]string
	Cleanup() CleanupReceipt
}

func startRuntime(root, profile string, validated ValidatedComposition, token, admin string) (subjectRuntime, error) {
	if profile == "local" {
		return startLocal(root, validated, token, admin)
	}
	if profile == "container" {
		return startContainer(root, validated, token, admin)
	}
	return nil, fmt.Errorf("Profile %s は実行Driverを持ちません（cloud-liveは任意Profileです）", profile)
}

type localRuntime struct {
	compositionID string
	urls          map[string]string
	commands      []*exec.Cmd
	workDir       string
}

func startLocal(root string, validated ValidatedComposition, token, admin string) (*localRuntime, error) {
	runID := shortID()
	workDir := filepath.Join(root, ".lab", "runs", runID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	first := validated.Manifest.Subjects[0]
	artifact := validated.Releases[first.Name].Artifact.Path
	bin := filepath.Join(workDir, "fixture-subject")
	build := exec.Command("go", "build", "-trimpath", "-o", bin, artifact)
	build.Dir = root
	build.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, ".cache", "go-build"))
	if output, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("Fixture build失敗: %v: %s", err, output)
	}
	r := &localRuntime{compositionID: validated.Manifest.ID, urls: map[string]string{}, workDir: workDir}
	ports := map[string]int{}
	for _, ref := range validated.Manifest.Subjects {
		port, err := freePort()
		if err != nil {
			return nil, err
		}
		ports[ref.Name] = port
	}
	for _, ref := range orderedSubjects(validated.Manifest.Subjects) {
		release := validated.Releases[ref.Name]
		args := append([]string{}, release.Launch.Args...)
		args = append(args, "-listen", fmt.Sprintf("127.0.0.1:%d", ports[ref.Name]))
		if ref.Name == "source" {
			args = append(args, "-peer", fmt.Sprintf("http://127.0.0.1:%d", ports["sink"]))
		}
		cmd := exec.Command(bin, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "ATLAS_FIXTURE_TOKEN="+token, "ATLAS_FIXTURE_ADMIN_TOKEN="+admin)
		logFile, err := os.Create(filepath.Join(workDir, ref.Name+".log"))
		if err != nil {
			r.Cleanup()
			return nil, err
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			_ = logFile.Close()
			r.Cleanup()
			return nil, err
		}
		r.commands = append(r.commands, cmd)
		r.urls[ref.Name] = fmt.Sprintf("http://127.0.0.1:%d", ports[ref.Name])
		if err := waitReady(r.urls[ref.Name] + "/health"); err != nil {
			r.Cleanup()
			return nil, err
		}
	}
	return r, nil
}

func (r *localRuntime) URLs() map[string]string { return r.urls }
func (r *localRuntime) Cleanup() CleanupReceipt {
	remaining := 0
	for i := len(r.commands) - 1; i >= 0; i-- {
		cmd := r.commands[i]
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		if cmd.ProcessState == nil {
			remaining++
		}
	}
	_ = os.RemoveAll(r.workDir)
	verdict := "pass"
	if remaining != 0 {
		verdict = "fail"
	}
	return CleanupReceipt{SchemaVersion: 1, Profile: "local", CompositionID: r.compositionID, Processes: remaining, CredentialsPersisted: false, Verdict: verdict}
}

type containerRuntime struct {
	compositionID, runID, network, image, root string
	names                                      []string
	urls                                       map[string]string
}

func startContainer(root string, validated ValidatedComposition, token, admin string) (*containerRuntime, error) {
	runID := "atlas-lab-" + shortID()
	r := &containerRuntime{compositionID: validated.Manifest.ID, runID: runID, network: runID, root: root, urls: map[string]string{}}
	archOutput, err := commandOutput("docker", "info", "--format", "{{.Architecture}}")
	if err != nil {
		return nil, fmt.Errorf("Docker daemonを利用できません: %w", err)
	}
	arch := normalizeArch(strings.TrimSpace(archOutput))
	if arch == "" {
		return nil, fmt.Errorf("Docker architectureが未対応です: %s", archOutput)
	}
	workDir := filepath.Join(root, ".lab", "runs", runID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, err
	}
	artifact := validated.Releases[validated.Manifest.Subjects[0].Name].Artifact.Path
	bin := filepath.Join(workDir, "fixture-subject")
	build := exec.Command("go", "build", "-trimpath", "-o", bin, artifact)
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch, "GOCACHE="+filepath.Join(root, ".cache", "go-build"))
	if output, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("Linux fixture build失敗: %v: %s", err, output)
	}
	r.image = runID + ":fixture"
	dockerfile := filepath.Join(root, "environments", "Dockerfile.fixture")
	if _, err := commandOutput("docker", "build", "--label", "org.atlas-lab.run="+runID, "-t", r.image, "-f", dockerfile, workDir); err != nil {
		r.Cleanup()
		return nil, err
	}
	if _, err := commandOutput("docker", "network", "create", "--label", "org.atlas-lab.run="+runID, r.network); err != nil {
		r.Cleanup()
		return nil, err
	}
	for _, ref := range orderedSubjects(validated.Manifest.Subjects) {
		release := validated.Releases[ref.Name]
		name := runID + "-" + ref.Name
		r.names = append(r.names, name)
		args := []string{"run", "-d", "--name", name, "--network", r.network, "--network-alias", ref.Name, "--label", "org.atlas-lab.run=" + runID, "-e", "ATLAS_FIXTURE_TOKEN=" + token, "-e", "ATLAS_FIXTURE_ADMIN_TOKEN=" + admin}
		if ref.Name == "source" {
			args = append(args, "-p", "127.0.0.1::8080")
		}
		args = append(args, r.image)
		args = append(args, release.Launch.Args...)
		args = append(args, "-listen", "0.0.0.0:8080")
		if ref.Name == "source" {
			args = append(args, "-peer", "http://sink:8080")
		}
		if _, err := commandOutput("docker", args...); err != nil {
			r.Cleanup()
			return nil, err
		}
	}
	port, err := commandOutput("docker", "port", runID+"-source", "8080/tcp")
	if err != nil {
		r.Cleanup()
		return nil, err
	}
	parts := strings.Split(strings.TrimSpace(port), ":")
	if len(parts) < 2 {
		r.Cleanup()
		return nil, fmt.Errorf("Docker publish portを解決できません: %s", port)
	}
	r.urls["source"] = "http://127.0.0.1:" + parts[len(parts)-1]
	if err := waitReady(r.urls["source"] + "/health"); err != nil {
		r.Cleanup()
		return nil, err
	}
	return r, nil
}

func (r *containerRuntime) URLs() map[string]string { return r.urls }
func (r *containerRuntime) Cleanup() CleanupReceipt {
	for i := len(r.names) - 1; i >= 0; i-- {
		_, _ = commandOutput("docker", "rm", "-f", r.names[i])
	}
	if r.network != "" {
		_, _ = commandOutput("docker", "network", "rm", r.network)
	}
	if r.image != "" {
		_, _ = commandOutput("docker", "image", "rm", "-f", r.image)
	}
	_ = os.RemoveAll(filepath.Join(r.root, ".lab", "runs", r.runID))
	containers := dockerCount("ps", "-aq", "--filter", "label=org.atlas-lab.run="+r.runID)
	networks := dockerCount("network", "ls", "-q", "--filter", "label=org.atlas-lab.run="+r.runID)
	images := dockerCount("image", "ls", "-q", "--filter", "label=org.atlas-lab.run="+r.runID)
	verdict := "pass"
	if containers+networks+images != 0 {
		verdict = "fail"
	}
	return CleanupReceipt{SchemaVersion: 1, Profile: "container", CompositionID: r.compositionID, Containers: containers, Networks: networks, Images: images, CredentialsPersisted: false, Verdict: verdict}
}

func orderedSubjects(subjects []SubjectRef) []SubjectRef {
	result := make([]SubjectRef, 0, len(subjects))
	for _, item := range subjects {
		if item.Name == "sink" {
			result = append(result, item)
		}
	}
	for _, item := range subjects {
		if item.Name != "sink" {
			result = append(result, item)
		}
	}
	return result
}
func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
func shortID() string {
	data := make([]byte, 6)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data)
}
func normalizeArch(value string) string {
	switch value {
	case "arm64", "aarch64":
		return "arm64"
	case "amd64", "x86_64":
		return "amd64"
	}
	return ""
}
func commandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, output)
	}
	return string(output), nil
}
func dockerCount(args ...string) int {
	output, err := commandOutput("docker", args...)
	if err != nil || strings.TrimSpace(output) == "" {
		return 0
	}
	return len(strings.Fields(output))
}
func waitReady(url string) error {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		cancel()
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("health timeout: %s", url)
}
