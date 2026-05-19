package rtk

import (
	"regexp"
	"strings"
)

type InfraFilter struct{}

func (f *InfraFilter) Name() string { return "infra" }

func (f *InfraFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	idPattern := regexp.MustCompile(`[a-f0-9]{12}|[a-f0-9]{64}`)
	timePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z`)
	agoPattern := regexp.MustCompile(`\d+\s+(second|minute|hour|day|week|month|year)s?\s+ago`)

	for _, line := range lines {
		line = idPattern.ReplaceAllString(line, "ID")
		line = timePattern.ReplaceAllString(line, "TIME")
		line = agoPattern.ReplaceAllString(line, "TIME_AGO")
		result = append(result, line)
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}
