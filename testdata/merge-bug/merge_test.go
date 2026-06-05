package merge

import (
	"encoding/json"
	"testing"
)

func TestMergeJSON_RecursiveMerge(t *testing.T) {
	base := map[string]any{
		"a": float64(1),
		"nested": map[string]any{
			"x": float64(1),
			"y": float64(2),
		},
	}
	override := map[string]any{
		"b": float64(2),
		"nested": map[string]any{
			"y": float64(99),
			"z": float64(3),
		},
	}
	result := MergeJSON(base, override)

	if result["a"] != float64(1) {
		t.Errorf("base.a should be 1, got %v", result["a"])
	}
	if result["b"] != float64(2) {
		t.Errorf("override.b should be 2, got %v", result["b"])
	}

	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatal("nested should be a map")
	}
	if nested["x"] != float64(1) {
		t.Errorf("nested.x should be 1 (recursive merge preserves base keys), got %v", nested["x"])
	}
	if nested["y"] != float64(99) {
		t.Errorf("nested.y should be 99 (override value wins), got %v", nested["y"])
	}
	if nested["z"] != float64(3) {
		t.Errorf("nested.z should be 3 (override adds new keys), got %v", nested["z"])
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("result: %s", data)
}
