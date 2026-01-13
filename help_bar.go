package main

import (
	"strings"
	"github.com/charmbracelet/lipgloss"
)

type HelpItem struct {
	Key    string
	Action string
	Global bool // Whether this shortcut is available globally
}

type HelpBar struct {
	width        int
	items        []HelpItem
	showFullHelp bool
	context      HelpContext
}

type HelpContext struct {
	ActivePane   ActivePane
	LayoutMode   LayoutMode
	FileFocus    FileFocus
	QueueSection QueueSection
}

// Define all shortcuts
var (
	globalShortcuts = []HelpItem{
		{Key: "P", Action: "print staged", Global: true},
		{Key: "X", Action: "clear staged", Global: true},
		{Key: "q", Action: "quit", Global: true},
	}

	queueActiveShortcuts = []HelpItem{
		{Key: "↑↓", Action: "navigate"},
		{Key: "x", Action: "cancel job"},
		{Key: "o", Action: "open file"},
		{Key: "tab", Action: "switch section"},
	}

	queueStagedShortcuts = []HelpItem{
		{Key: "↑↓", Action: "navigate"},
		{Key: "←→", Action: "copies"},
		{Key: "x", Action: "remove"},
		{Key: "o", Action: "open file"},
	}

	filesInputShortcuts = []HelpItem{
		{Key: "↓", Action: "to list"},
		{Key: "enter", Action: "apply pattern"},
		{Key: "esc", Action: "back"},
	}

	filesListShortcuts = []HelpItem{
		{Key: "↑↓", Action: "navigate"},
		{Key: "←→", Action: "dirs"},
		{Key: "space", Action: "mark"},
		{Key: "↑", Action: "to input"},
		{Key: "pgup/pgdn", Action: "page"},
	}

	printShortcuts = []HelpItem{
		{Key: "↑↓", Action: "navigate"},
		{Key: "r", Action: "retry failed"},
		{Key: "x", Action: "cancel/remove"},
		{Key: "X", Action: "clear completed"},
		{Key: "o", Action: "open file"},
	}

	splitViewShortcuts = []HelpItem{
		{Key: "tab", Action: "switch pane"},
	}

	// Styles for help items
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(theme.Lavender).
			Bold(true)

	helpActionStyle = lipgloss.NewStyle().
			Foreground(theme.Overlay0)

	helpSeparatorStyle = lipgloss.NewStyle().
			Foreground(theme.Surface2)

	helpIndicatorStyle = lipgloss.NewStyle().
			Foreground(theme.Yellow).
			Bold(true)

	// Full help window styles
	helpWindowStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Lavender).
			Padding(1, 2).
			Background(theme.Base)

	helpWindowTitleStyle = lipgloss.NewStyle().
			Foreground(theme.Lavender).
			Bold(true).
			MarginBottom(1)

	helpSectionStyle = lipgloss.NewStyle().
			Foreground(theme.Green).
			Bold(true).
			MarginTop(1)
)

func NewHelpBar(width int) *HelpBar {
	return &HelpBar{
		width:        width,
		items:        []HelpItem{},
		showFullHelp: false,
	}
}

// Update the help bar context and dimensions
func (h *HelpBar) Update(width int, activePane ActivePane, layoutMode LayoutMode, fileFocus FileFocus, queueSection QueueSection) {
	h.width = width
	h.context = HelpContext{
		ActivePane:   activePane,
		LayoutMode:   layoutMode,
		FileFocus:    fileFocus,
		QueueSection: queueSection,
	}
	h.buildItems()
}

// Handle key input - returns true if the key was handled by the help bar
func (h *HelpBar) HandleKey(key string) bool {
	if h.showFullHelp {
		switch key {
		case "?", "esc", "q":
			h.showFullHelp = false
			return true
		}
		// While help is showing, consume all other keys
		return true
	} else {
		// Check if user is pressing ? to open help
		if key == "?" {
			h.showFullHelp = true
			return true
		}
	}
	return false
}

// Build help items based on current context
func (h *HelpBar) buildItems() {
	h.items = []HelpItem{}

	// Always show global shortcuts first
	h.items = append(h.items, globalShortcuts...)

	// Add context-specific shortcuts
	if h.context.LayoutMode != LayoutSingle {
		h.items = append(h.items, splitViewShortcuts...)
	}

	switch h.context.ActivePane {
	case PaneQueue:
		if h.context.QueueSection == SectionStaged {
			h.items = append(h.items, queueStagedShortcuts...)
		} else {
			h.items = append(h.items, queueActiveShortcuts...)
		}
	case PaneFiles:
		if h.context.FileFocus == FocusInput {
			h.items = append(h.items, filesInputShortcuts...)
		} else {
			h.items = append(h.items, filesListShortcuts...)
		}
	}
}

// Render a single help item with colored key and dim action
func renderHelpItem(item HelpItem) string {
	key := helpKeyStyle.Render(item.Key)
	action := helpActionStyle.Render(item.Action)
	return key + helpSeparatorStyle.Render(": ") + action
}

// Calculate if help text fits and needs truncation
func (h *HelpBar) calculateFit() (string, bool) {
	fullText := ""
	separator := helpSeparatorStyle.Render(" • ")
	
	for i, item := range h.items {
		if i > 0 {
			fullText += separator
		}
		fullText += renderHelpItem(item)
	}

	// Account for the "?" indicator (4 chars: " [?]")
	availableWidth := h.width - 4
	fullWidth := lipgloss.Width(fullText)

	if fullWidth <= availableWidth {
		return fullText, false
	}

	// Need to truncate - try to fit as many complete items as possible
	truncated := ""
	currentWidth := 0
	maxWidth := availableWidth - 10 // Leave room for "..." and "[?]"

	for i, item := range h.items {
		itemText := renderHelpItem(item)
		itemWidth := lipgloss.Width(itemText)
		
		if i > 0 {
			itemWidth += lipgloss.Width(separator)
		}

		if currentWidth + itemWidth > maxWidth {
			if i > 0 {
				truncated += helpSeparatorStyle.Render("...")
			}
			break
		}

		if i > 0 {
			truncated += separator
		}
		truncated += itemText
		currentWidth += itemWidth
	}

	return truncated, true
}

// Check if full help is showing
func (h *HelpBar) IsShowingFullHelp() bool {
	return h.showFullHelp
}

// Render the help bar or full help overlay
func (h *HelpBar) Render() string {
	if h.showFullHelp {
		return h.RenderFullHelp()
	}
	
	helpText, truncated := h.calculateFit()

	if truncated {
		// Add the "?" indicator on the right
		indicator := helpIndicatorStyle.Render(" [?]")
		remainingWidth := h.width - lipgloss.Width(helpText) - lipgloss.Width(indicator)
		
		if remainingWidth > 0 {
			helpText += strings.Repeat(" ", remainingWidth)
		}
		helpText += indicator
	}

	return helpStyle.Copy().
		Width(h.width).
		Render(helpText)
}

// Render the full help window (floating overlay)
func (h *HelpBar) RenderFullHelp() string {
	var content strings.Builder

	content.WriteString(helpWindowTitleStyle.Render("Keyboard Shortcuts"))
	content.WriteString("\n\n")

	// Global shortcuts
	content.WriteString(helpSectionStyle.Render("Global"))
	content.WriteString("\n")
	for _, item := range globalShortcuts {
		content.WriteString("  ")
		content.WriteString(renderHelpItem(item))
		content.WriteString("\n")
	}

	// Queue pane shortcuts - Active section
	content.WriteString("\n")
	content.WriteString(helpSectionStyle.Render("Queue - Active Jobs"))
	content.WriteString("\n")
	for _, item := range queueActiveShortcuts {
		content.WriteString("  ")
		content.WriteString(renderHelpItem(item))
		content.WriteString("\n")
	}

	// Queue pane shortcuts - Staged section
	content.WriteString("\n")
	content.WriteString(helpSectionStyle.Render("Queue - Staged Files"))
	content.WriteString("\n")
	for _, item := range queueStagedShortcuts {
		content.WriteString("  ")
		content.WriteString(renderHelpItem(item))
		content.WriteString("\n")
	}

	// Files pane shortcuts
	content.WriteString("\n")
	content.WriteString(helpSectionStyle.Render("Files Pane - Input Mode"))
	content.WriteString("\n")
	for _, item := range filesInputShortcuts {
		content.WriteString("  ")
		content.WriteString(renderHelpItem(item))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(helpSectionStyle.Render("Files Pane - List Mode"))
	content.WriteString("\n")
	for _, item := range filesListShortcuts {
		content.WriteString("  ")
		content.WriteString(renderHelpItem(item))
		content.WriteString("\n")
	}

	// Print pane shortcuts
	content.WriteString("\n")
	content.WriteString(helpSectionStyle.Render("Print Pane"))
	content.WriteString("\n")
	for _, item := range printShortcuts {
		content.WriteString("  ")
		content.WriteString(renderHelpItem(item))
		content.WriteString("\n")
	}

	// Navigation
	content.WriteString("\n")
	content.WriteString(helpSectionStyle.Render("Layout"))
	content.WriteString("\n")
	for _, item := range splitViewShortcuts {
		content.WriteString("  ")
		content.WriteString(renderHelpItem(item))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(helpActionStyle.Render("Press ? or esc to close"))

	return helpWindowStyle.Render(content.String())
}