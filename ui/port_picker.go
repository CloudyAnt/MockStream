package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type PortPicker struct {
	entry       *widget.Entry
	btnUp       *widget.Button
	btnDown     *widget.Button
	current     int
	defaultPort int
}

var gui fyne.CanvasObject

func NewPortPicker(name string, initialPort int) *PortPicker {
	p := &PortPicker{current: initialPort, defaultPort: initialPort}

	p.entry = widget.NewEntry()
	p.entry.SetText(strconv.Itoa(initialPort))
	p.entry.SetPlaceHolder(name + ". default: " + strconv.Itoa(initialPort))
	p.entry.Validator = func(s string) error {
		val, err := strconv.Atoi(s)
		if err != nil || val < 1 || val > 65535 {
			return fmt.Errorf("invalid port")
		}
		return nil
	}

	updateEntry := func() {
		p.entry.SetText(strconv.Itoa(p.current))
	}

	// Create smaller buttons with custom size
	p.btnUp = widget.NewButton("▲", func() {
		if p.current < 65535 {
			p.current++
			updateEntry()
		}
	})
	p.btnUp.Importance = widget.LowImportance

	p.btnDown = widget.NewButton("▼", func() {
		if p.current > 1 {
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

func (p *PortPicker) GetPort() int {
	if p.current == 0 {
		return p.defaultPort
	}
	return p.current
}

func (p *PortPicker) GetUI() fyne.CanvasObject {
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

func (p *PortPicker) Disable() {
	p.entry.Disable()
	p.btnUp.Disable()
	p.btnDown.Disable()
}

func (p *PortPicker) Enable() {
	p.entry.Enable()
	p.btnUp.Enable()
	p.btnDown.Enable()
}
