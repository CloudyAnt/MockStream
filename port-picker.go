package main

import (
	"fmt"
	"fyne.io/fyne/v2/layout"
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

	p.btnUp = widget.NewButton("↑", func() {
		if p.current < 65535 {
			p.current++
			updateEntry()
		}
	})

	p.btnDown = widget.NewButton("↓", func() {
		if p.current > 1 {
			p.current--
			updateEntry()
		}
	})

	p.entry.OnChanged = func(s string) {
		fmt.Println(s)
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
		buttons := container.NewVBox(
			p.btnUp,
			p.btnDown,
		)
		gui = container.New(
			layout.NewBorderLayout(nil, nil, nil, buttons),
			p.entry,
			buttons,
		)
	}
	return gui
}
