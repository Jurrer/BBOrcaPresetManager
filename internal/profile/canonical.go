package profile

// JSONString extracts a string canonical value from a profile's JSON map.
// Returns "" if the field is absent or not a string.
func JSONString(m map[string]Value, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.Canonical.(string); ok {
			return s
		}
	}
	return ""
}
