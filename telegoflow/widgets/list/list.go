package list

import (
	"fmt"
	"strconv"

	"github.com/mymmrac/telego"
	tf "github.com/mymmrac/telego/telegoflow"
	"github.com/mymmrac/telego/telegoflow/widget"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	actionNoop    = "noop"
	actionPage    = "p"
	actionSelect  = "s"
	actionConfirm = "c"
)

// ItemsFunc loads all list items for the current flow context.
//
// Use PageItemsFunc for large or database-backed lists that should load one page at a time.
type ItemsFunc[T any, Item any] func(ctx *tf.Context[T]) ([]Item, error)

// PageRequest describes which page the widget needs.
type PageRequest struct {
	// Page is zero-based page number.
	Page int
	// Offset is Page * Limit and can be passed directly to SQL OFFSET.
	Offset int
	// Limit is the configured List page size.
	Limit int
}

// PageResult contains one loaded list page.
type PageResult[Item any] struct {
	Items []Item
	// Total is optional. When set, the widget renders "current/total pages" and computes next page availability.
	Total *int
	// HasNext is used when Total is nil, for cursor-like or count-less pagination.
	HasNext bool
}

// Page returns a page result when total item count is unknown.
func Page[Item any](items []Item, hasNext bool) PageResult[Item] {
	return PageResult[Item]{Items: items, HasNext: hasNext}
}

// PageWithTotal returns a page result with total item count.
func PageWithTotal[Item any](items []Item, total int) PageResult[Item] {
	return PageResult[Item]{Items: items, Total: &total}
}

// PageItemsFunc loads only the requested page for the current flow context.
type PageItemsFunc[T any, Item any] func(ctx *tf.Context[T], page PageRequest) (PageResult[Item], error)

// KeyFunc returns a stable item key for callback_data and selected state.
type KeyFunc[Item any] func(item Item) string

// LabelFunc returns item button text.
type LabelFunc[Item any] func(item Item) string

// ButtonFunc builds a custom item button. CallbackData is overwritten by the list widget.
type ButtonFunc[Item any] func(item Item, selected bool) telego.InlineKeyboardButton

// SelectedKeysFunc returns selected item keys for multi-select mode.
type SelectedKeysFunc[T any] func(data *T) []string

// ToggleHandler handles multi-select item toggles. selected is the new selected state.
type ToggleHandler[T any, Item any] func(ctx *tf.Context[T], item Item, selected bool) error

// ConfirmHandler handles the multi-select confirmation button.
type ConfirmHandler[T any] func(ctx *tf.Context[T]) error

// Builder builds a List widget.
type Builder[T any, Item any] struct {
	id           string
	items        ItemsFunc[T, Item]
	pageItems    PageItemsFunc[T, Item]
	key          KeyFunc[Item]
	label        LabelFunc[Item]
	button       ButtonFunc[Item]
	selected     func(data *T) string
	selectedKeys SelectedKeysFunc[T]
	onSelect     func(ctx *tf.Context[T], item Item) error
	onToggle     ToggleHandler[T, Item]
	onConfirm    ConfirmHandler[T]
	multiSelect  bool
	labels       Labels
	pageSize     int
	columns      int
}

// New creates a list widget builder.
func New[T any, Item any](id string) *Builder[T, Item] {
	return &Builder[T, Item]{
		id:       id,
		labels:   EnglishLabels(),
		pageSize: 8,
		columns:  1,
	}
}

// Items sets an in-memory item source. The widget loads all items and paginates them locally.
func (b *Builder[T, Item]) Items(items ItemsFunc[T, Item]) *Builder[T, Item] {
	b.items = items
	b.pageItems = nil
	return b
}

// PageItems sets a paginated item source. Use it for database-backed lists.
//
// The callback receives PageRequest with Page, Offset, and Limit. Return PageWithTotal
// if your query also knows total count, or Page when only HasNext is known.
func (b *Builder[T, Item]) PageItems(items PageItemsFunc[T, Item]) *Builder[T, Item] {
	b.pageItems = items
	b.items = nil
	return b
}

// Key sets a stable item key for selected-state highlighting. With PageItems,
// selection callbacks use page/index addressing so the widget can resolve the selected item
// by reloading the clicked page instead of loading the whole data set.
// If omitted, callbacks use page/index addressing.
func (b *Builder[T, Item]) Key(key KeyFunc[Item]) *Builder[T, Item] {
	b.key = key
	return b
}

// Label sets item button labels. Required unless Button is set.
func (b *Builder[T, Item]) Label(label LabelFunc[Item]) *Builder[T, Item] {
	b.label = label
	return b
}

// Button sets custom item buttons. CallbackData is overwritten by the list widget.
func (b *Builder[T, Item]) Button(button ButtonFunc[Item]) *Builder[T, Item] {
	b.button = button
	return b
}

// Selected sets a function that returns the currently selected item key for single-select highlighting.
//
// It only controls rendering. Single-select clicks are handled by OnSelect.
func (b *Builder[T, Item]) Selected(selected func(data *T) string) *Builder[T, Item] {
	b.selected = selected
	return b
}

// MultiSelect enables multi-select mode.
//
// selectedKeys returns keys already selected in flow data. onToggle is called on each item click
// with selected set to the new state. Use OnConfirm to handle the confirmation button.
func (b *Builder[T, Item]) MultiSelect(selectedKeys SelectedKeysFunc[T], onToggle ToggleHandler[T, Item]) *Builder[T, Item] {
	b.multiSelect = true
	b.selectedKeys = selectedKeys
	b.onToggle = onToggle
	return b
}

// OnConfirm sets a handler called when the multi-select confirmation button is pressed.
func (b *Builder[T, Item]) OnConfirm(handler ConfirmHandler[T]) *Builder[T, Item] {
	b.onConfirm = handler
	return b
}

// PageSize sets number of items per page.
func (b *Builder[T, Item]) PageSize(size int) *Builder[T, Item] {
	b.pageSize = size
	return b
}

// Columns sets number of item columns.
func (b *Builder[T, Item]) Columns(columns int) *Builder[T, Item] {
	b.columns = columns
	return b
}

// Labels sets list labels.
func (b *Builder[T, Item]) Labels(labels Labels) *Builder[T, Item] {
	b.labels = labels
	return b
}

// OnSelect sets a handler called when user selects an item.
func (b *Builder[T, Item]) OnSelect(handler func(ctx *tf.Context[T], item Item) error) *Builder[T, Item] {
	b.onSelect = handler
	return b
}

// Build validates and creates a List widget.
func (b *Builder[T, Item]) Build() (*List[T, Item], error) {
	if err := widget.ValidateID(b.id); err != nil {
		return nil, err
	}
	if b.items == nil && b.pageItems == nil {
		return nil, fmt.Errorf("list: items or page items function is required")
	}
	if b.label == nil && b.button == nil {
		return nil, fmt.Errorf("list: label or button function is required")
	}
	if b.pageSize <= 0 {
		return nil, fmt.Errorf("list: page size must be positive")
	}
	if b.columns <= 0 {
		return nil, fmt.Errorf("list: columns must be positive")
	}
	if b.multiSelect {
		if b.key == nil {
			return nil, fmt.Errorf("list: key function is required for multi-select")
		}
		if b.selectedKeys == nil {
			return nil, fmt.Errorf("list: selected keys function is required for multi-select")
		}
		if b.onToggle == nil {
			return nil, fmt.Errorf("list: toggle handler is required for multi-select")
		}
		if b.onConfirm == nil {
			return nil, fmt.Errorf("list: confirm handler is required for multi-select")
		}
	} else if b.onSelect == nil {
		return nil, fmt.Errorf("list: select handler is required")
	}
	return &List[T, Item]{
		id:           b.id,
		items:        b.items,
		pageItems:    b.pageItems,
		key:          b.key,
		label:        b.label,
		button:       b.button,
		selected:     b.selected,
		selectedKeys: b.selectedKeys,
		onSelect:     b.onSelect,
		onToggle:     b.onToggle,
		onConfirm:    b.onConfirm,
		multiSelect:  b.multiSelect,
		labels:       b.labels.normalize(),
		pageSize:     b.pageSize,
		columns:      b.columns,
	}, nil
}

// List is a paginated inline list widget for telegoflow steps.
type List[T any, Item any] struct {
	id           string
	items        ItemsFunc[T, Item]
	pageItems    PageItemsFunc[T, Item]
	key          KeyFunc[Item]
	label        LabelFunc[Item]
	button       ButtonFunc[Item]
	selected     func(data *T) string
	selectedKeys SelectedKeysFunc[T]
	onSelect     func(ctx *tf.Context[T], item Item) error
	onToggle     ToggleHandler[T, Item]
	onConfirm    ConfirmHandler[T]
	multiSelect  bool
	labels       Labels
	pageSize     int
	columns      int
}

// ID returns widget ID.
func (l *List[T, Item]) ID() string { return l.id }

// Predicate returns a predicate that matches callback queries for this list.
func (l *List[T, Item]) Predicate() th.Predicate { return widget.CallbackPredicate(l.id) }

// Send returns an Enter handler that sends this list with static text.
func (l *List[T, Item]) Send(text string) tf.Handler[T] { return l.SendText(widget.Text[T](text)) }

// SendText returns an Enter handler that sends this list with dynamic text.
func (l *List[T, Item]) SendText(text widget.TextFunc[T]) tf.Handler[T] {
	return func(ctx *tf.Context[T]) error {
		message, err := text(ctx)
		if err != nil {
			return err
		}
		markup, err := l.Markup(ctx)
		if err != nil {
			return err
		}
		return widget.SendView(ctx, widget.View{Text: message, Markup: markup})
	}
}

// Step creates a telegoflow step that renders and handles this list.
func (l *List[T, Item]) Step(stepID, text string) *tf.Step[T] {
	return tf.NewStep[T](stepID).Enter(l.Send(text)).Handle(l.Handle, l.Predicate())
}

// Markup builds inline keyboard markup for the first page.
func (l *List[T, Item]) Markup(ctx *tf.Context[T]) (*telego.InlineKeyboardMarkup, error) {
	return l.renderPage(ctx, 0)
}

// Handle handles callback queries for this list.
func (l *List[T, Item]) Handle(ctx *tf.Context[T]) error {
	query := ctx.CallbackQuery()
	if query == nil {
		return nil
	}
	callback, ok := widget.DecodeCallback(query.Data)
	if !ok || callback.WidgetID != l.id {
		return nil
	}
	if err := widget.AnswerCallback(ctx, ""); err != nil {
		return err
	}

	switch callback.Action {
	case actionNoop:
		return nil
	case actionPage:
		if len(callback.Args) != 1 {
			return nil
		}
		page, err := strconv.Atoi(callback.Args[0])
		if err != nil || page < 0 {
			return nil
		}
		markup, err := l.renderPage(ctx, page)
		if err != nil {
			return err
		}
		return widget.EditCallbackView(ctx, widget.View{Text: widget.CallbackMessageText(query, "List"), Markup: markup})
	case actionSelect:
		item, page, ok, err := l.itemFromCallback(ctx, callback.Args)
		if err != nil || !ok {
			return err
		}
		if l.multiSelect {
			selected := !l.itemSelected(ctx.Data(), item)
			if err = l.onToggle(ctx, item, selected); err != nil {
				return err
			}
			markup, err := l.renderPage(ctx, page)
			if err != nil {
				return err
			}
			return widget.EditCallbackView(ctx, widget.View{Text: widget.CallbackMessageText(query, "List"), Markup: markup})
		}
		return l.onSelect(ctx, item)
	case actionConfirm:
		return l.onConfirm(ctx)
	default:
		return nil
	}
}

func (l *List[T, Item]) renderPage(ctx *tf.Context[T], page int) (*telego.InlineKeyboardMarkup, error) {
	if page < 0 {
		page = 0
	}
	if l.pageItems != nil {
		return l.renderLoadedPage(ctx, page, true)
	}
	return l.renderLoadedPage(ctx, page, false)
}

func (l *List[T, Item]) renderLoadedPage(ctx *tf.Context[T], page int, serverPaged bool) (*telego.InlineKeyboardMarkup, error) {
	items, pageCount, hasNext, totalKnown, page, err := l.loadPage(ctx, page)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && page == 0 {
		return tu.InlineKeyboard(tu.InlineKeyboardRow(l.noopButton(l.labels.Empty))), nil
	}

	selected := l.selectedSet(ctx.Data())

	rows := make([][]telego.InlineKeyboardButton, 0, (len(items)+l.columns-1)/l.columns+2)
	row := make([]telego.InlineKeyboardButton, 0, l.columns)
	for i, item := range items {
		button, err := l.itemButton(item, page, i, selected, serverPaged)
		if err != nil {
			return nil, err
		}
		row = append(row, button)
		if len(row) == l.columns {
			rows = append(rows, row)
			row = make([]telego.InlineKeyboardButton, 0, l.columns)
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, l.paginationRow(page, pageCount, hasNext, totalKnown))
	if l.multiSelect {
		button, err := l.confirmButton()
		if err != nil {
			return nil, err
		}
		rows = append(rows, tu.InlineKeyboardRow(button))
	}
	return tu.InlineKeyboard(rows...), nil
}

func (l *List[T, Item]) loadPage(ctx *tf.Context[T], page int) ([]Item, int, bool, bool, int, error) {
	if l.pageItems != nil {
		request := PageRequest{Page: page, Offset: page * l.pageSize, Limit: l.pageSize}
		result, err := l.pageItems(ctx, request)
		if err != nil {
			return nil, 0, false, false, page, err
		}
		if result.Total != nil {
			pages := pageCount(*result.Total, l.pageSize)
			if pages > 0 && page >= pages {
				page = pages - 1
				request = PageRequest{Page: page, Offset: page * l.pageSize, Limit: l.pageSize}
				result, err = l.pageItems(ctx, request)
				if err != nil {
					return nil, 0, false, false, page, err
				}
			}
			return result.Items, pages, page+1 < pages, true, page, nil
		}
		return result.Items, 0, result.HasNext, false, page, nil
	}

	items, err := l.items(ctx)
	if err != nil {
		return nil, 0, false, false, page, err
	}
	pages := pageCount(len(items), l.pageSize)
	if pages == 0 {
		return nil, 0, false, true, 0, nil
	}
	if page >= pages {
		page = pages - 1
	}
	start := page * l.pageSize
	end := min(start+l.pageSize, len(items))
	return items[start:end], pages, page+1 < pages, true, page, nil
}

func (l *List[T, Item]) itemButton(item Item, page, pageIndex int, selected map[string]struct{}, forcePageIndex bool) (telego.InlineKeyboardButton, error) {
	itemKey := ""
	if l.key != nil {
		itemKey = l.key(item)
	}
	_, isSelected := selected[itemKey]

	var button telego.InlineKeyboardButton
	if l.button != nil {
		button = l.button(item, isSelected)
	} else {
		label := l.label(item)
		if isSelected {
			label = fmt.Sprintf(l.labels.Selected, label)
		}
		button = tu.InlineKeyboardButton(label)
	}

	var data string
	var err error
	if l.key != nil && !forcePageIndex && !l.multiSelect {
		data, err = widget.EncodeCallback(l.id, actionSelect, itemKey)
	} else {
		data, err = widget.EncodeCallback(l.id, actionSelect, strconv.Itoa(page), strconv.Itoa(pageIndex))
	}
	if err != nil {
		return telego.InlineKeyboardButton{}, err
	}
	button.CallbackData = data
	return button, nil
}

func (l *List[T, Item]) paginationRow(page, pageCount int, hasNext, totalKnown bool) []telego.InlineKeyboardButton {
	prev := l.noopButton(l.labels.Prev)
	if page > 0 {
		prev = l.pageButton(l.labels.Prev, page-1)
	}
	next := l.noopButton(l.labels.Next)
	if hasNext {
		next = l.pageButton(l.labels.Next, page+1)
	}

	pageLabel := fmt.Sprintf(l.labels.PageUnknown, page+1)
	if totalKnown {
		pageLabel = fmt.Sprintf(l.labels.Page, page+1, pageCount)
	}
	return tu.InlineKeyboardRow(prev, l.noopButton(pageLabel), next)
}

func (l *List[T, Item]) confirmButton() (telego.InlineKeyboardButton, error) {
	data, err := widget.EncodeCallback(l.id, actionConfirm)
	if err != nil {
		return telego.InlineKeyboardButton{}, err
	}
	return tu.InlineKeyboardButton(l.labels.Confirm).WithCallbackData(data), nil
}

func (l *List[T, Item]) pageButton(label string, page int) telego.InlineKeyboardButton {
	data, err := widget.EncodeCallback(l.id, actionPage, strconv.Itoa(page))
	if err != nil {
		return l.noopButton(label)
	}
	return tu.InlineKeyboardButton(label).WithCallbackData(data)
}

func (l *List[T, Item]) noopButton(label string) telego.InlineKeyboardButton {
	data, _ := widget.EncodeCallback(l.id, actionNoop)
	return tu.InlineKeyboardButton(label).WithCallbackData(data)
}

func (l *List[T, Item]) selectedSet(data *T) map[string]struct{} {
	selected := map[string]struct{}{}
	if l.multiSelect {
		for _, key := range l.selectedKeys(data) {
			selected[key] = struct{}{}
		}
		return selected
	}
	if l.selected != nil {
		if key := l.selected(data); key != "" {
			selected[key] = struct{}{}
		}
	}
	return selected
}

func (l *List[T, Item]) itemSelected(data *T, item Item) bool {
	if l.key == nil {
		return false
	}
	_, ok := l.selectedSet(data)[l.key(item)]
	return ok
}

func (l *List[T, Item]) itemFromCallback(ctx *tf.Context[T], args []string) (Item, int, bool, error) {
	var zero Item
	if l.pageItems != nil {
		return l.pagedItemFromCallback(ctx, args)
	}

	items, err := l.items(ctx)
	if err != nil {
		return zero, 0, false, err
	}
	if l.key != nil && !l.multiSelect {
		if len(args) != 1 {
			return zero, 0, false, nil
		}
		for _, item := range items {
			if l.key(item) == args[0] {
				return item, 0, true, nil
			}
		}
		return zero, 0, false, nil
	}

	page, pageIndex, ok := parsePageIndex(args, l.pageSize)
	if !ok {
		return zero, 0, false, nil
	}
	index := page*l.pageSize + pageIndex
	if index < 0 || index >= len(items) {
		return zero, 0, false, nil
	}
	return items[index], page, true, nil
}

func (l *List[T, Item]) pagedItemFromCallback(ctx *tf.Context[T], args []string) (Item, int, bool, error) {
	var zero Item
	page, pageIndex, ok := parsePageIndex(args, l.pageSize)
	if !ok {
		return zero, 0, false, nil
	}
	request := PageRequest{Page: page, Offset: page * l.pageSize, Limit: l.pageSize}
	result, err := l.pageItems(ctx, request)
	if err != nil {
		return zero, 0, false, err
	}
	if pageIndex >= len(result.Items) {
		return zero, 0, false, nil
	}
	return result.Items[pageIndex], page, true, nil
}

func parsePageIndex(args []string, pageSize int) (int, int, bool) {
	if len(args) != 2 {
		return 0, 0, false
	}
	page, err := strconv.Atoi(args[0])
	if err != nil || page < 0 {
		return 0, 0, false
	}
	pageIndex, err := strconv.Atoi(args[1])
	if err != nil || pageIndex < 0 || pageIndex >= pageSize {
		return 0, 0, false
	}
	return page, pageIndex, true
}

func pageCount(items, pageSize int) int {
	if items == 0 {
		return 0
	}
	return (items + pageSize - 1) / pageSize
}
