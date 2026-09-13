package domain

import "math"

func round1(n float64) float64 { return math.Round(n*10) / 10 }

// ABV returns the standard (131.25 factor) and the alcohol-corrected estimate.
func ABV(og, fg float64) (standard, corrected float64) {
	standard = (og - fg) * 131.25
	corrected = (76.08 * (og - fg) / (1.775 - og)) * (fg / 0.794)
	return round1(standard), round1(corrected)
}
