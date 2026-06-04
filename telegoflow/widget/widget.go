package widget

import (
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
	tf "github.com/mymmrac/telego/telegoflow"
	tu "github.com/mymmrac/telego/telegoutil"
)

// TextFunc builds a message text for a widget render.
type TextFunc[T any] func(ctx *tf.Context[T]) (string, error)

// Text returns a static widget text function.
func Text[T any](text string) TextFunc[T] {
	return func(_ *tf.Context[T]) (string, error) {
		return text, nil
	}
}

// View contains text and inline markup to send or edit.
type View struct {
	Text   string
	Markup *telego.InlineKeyboardMarkup
}

// SendView sends a widget view as a new message.
func SendView[T any](ctx *tf.Context[T], view View) error {
	_, err := ctx.Bot().SendMessage(ctx, tu.Message(ctx.ChatID(), view.Text).WithReplyMarkup(view.Markup))
	return err
}

// EditCallbackView edits the callback query message with a widget view.
func EditCallbackView[T any](ctx *tf.Context[T], view View) error {
	query := ctx.CallbackQuery()
	if query == nil {
		return fmt.Errorf("widget: callback query is missing")
	}

	params := &telego.EditMessageTextParams{
		Text:        view.Text,
		ReplyMarkup: view.Markup,
	}
	if query.InlineMessageID != "" {
		params.InlineMessageID = query.InlineMessageID
	} else if query.Message != nil {
		params.ChatID = query.Message.GetChat().ChatID()
		params.MessageID = query.Message.GetMessageID()
	} else {
		return fmt.Errorf("widget: callback message is missing")
	}

	_, err := ctx.Bot().EditMessageText(ctx, params)
	if isMessageNotModified(err) {
		return nil
	}
	return err
}

// AnswerCallback answers the current callback query, if any.
func AnswerCallback[T any](ctx *tf.Context[T], text string) error {
	query := ctx.CallbackQuery()
	if query == nil {
		return nil
	}
	params := tu.CallbackQuery(query.ID)
	if text != "" {
		params.WithText(text)
	}
	return ctx.Bot().AnswerCallbackQuery(ctx, params)
}

// CallbackMessageText returns callback message text or fallback.
func CallbackMessageText(query *telego.CallbackQuery, fallback string) string {
	if query == nil || query.Message == nil || query.Message.Message() == nil || query.Message.Message().Text == "" {
		return fallback
	}
	return query.Message.Message().Text
}

func isMessageNotModified(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "message is not modified")
}
