package rtk

import (
	"regexp"
	"strings"
)

type BuildFilter struct{}

func (f *BuildFilter) Name() string { return "build" }

func (f *BuildFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	progressPattern := regexp.MustCompile(`(\d+)/(\d+)`)
	timePattern := regexp.MustCompile(`\d+m\d+s|^\d+\.\d+s`)
	hashPattern := regexp.MustCompile(`[a-f0-9]{7,}`)
	warningPrefix := regexp.MustCompile(`^warning:`)
	errorPrefix := regexp.MustCompile(`^error:`)

	for _, line := range lines {
		line = progressPattern.ReplaceAllString(line, "${1}/${2}")
		line = timePattern.ReplaceAllString(line, "TIME")
		line = hashPattern.ReplaceAllString(line, "HASH")
		if !warningPrefix.MatchString(line) && !errorPrefix.MatchString(line) {
			if strings.Contains(line, "Compiling") ||
				strings.Contains(line, "Building") ||
				strings.Contains(line, "Running") ||
				strings.Contains(line, "Finished") {
				continue
			}
		}
		result = append(result, line)
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}