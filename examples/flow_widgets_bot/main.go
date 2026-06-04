package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nlypage/telegox"
	tf "github.com/nlypage/telegox/telegoflow"
	cal "github.com/nlypage/telegox/telegoflow/widgets/calendar"
	lst "github.com/nlypage/telegox/telegoflow/widgets/list"
	th "github.com/nlypage/telegox/telegohandler"
	tu "github.com/nlypage/telegox/telegoutil"
)

// BookingData is typed flow session data. Widgets don't own business state:
// OnSelect handlers write selected city/date here, and telegoflow persists it as JSON.
type BookingData struct {
	CityID   string   `json:"city_id"`
	CityName string   `json:"city_name"`
	Date     cal.Date `json:"date"`
	ExtraIDs []string `json:"extra_ids"`
}

type City struct {
	ID   string
	Name string
}

type Extra struct {
	ID   string
	Name string
}

var cities = []City{
	{ID: "nyc", Name: "New York"},
	{ID: "lon", Name: "London"},
	{ID: "ber", Name: "Berlin"},
	{ID: "par", Name: "Paris"},
	{ID: "tok", Name: "Tokyo"},
	{ID: "syd", Name: "Sydney"},
}

var extras = []Extra{
	{ID: "breakfast", Name: "Breakfast"},
	{ID: "transfer", Name: "Airport transfer"},
	{ID: "checkout", Name: "Late checkout"},
	{ID: "spa", Name: "Spa access"},
	{ID: "parking", Name: "Parking"},
}

func loadCitiesPage(page lst.PageRequest) lst.PageResult[City] {
	// In a real application this function would run something like:
	// SELECT id, name FROM cities ORDER BY name LIMIT page.Limit OFFSET page.Offset
	// and, if available, SELECT COUNT(*) FROM cities for PageWithTotal.
	start := min(page.Offset, len(cities))
	end := min(start+page.Limit, len(cities))
	return lst.PageWithTotal(cities[start:end], len(cities))
}

func loadExtrasPage(page lst.PageRequest) lst.PageResult[Extra] {
	start := min(page.Offset, len(extras))
	end := min(start+page.Limit, len(extras))
	return lst.PageWithTotal(extras[start:end], len(extras))
}

func toggleExtra(data *BookingData, extra Extra, selected bool) {
	for i, id := range data.ExtraIDs {
		if id != extra.ID {
			continue
		}
		if !selected {
			data.ExtraIDs = append(data.ExtraIDs[:i], data.ExtraIDs[i+1:]...)
		}
		return
	}
	if selected {
		data.ExtraIDs = append(data.ExtraIDs, extra.ID)
	}
}

func extraNames(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, extra := range extras {
			if extra.ID == id {
				names = append(names, extra.Name)
				break
			}
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func main() {
	ctx := context.Background()
	botToken := os.Getenv("TOKEN")

	bot, err := telego.NewBot(botToken, telego.WithDefaultDebugLogger())
	if err != nil {
		log.Fatalf("Create bot: %s", err)
	}

	updates, err := bot.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		log.Fatalf("Updates via long polling: %s", err)
	}

	bh, err := th.NewBotHandler(bot, updates)
	if err != nil {
		log.Fatalf("Create bot handler: %s", err)
	}
	defer func() { _ = bh.Stop() }()

	// Flow manager stores active sessions and routes updates to the current step.
	// Memory storage is fine for examples; use Redis/PostgreSQL storage in production.
	flows := tf.NewManager(tf.NewMemoryStorage())

	booking, err := newBookingFlow()
	if err != nil {
		log.Fatalf("Create booking flow: %s", err)
	}
	if err = flows.Register(booking); err != nil {
		log.Fatalf("Register booking flow: %s", err)
	}

	// Commands that must work inside a flow are registered before flow middleware.
	// In this example /start always restarts the widgets flow from the beginning.
	bh.Handle(booking.Start, th.CommandEqual("start"))

	bh.Handle(func(ctx *th.Context, update telego.Update) error {
		if err := flows.Cancel(ctx, update); err != nil {
			_, sendErr := ctx.Bot().SendMessage(ctx, tu.Message(update.Message.Chat.ChatID(), "There is no active flow"))
			return sendErr
		}
		_, err := ctx.Bot().SendMessage(ctx, tu.Message(update.Message.Chat.ChatID(), "Flow canceled"))
		return err
	}, th.CommandEqual("cancel"))

	// Active flow sessions are handled by this middleware. If no session exists,
	// processing continues to regular handlers below.
	bh.Use(flows.Middleware())

	bh.HandleMessage(func(ctx *th.Context, msg telego.Message) error {
		_, err := ctx.Bot().SendMessage(ctx, tu.Message(
			msg.Chat.ChatID(),
			"Use /start to open the calendar and list widgets example. /cancel cancels the active flow.",
		))
		return err
	})

	fmt.Println(booking.Graph())
	log.Println("Handling updates...")
	if err = bh.Start(); err != nil {
		log.Fatalf("Start bot handler: %s", err)
	}
}

func newBookingFlow() (*tf.Flow[BookingData], error) {
	// List widget can either load all items with Items(...) or load only the requested
	// page with PageItems(...). PageItems is the production-friendly option for DB-backed
	// lists: use PageRequest.Offset and PageRequest.Limit in SQL LIMIT/OFFSET queries.
	cityList, err := lst.New[BookingData, City]("city").
		PageItems(func(_ *tf.Context[BookingData], page lst.PageRequest) (lst.PageResult[City], error) {
			return loadCitiesPage(page), nil
		}).
		// Key is used for selected-state highlighting. With PageItems, selection callbacks
		// still use page/index internally, so the widget can resolve the clicked item by
		// reloading only that page instead of loading the whole list.
		Key(func(city City) string {
			return city.ID
		}).
		Label(func(city City) string {
			return city.Name
		}).
		Selected(func(data *BookingData) string {
			return data.CityID
		}).
		PageSize(4).
		// OnSelect is the bridge from widget UI state to business flow state.
		OnSelect(func(ctx *tf.Context[BookingData], city City) error {
			ctx.Data().CityID = city.ID
			ctx.Data().CityName = city.Name
			return ctx.Go("date")
		}).
		Build()
	if err != nil {
		return nil, err
	}

	// Calendar widget renders month navigation in callback_data and stores the final
	// current date only when OnSelect writes it to BookingData.
	calendar, err := cal.New[BookingData]("date").
		Labels(cal.EnglishLabels()).
		Min(cal.DateFromTime(time.Now())).
		Current(func(data *BookingData) (cal.Date, bool) {
			return data.Date, !data.Date.IsZero()
		}).
		OnSelect(func(ctx *tf.Context[BookingData], date cal.Date) error {
			ctx.Data().Date = date
			return ctx.Go("extras")
		}).
		OnCancel(func(ctx *tf.Context[BookingData]) error {
			return ctx.Cancel()
		}).
		Build()
	if err != nil {
		return nil, err
	}

	// MultiSelect mode toggles items in flow data and renders a confirmation button.
	// The widget itself doesn't keep selected IDs: selected keys come from BookingData.ExtraIDs.
	extraList, err := lst.New[BookingData, Extra]("extras").
		PageItems(func(_ *tf.Context[BookingData], page lst.PageRequest) (lst.PageResult[Extra], error) {
			return loadExtrasPage(page), nil
		}).
		Key(func(extra Extra) string {
			return extra.ID
		}).
		Label(func(extra Extra) string {
			return extra.Name
		}).
		Labels(lst.Labels{Confirm: "Continue", Selected: "[x] %s"}).
		PageSize(3).
		MultiSelect(
			func(data *BookingData) []string {
				return data.ExtraIDs
			},
			func(ctx *tf.Context[BookingData], extra Extra, selected bool) error {
				toggleExtra(ctx.Data(), extra, selected)
				return nil
			},
		).
		OnConfirm(func(ctx *tf.Context[BookingData]) error {
			return ctx.Go("confirm")
		}).
		Build()
	if err != nil {
		return nil, err
	}

	return tf.New[BookingData]("booking_widgets").
		Steps(
			// Step(...) is shorthand for:
			// tf.NewStep(...).Enter(widget.Send(...)).Handle(widget.Handle, widget.Predicate())
			cityList.Step("city", "Choose a city:").CanGo("date"),

			calendar.Step("date", "Choose a date:").CanGo("extras"),

			extraList.Step("extras", "Choose optional extras, then press Continue:").CanGo("confirm"),

			// The final step is a regular telegoflow step. Widgets are optional building blocks,
			// not a separate session mechanism, so custom handlers can be mixed in freely.
			tf.NewStep[BookingData]("confirm").
				Enter(func(ctx *tf.Context[BookingData]) error {
					data := ctx.Data()
					keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("Confirm").WithCallbackData("booking_confirm"),
						tu.InlineKeyboardButton("Start over").WithCallbackData("booking_restart"),
					))
					_, err := ctx.Bot().SendMessage(ctx, tu.Messagef(
						ctx.ChatID(),
						"City: %s\nDate: %s\nExtras: %s\n\nConfirm?",
						data.CityName,
						data.Date.Format("January 2, 2006"),
						extraNames(data.ExtraIDs),
					).WithReplyMarkup(keyboard))
					return err
				}).
				Handle(func(ctx *tf.Context[BookingData]) error {
					query := ctx.CallbackQuery()
					if query == nil {
						return nil
					}
					if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID)); err != nil {
						return err
					}

					switch query.Data {
					case "booking_confirm":
						return ctx.Complete()
					case "booking_restart":
						ctx.Data().CityID = ""
						ctx.Data().CityName = ""
						ctx.Data().Date = cal.Date{}
						ctx.Data().ExtraIDs = nil
						return ctx.Go("city")
					default:
						return nil
					}
				}, th.AnyCallbackQuery(), th.Or(
					th.CallbackDataEqual("booking_confirm"),
					th.CallbackDataEqual("booking_restart"),
				)).
				Fallback(func(ctx *tf.Context[BookingData]) error {
					_, err := ctx.Bot().SendMessage(ctx, tu.Message(ctx.ChatID(), "Please use the confirmation buttons"))
					return err
				}).
				CanGo("city").
				CanComplete(),
		).
		StartWith("city").
		OnComplete(func(ctx *tf.Context[BookingData]) error {
			_, err := ctx.Bot().SendMessage(ctx, tu.Message(ctx.ChatID(), "Booking confirmed ✅"))
			return err
		}).
		OnCancel(func(ctx *tf.Context[BookingData]) error {
			_, err := ctx.Bot().SendMessage(ctx, tu.Message(ctx.ChatID(), "Booking canceled"))
			return err
		}).
		Build()
}
