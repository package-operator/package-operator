package utils

// Slice contains check.
func Contains[T comparable](elems []T, v T) bool {
	for _, s := range elems {
		if v == s {
			return true
		}
	}
	return false
}

func MergeKeysFrom(base, additional map[string]string) map[string]string {
	if base == nil {
		base = map[string]string{}
	}
	for k, v := range additional {
		base[k] = v
	}
	if len(base) == 0 {
		return nil
	}
	return base
}

func CopyMap[K comparable, V interface{}](toCopy map[K]V) map[K]V {
	out := map[K]V{}
	for k, v := range toCopy {
		out[k] = v
	}
	return out
}
