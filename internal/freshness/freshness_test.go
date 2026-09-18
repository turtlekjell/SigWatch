package freshness

import (
	"testing"
	"time"
)

func TestTransitions(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		last time.Time
		want State
	}{{"none", time.Time{}, Unavailable}, {"fresh", now.Add(-14 * time.Minute), Fresh}, {"stale", now.Add(-15 * time.Minute), Stale}, {"expired", now.Add(-time.Hour), Expired}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Evaluate(now, tc.last, 15*time.Minute, time.Hour)
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
