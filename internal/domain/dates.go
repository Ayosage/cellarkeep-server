package domain

import (
	"fmt"
	"time"
)

// Day is a calendar date in YYYY-MM-DD with no timezone.
type Day string

func ParseDay(s string) (Day, error) {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", fmt.Errorf("invalid date %q: must be YYYY-MM-DD", s)
	}
	return Day(s), nil
}

func DayOf(t time.Time) Day { return Day(t.UTC().Format("2006-01-02")) }

func (d Day) Time() time.Time {
	t, _ := time.Parse("2006-01-02", string(d))
	return t
}

func (d Day) Add(days int) Day { return DayOf(d.Time().AddDate(0, 0, days)) }

func (d Day) Before(o Day) bool { return string(d) < string(o) }

func (d Day) String() string { return string(d) }
