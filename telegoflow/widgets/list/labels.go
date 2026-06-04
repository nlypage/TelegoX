package list

// Labels contains user-visible list button labels.
type Labels struct {
	Prev        string
	Next        string
	Page        string
	PageUnknown string
	Empty       string
	// Selected formats selected item labels. Use "%s" to hide the marker.
	Selected string
	Confirm  string
}

// EnglishLabels returns English list labels.
func EnglishLabels() Labels {
	return Labels{
		Prev:        "‹",
		Next:        "›",
		Page:        "%d/%d",
		PageUnknown: "%d",
		Empty:       "No items",
		Selected:    "✓ %s",
		Confirm:     "Done",
	}
}

// RussianLabels returns Russian list labels.
func RussianLabels() Labels {
	return Labels{
		Prev:        "‹",
		Next:        "›",
		Page:        "%d/%d",
		PageUnknown: "%d",
		Empty:       "Пусто",
		Selected:    "✓ %s",
		Confirm:     "Готово",
	}
}

func (l Labels) normalize() Labels {
	defaults := EnglishLabels()
	if l.Prev == "" {
		l.Prev = defaults.Prev
	}
	if l.Next == "" {
		l.Next = defaults.Next
	}
	if l.Page == "" {
		l.Page = defaults.Page
	}
	if l.PageUnknown == "" {
		l.PageUnknown = defaults.PageUnknown
	}
	if l.Empty == "" {
		l.Empty = defaults.Empty
	}
	if l.Selected == "" {
		l.Selected = defaults.Selected
	}
	if l.Confirm == "" {
		l.Confirm = defaults.Confirm
	}
	return l
}
