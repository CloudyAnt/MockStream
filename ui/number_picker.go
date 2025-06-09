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
	name       string
	gui        fyne.CanvasObject
}

// NewPortPicker creates a NumberPicker specifically for port selection (1-65535)
func NewPortPicker(name string, initialPort int) *NumberPicker {
	return NewNumberPicker(name, initialPort, 1, 65535, true)
}

func NewNumberPicker(name string, initialVal, minVal, maxVal int, withUpDown bool) *NumberPicker {
	p := &NumberPicker{
		current:    initialVal,
		defaultVal: initialVal,
		minVal:     minVal,
		maxVal:     maxVal,
		name:       name,
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

	p.btnUp = nil
	p.btnDown = nil
	if withUpDown {
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
	}

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
	if p.gui == nil {
		// Create a horizontal layout for the buttons
		buttons := container.NewHBox()
		if p.btnUp != nil {
			buttons.Add(p.btnUp)
		}
		if p.btnDown != nil {
			buttons.Add(p.btnDown)
		}

		// Create a label for the title
		title := widget.NewLabel(p.name + ":")
		title.TextStyle = fyne.TextStyle{Bold: true}

		// Create a container that combines the title, entry and buttons
		p.gui = container.NewBorder(
			nil, nil, title, buttons,
			p.entry,
		)

		// Calculate minimum width based on the maximum value's digit count
		// Each digit needs about 8-10 pixels, plus some padding
		digitCount := len(strconv.Itoa(p.maxVal))
		minWidth := float32(digitCount*10 + 20) // 10 pixels per digit + 20 pixels padding for the buttons
		fmt.Println("minWidth", minWidth)
		p.gui.Resize(fyne.NewSize(minWidth*2, p.gui.MinSize().Height))
	}
	return p.gui
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
