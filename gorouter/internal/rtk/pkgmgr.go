package rtk

import (
	"regexp"
	"strings"
)

type PkgMgrFilter struct{}

func (f *PkgMgrFilter) Name() string { return "pkgmgr" }

func (f *PkgMgrFilter) Process(content string) (string, int) {
	originalLen := len(content)
	lines := strings.Split(content, "\n")
	var result []string

	progressPattern := regexp.MustCompile(`[█▓░]+|\[[=#*\-]+\]`)
	percentPattern := regexp.MustCompile(`\d+%`)
	timePattern := regexp.MustCompile(`\d+(\.\d+)?(ms|s|m)`)

	for _, line := range lines {
		line = progressPattern.ReplaceAllString(line, "")
		line = percentPattern.ReplaceAllString(line, "PCT")
		line = timePattern.ReplaceAllString(line, "TIME")

		if strings.Contains(line, "packages are looking for funding") {
			continue
		}
		if strings.Contains(line, "found 0 vulnerabilities") {
			continue
		}
		if strings.Contains(line, "audited") && strings.Contains(line, "packages in") {
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
