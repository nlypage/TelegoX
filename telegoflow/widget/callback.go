package widget

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/nlypage/telegox"
	th "github.com/nlypage/telegox/telegohandler"
)

const (
	callbackPrefix       = "tfw" // telegoflow widget
	callbackSeparator    = ":"
	callbackBaseParts    = 3
	maxCallbackDataBytes = 64
)

// Callback contains decoded widget callback data.
type Callback struct {
	WidgetID string
	Action   string
	Args     []string
}

// EncodeCallback encodes widget callback data and validates the Telegram 64-byte callback_data limit.
func EncodeCallback(widgetID, action string, args ...string) (string, error) {
	if err := ValidateID(widgetID); err != nil {
		return "", err
	}
	if err := validateSegment("action", action); err != nil {
		return "", err
	}
	for i, arg := range args {
		if err := validateSegment(fmt.Sprintf("arg %d", i), arg); err != nil {
			return "", err
		}
	}

	parts := make([]string, 0, callbackBaseParts+len(args))
	parts = append(parts, callbackPrefix, widgetID, action)
	parts = append(parts, args...)
	data := strings.Join(parts, callbackSeparator)
	if len([]byte(data)) > maxCallbackDataBytes {
		return "", fmt.Errorf(
			"widget: callback data is %d bytes, maximum is %d",
			len([]byte(data)), maxCallbackDataBytes,
		)
	}
	return data, nil
}

// DecodeCallback decodes widget callback data.
func DecodeCallback(data string) (Callback, bool) {
	parts := strings.Split(data, callbackSeparator)
	if len(parts) < 3 || parts[0] != callbackPrefix || parts[1] == "" || parts[2] == "" {
		return Callback{}, false
	}
	return Callback{WidgetID: parts[1], Action: parts[2], Args: parts[3:]}, true
}

// CallbackPredicate returns a telegohandler predicate that matches callback queries for a specific widget.
func CallbackPredicate(widgetID string) th.Predicate {
	return func(_ context.Context, update telego.Update) bool {
		query := update.CallbackQuery
		if query == nil {
			return false
		}
		callback, ok := DecodeCallback(query.Data)
		return ok && callback.WidgetID == widgetID
	}
}

// ValidateID validates a widget ID segment.
func ValidateID(id string) error {
	if err := validateSegment("widget ID", id); err != nil {
		return err
	}
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("widget: widget ID %q contains invalid character %q", id, r)
	}
	return nil
}

func validateSegment(name, value string) error {
	if value == "" {
		return fmt.Errorf("widget: %s must not be empty", name)
	}
	if strings.Contains(value, callbackSeparator) {
		return fmt.Errorf("widget: %s must not contain %q", name, callbackSeparator)
	}
	return nil
}
