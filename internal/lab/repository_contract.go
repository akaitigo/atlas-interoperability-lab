package lab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var repositoryContractLines = []string{
	"schemaVersion: 1",
	"id: atlas-interoperability-lab",
	"classification: reference-implementation",
	"lifecycle: active",
	"sensitivity: public",
	"hostClass: managed-development-mac",
	"commands:",
	"  verify: make check",
	"externalWrites:",
	"  github: allowed",
	"  cloud: denied",
	"  existingRepositories: denied",
}

func ValidateRepositoryContract(root string) error {
	return validateRepositoryContract(func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	})
}

func validateRepositoryContract(reader repositoryReader) error {
	data, err := reader("repo.yaml")
	if err != nil {
		return violation("repository-contract-removed", "repo.yamlがありません")
	}
	lines := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != len(repositoryContractLines) {
		return violation("repository-contract-regressed", "repo.yamlのfield集合が正規fleet contractと一致しません")
	}
	for index, expected := range repositoryContractLines {
		if lines[index] != expected {
			return violation("repository-contract-regressed", fmt.Sprintf("repo.yamlのfieldまたは境界が不正です: line=%d", index+1))
		}
	}
	return nil
}
