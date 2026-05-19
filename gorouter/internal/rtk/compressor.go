package rtk

import (
	"log"
)

type CompressionResult struct {
	Content      string
	BytesSaved   int
	Filter       string
	PercentSaved float64
}

func Compress(content string) (string, int) {
	if content == "" {
		return "", 0
	}

	filtered, saved := NewAutoDetectFilter().Process(content)

	if saved > 0 {
		logCompression("autodetect", len(content), saved)
	}

	return filtered, saved
}

func CompressWithFilter(content string, filterName string) (string, int) {
	var filter Filter
	switch filterName {
	case "gitdiff":
		filter = NewGitDiffFilter()
	case "ls":
		filter = NewLsFilter()
	case "grep":
		filter = NewGrepFilter()
	case "build":
		filter = NewBuildFilter()
	default:
		filter = NewAutoDetectFilter()
	}

	compressed, saved := filter.Process(content)
	logCompression(filter.Name(), len(content), saved)
	return compressed, saved
}

func logCompression(filter string, original, saved int) {
	percent := 0.0
	if original > 0 {
		percent = float64(saved) / float64(original) * 100
	}
	log.Printf("[RTK] filter: %s, saved: %d bytes (%.2f%%)", filter, saved, percent)
}

func GetAvailableFilters() []string {
	return []string{"gitdiff", "ls", "grep", "build", "autodetect"}
}