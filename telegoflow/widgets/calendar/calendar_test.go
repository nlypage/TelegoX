package calendar

import (
	"testing"
	"time"
)

type calendarTestData struct {
	Current Date
}

func TestCalendarRenderMonth(t *testing.T) {
	calendar, err := New[calendarTestData]("date").
		Labels(RussianLabels()).
		Current(func(data *calendarTestData) (Date, bool) {
			return data.Current, !data.Current.IsZero()
		}).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := calendar.renderMonth(nil, &calendarTestData{Current: Date{Year: 2026, Month: time.June, Day: 2}}, 2026, time.June)
	if err != nil {
		t.Fatalf("renderMonth() error = %v", err)
	}
	if len(markup.InlineKeyboard) != 8 {
		t.Fatalf("rows = %d; want 8", len(markup.InlineKeyboard))
	}
	if got := markup.InlineKeyboard[0][1].Text; got != "Июнь 2026" {
		t.Fatalf("title = %q; want %q", got, "Июнь 2026")
	}
	if got := markup.InlineKeyboard[2][1].Text; got != "✓ 2" {
		t.Fatalf("current day label = %q; want %q", got, "✓ 2")
	}
	if markup.InlineKeyboard[2][1].CallbackData != "tfw:date:d:20260602" {
		t.Fatalf("current day callback = %q", markup.InlineKeyboard[2][1].CallbackData)
	}
}

func TestCalendarCurrentDayUsesLabels(t *testing.T) {
	calendar, err := New[calendarTestData]("date").
		Labels(Labels{SelectedDay: "[%s]"}).
		Current(func(data *calendarTestData) (Date, bool) {
			return data.Current, !data.Current.IsZero()
		}).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	data := &calendarTestData{Current: Date{Year: 2026, Month: time.June, Day: 2}}
	markup, err := calendar.renderMonth(nil, data, 2026, time.June)
	if err != nil {
		t.Fatalf("renderMonth() error = %v", err)
	}
	if got := markup.InlineKeyboard[2][1].Text; got != "[2]" {
		t.Fatalf("current day label = %q; want [2]", got)
	}
}

func TestCalendarMinMaxDisableDatesAndNavigation(t *testing.T) {
	calendar, err := New[calendarTestData]("date").
		Min(Date{Year: 2026, Month: time.June, Day: 10}).
		Max(Date{Year: 2026, Month: time.June, Day: 20}).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := calendar.renderMonth(nil, &calendarTestData{}, 2026, time.June)
	if err != nil {
		t.Fatalf("renderMonth() error = %v", err)
	}
	if got := markup.InlineKeyboard[0][0].CallbackData; got != "tfw:date:noop" {
		t.Fatalf("prev callback = %q; want noop", got)
	}
	if got := markup.InlineKeyboard[0][2].CallbackData; got != "tfw:date:noop" {
		t.Fatalf("next callback = %q; want noop", got)
	}
	if got := markup.InlineKeyboard[2][1].Text; got != " " { // June 2 is before min.
		t.Fatalf("out-of-range day text = %q; want empty day", got)
	}
	if got := markup.InlineKeyboard[2][1].CallbackData; got != "tfw:date:noop" {
		t.Fatalf("out-of-range day callback = %q; want noop", got)
	}
}

func TestParseCalendarDateRejectsInvalidDate(t *testing.T) {
	if _, ok := parseCalendarDate("20260231"); ok {
		t.Fatal("parseCalendarDate() accepted invalid date")
	}
}
