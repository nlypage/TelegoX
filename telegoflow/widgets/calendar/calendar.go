package calendar

import (
	"errors"
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

	calendarRowsCap     = 9
	calendarMonthDigits = 6
	calendarDateDigits  = 8
	daysPerWeek         = 7
)

// Builder builds a Calendar widget.
type Builder[T any] struct {
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

// New creates a calendar widget builder.
func New[T any](id string) *Builder[T] {
	return &Builder[T]{
		id:        id,
		weekStart: time.Monday,
		location:  time.UTC,
		labels:    EnglishLabels(),
	}
}

// WeekStart sets the first day of calendar week.
func (b *Builder[T]) WeekStart(day time.Weekday) *Builder[T] {
	b.weekStart = day
	return b
}

// Location sets location used for Today and Date.Time conversions.
func (b *Builder[T]) Location(loc *time.Location) *Builder[T] {
	if loc != nil {
		b.location = loc
	}
	return b
}

// Min disables dates before date.
func (b *Builder[T]) Min(date Date) *Builder[T] {
	b.min = date
	b.hasMin = !date.IsZero()
	return b
}

// Max disables dates after date.
func (b *Builder[T]) Max(date Date) *Builder[T] {
	b.max = date
	b.hasMax = !date.IsZero()
	return b
}

// Labels sets calendar labels.
func (b *Builder[T]) Labels(labels Labels) *Builder[T] {
	b.labels = labels
	return b
}

// Current sets a function that returns the current date value from flow data.
//
// The current date is used to open the calendar on that month and format the day
// with Labels.SelectedDay.
func (b *Builder[T]) Current(current func(data *T) (Date, bool)) *Builder[T] {
	b.current = current
	return b
}

// Disabled sets a function that disables individual dates.
func (b *Builder[T]) Disabled(disabled func(ctx *tf.Context[T], date Date) bool) *Builder[T] {
	b.disabled = disabled
	return b
}

// OnSelect sets a handler called when user selects a date.
func (b *Builder[T]) OnSelect(handler func(ctx *tf.Context[T], date Date) error) *Builder[T] {
	b.onSelect = handler
	return b
}

// OnCancel sets a handler called when user presses the cancel button.
func (b *Builder[T]) OnCancel(handler func(ctx *tf.Context[T]) error) *Builder[T] {
	b.onCancel = handler
	return b
}

// Build validates and creates a Calendar widget.
func (b *Builder[T]) Build() (*Calendar[T], error) {
	if err := widget.ValidateID(b.id); err != nil {
		return nil, err
	}
	if b.hasMin && b.hasMax && b.min.after(b.max) {
		return nil, errors.New("widgets: calendar min date must not be after max date")
	}
	labels := b.labels.normalize()
	loc := b.location
	if loc == nil {
		loc = time.UTC
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

type dateSelection struct {
	date Date
	ok   bool
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
		return widget.EditCallbackView(ctx, widget.View{
			Text:   widget.CallbackMessageText(query, "Calendar"),
			Markup: markup,
		})
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

func (c *Calendar[T]) renderMonth(
	ctx *tf.Context[T],
	data *T,
	year int,
	month time.Month,
) (*telego.InlineKeyboardMarkup, error) {
	first := time.Date(year, month, 1, 0, 0, 0, 0, c.location)
	year, month = first.Year(), first.Month()

	rows := make([][]telego.InlineKeyboardButton, 0, calendarRowsCap)
	rows = append(rows, c.renderHeader(first), c.renderWeekdays())

	dayRows, err := c.renderDays(ctx, data, first, year, month)
	if err != nil {
		return nil, err
	}
	rows = append(rows, dayRows...)

	footer, err := c.renderFooter(ctx)
	if err != nil {
		return nil, err
	}
	rows = append(rows, footer)

	return tu.InlineKeyboard(rows...), nil
}

func (c *Calendar[T]) renderHeader(first time.Time) []telego.InlineKeyboardButton {
	year, month := first.Year(), first.Month()
	prevMonth := first.AddDate(0, -1, 0)
	nextMonth := first.AddDate(0, 1, 0)
	title := fmt.Sprintf("%s %d", c.labels.Months[int(month)-1], year)

	return tu.InlineKeyboardRow(
		c.navButton(c.labels.PrevMonth, calendarActionPrev, prevMonth),
		c.noopButton(title),
		c.navButton(c.labels.NextMonth, calendarActionNext, nextMonth),
	)
}

func (c *Calendar[T]) renderWeekdays() []telego.InlineKeyboardButton {
	row := make([]telego.InlineKeyboardButton, 0, daysPerWeek)
	for i := range daysPerWeek {
		weekday := time.Weekday((int(c.weekStart) + i) % daysPerWeek)
		row = append(row, c.noopButton(c.labels.Weekdays[int(weekday)]))
	}
	return row
}

func (c *Calendar[T]) renderDays(
	ctx *tf.Context[T],
	data *T,
	first time.Time,
	year int,
	month time.Month,
) ([][]telego.InlineKeyboardButton, error) {
	current := c.currentDate(data)
	daysInMonth := daysIn(year, month, c.location)
	firstOffset := (int(first.Weekday()) - int(c.weekStart) + daysPerWeek) % daysPerWeek

	rows := make([][]telego.InlineKeyboardButton, 0, weeksInMonth(daysInMonth, firstOffset))
	day := 1
	for day <= daysInMonth {
		row, nextDay, err := c.renderWeek(ctx, year, month, day, daysInMonth, firstOffset, current)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		day = nextDay
		firstOffset = 0
	}
	return rows, nil
}

func (c *Calendar[T]) currentDate(data *T) dateSelection {
	if c.current == nil {
		return dateSelection{}
	}
	date, ok := c.current(data)
	return dateSelection{date: date, ok: ok}
}

func (c *Calendar[T]) renderWeek(
	ctx *tf.Context[T],
	year int,
	month time.Month,
	day int,
	daysInMonth int,
	firstOffset int,
	current dateSelection,
) ([]telego.InlineKeyboardButton, int, error) {
	row := make([]telego.InlineKeyboardButton, 0, daysPerWeek)
	for col := range daysPerWeek {
		if col < firstOffset || day > daysInMonth {
			row = append(row, c.noopButton(c.labels.EmptyDay))
			continue
		}

		button, err := c.renderDayButton(ctx, year, month, day, current)
		if err != nil {
			return nil, 0, err
		}
		row = append(row, button)
		day++
	}
	return row, day, nil
}

func (c *Calendar[T]) renderDayButton(
	ctx *tf.Context[T],
	year int,
	month time.Month,
	day int,
	current dateSelection,
) (telego.InlineKeyboardButton, error) {
	date := Date{Year: year, Month: month, Day: day}
	label := strconv.Itoa(day)
	if current.ok && date.equal(current.date) {
		label = fmt.Sprintf(c.labels.SelectedDay, label)
	}

	switch {
	case !c.dateInRange(date):
		return c.noopButton(c.labels.EmptyDay), nil
	case !c.dateAllowed(ctx, date):
		return c.noopButton(label), nil
	default:
		return c.dateButton(label, date)
	}
}

func (c *Calendar[T]) renderFooter(ctx *tf.Context[T]) ([]telego.InlineKeyboardButton, error) {
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
	return footer, nil
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
	data, err := widget.EncodeCallback(c.id, calendarActionNoop)
	if err != nil {
		return tu.InlineKeyboardButton(label)
	}
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
	last := Date{Year: year, Month: month, Day: daysIn(year, month, c.location)}
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

func daysIn(year int, month time.Month, loc *time.Location) int {
	firstOfNextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, loc)
	return firstOfNextMonth.AddDate(0, 0, -1).Day()
}

func weeksInMonth(daysInMonth, firstOffset int) int {
	return (daysInMonth + firstOffset + daysPerWeek - 1) / daysPerWeek
}

func parseCalendarMonth(value string) (int, time.Month, bool) {
	if len(value) != calendarMonthDigits {
		return 0, 0, false
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return 0, 0, false
	}
	monthInt, err := strconv.Atoi(value[4:])
	if err != nil || monthInt < 1 || monthInt > monthsPerYear {
		return 0, 0, false
	}
	return year, time.Month(monthInt), true
}

func formatCalendarDate(date Date) string {
	return fmt.Sprintf("%04d%02d%02d", date.Year, int(date.Month), date.Day)
}

func parseCalendarDate(value string) (Date, bool) {
	if len(value) != calendarDateDigits {
		return Date{}, false
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil {
		return Date{}, false
	}
	monthInt, err := strconv.Atoi(value[4:6])
	if err != nil || monthInt < 1 || monthInt > monthsPerYear {
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
