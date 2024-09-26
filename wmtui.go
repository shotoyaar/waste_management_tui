package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	gloss "github.com/charmbracelet/lipgloss"
	_ "github.com/mattn/go-sqlite3"
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
	db        *sql.DB
	waste     []wasteItem
	cursor    int
	input     string
	inputmode inputmode
	err       error
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

	return model{
		db:        db,
		waste:     waste,
		inputmode: normal,
	}
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
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.inputmode {
		case normal:
			return m.updateNormal(msg)

		case addingName, addingWasteType, addingLocation, addingMethod:
			return m.updateAddingText(msg)

		case addingQuantity:
			return m.updateAddingQuantity(msg)
		}
	}

	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.waste)-1 {
			m.cursor++
		}

	case "a":
		m.inputmode = addingName
		m.input = ""

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

func (m model) updateAddingText(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		switch m.inputmode {
		case addingName:
			m.waste = append(m.waste, wasteItem{name: m.input})
			m.inputmode = addingQuantity

		case addingWasteType:
			m.waste[len(m.waste)-1].wasteType = m.input
			m.inputmode = addingLocation

		case addingLocation:
			m.waste[len(m.waste)-1].location = m.input
			m.inputmode = addingMethod

		case addingMethod:
			m.waste[len(m.waste)-1].method = m.input
			err := m.addWasteItem(m.waste[len(m.waste)-1])
			if err != nil {
				m.err = fmt.Errorf("failed to add item: %v", err)
			}

			m.inputmode = normal
		}

		m.input = ""

	case "esc":
		m.inputmode = normal
		m.input = ""

	default:
		m.input += msg.String()
	}

	return m, nil
}

func (m model) updateAddingQuantity(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		quantity, err := strconv.ParseFloat(m.input, 64)

		if err != nil {
			m.err = fmt.Errorf("invalid quantity entered: %v", err)
		} else {
			m.waste[len(m.waste)-1].quantity = quantity
			m.inputmode = addingWasteType
			m.err = nil
		}

		m.input = ""

	case "esc":
		m.inputmode = normal
		m.input = ""

	default:
		m.input += msg.String()
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
	s := "Waste Management System\n\n"

	// Define Styles
	titleStyle := gloss.NewStyle().
		Bold(true).
		Foreground(gloss.Color("#FAFAFA")).
		Background(gloss.Color("#7D56F4")).
		Padding(0, 1)

	selectedStyle := gloss.NewStyle().
		Foreground(gloss.Color("#FFFFFF")).
		Background(gloss.Color("#0000FF"))

	s += titleStyle.Render("Name | Type | Quantity | Location | Disposal Method") + "\n"

	for i, item := range m.waste {
		line := fmt.Sprintf("%-10s | %-10s | %-8.2f | %-10s | %-15s", item.name, item.wasteType, item.quantity, item.location, item.method)

		if m.cursor == i {
			s += selectedStyle.Render(line)
		} else {
			s += line
		}

		s += "\n"
	}

	s += "\n"

	if m.inputmode == normal {
		s += "Press (a) to add, (d) to delete, up/down to move, (q) to quit\n"
	} else {
		s += fmt.Sprintf("Enter %s: %s", getInputModeName(m.inputmode), m.input)

		if m.err != nil {
			s += fmt.Sprintf("\nError: %v", m.err)
		}
	}

	return s
}

func getInputModeName(mode inputmode) string {
	switch mode {
	case addingName:
		return "waste name"

	case addingQuantity:
		return "quantity"

	case addingWasteType:
		return "waste type"

	case addingLocation:
		return "location"

	case addingMethod:
		return "disposal method"

	default:
		return ""
	}
}

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
