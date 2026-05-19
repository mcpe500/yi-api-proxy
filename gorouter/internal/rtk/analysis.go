package rtk

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type AnalysisFilter struct {
	Mode string
}

func (f *AnalysisFilter) Name() string { return "analysis:" + f.Mode }

func (f *AnalysisFilter) Process(content string) (string, int) {
	switch f.Mode {
	case "err":
		return f.processErr(content)
	case "log":
		return f.processLog(content)
	case "json":
		return f.processJson(content)
	case "summary":
		return f.processSummary(content)
	}
	return content, 0
}

func (f *AnalysisFilter) processErr(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string
	errPattern := regexp.MustCompile(`(?i)err|fail|fatal|exception|panic`)

	for _, line := range lines {
		if errPattern.MatchString(line) {
			result = append(result, line)
		}
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}

func (f *AnalysisFilter) processLog(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", originalLen
	}

	var result []string
	count := 1
	lastLine := lines[0]

	for i := 1; i < len(lines); i++ {
		if lines[i] == lastLine {
			count++
		} else {
			if count > 1 {
				result = append(result, fmt.Sprintf("%s (x%d)", lastLine, count))
			} else {
				result = append(result, lastLine)
			}
			lastLine = lines[i]
			count = 1
		}
	}
	if count > 1 {
		result = append(result, fmt.Sprintf("%s (x%d)", lastLine, count))
	} else {
		result = append(result, lastLine)
	}

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}

func (f *AnalysisFilter) processJson(content string) (string, int) {
	originalLen := len(content)
	var data interface{}
	err := json.Unmarshal([]byte(content), &data)
	if err != nil {
		return content, 0
	}

	stripped := stripValues(data)
	out, _ := json.MarshalIndent(stripped, "", "  ")
	compressed := string(out)
	return compressed, originalLen - len(compressed)
}

func stripValues(i interface{}) interface{} {
	switch v := i.(type) {
	case map[string]interface{}:
		res := make(map[string]interface{})
		for k, val := range v {
			res[k] = stripValues(val)
		}
		return res
	case []interface{}:
		if len(v) == 0 {
			return v
		}
		return []interface{}{stripValues(v[0])}
	default:
		return "VAL"
	}
}

func (f *AnalysisFilter) processSummary(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	if len(lines) <= 20 {
		return content, 0
	}

	result := append(lines[:10], "...", "--- TRUNCATED ---", "...")
	result = append(result, lines[len(lines)-10:]...)

	compressed := strings.Join(result, "\n")
	return compressed, originalLen - len(compressed)
}
