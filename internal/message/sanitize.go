package message

import "strings"

func SanitizeMessageContent(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if strings.EqualFold(key, "jpegThumbnail") || strings.EqualFold(key, "jpeg_thumbnail") {
				continue
			}
			out[key] = SanitizeMessageContent(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = SanitizeMessageContent(item)
		}
		return out
	default:
		return value
	}
}
