package engine

import (
	"fmt"
	"math/rand"
)

func randN(n int) int        { return rand.Intn(n) }
func randFloat(lo, hi float64) float64 { return lo + rand.Float64()*(hi-lo) }
func clamp(v, lo, hi float64) float64 {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}
func randDigits(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += fmt.Sprintf("%d", rand.Intn(10))
	}
	return s
}
func hasSkill(skills []string, queue string) bool {
	for _, s := range skills {
		if s == queue {
			return true
		}
	}
	return false
}
