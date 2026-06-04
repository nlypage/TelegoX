package calendar

import "time"

// Date represents a calendar date without time or timezone.
type Date struct {
	Year  int        `json:"year"`
	Month time.Month `json:"month"`
	Day   int        `json:"day"`
}

// DateFromTime converts time to Date in the time's location.
func DateFromTime(t time.Time) Date {
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// Time converts Date to time at midnight in the provided location.
func (d Date) Time(loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
}

// IsZero reports whether Date is unset.
func (d Date) IsZero() bool {
	return d.Year == 0 || d.Month == 0 || d.Day == 0
}

// Format formats Date using time layout.
func (d Date) Format(layout string) string {
	return d.Time(time.Local).Format(layout)
}

func (d Date) before(other Date) bool {
	return d.compare(other) < 0
}

func (d Date) after(other Date) bool {
	return d.compare(other) > 0
}

func (d Date) equal(other Date) bool {
	return d.compare(other) == 0
}

func (d Date) compare(other Date) int {
	if d.Year != other.Year {
		return d.Year - other.Year
	}
	if d.Month != other.Month {
		return int(d.Month - other.Month)
	}
	return d.Day - other.Day
}
