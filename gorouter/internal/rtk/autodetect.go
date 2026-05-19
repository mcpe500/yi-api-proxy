package rtk

import (
	"regexp"
	"strings"
)

type AutoDetectFilter struct{}

func NewAutoDetectFilter() *AutoDetectFilter  { return &AutoDetectFilter{} }
func NewGitDiffFilter() *GitDiffFilter        { return &GitDiffFilter{} }
func NewLsFilter() *LsFilter                   { return &LsFilter{} }
func NewGrepFilter() *GrepFilter               { return &GrepFilter{} }
func NewBuildFilter() *BuildFilter             { return &BuildFilter{} }
func NewTestFilter() *TestFilter               { return &TestFilter{} }
func NewGitOpsFilter() *GitOpsFilter           { return &GitOpsFilter{} }
func NewGithubFilter() *GithubFilter          { return &GithubFilter{} }
func NewPkgMgrFilter() *PkgMgrFilter           { return &PkgMgrFilter{} }
func NewInfraFilter() *InfraFilter             { return &InfraFilter{} }
func NewNetworkFilter() *NetworkFilter         { return &NetworkFilter{} }

func (f *AutoDetectFilter) Name() string { return "autodetect" }

func (f *AutoDetectFilter) Process(content string) (string, int) {
	lines := strings.Split(content, "\n")

	if hasGitDiffMarkers(lines) {
		return NewGitDiffFilter().Process(content)
	}
	if hasTestMarkers(lines) {
		return NewTestFilter().Process(content)
	}
	if hasGitOpsMarkers(content) {
		return NewGitOpsFilter().Process(content)
	}
	if hasGithubMarkers(lines) {
		return NewGithubFilter().Process(content)
	}
	if hasPkgMgrMarkers(lines) {
		return NewPkgMgrFilter().Process(content)
	}
	if hasInfraMarkers(lines) {
		return NewInfraFilter().Process(content)
	}
	if hasNetworkMarkers(lines) {
		return NewNetworkFilter().Process(content)
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

func hasTestMarkers(lines []string) bool {
	testPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)test.*pass|fail|run|ok`),
		regexp.MustCompile(`^  [✓✕○]`),
		regexp.MustCompile(`^Test |PASS|FAIL|FAILED`),
		regexp.MustCompile(`running \d+ tests?`),
	}
	matches := 0
	for _, line := range lines {
		for _, p := range testPatterns {
			if p.MatchString(line) {
				matches++
				break
			}
		}
	}
	return matches >= 5
}

func hasGitOpsMarkers(content string) bool {
	gitPatterns := []*regexp.Regexp{
		regexp.MustCompile(`On branch|HEAD detached|Your branch`),
		regexp.MustCompile(`Changes to be committed|Untracked files|Changes not staged`),
		regexp.MustCompile(`\d+ files? changed`),
		regexp.MustCompile(`[a-f0-9]{7,40} \w+ \w+`),
	}
	matches := 0
	for _, p := range gitPatterns {
		if p.MatchString(content) {
			matches++
		}
	}
	return matches >= 1
}

func hasGithubMarkers(lines []string) bool {
	ghPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^#|Pull Request|Issue #|PR #`),
		regexp.MustCompile(`[a-f0-9]{7,40} \d{4}-\d{2}-\d{2}`),
		regexp.MustCompile(`(Open|In Progress|Closed|Merged)`),
		regexp.MustCompile(`reviewed|approved|checks? passed`),
	}
	matches := 0
	for _, line := range lines {
		for _, p := range ghPatterns {
			if p.MatchString(line) {
				matches++
				break
			}
		}
	}
	return matches >= 3
}

func hasPkgMgrMarkers(lines []string) bool {
	pkgPatterns := []*regexp.Regexp{
		regexp.MustCompile(`added \d+ packages`),
		regexp.MustCompile(`\d+ packages? (found|audited|installed)`),
		regexp.MustCompile(`packages can be updated`),
		regexp.MustCompile(`\-> \S+@\S+`),
	}
	matches := 0
	for _, line := range lines {
		for _, p := range pkgPatterns {
			if p.MatchString(line) {
				matches++
				break
			}
		}
	}
	return matches >= 2
}

func hasInfraMarkers(lines []string) bool {
	infraPatterns := []*regexp.Regexp{
		regexp.MustCompile(`CONTAINER ID|NAMES|IMAGE`),
		regexp.MustCompile(`NAME|READY|STATUS|RESTARTS`),
		regexp.MustCompile(`[a-f0-9]{12} [a-z0-9\-]+`),
		regexp.MustCompile(`Up \d+|Exited|Healthy|NotReady`),
	}
	matches := 0
	for _, line := range lines {
		for _, p := range infraPatterns {
			if p.MatchString(line) {
				matches++
				break
			}
		}
	}
	return matches >= 2
}

func hasNetworkMarkers(lines []string) bool {
	netPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\d+%\s+\d+[KMGT]?B\s+\d+[KMGT]?B/s`),
		regexp.MustCompile(`Saving to|wget|curl`),
		regexp.MustCompile(`HTTP/[12] \d+`),
	}
	matches := 0
	for _, line := range lines {
		for _, p := range netPatterns {
			if p.MatchString(line) {
				matches++
				break
			}
		}
	}
	return matches >= 2
}