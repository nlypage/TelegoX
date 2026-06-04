package widget

import (
	"context"
	"strings"
	"testing"

	"github.com/mymmrac/telego"
)

func TestEncodeDecodeCallback(t *testing.T) {
	got, err := EncodeCallback("calendar", "d", "20260602")
	if err != nil {
		t.Fatalf("EncodeCallback() error = %v", err)
	}
	if got != "tfw:calendar:d:20260602" {
		t.Fatalf("EncodeCallback() = %q; want %q", got, "tfw:calendar:d:20260602")
	}

	callback, ok := DecodeCallback(got)
	if !ok {
		t.Fatal("DecodeCallback() did not decode valid callback")
	}
	if callback.WidgetID != "calendar" || callback.Action != "d" {
		t.Fatalf("DecodeCallback() = %+v", callback)
	}
	if len(callback.Args) != 1 || callback.Args[0] != "20260602" {
		t.Fatalf("DecodeCallback() args = %v", callback.Args)
	}
}

func TestEncodeCallbackValidation(t *testing.T) {
	tests := []struct {
		name     string
		widgetID string
		action   string
		args     []string
	}{
		{name: "empty widget", widgetID: "", action: "d"},
		{name: "bad widget char", widgetID: "bad:id", action: "d"},
		{name: "empty action", widgetID: "calendar", action: ""},
		{name: "bad arg", widgetID: "calendar", action: "d", args: []string{"bad:arg"}},
		{name: "too long", widgetID: strings.Repeat("a", 60), action: "d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := EncodeCallback(tt.widgetID, tt.action, tt.args...); err == nil {
				t.Fatal("EncodeCallback() error = nil; want error")
			}
		})
	}
}

func TestCallbackPredicate(t *testing.T) {
	data, err := EncodeCallback("list", "p", "2")
	if err != nil {
		t.Fatalf("EncodeCallback() error = %v", err)
	}
	predicate := CallbackPredicate("list")
	if !predicate(context.Background(), telego.Update{CallbackQuery: &telego.CallbackQuery{Data: data}}) {
		t.Fatal("CallbackPredicate() = false; want true")
	}
	if predicate(context.Background(), telego.Update{CallbackQuery: &telego.CallbackQuery{Data: "other"}}) {
		t.Fatal("CallbackPredicate() = true; want false")
	}
}
