package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	_ "github.com/mattn/go-sqlite3"
)

var (
	focusedStyle        = gloss.NewStyle().Foreground(gloss.Color("205"))
	blurredStyle        = gloss.NewStyle().Foreground(gloss.Color("240"))
	cursorStyle         = focusedStyle
	noStyle             = gloss.NewStyle()
	helpStyle           = blurredStyle
	cursorModeHelpStyle = gloss.NewStyle().Foreground(gloss.Color("244"))

	focusedButton = focusedStyle.Render("[Submit]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))

	titleStyle = gloss.NewStyle().
			Bold(true).
			Foreground(gloss.Color("#FAFAFA")).
			Background(gloss.Color("#7D56F4")).
			Padding(0, 1)

	selectedStyle = gloss.NewStyle().
			Foreground(gloss.Color("#FFFFFF")).
			Background(gloss.Color("#0000FF"))

	errorStyle = gloss.NewStyle().Foreground(gloss.Color("9"))
)

type wasteItem struct {
	id        int
	name      string
	quantity  float64
	wasteType string
	location  string
	method    string
}

type model struct {
	db         *sql.DB
	waste      []wasteItem
	cursor     int
	inputs     []textinput.Model
	inputmode  inputmode
	err        error
	cursorMode cursor.Mode
	focusIndex int
}

type inputmode int

const (
	normal inputmode = iota
	addingName
	addingQuantity
	addingWasteType
	addingLocation
	addingMethod
)

func initialModel(db *sql.DB) model {
	waste, err := loadWasteItems(db)
	if err != nil {
		log.Fatalf("Error loading waste items: %v", err)
	}

	m := model{
		inputs:    make([]textinput.Model, 5),
		db:        db,
		waste:     waste,
		inputmode: normal,
	}

	var t textinput.Model

	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 64

		switch i {
		case 0:
			t.Placeholder = "Waste Name"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle

		case 1:
			t.Placeholder = "Waste Quantity"

		case 2:
			t.Placeholder = "Waste Type"

		case 3:
			t.Placeholder = "Waste Location"

		case 4:
			t.Placeholder = "Disposal Method"
		}

		m.inputs[i] = t
	}

	return m
}

func loadWasteItems(db *sql.DB) ([]wasteItem, error) {
	rows, err := db.Query("SELECT id, name, quantity, wasteType, location, method FROM waste_items")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var items []wasteItem

	for rows.Next() {
		var item wasteItem
		err := rows.Scan(&item.id, &item.name, &item.quantity, &item.wasteType, &item.location, &item.method)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.inputmode {
		case normal:
			return m.updateNormal(msg)
		case addingName, addingWasteType, addingLocation, addingMethod, addingQuantity:
			return m.updateAdding(msg)
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "ctrl+r":
		m.cursorMode++

		if m.cursorMode > cursor.CursorHide {
			m.cursorMode = cursor.CursorBlink
		}

		cmds := make([]tea.Cmd, len(m.inputs))
		for i := range m.inputs {
			cmds[i] = m.inputs[i].Cursor.SetMode(m.cursorMode)
		}

		return m, tea.Batch(cmds...)

	case "tab", "shift+tab", "up", "down":
		s := msg.String()

		if s == "enter" && m.focusIndex == len(m.inputs) {
			return m, tea.Quit
		}

		if s == "up" || s == "shift+tab" {
			m.focusIndex--
		} else {
			m.focusIndex++
		}

		if m.focusIndex > len(m.inputs) {
			m.focusIndex = 0
		} else if m.focusIndex < 0 {
			m.focusIndex = len(m.inputs)
		}

		cmds := make([]tea.Cmd, len(m.inputs))
		for i := 0; i <= len(m.inputs)-1; i++ {
			if i == m.focusIndex {
				cmds[i] = m.inputs[i].Focus()
				m.inputs[i].PromptStyle = focusedStyle
				m.inputs[i].TextStyle = focusedStyle
				continue
			}

			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = noStyle
			m.inputs[i].TextStyle = noStyle
		}

		return m, tea.Batch(cmds...)

	case "a":
		m.inputmode = addingName
		m.focusIndex = 0
		return m, m.inputs[0].Focus()

	case "d":
		if len(m.waste) > 0 {
			err := m.deleteWasteItem(m.waste[m.cursor].id)

			if err != nil {
				m.err = fmt.Errorf("failed to delete item: %v", err)
			} else {
				m.waste = append(m.waste[:m.cursor], m.waste[m.cursor+1:]...)

				if m.cursor >= len(m.waste) {
					m.cursor = len(m.waste) - 1
				}
			}
		}
	}

	return m, nil
}

func (m model) updateAdding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.focusIndex < len(m.inputs)-1 {
			m.focusIndex++
			return m, m.inputs[m.focusIndex].Focus()
		} else {
			return m.submitWasteItem()
		}

	case "esc":
		m.inputmode = normal
		m.focusIndex = 0
		return m, nil
	}
	return m, nil
}

func (m model) submitWasteItem() (tea.Model, tea.Cmd) {
	quantity, err := strconv.ParseFloat(m.inputs[1].Value(), 64)
	if err != nil {
		m.err = fmt.Errorf("invalid quantity: %v", err)
		return m, nil
	}

	newItem := wasteItem{
		name:      m.inputs[0].Value(),
		quantity:  quantity,
		wasteType: m.inputs[2].Value(),
		location:  m.inputs[3].Value(),
		method:    m.inputs[4].Value(),
	}

	err = m.addWasteItem(newItem)
	if err != nil {
		m.err = fmt.Errorf("failed to add item: %v", err)
	} else {
		m.inputmode = normal

		for i := range m.inputs {
			m.inputs[i].SetValue("")
		}

		m.focusIndex = 0
	}

	return m, nil
}

func (m model) addWasteItem(item wasteItem) error {
	result, err := m.db.Exec("INSERT INTO waste_items (name, quantity, wasteType, location, method) VALUES (?, ?, ?, ?, ?)",
		item.name, item.quantity, item.wasteType, item.location, item.method)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	item.id = int(id)
	m.waste[len(m.waste)-1] = item

	return nil
}

func (m model) deleteWasteItem(id int) error {
	_, err := m.db.Exec("DELETE FROM waste_items WHERE id = ?", id)
	return err
}

func (m model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("Waste Management System"))
	b.WriteString("\n\n")

	// Waste Items Table
	if len(m.waste) > 0 {
		b.WriteString(titleStyle.Render("Current Waste Items"))
		b.WriteString("\n")
		b.WriteString(titleStyle.Render("Name | Type | Quantity | Location | Disposal Method"))
		b.WriteString("\n")

		for i, item := range m.waste {
			line := fmt.Sprintf("%-10s | %-10s | %-8.2f | %-10s | %-15s",
				item.name, item.wasteType, item.quantity, item.location, item.method)

			if m.cursor == i && m.inputmode == normal {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Input Fields
	if m.inputmode != normal {
		b.WriteString(titleStyle.Render("Add New Waste Item"))
		b.WriteString("\n")

		for i := range m.inputs {
			b.WriteString(m.inputs[i].View())
			if i < len(m.inputs)-1 {
				b.WriteRune('\n')
			}
		}

		button := &blurredButton
		if m.focusIndex == len(m.inputs) {
			button = &focusedButton
		}
		fmt.Fprintf(&b, "\n\n%s\n\n", *button)
	}

	// Help Text
	b.WriteString(helpStyle.Render("cursor mode is "))
	b.WriteString(cursorModeHelpStyle.Render(m.cursorMode.String()))
	b.WriteString(helpStyle.Render(" (ctrl+r to change style)"))
	b.WriteString("\n")

	// Instructions
	if m.inputmode == normal {
		b.WriteString(helpStyle.Render("Press (a) to add, (d) to delete, up/down to move, (q) to quit"))
	} else {
		b.WriteString(helpStyle.Render("Press (enter) to move to next field, (esc) to cancel"))
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return b.String()
}

// func getInputModeName(mode inputmode) string {
// 	switch mode {
// 	case addingName:
// 		return "waste name"

// 	case addingQuantity:
// 		return "quantity"

// 	case addingWasteType:
// 		return "waste type"

// 	case addingLocation:
// 		return "location"

// 	case addingMethod:
// 		return "disposal method"

// 	default:
// 		return ""
// 	}
// }

func main() {
	db, err := sql.Open("sqlite3", "./waste_management.db")
	if err != nil {
		log.Fatalf("error opening database: %v", err)
	}

	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS waste_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		quantity REAL,
		wasteType TEXT,
		location TEXT,
		method TEXT
	)`)
	if err != nil {
		log.Fatalf("error creating table: %v", err)
	}

	p := tea.NewProgram(initialModel(db))

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
	}
}
