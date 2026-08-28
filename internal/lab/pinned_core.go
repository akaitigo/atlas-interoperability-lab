package lab

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runPinnedV1Core executes the immutable Core revision locked by the v1
// Composition. It never depends on the current Core branch or worktree.
func runPinnedV1Core(root string, args ...string) ([]byte, error) {
	var composition Composition
	if err := LoadJSON(filepath.Join(root, "compositions", "fixture-stage2.json"), &composition); err != nil {
		return nil, err
	}
	return runPinnedCoreCommit(root, composition.CoreContract.Repository, composition.CoreContract.Commit, args...)
}

func runPinnedCoreCommit(root, repository, commit string, args ...string) ([]byte, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root = absoluteRoot
	temporaryRoot, cleanup, err := extractPinnedCore(root, repository, commit)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	commandArgs := append([]string{"-C", temporaryRoot, "run", "./cmd/atlas"}, args...)
	cmd := exec.Command("go", commandArgs...)
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(root, ".cache", "go-build"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("固定Core %s失敗: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func extractPinnedCore(root, repository, commit string) (string, func(), error) {
	if err := verifyPinnedCoreCommit(root, repository, commit); err != nil {
		return "", nil, err
	}
	coreRepository := filepath.Join(root, "..", repository)
	archive, err := exec.Command("git", "-C", coreRepository, "archive", "--format=tar", commit).Output()
	if err != nil {
		return "", nil, fmt.Errorf("固定Core Archiveを取得できません: %w", err)
	}
	temporaryRoot, err := os.MkdirTemp("", "atlas-lab-pinned-core-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(temporaryRoot) }
	if err := extractSafeTar(archive, temporaryRoot); err != nil {
		cleanup()
		return "", nil, err
	}
	return temporaryRoot, cleanup, nil
}

func ValidateLegacyV1Bundle(root string) error {
	if _, err := Preflight(root, "compositions/fixture-stage2.json", "local"); err != nil {
		return err
	}
	if err := ValidateCertificate(root); err != nil {
		return err
	}
	if output, err := runPinnedV1Core(root, "audit", root); err != nil {
		return fmt.Errorf("固定Core v1 Audit失敗: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func extractSafeTar(archive []byte, destination string) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("Core Archiveに不正Pathがあります: %s", header.Name)
		}
		target := filepath.Join(destination, clean)
		switch header.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("Core Archiveに未対応Entryがあります: %s", header.Name)
		}
	}
}
