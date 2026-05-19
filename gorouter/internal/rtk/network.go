package rtk

import (
	"regexp"
	"strings"
)

type NetworkFilter struct{}

func (f *NetworkFilter) Name() string { return "network" }

func (f *NetworkFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	progressPattern := regexp.MustCompile(`[█▓░]+`)
	sizePattern := regexp.MustCompile(`\d+[KMGT]?B`)
	speedPattern := regexp.MustCompile(`\d+(\.\d+)?[KMGT]?B/s`)
	timePattern := regexp.MustCompile(`\d+ms|\d+\.\d+s`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "% Total") || strings.HasPrefix(trimmed, "100 ") || strings.HasPrefix(trimmed, "0 ") {
			continue
		}
		if strings.Contains(trimmed, "--:--:--") || strings.Contains(trimmed, "ETA") {
			continue
		}

		line = progressPattern.ReplaceAllString(line, "")
		line = sizePattern.ReplaceAllString(line, "SIZE")
		line = speedPattern.ReplaceAllString(line, "SPEED")
		line = timePattern.ReplaceAllString(line, "TIME")

		if strings.Contains(line, "Saving to") || strings.Contains(line, "saved") {
			continue
		}
		if strings.Contains(line, "' saved") {
			continue
		}

		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}
