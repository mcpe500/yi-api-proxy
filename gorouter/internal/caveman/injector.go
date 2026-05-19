package caveman

import "github.com/gorouter/gorouter/internal/translator"

func Inject(messages []translator.NormalizedMessage, level string) []translator.NormalizedMessage {
	if level == "" {
		return messages
	}

	prompt := GetPrompt(level)
	if prompt == "" {
		return messages
	}

	injected := make([]translator.NormalizedMessage, 0, len(messages)+1)
	injected = append(injected, translator.NormalizedMessage{
		Role:    "system",
		Content: prompt,
	})
	injected = append(injected, messages...)
	return injected
}

func GetPrompt(level string) string {
	switch level {
	case "full":
		return Full
	case "lite":
		return Lite
	case "ultra":
		return Ultra
	default:
		return ""
	}
}