package rtk

import (
	"regexp"
	"strings"
)

type GitDiffFilter struct{}

func (f *GitDiffFilter) Name() string { return "gitdiff" }

func (f *GitDiffFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	hunkHeader := regexp.MustCompile(`^@@\s*[\-\+0-9,\s]+\s*@@`)

	for _, line := range lines {
		if strings.HasPrefix(line, "diff ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}
		if hunkHeader.MatchString(line) {
			if len(result) > 0 && result[len(result)-1] != "..." {
				result = append(result, "...")
			}
			continue
		}
		result = append(result, line)
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}