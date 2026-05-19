package rtk

import (
	"regexp"
	"strings"
)

type TestFilter struct{}

func (f *TestFilter) Name() string { return "test" }

func (f *TestFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	// Patterns to keep (failures, summaries)
	failurePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)fail|error|fatal|exception`),
		regexp.MustCompile(`(?i)Expected|Actual`),
		regexp.MustCompile(`(?i)FAILED`),
		regexp.MustCompile(`^  ✕`), // Vitest/Jest failure
		regexp.MustCompile(`(?i)Summary|total|passed`),
	}

	// Patterns to drop (passing, progress)
	passPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^  ✓`), // Vitest/Jest pass
		regexp.MustCompile(`^  ○`), // Skip
		regexp.MustCompile(`(?i)PASS`),
		regexp.MustCompile(`(?i)ok\s+[\w\./]+`),
	}

	for _, line := range lines {
		if line == "" {
			result = append(result, line)
			continue
		}

		keep := false
		for _, p := range failurePatterns {
			if p.MatchString(line) {
				keep = true
				break
			}
		}

		if keep {
			result = append(result, line)
			continue
		}

		shouldDrop := false
		for _, p := range passPatterns {
			if p.MatchString(line) {
				shouldDrop = true
				break
			}
		}

		if !shouldDrop {
			result = append(result, line)
		}
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}
