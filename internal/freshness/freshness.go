package freshness

import "time"

type State string

const (
	Fresh       State = "fresh"
	Stale       State = "stale"
	Expired     State = "expired"
	Unavailable State = "unavailable"
)

func Evaluate(now, lastSuccess time.Time, warningAfter, expireAfter time.Duration) State {
	if lastSuccess.IsZero() {
		return Unavailable
	}
	age := now.Sub(lastSuccess)
	if age >= expireAfter {
		return Expired
	}
	if age >= warningAfter {
		return Stale
	}
	return Fresh
}
