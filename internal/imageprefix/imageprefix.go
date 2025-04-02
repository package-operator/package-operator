package imageprefix

import (
	"strings"
)

// Override is a prefix replacement rule.
type Override struct {
	From, To string
}

// Replace replaces image prefix with most specific matching override.
func Replace(image string, overrides []Override) string {
	// Find most specific override.
	bestMatch := mostSpecificIndex(image, overrides)
	// No match found. Return original string.
	if bestMatch == -1 {
		return image
	}

	// Return image address with replaced prefix.
	override := overrides[bestMatch]
	return override.To + image[len(override.From):]
}

func mostSpecificIndex(image string, overrides []Override) int {
	bestMatchLen := 0
	bestMatch := -1

	for i, override := range overrides {
		// Skip not matching prefixes.
		if !strings.HasPrefix(image, override.From) {
			continue
		}
		// Skip if match is not longer than previous best match.
		if bestMatchLen >= len(override.From) {
			continue
		}

		// This override was the longest match yet.
		// Store it:
		bestMatch = i
		bestMatchLen = len(override.From)
	}

	return bestMatch
}
