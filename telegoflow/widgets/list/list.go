package list

import (
	"errors"
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

	defaultPageSize       = 8
	defaultColumns        = 1
	paginationRows        = 1
	confirmRows           = 1
	pageIndexCallbackArgs = 2
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
		pageSize: defaultPageSize,
		columns:  defaultColumns,
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
func (b *Builder[T, Item]) MultiSelect(
	selectedKeys SelectedKeysFunc[T],
	onToggle ToggleHandler[T, Item],
) *Builder[T, Item] {
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
	if err := b.validate(); err != nil {
		return nil, err
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

func (b *Builder[T, Item]) validate() error {
	if err := widget.ValidateID(b.id); err != nil {
		return err
	}
	if b.items == nil && b.pageItems == nil {
		return errors.New("list: items or page items function is required")
	}
	if b.label == nil && b.button == nil {
		return errors.New("list: label or button function is required")
	}
	if b.pageSize <= 0 {
		return errors.New("list: page size must be positive")
	}
	if b.columns <= 0 {
		return errors.New("list: columns must be positive")
	}
	if !b.multiSelect {
		return b.validateSingleSelect()
	}
	return b.validateMultiSelect()
}

func (b *Builder[T, Item]) validateSingleSelect() error {
	if b.onSelect == nil {
		return errors.New("list: select handler is required")
	}
	return nil
}

func (b *Builder[T, Item]) validateMultiSelect() error {
	if b.key == nil {
		return errors.New("list: key function is required for multi-select")
	}
	if b.selectedKeys == nil {
		return errors.New("list: selected keys function is required for multi-select")
	}
	if b.onToggle == nil {
		return errors.New("list: toggle handler is required for multi-select")
	}
	if b.onConfirm == nil {
		return errors.New("list: confirm handler is required for multi-select")
	}
	return nil
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

type itemAddressMode int

const (
	itemAddressDefault itemAddressMode = iota
	itemAddressByPageIndex
)

type loadedPage[Item any] struct {
	items      []Item
	pageCount  int
	hasNext    bool
	totalKnown bool
	page       int
}

type callbackItem[Item any] struct {
	item  Item
	page  int
	found bool
}

type itemRenderState struct {
	selected bool
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
	case actionPage:
		return l.handlePage(ctx, query, callback.Args)
	case actionSelect:
		return l.handleSelect(ctx, query, callback.Args)
	case actionConfirm:
		return l.onConfirm(ctx)
	default:
		return nil
	}
}

func (l *List[T, Item]) handlePage(ctx *tf.Context[T], query *telego.CallbackQuery, args []string) error {
	if len(args) != defaultColumns {
		return nil
	}
	page, ok := parsePageArg(args[0])
	if !ok {
		return nil
	}
	markup, err := l.renderPage(ctx, page)
	if err != nil {
		return err
	}
	return editCallbackView(ctx, query, markup)
}

func (l *List[T, Item]) handleSelect(ctx *tf.Context[T], query *telego.CallbackQuery, args []string) error {
	callbackItem, err := l.itemFromCallback(ctx, args)
	if err != nil {
		return err
	}
	if !callbackItem.found {
		return nil
	}
	if !l.multiSelect {
		return l.onSelect(ctx, callbackItem.item)
	}

	selected := !l.itemSelected(ctx.Data(), callbackItem.item)
	if err = l.onToggle(ctx, callbackItem.item, selected); err != nil {
		return err
	}
	markup, err := l.renderPage(ctx, callbackItem.page)
	if err != nil {
		return err
	}
	return editCallbackView(ctx, query, markup)
}

func editCallbackView[T any](
	ctx *tf.Context[T],
	query *telego.CallbackQuery,
	markup *telego.InlineKeyboardMarkup,
) error {
	return widget.EditCallbackView(ctx, widget.View{
		Text:   widget.CallbackMessageText(query, "List"),
		Markup: markup,
	})
}

func (l *List[T, Item]) renderPage(ctx *tf.Context[T], page int) (*telego.InlineKeyboardMarkup, error) {
	if page < 0 {
		page = 0
	}
	addressMode := itemAddressDefault
	if l.pageItems != nil {
		addressMode = itemAddressByPageIndex
	}
	return l.renderLoadedPage(ctx, page, addressMode)
}

func (l *List[T, Item]) renderLoadedPage(
	ctx *tf.Context[T],
	page int,
	addressMode itemAddressMode,
) (*telego.InlineKeyboardMarkup, error) {
	loaded, err := l.loadPage(ctx, page)
	if err != nil {
		return nil, err
	}
	if len(loaded.items) == 0 && loaded.page == 0 {
		return tu.InlineKeyboard(tu.InlineKeyboardRow(l.noopButton(l.labels.Empty))), nil
	}

	selected := l.selectedSet(ctx.Data())
	rows := make([][]telego.InlineKeyboardButton, 0, l.rowCapacity(len(loaded.items)))
	row := make([]telego.InlineKeyboardButton, 0, l.columns)
	for i, item := range loaded.items {
		button, err := l.itemButton(item, loaded.page, i, selected, addressMode)
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
	rows = append(rows, l.paginationRow(loaded))
	if l.multiSelect {
		button, err := l.confirmButton()
		if err != nil {
			return nil, err
		}
		rows = append(rows, tu.InlineKeyboardRow(button))
	}
	return tu.InlineKeyboard(rows...), nil
}

func (l *List[T, Item]) rowCapacity(items int) int {
	extraRows := paginationRows
	if l.multiSelect {
		extraRows += confirmRows
	}
	return (items+l.columns-1)/l.columns + extraRows
}

func (l *List[T, Item]) loadPage(ctx *tf.Context[T], page int) (loadedPage[Item], error) {
	if l.pageItems != nil {
		return l.loadServerPage(ctx, page)
	}
	return l.loadLocalPage(ctx, page)
}

func (l *List[T, Item]) loadServerPage(ctx *tf.Context[T], page int) (loadedPage[Item], error) {
	request := PageRequest{Page: page, Offset: page * l.pageSize, Limit: l.pageSize}
	result, err := l.pageItems(ctx, request)
	if err != nil {
		return loadedPage[Item]{page: page}, err
	}
	if result.Total == nil {
		return loadedPage[Item]{items: result.Items, hasNext: result.HasNext, page: page}, nil
	}

	pages := pageCount(*result.Total, l.pageSize)
	if pages > 0 && page >= pages {
		page = pages - 1
		request = PageRequest{Page: page, Offset: page * l.pageSize, Limit: l.pageSize}
		result, err = l.pageItems(ctx, request)
		if err != nil {
			return loadedPage[Item]{page: page}, err
		}
	}
	return loadedPage[Item]{
		items:      result.Items,
		pageCount:  pages,
		hasNext:    page+1 < pages,
		totalKnown: true,
		page:       page,
	}, nil
}

func (l *List[T, Item]) loadLocalPage(ctx *tf.Context[T], page int) (loadedPage[Item], error) {
	items, err := l.items(ctx)
	if err != nil {
		return loadedPage[Item]{page: page}, err
	}
	pages := pageCount(len(items), l.pageSize)
	if pages == 0 {
		return loadedPage[Item]{totalKnown: true}, nil
	}
	if page >= pages {
		page = pages - 1
	}
	start := page * l.pageSize
	end := min(start+l.pageSize, len(items))
	return loadedPage[Item]{
		items:      items[start:end],
		pageCount:  pages,
		hasNext:    page+1 < pages,
		totalKnown: true,
		page:       page,
	}, nil
}

func (l *List[T, Item]) itemButton(
	item Item,
	page int,
	pageIndex int,
	selected map[string]struct{},
	addressMode itemAddressMode,
) (telego.InlineKeyboardButton, error) {
	itemKey := ""
	if l.key != nil {
		itemKey = l.key(item)
	}
	_, isSelected := selected[itemKey]

	button := l.renderItemButton(item, itemRenderState{selected: isSelected})
	data, err := l.itemCallbackData(itemKey, page, pageIndex, addressMode)
	if err != nil {
		return telego.InlineKeyboardButton{}, err
	}
	button.CallbackData = data
	return button, nil
}

func (l *List[T, Item]) renderItemButton(item Item, state itemRenderState) telego.InlineKeyboardButton {
	if l.button != nil {
		return l.button(item, state.selected)
	}
	label := l.label(item)
	if state.selected {
		label = fmt.Sprintf(l.labels.Selected, label)
	}
	return tu.InlineKeyboardButton(label)
}

func (l *List[T, Item]) itemCallbackData(
	itemKey string,
	page int,
	pageIndex int,
	addressMode itemAddressMode,
) (string, error) {
	if l.key != nil && addressMode == itemAddressDefault && !l.multiSelect {
		return widget.EncodeCallback(l.id, actionSelect, itemKey)
	}
	return widget.EncodeCallback(l.id, actionSelect, strconv.Itoa(page), strconv.Itoa(pageIndex))
}

func (l *List[T, Item]) paginationRow(loaded loadedPage[Item]) []telego.InlineKeyboardButton {
	prev := l.noopButton(l.labels.Prev)
	if loaded.page > 0 {
		prev = l.pageButton(l.labels.Prev, loaded.page-1)
	}
	next := l.noopButton(l.labels.Next)
	if loaded.hasNext {
		next = l.pageButton(l.labels.Next, loaded.page+1)
	}

	pageLabel := fmt.Sprintf(l.labels.PageUnknown, loaded.page+1)
	if loaded.totalKnown {
		pageLabel = fmt.Sprintf(l.labels.Page, loaded.page+1, loaded.pageCount)
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
	data, err := widget.EncodeCallback(l.id, actionNoop)
	if err != nil {
		return tu.InlineKeyboardButton(label)
	}
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

func (l *List[T, Item]) itemFromCallback(
	ctx *tf.Context[T],
	args []string,
) (callbackItem[Item], error) {
	if l.pageItems != nil {
		return l.pagedItemFromCallback(ctx, args)
	}

	items, err := l.items(ctx)
	if err != nil {
		return callbackItem[Item]{}, err
	}
	if l.key != nil && !l.multiSelect {
		return l.itemByKey(items, args), nil
	}

	page, pageIndex, ok := parsePageIndex(args, l.pageSize)
	if !ok {
		return callbackItem[Item]{}, nil
	}
	index := page*l.pageSize + pageIndex
	if index < 0 || index >= len(items) {
		return callbackItem[Item]{}, nil
	}
	return callbackItem[Item]{item: items[index], page: page, found: true}, nil
}

func (l *List[T, Item]) itemByKey(items []Item, args []string) callbackItem[Item] {
	if len(args) != defaultColumns {
		return callbackItem[Item]{}
	}
	for _, item := range items {
		if l.key(item) == args[0] {
			return callbackItem[Item]{item: item, found: true}
		}
	}
	return callbackItem[Item]{}
}

func (l *List[T, Item]) pagedItemFromCallback(
	ctx *tf.Context[T],
	args []string,
) (callbackItem[Item], error) {
	page, pageIndex, ok := parsePageIndex(args, l.pageSize)
	if !ok {
		return callbackItem[Item]{}, nil
	}
	request := PageRequest{Page: page, Offset: page * l.pageSize, Limit: l.pageSize}
	result, err := l.pageItems(ctx, request)
	if err != nil {
		return callbackItem[Item]{}, err
	}
	if pageIndex >= len(result.Items) {
		return callbackItem[Item]{}, nil
	}
	return callbackItem[Item]{item: result.Items[pageIndex], page: page, found: true}, nil
}

func parsePageIndex(args []string, pageSize int) (page int, pageIndex int, ok bool) {
	if len(args) != pageIndexCallbackArgs {
		return 0, 0, false
	}
	page, ok = parsePageArg(args[0])
	if !ok {
		return 0, 0, false
	}
	pageIndex, ok = parsePageArg(args[1])
	if !ok || pageIndex >= pageSize {
		return 0, 0, false
	}
	return page, pageIndex, true
}

func parsePageArg(value string) (int, bool) {
	page, convErr := strconv.Atoi(value)
	if convErr != nil || page < 0 {
		return 0, false
	}
	return page, true
}

func pageCount(items, pageSize int) int {
	if items == 0 {
		return 0
	}
	return (items + pageSize - 1) / pageSize
}
