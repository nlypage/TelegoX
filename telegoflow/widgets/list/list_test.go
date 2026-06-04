package list

import (
	"testing"

	tf "github.com/mymmrac/telego/telegoflow"
)

type testData struct {
	Selected string
}

type testItem struct {
	ID   string
	Name string
}

func testItems(_ *tf.Context[testData]) ([]testItem, error) {
	return []testItem{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Beta"},
		{ID: "c", Name: "Gamma"},
	}, nil
}

func TestListRenderPageWithKeys(t *testing.T) {
	list, err := New[testData, testItem]("cities").
		Items(testItems).
		Key(func(item testItem) string { return item.ID }).
		Label(func(item testItem) string { return item.Name }).
		OnSelect(nopSelect).
		PageSize(2).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := list.renderPage(fakeListContext(), 0)
	if err != nil {
		t.Fatalf("renderPage() error = %v", err)
	}
	if len(markup.InlineKeyboard) != 3 {
		t.Fatalf("rows = %d; want 3", len(markup.InlineKeyboard))
	}
	if got := markup.InlineKeyboard[0][0].CallbackData; got != "tfw:cities:s:a" {
		t.Fatalf("first callback = %q", got)
	}
	if got := markup.InlineKeyboard[1][0].Text; got != "Beta" {
		t.Fatalf("second text = %q; want Beta", got)
	}
	if got := markup.InlineKeyboard[2][2].CallbackData; got != "tfw:cities:p:1" {
		t.Fatalf("next callback = %q", got)
	}
}

func TestListRenderPageWithIndexCallbacks(t *testing.T) {
	list, err := New[testData, testItem]("items").
		Items(testItems).
		Label(func(item testItem) string { return item.Name }).
		OnSelect(nopSelect).
		PageSize(2).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := list.renderPage(fakeListContext(), 1)
	if err != nil {
		t.Fatalf("renderPage() error = %v", err)
	}
	if got := markup.InlineKeyboard[0][0].CallbackData; got != "tfw:items:s:1:0" {
		t.Fatalf("page index callback = %q", got)
	}
	if got := markup.InlineKeyboard[1][0].CallbackData; got != "tfw:items:p:0" {
		t.Fatalf("prev callback = %q", got)
	}
}

func TestListSelectedLabelUsesLabels(t *testing.T) {
	list, err := New[testData, testItem]("cities").
		Items(testItems).
		Key(func(item testItem) string { return item.ID }).
		Label(func(item testItem) string { return item.Name }).
		Selected(func(_ *testData) string { return "b" }).
		Labels(Labels{Selected: "[%s]"}).
		OnSelect(nopSelect).
		PageSize(2).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := list.renderPage(fakeListContext(), 0)
	if err != nil {
		t.Fatalf("renderPage() error = %v", err)
	}
	if got := markup.InlineKeyboard[1][0].Text; got != "[Beta]" {
		t.Fatalf("selected label = %q; want [Beta]", got)
	}
}

func TestListMultiSelectRender(t *testing.T) {
	list, err := New[testData, testItem]("multi").
		Items(testItems).
		Key(func(item testItem) string { return item.ID }).
		Label(func(item testItem) string { return item.Name }).
		MultiSelect(
			func(_ *testData) []string { return []string{"b"} },
			func(_ *tf.Context[testData], _ testItem, _ bool) error { return nil },
		).
		OnConfirm(func(_ *tf.Context[testData]) error { return nil }).
		PageSize(2).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := list.renderPage(fakeListContext(), 0)
	if err != nil {
		t.Fatalf("renderPage() error = %v", err)
	}
	if got := markup.InlineKeyboard[0][0].CallbackData; got != "tfw:multi:s:0:0" {
		t.Fatalf("multi-select callback = %q", got)
	}
	if got := markup.InlineKeyboard[1][0].Text; got != "✓ Beta" {
		t.Fatalf("selected text = %q; want ✓ Beta", got)
	}
	if got := markup.InlineKeyboard[3][0].CallbackData; got != "tfw:multi:c" {
		t.Fatalf("confirm callback = %q", got)
	}
}

func TestListEmpty(t *testing.T) {
	list, err := New[testData, testItem]("empty").
		Items(func(_ *tf.Context[testData]) ([]testItem, error) { return nil, nil }).
		Label(func(item testItem) string { return item.Name }).
		OnSelect(nopSelect).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := list.renderPage(fakeListContext(), 0)
	if err != nil {
		t.Fatalf("renderPage() error = %v", err)
	}
	if got := markup.InlineKeyboard[0][0].Text; got != "No items" {
		t.Fatalf("empty text = %q", got)
	}
}

func TestListRenderPageItemsWithTotal(t *testing.T) {
	var gotRequest PageRequest
	list, err := New[testData, testItem]("db").
		PageItems(func(_ *tf.Context[testData], page PageRequest) (PageResult[testItem], error) {
			gotRequest = page
			return PageWithTotal([]testItem{{ID: "c", Name: "Gamma"}}, 3), nil
		}).
		Key(func(item testItem) string { return item.ID }).
		Label(func(item testItem) string { return item.Name }).
		OnSelect(nopSelect).
		PageSize(2).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := list.renderPage(fakeListContext(), 1)
	if err != nil {
		t.Fatalf("renderPage() error = %v", err)
	}
	if gotRequest != (PageRequest{Page: 1, Offset: 2, Limit: 2}) {
		t.Fatalf("page request = %+v", gotRequest)
	}
	if got := markup.InlineKeyboard[0][0].CallbackData; got != "tfw:db:s:1:0" {
		t.Fatalf("server paged select callback = %q", got)
	}
	if got := markup.InlineKeyboard[1][1].Text; got != "2/2" {
		t.Fatalf("page label = %q", got)
	}
}

func TestListMultiSelectRequiresConfirm(t *testing.T) {
	_, err := New[testData, testItem]("bad").
		Items(testItems).
		Key(func(item testItem) string { return item.ID }).
		Label(func(item testItem) string { return item.Name }).
		MultiSelect(
			func(_ *testData) []string { return nil },
			func(_ *tf.Context[testData], _ testItem, _ bool) error { return nil },
		).
		Build()
	if err == nil {
		t.Fatal("Build() error = nil; want error")
	}
}

func TestListRenderPageItemsWithoutTotal(t *testing.T) {
	list, err := New[testData, testItem]("cursor").
		PageItems(func(_ *tf.Context[testData], page PageRequest) (PageResult[testItem], error) {
			return Page([]testItem{{ID: "a", Name: "Alpha"}}, page.Page == 0), nil
		}).
		Label(func(item testItem) string { return item.Name }).
		OnSelect(nopSelect).
		PageSize(1).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	markup, err := list.renderPage(fakeListContext(), 0)
	if err != nil {
		t.Fatalf("renderPage() error = %v", err)
	}
	if got := markup.InlineKeyboard[1][1].Text; got != "1" {
		t.Fatalf("unknown total page label = %q", got)
	}
	if got := markup.InlineKeyboard[1][2].CallbackData; got != "tfw:cursor:p:1" {
		t.Fatalf("next callback = %q", got)
	}
}

func nopSelect(_ *tf.Context[testData], _ testItem) error { return nil }

func fakeListContext() *tf.Context[testData] {
	return &tf.Context[testData]{}
}
