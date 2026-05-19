package rtk

import (
	"regexp"
	"strings"
)

type AutoDetectFilter struct{}

func NewAutoDetectFilter() *AutoDetectFilter { return &AutoDetectFilter{} }
func NewGitDiffFilter() *GitDiffFilter       { return &GitDiffFilter{} }
func NewLsFilter() *LsFilter                 { return &LsFilter{} }
func NewGrepFilter() *GrepFilter             { return &GrepFilter{} }
func NewBuildFilter() *BuildFilter           { return &BuildFilter{} }

func (f *AutoDetectFilter) Name() string { return "autodetect" }

func (f *AutoDetectFilter) Process(content string) (string, int) {
	lines := strings.Split(content, "\n")

	if hasGitDiffMarkers(lines) {
		return NewGitDiffFilter().Process(content)
	}
	if hasBuildMarkers(content) {
		return NewBuildFilter().Process(content)
	}
	if hasGrepMarkers(lines) {
		return NewGrepFilter().Process(content)
	}
	if hasLsMarkers(lines) {
		return NewLsFilter().Process(content)
	}

	return content, 0
}

func hasGitDiffMarkers(lines []string) bool {
	diffMarkers := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "diff ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.Contains(line, "@@") {
			diffMarkers++
		}
	}
	return diffMarkers >= 2
}

func hasBuildMarkers(content string) bool {
	buildPatterns := []*regexp.Regexp{
		regexp.MustCompile(`Compiling|BUILDING|Building`),
		regexp.MustCompile(`Finished|finished`),
		regexp.MustCompile(`cargo build|npm build|go build|tsc|npm run`),
	}
	matches := 0
	for _, p := range buildPatterns {
		if p.MatchString(content) {
			matches++
		}
	}
	return matches >= 2
}

func hasGrepMarkers(lines []string) bool {
	fileLinePattern := regexp.MustCompile(`^[^:\s]+:\d+:`)
	matches := 0
	for _, line := range lines {
		if fileLinePattern.MatchString(line) {
			matches++
		}
	}
	return matches >= 3
}

func hasLsMarkers(lines []string) bool {
	permPattern := regexp.MustCompile(`^[d-][rwx-]{9}`)
	timePattern := regexp.MustCompile(`\d{2}:\d{2}`)
	matches := 0
	for _, line := range lines {
		if permPattern.MatchString(line) {
			matches++
		} else if timePattern.MatchString(line) && strings.Contains(line, " ") {
			matches++
		}
	}
	return matches >= 3
}