package calendar

const monthsPerYear = 12

// Labels contains user-visible calendar button labels.
type Labels struct {
	Months   []string
	Weekdays []string

	PrevMonth   string
	NextMonth   string
	Today       string
	Cancel      string
	EmptyDay    string
	SelectedDay string
}

// EnglishLabels returns English calendar labels.
func EnglishLabels() Labels {
	return Labels{
		Months: []string{
			"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December",
		},
		Weekdays:    []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		PrevMonth:   "‹",
		NextMonth:   "›",
		Today:       "Today",
		Cancel:      "Cancel",
		EmptyDay:    " ",
		SelectedDay: "✓ %s",
	}
}

// RussianLabels returns Russian calendar labels.
func RussianLabels() Labels {
	return Labels{
		Months: []string{
			"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
			"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
		},
		Weekdays:    []string{"Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"},
		PrevMonth:   "‹",
		NextMonth:   "›",
		Today:       "Сегодня",
		Cancel:      "Отмена",
		EmptyDay:    " ",
		SelectedDay: "✓ %s",
	}
}

func (l Labels) normalize() Labels {
	defaults := EnglishLabels()
	if len(l.Months) != monthsPerYear {
		l.Months = defaults.Months
	}
	if len(l.Weekdays) != daysPerWeek {
		l.Weekdays = defaults.Weekdays
	}
	if l.PrevMonth == "" {
		l.PrevMonth = defaults.PrevMonth
	}
	if l.NextMonth == "" {
		l.NextMonth = defaults.NextMonth
	}
	if l.Today == "" {
		l.Today = defaults.Today
	}
	if l.Cancel == "" {
		l.Cancel = defaults.Cancel
	}
	if l.EmptyDay == "" {
		l.EmptyDay = defaults.EmptyDay
	}
	if l.SelectedDay == "" {
		l.SelectedDay = defaults.SelectedDay
	}
	return l
}
