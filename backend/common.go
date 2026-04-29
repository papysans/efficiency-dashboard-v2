package main

import "math"

/**
 *	ancientMinutes: 10
 *	realMinutes: 1
 *	efficiencyRatio: (ancientMinutes - realMinutes) / realMinutes = 900%
 */
func calcEfficiencyRatio(ancientMinutes, realMinutes float64) float64 {
	if ancientMinutes <= 0 || realMinutes <= 0 || math.IsInf(realMinutes, 0) {
		return 0
	}
	percent := ((ancientMinutes - realMinutes) / realMinutes) * 100
	percent = math.Round(percent*10) / 10
	return percent
}
