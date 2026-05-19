package rtk

import (
	"regexp"
	"strings"
)

type LsFilter struct{}

func (f *LsFilter) Name() string { return "ls" }

func (f *LsFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}|\d{2}/\d{2}/\d{4}`)
	timePattern := regexp.MustCompile(`\d{2}:\d{2}(:\d{2})?`)
	permPattern := regexp.MustCompile(`[d-][rwx-]{9}`)

	for _, line := range lines {
		line = datePattern.ReplaceAllString(line, "DATE")
		line = timePattern.ReplaceAllString(line, "TIME")
		line = permPattern.ReplaceAllString(line, "PERM")
		result = append(result, line)
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}