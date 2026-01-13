package main

import (
	"fmt"
	"strings"
)

func (m *model) renderQueueContent(width, height int) string {
	if height <= 0 {
		return ""
	}

	var result strings.Builder

	// Count active jobs (deduplicated)
	totalJobs := len(m.jobs)
	for _, op := range m.printOps {
		hasSystemJob := false
		if op.SystemJobID != "" {
			for _, job := range m.jobs {
				if job.ID == op.SystemJobID {
					hasSystemJob = true
					break
				}
			}
		}
		if !hasSystemJob && op.Status != StatusSent {
			totalJobs++
		}
	}

	// Printer header with status
	printer := getDefaultPrinter()
	printerName := printerNameStyle.Render(fmt.Sprintf("🖨  %s", printer.Name))
	var statusStyled string
	if printer.Status == "idle" || printer.Status == "" {
		statusStyled = printerStatusIdleStyle.Render(fmt.Sprintf(" - %s", printer.Status))
	} else {
		statusStyled = printerStatusActiveStyle.Render(fmt.Sprintf(" - %s", printer.Status))
	}
	result.WriteString(printerName + statusStyled)
	result.WriteString("\n")

	// Calculate available height for scrollable sections
	// Overhead: printer (1) + active header (1) + staged header (1) = 3 fixed lines
	fixedOverhead := 3
	availableHeight := height - fixedOverhead

	if availableHeight <= 0 {
		return result.String()
	}

	// Dynamic height allocation
	var activeScrollHeight, stagedScrollHeight int
	relativeStagedFiles := m.getRelativeStagedFiles()

	if totalJobs == 0 {
		activeScrollHeight = 1
		stagedScrollHeight = availableHeight - activeScrollHeight
		if stagedScrollHeight < 1 {
			stagedScrollHeight = 1
		}
	} else {
		activeJobLines := totalJobs
		maxActiveHeight := availableHeight / 2
		if maxActiveHeight < 3 {
			maxActiveHeight = 3
		}
		if activeJobLines <= maxActiveHeight {
			activeScrollHeight = activeJobLines
		} else {
			activeScrollHeight = maxActiveHeight
		}
		stagedScrollHeight = availableHeight - activeScrollHeight
		if activeScrollHeight < 1 {
			activeScrollHeight = 1
		}
		if stagedScrollHeight < 1 {
			stagedScrollHeight = 1
		}
	}

	// Tree characters
	treeBranch := treeStyle.Render("├─ ")
	treeVert := treeStyle.Render("│")
	treeLast := treeStyle.Render("└─ ")

	// Active section header
	activeHeader := fmt.Sprintf("📄 Active (%d)", totalJobs)
	result.WriteString(treeBranch + activeHeaderStyle.Render(activeHeader))
	result.WriteString("\n")

	// Build active jobs content
	printOpsByJobID := make(map[string]*PrintOperation)
	for i := range m.printOps {
		op := &m.printOps[i]
		if op.SystemJobID != "" {
			printOpsByJobID[op.SystemJobID] = op
		}
	}
	shownOpIDs := make(map[string]bool)

	var activeContent strings.Builder
	if totalJobs == 0 {
		activeContent.WriteString(treeVert + dimStyle.Render("     · No active jobs"))
	} else {
		itemIndex := 0

		for _, job := range m.jobs {
			isCursor := itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive

			var statusSymbol string
			var statusStyle = normalStyle
			fileName := job.FileName

			if op, exists := printOpsByJobID[job.ID]; exists {
				shownOpIDs[op.ID] = true
				switch op.Status {
				case StatusPending:
					statusSymbol = "⏳"
					statusStyle = dimStyle
				case StatusSending:
					statusSymbol = "📤"
					statusStyle = normalStyle
				case StatusSent:
					statusSymbol = "●"
					statusStyle = normalStyle
				case StatusFailed:
					statusSymbol = "✗"
					statusStyle = errorStyle
				case StatusCanceled:
					statusSymbol = "⊘"
					statusStyle = dimStyle
				}
				if op.FileName != "" {
					fileName = op.FileName
				}
			} else {
				statusSymbol = "●"
			}

			maxNameLen := width - 15
			if maxNameLen > 0 && len(fileName) > maxNameLen {
				fileName = fileName[:maxNameLen-3] + "..."
			}

			content := fmt.Sprintf("%s %s", statusSymbol, fileName)
			activeContent.WriteString(treeVert + renderSelectable(isCursor, 5, content, selectedFileStyle, statusStyle))

			if itemIndex < totalJobs-1 {
				activeContent.WriteString("\n")
			}
			itemIndex++
		}

		for _, op := range m.printOps {
			if shownOpIDs[op.ID] {
				continue
			}
			if op.SystemJobID != "" {
				hasSystemJob := false
				for _, job := range m.jobs {
					if job.ID == op.SystemJobID {
						hasSystemJob = true
						break
					}
				}
				if hasSystemJob {
					continue
				}
			}
			if op.Status == StatusSent || op.Status == StatusCanceled {
				continue
			}

			isCursor := itemIndex == m.activeCursor && m.activePane == PaneQueue && m.queueSection == SectionActive

			var statusSymbol string
			var statusStyle = normalStyle
			switch op.Status {
			case StatusPending:
				statusSymbol = "⏳"
				statusStyle = dimStyle
			case StatusSending:
				statusSymbol = "📤"
				statusStyle = normalStyle
			case StatusFailed:
				statusSymbol = "✗"
				statusStyle = errorStyle
			}

			fileName := op.FileName
			maxNameLen := width - 15
			if maxNameLen > 0 && len(fileName) > maxNameLen {
				fileName = fileName[:maxNameLen-3] + "..."
			}

			content := fmt.Sprintf("%s %s", statusSymbol, fileName)
			activeContent.WriteString(treeVert + renderSelectable(isCursor, 5, content, selectedFileStyle, statusStyle))

			if itemIndex < totalJobs-1 {
				activeContent.WriteString("\n")
			}
			itemIndex++
		}
	}

	activeScroll := NewScrollableArea(width, activeScrollHeight)
	activeScroll.SetContent(activeContent.String())
	if m.queueSection == SectionActive && totalJobs > 0 {
		activeScroll.ScrollToLine(m.activeCursor)
	}
	result.WriteString(activeScroll.Render())
	result.WriteString("\n")

	// Staged section header
	stagedHeader := fmt.Sprintf("📋 Staged (%d)", len(relativeStagedFiles))
	result.WriteString(treeLast + stagedHeaderStyle.Render(stagedHeader))
	result.WriteString("\n")

	// Build staged files content
	var stagedContent strings.Builder
	if len(relativeStagedFiles) == 0 {
		stagedContent.WriteString(dimStyle.Render("      · No staged files"))
	} else {
		for i, file := range relativeStagedFiles {
			isCursor := i == m.stagedCursor && m.activePane == PaneQueue && m.queueSection == SectionStaged

			fileName := m.formatStagedFileName(file)
			maxNameLen := width - 14 // Extra space for copy indicator
			if maxNameLen > 0 && len(fileName) > maxNameLen {
				fileName = fileName[:maxNameLen-3] + "..."
			}

			// Show ? for pending remove, ×N for multiple copies, ◉ for single
			var indicator string
			var style = printableStyle
			if file.PendingRemove {
				indicator = "?"
				style = errorStyle
			} else if file.Copies > 1 {
				indicator = fmt.Sprintf("×%d", file.Copies)
			} else {
				indicator = "◉"
			}

			content := fmt.Sprintf("%s %s", indicator, fileName)
			stagedContent.WriteString(renderSelectable(isCursor, 6, content, selectedFileStyle, style))

			if i < len(relativeStagedFiles)-1 {
				stagedContent.WriteString("\n")
			}
		}
	}

	stagedScroll := NewScrollableArea(width, stagedScrollHeight)
	stagedScroll.SetContent(stagedContent.String())
	if m.queueSection == SectionStaged && len(relativeStagedFiles) > 0 {
		stagedScroll.ScrollToLine(m.stagedCursor)
	}
	result.WriteString(stagedScroll.Render())

	return result.String()
}
