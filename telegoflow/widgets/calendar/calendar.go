package calendar

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mymmrac/telego"
	tf "github.com/mymmrac/telego/telegoflow"
	"github.com/mymmrac/telego/telegoflow/widget"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	calendarActionNoop   = "noop"
	calendarActionPrev   = "p"
	calendarActionNext   = "n"
	calendarActionDate   = "d"
	calendarActionToday  = "t"
	calendarActionCancel = "x"
)

// CalendarBuilder builds a Calendar widget.
type CalendarBuilder[T any] struct {
	id        string
	weekStart time.Weekday
	location  *time.Location
	labels    Labels

	min    Date
	hasMin bool
	max    Date
	hasMax bool

	current  func(data *T) (Date, bool)
	disabled func(ctx *tf.Context[T], date Date) bool
	onSelect func(ctx *tf.Context[T], date Date) error
	onCancel func(ctx *tf.Context[T]) error
}

// NewCalendar creates a calendar widget builder.
func New[T any](id string) *CalendarBuilder[T] {
	return &CalendarBuilder[T]{
		id:        id,
		weekStart: time.Monday,
		location:  time.Local,
		labels:    EnglishLabels(),
	}
}

// WeekStart sets the first day of calendar week.
func (b *CalendarBuilder[T]) WeekStart(day time.Weekday) *CalendarBuilder[T] {
	b.weekStart = day
	return b
}

// Location sets location used for Today and Date.Time conversions.
func (b *CalendarBuilder[T]) Location(loc *time.Location) *CalendarBuilder[T] {
	if loc != nil {
		b.location = loc
	}
	return b
}

// Min disables dates before date.
func (b *CalendarBuilder[T]) Min(date Date) *CalendarBuilder[T] {
	b.min = date
	b.hasMin = !date.IsZero()
	return b
}

// Max disables dates after date.
func (b *CalendarBuilder[T]) Max(date Date) *CalendarBuilder[T] {
	b.max = date
	b.hasMax = !date.IsZero()
	return b
}

// Labels sets calendar labels.
func (b *CalendarBuilder[T]) Labels(labels Labels) *CalendarBuilder[T] {
	b.labels = labels
	return b
}

// Current sets a function that returns the current date value from flow data.
//
// The current date is used to open the calendar on that month and format the day
// with Labels.SelectedDay.
func (b *CalendarBuilder[T]) Current(current func(data *T) (Date, bool)) *CalendarBuilder[T] {
	b.current = current
	return b
}

// Disabled sets a function that disables individual dates.
func (b *CalendarBuilder[T]) Disabled(disabled func(ctx *tf.Context[T], date Date) bool) *CalendarBuilder[T] {
	b.disabled = disabled
	return b
}

// OnSelect sets a handler called when user selects a date.
func (b *CalendarBuilder[T]) OnSelect(handler func(ctx *tf.Context[T], date Date) error) *CalendarBuilder[T] {
	b.onSelect = handler
	return b
}

// OnCancel sets a handler called when user presses the cancel button.
func (b *CalendarBuilder[T]) OnCancel(handler func(ctx *tf.Context[T]) error) *CalendarBuilder[T] {
	b.onCancel = handler
	return b
}

// Build validates and creates a Calendar widget.
func (b *CalendarBuilder[T]) Build() (*Calendar[T], error) {
	if err := widget.ValidateID(b.id); err != nil {
		return nil, err
	}
	if b.hasMin && b.hasMax && b.min.after(b.max) {
		return nil, fmt.Errorf("widgets: calendar min date must not be after max date")
	}
	labels := b.labels.normalize()
	loc := b.location
	if loc == nil {
		loc = time.Local
	}
	return &Calendar[T]{
		id:        b.id,
		weekStart: b.weekStart,
		location:  loc,
		labels:    labels,
		min:       b.min,
		hasMin:    b.hasMin,
		max:       b.max,
		hasMax:    b.hasMax,
		current:   b.current,
		disabled:  b.disabled,
		onSelect:  b.onSelect,
		onCancel:  b.onCancel,
	}, nil
}

// Calendar is an inline calendar widget for telegoflow steps.
type Calendar[T any] struct {
	id        string
	weekStart time.Weekday
	location  *time.Location
	labels    Labels

	min    Date
	hasMin bool
	max    Date
	hasMax bool

	current  func(data *T) (Date, bool)
	disabled func(ctx *tf.Context[T], date Date) bool
	onSelect func(ctx *tf.Context[T], date Date) error
	onCancel func(ctx *tf.Context[T]) error
}

// ID returns widget ID.
func (c *Calendar[T]) ID() string { return c.id }

// Predicate returns a predicate that matches callback queries for this calendar.
func (c *Calendar[T]) Predicate() th.Predicate { return widget.CallbackPredicate(c.id) }

// Send returns an Enter handler that sends this calendar with static text.
func (c *Calendar[T]) Send(text string) tf.Handler[T] { return c.SendText(widget.Text[T](text)) }

// SendText returns an Enter handler that sends this calendar with dynamic text.
func (c *Calendar[T]) SendText(text widget.TextFunc[T]) tf.Handler[T] {
	return func(ctx *tf.Context[T]) error {
		message, err := text(ctx)
		if err != nil {
			return err
		}
		markup, err := c.Markup(ctx)
		if err != nil {
			return err
		}
		return widget.SendView(ctx, widget.View{Text: message, Markup: markup})
	}
}

// Step creates a telegoflow step that renders and handles this calendar.
func (c *Calendar[T]) Step(stepID, text string) *tf.Step[T] {
	return tf.NewStep[T](stepID).Enter(c.Send(text)).Handle(c.Handle, c.Predicate())
}

// Markup builds inline keyboard markup for the current date value or current month.
func (c *Calendar[T]) Markup(ctx *tf.Context[T]) (*telego.InlineKeyboardMarkup, error) {
	year, month := c.initialMonth(ctx.Data())
	return c.renderMonth(ctx, ctx.Data(), year, month)
}

// Handle handles callback queries for this calendar.
func (c *Calendar[T]) Handle(ctx *tf.Context[T]) error {
	query := ctx.CallbackQuery()
	if query == nil {
		return nil
	}
	callback, ok := widget.DecodeCallback(query.Data)
	if !ok || callback.WidgetID != c.id {
		return nil
	}
	if err := widget.AnswerCallback(ctx, ""); err != nil {
		return err
	}

	switch callback.Action {
	case calendarActionNoop:
		return nil
	case calendarActionPrev, calendarActionNext:
		if len(callback.Args) != 1 {
			return nil
		}
		year, month, ok := parseCalendarMonth(callback.Args[0])
		if !ok {
			return nil
		}
		markup, err := c.renderMonth(ctx, ctx.Data(), year, month)
		if err != nil {
			return err
		}
		return widget.EditCallbackView(ctx, widget.View{Text: widget.CallbackMessageText(query, "Calendar"), Markup: markup})
	case calendarActionDate, calendarActionToday:
		if len(callback.Args) != 1 {
			return nil
		}
		date, ok := parseCalendarDate(callback.Args[0])
		if !ok || !c.dateAllowed(ctx, date) {
			return nil
		}
		if c.onSelect != nil {
			return c.onSelect(ctx, date)
		}
		return nil
	case calendarActionCancel:
		if c.onCancel != nil {
			return c.onCancel(ctx)
		}
		return nil
	default:
		return nil
	}
}

func (c *Calendar[T]) initialMonth(data *T) (int, time.Month) {
	if c.current != nil {
		if current, ok := c.current(data); ok && !current.IsZero() {
			return current.Year, current.Month
		}
	}
	now := time.Now().In(c.location)
	return now.Year(), now.Month()
}

func (c *Calendar[T]) renderMonth(ctx *tf.Context[T], data *T, year int, month time.Month) (*telego.InlineKeyboardMarkup, error) {
	first := time.Date(year, month, 1, 0, 0, 0, 0, c.location)
	year, month = first.Year(), first.Month()

	rows := make([][]telego.InlineKeyboardButton, 0, 9)

	prevMonth := first.AddDate(0, -1, 0)
	nextMonth := first.AddDate(0, 1, 0)
	title := fmt.Sprintf("%s %d", c.labels.Months[int(month)-1], year)
	rows = append(rows, tu.InlineKeyboardRow(
		c.navButton(c.labels.PrevMonth, calendarActionPrev, prevMonth),
		c.noopButton(title),
		c.navButton(c.labels.NextMonth, calendarActionNext, nextMonth),
	))

	weekdayRow := make([]telego.InlineKeyboardButton, 0, 7)
	for i := 0; i < 7; i++ {
		weekday := time.Weekday((int(c.weekStart) + i) % 7)
		weekdayRow = append(weekdayRow, c.noopButton(c.labels.Weekdays[int(weekday)]))
	}
	rows = append(rows, weekdayRow)

	current, hasCurrent := Date{}, false
	if c.current != nil {
		current, hasCurrent = c.current(data)
	}

	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, c.location).Day()
	firstOffset := (int(first.Weekday()) - int(c.weekStart) + 7) % 7
	day := 1
	for day <= daysInMonth {
		row := make([]telego.InlineKeyboardButton, 0, 7)
		for col := 0; col < 7; col++ {
			if len(rows) == 2 && col < firstOffset || day > daysInMonth {
				row = append(row, c.noopButton(c.labels.EmptyDay))
				continue
			}

			date := Date{Year: year, Month: month, Day: day}
			label := strconv.Itoa(day)
			if hasCurrent && date.equal(current) {
				label = fmt.Sprintf(c.labels.SelectedDay, label)
			}
			if !c.dateInRange(date) {
				row = append(row, c.noopButton(c.labels.EmptyDay))
			} else if !c.dateAllowed(ctx, date) {
				row = append(row, c.noopButton(label))
			} else {
				button, err := c.dateButton(label, date)
				if err != nil {
					return nil, err
				}
				row = append(row, button)
			}
			day++
		}
		rows = append(rows, row)
	}

	today := DateFromTime(time.Now().In(c.location))
	footer := []telego.InlineKeyboardButton{}
	if c.dateAllowed(ctx, today) {
		todayButton, err := c.actionButton(c.labels.Today, calendarActionToday, formatCalendarDate(today))
		if err != nil {
			return nil, err
		}
		footer = append(footer, todayButton)
	}
	cancelButton, err := c.actionButton(c.labels.Cancel, calendarActionCancel)
	if err != nil {
		return nil, err
	}
	footer = append(footer, cancelButton)
	rows = append(rows, footer)

	return tu.InlineKeyboard(rows...), nil
}

func (c *Calendar[T]) navButton(label, action string, month time.Time) telego.InlineKeyboardButton {
	if !c.monthAllowed(month.Year(), month.Month()) {
		return c.noopButton(label)
	}
	data, err := widget.EncodeCallback(c.id, action, formatCalendarMonth(month.Year(), month.Month()))
	if err != nil {
		return c.noopButton(label)
	}
	return tu.InlineKeyboardButton(label).WithCallbackData(data)
}

func (c *Calendar[T]) dateButton(label string, date Date) (telego.InlineKeyboardButton, error) {
	data, err := widget.EncodeCallback(c.id, calendarActionDate, formatCalendarDate(date))
	if err != nil {
		return telego.InlineKeyboardButton{}, err
	}
	return tu.InlineKeyboardButton(label).WithCallbackData(data), nil
}

func (c *Calendar[T]) actionButton(label, action string, args ...string) (telego.InlineKeyboardButton, error) {
	data, err := widget.EncodeCallback(c.id, action, args...)
	if err != nil {
		return telego.InlineKeyboardButton{}, err
	}
	return tu.InlineKeyboardButton(label).WithCallbackData(data), nil
}

func (c *Calendar[T]) noopButton(label string) telego.InlineKeyboardButton {
	data, _ := widget.EncodeCallback(c.id, calendarActionNoop)
	return tu.InlineKeyboardButton(label).WithCallbackData(data)
}

func (c *Calendar[T]) dateAllowed(ctx *tf.Context[T], date Date) bool {
	if !c.dateInRange(date) {
		return false
	}
	if c.disabled != nil && c.disabled(ctx, date) {
		return false
	}
	return true
}

func (c *Calendar[T]) dateInRange(date Date) bool {
	if date.IsZero() {
		return false
	}
	if c.hasMin && date.before(c.min) {
		return false
	}
	if c.hasMax && date.after(c.max) {
		return false
	}
	return true
}

func (c *Calendar[T]) monthAllowed(year int, month time.Month) bool {
	first := Date{Year: year, Month: month, Day: 1}
	last := Date{Year: year, Month: month, Day: time.Date(year, month+1, 0, 0, 0, 0, 0, c.location).Day()}
	if c.hasMin && last.before(c.min) {
		return false
	}
	if c.hasMax && first.after(c.max) {
		return false
	}
	return true
}

func formatCalendarMonth(year int, month time.Month) string {
	return fmt.Sprintf("%04d%02d", year, int(month))
}

func parseCalendarMonth(value string) (int, time.Month, bool) {
	if len(value) != 6 {
		return 0, 0, false
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return 0, 0, false
	}
	monthInt, err := strconv.Atoi(value[4:])
	if err != nil || monthInt < 1 || monthInt > 12 {
		return 0, 0, false
	}
	return year, time.Month(monthInt), true
}

func formatCalendarDate(date Date) string {
	return fmt.Sprintf("%04d%02d%02d", date.Year, int(date.Month), date.Day)
}

func parseCalendarDate(value string) (Date, bool) {
	if len(value) != 8 {
		return Date{}, false
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return Date{}, false
	}
	monthInt, err := strconv.Atoi(value[4:6])
	if err != nil || monthInt < 1 || monthInt > 12 {
		return Date{}, false
	}
	day, err := strconv.Atoi(value[6:])
	if err != nil || day < 1 || day > 31 {
		return Date{}, false
	}
	date := Date{Year: year, Month: time.Month(monthInt), Day: day}
	if date.Time(time.UTC).Month() != date.Month || date.Time(time.UTC).Day() != date.Day {
		return Date{}, false
	}
	return date, true
}
