package lab

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var promotionalLanguagePatterns = []*regexp.Regexp{
	regexp.MustCompile(`世界一`),
	regexp.MustCompile(`唯一`),
	regexp.MustCompile(`決定版`),
	regexp.MustCompile(`最高`),
	regexp.MustCompile(`最良`),
	regexp.MustCompile(`究極`),
	regexp.MustCompile(`圧倒`),
	regexp.MustCompile(`おすすめ`),
	regexp.MustCompile(`推奨したく`),
	regexp.MustCompile(`称賛`),
	regexp.MustCompile(`素晴ら`),
	regexp.MustCompile(`優秀`),
}

var authorPraisePattern = regexp.MustCompile(`(?i)akaitigo(?:氏|さん|様)`)
var authorReferencePattern = regexp.MustCompile(`(?i)akaitigo`)

// ValidateNeutralLanguage keeps user-facing and machine-readable material factual.
// Technical namespace, source, install, and rights references remain permitted.
func ValidateNeutralLanguage(root string) error {
	return validateNeutralLanguage(root, func(path string) ([]byte, error) {
		return os.ReadFile(resolve(root, path))
	})
}

func validateNeutralLanguage(root string, reader repositoryReader) error {
	paths, err := editorialPaths(root)
	if err != nil {
		return err
	}
	for _, path := range paths {
		data, err := reader(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if authorPraisePattern.MatchString(line) {
				return violation("author-praise", fmt.Sprintf("%s:%d", path, lineNumber))
			}
			for _, pattern := range promotionalLanguagePatterns {
				if pattern.MatchString(line) {
					return violation("promotional-language", fmt.Sprintf("%s:%d", path, lineNumber))
				}
			}
			if authorReferencePattern.MatchString(line) && !technicalAuthorReference(line) {
				return violation("nontechnical-author-reference", fmt.Sprintf("%s:%d", path, lineNumber))
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	return nil
}

func editorialPaths(root string) ([]string, error) {
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if relative == ".git" || relative == ".cache" || relative == ".lab" {
				return filepath.SkipDir
			}
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".md" || extension == ".json" || extension == ".yaml" || extension == ".yml" {
			paths = append(paths, filepath.ToSlash(relative))
		}
		return nil
	})
	return paths, err
}

func technicalAuthorReference(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "github.com/akaitigo/") ||
		strings.Contains(lower, "github: akaitigo") ||
		(strings.Contains(lower, "akaitigo") && strings.Contains(lower, "github namespace")) ||
		strings.Contains(lower, "copyright") ||
		strings.Contains(lower, "module github.com/akaitigo/") ||
		strings.Contains(lower, "go install github.com/akaitigo/")
}
