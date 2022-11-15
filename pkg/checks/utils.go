package checks

import (
	"fmt"
	"strings"
	"time"
)

const (
	day  = 24 * time.Hour
	year = 365 * day
)

func humanDuration(t time.Duration) string {
	if t < day {
		return t.String()
	}

	var b strings.Builder

	if t >= year {
		years := t / year
		t -= years * year
		fmt.Fprintf(&b, "%dy", years)
	}

	days := t / day
	t -= days * day
	fmt.Fprintf(&b, "%dd%s", days, t)

	return b.String()
}
