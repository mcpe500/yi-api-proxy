package rtk

import (
	"regexp"
	"strings"
)

type GrepFilter struct{}

func (f *GrepFilter) Name() string { return "grep" }

func (f *GrepFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	fileLinePattern := regexp.MustCompile(`^([^:\s]+):(\d+):`)

	for _, line := range lines {
		if matched := fileLinePattern.FindStringSubmatch(line); matched != nil {
			result = append(result, matched[1]+":"+matched[2]+":")
		} else {
			result = append(result, line)
		}
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}