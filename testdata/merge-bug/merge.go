package merge

// MergeJSON merges two JSON-like maps. Keys in override take precedence.
// BUG: when both base and override have a nested map for the same key,
// the override map replaces base's map wholesale instead of recursively
// merging them. This loses keys that exist only in base.
func MergeJSON(base, override map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
