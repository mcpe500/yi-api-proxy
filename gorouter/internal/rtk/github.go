package rtk

import (
	"regexp"
	"strings"
)

type GithubFilter struct{}

func (f *GithubFilter) Name() string { return "github" }

func (f *GithubFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	urlPattern := regexp.MustCompile(`https?://\S+`)
	timePattern := regexp.MustCompile(`\d+\s+(second|minute|hour|day|week|month|year)s?\s+ago`)
	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2})?`)
	shaPattern := regexp.MustCompile(`[a-f0-9]{7,40}`)

	for _, line := range lines {
		line = urlPattern.ReplaceAllString(line, "URL")
		line = timePattern.ReplaceAllString(line, "TIME_AGO")
		line = datePattern.ReplaceAllString(line, "DATETIME")
		line = shaPattern.ReplaceAllString(line, "HASH")
		result = append(result, line)
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}
