package chatcompletions

func nonNegInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		if n < 0 || n != float64(int(n)) {
			return 0, false
		}
		return int(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}
