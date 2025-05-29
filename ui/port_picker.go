package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type NumberPicker struct {
	entry      *widget.Entry
	btnUp      *widget.Button
	btnDown    *widget.Button
	current    int
	defaultVal int
	minVal     int
	maxVal     int
}

var gui fyne.CanvasObject

// NewPortPicker creates a NumberPicker specifically for port selection (1-65535)
func NewPortPicker(name string, initialPort int) *NumberPicker {
	return NewNumberPicker(name, initialPort, 1, 65535)
}

func NewNumberPicker(name string, initialVal, minVal, maxVal int) *NumberPicker {
	p := &NumberPicker{
		current:    initialVal,
		defaultVal: initialVal,
		minVal:     minVal,
		maxVal:     maxVal,
	}

	p.entry = widget.NewEntry()
	p.entry.SetText(strconv.Itoa(initialVal))
	p.entry.SetPlaceHolder(fmt.Sprintf("%s. default: %d (range: %d-%d)", name, initialVal, minVal, maxVal))
	p.entry.Validator = func(s string) error {
		val, err := strconv.Atoi(s)
		if err != nil || val < p.minVal || val > p.maxVal {
			return fmt.Errorf("invalid number, must be between %d and %d", p.minVal, p.maxVal)
		}
		return nil
	}

	updateEntry := func() {
		p.entry.SetText(strconv.Itoa(p.current))
	}

	// Create smaller buttons with custom size
	p.btnUp = widget.NewButton("▲", func() {
		if p.current < p.maxVal {
			p.current++
			updateEntry()
		}
	})
	p.btnUp.Importance = widget.LowImportance

	p.btnDown = widget.NewButton("▼", func() {
		if p.current > p.minVal {
			p.current--
			updateEntry()
		}
	})
	p.btnDown.Importance = widget.LowImportance

	p.entry.OnChanged = func(s string) {
		if val, err := strconv.Atoi(s); err == nil {
			p.current = val
		}
	}

	return p
}

func (p *NumberPicker) GetValue() int {
	if p.current == 0 {
		return p.defaultVal
	}
	return p.current
}

func (p *NumberPicker) GetUI() fyne.CanvasObject {
	if gui == nil {
		// Create a horizontal layout for the buttons
		buttons := container.NewHBox(
			p.btnUp,
			p.btnDown,
		)

		// Create a container that combines the entry and buttons
		gui = container.NewBorder(
			nil, nil, nil, buttons,
			p.entry,
		)

		// Set minimum size for the entry to make it more compact
		p.entry.Resize(fyne.NewSize(100, p.entry.MinSize().Height))
	}
	return gui
}

func (p *NumberPicker) Disable() {
	p.entry.Disable()
	p.btnUp.Disable()
	p.btnDown.Disable()
}

func (p *NumberPicker) Enable() {
	p.entry.Enable()
	p.btnUp.Enable()
	p.btnDown.Enable()
}
