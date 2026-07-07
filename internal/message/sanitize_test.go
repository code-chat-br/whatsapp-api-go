package message

import "testing"

func TestSanitizeMessageContent(t *testing.T) {
	input := map[string]any{
		"jpegThumbnail": "root",
		"text":          "ok",
		"nested": map[string]any{
			"jpegThumbnail": "nested",
			"keep":          true,
		},
		"items": []any{
			map[string]any{"jpegThumbnail": "slice", "x": 1},
			"plain",
		},
	}

	output := SanitizeMessageContent(input).(map[string]any)
	if _, ok := output["jpegThumbnail"]; ok {
		t.Fatalf("root jpegThumbnail was not removed")
	}
	nested := output["nested"].(map[string]any)
	if _, ok := nested["jpegThumbnail"]; ok {
		t.Fatalf("nested jpegThumbnail was not removed")
	}
	items := output["items"].([]any)
	item := items[0].(map[string]any)
	if _, ok := item["jpegThumbnail"]; ok {
		t.Fatalf("slice jpegThumbnail was not removed")
	}
	if output["text"] != "ok" || nested["keep"] != true || item["x"] != 1 {
		t.Fatalf("non-thumbnail fields changed: %#v", output)
	}
	if SanitizeMessageContent(nil) != nil {
		t.Fatalf("nil should stay nil")
	}
}
