package rtk

import (
	"regexp"
	"strings"
)

type GitOpsFilter struct{}

func (f *GitOpsFilter) Name() string { return "gitops" }

func (f *GitOpsFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	timePattern := regexp.MustCompile(`\d+\s+(year|month|week|day|hour|minute|second|year|month|week|day|hour|min|sec)\s+(ago|since)`)
	hashPattern := regexp.MustCompile(`[a-f0-9]{7,40}`)
	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

	for _, line := range lines {
		line = timePattern.ReplaceAllString(line, "TIME_AGO")
		line = hashPattern.ReplaceAllString(line, "HASH")
		line = datePattern.ReplaceAllString(line, "DATE")
		result = append(result, line)
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}
