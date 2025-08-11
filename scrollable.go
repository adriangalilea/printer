package main

import (
	"strings"
	"github.com/charmbracelet/lipgloss"
)

// ScrollableArea represents a scrollable content area with optional scrollbar
type ScrollableArea struct {
	content      []string // Lines of content
	width        int      // Available width (including scrollbar)
	height       int      // Visible height
	scrollOffset int      // Current scroll position
	showScrollbar bool    // Whether content needs scrollbar
}

// NewScrollableArea creates a new scrollable area
func NewScrollableArea(width, height int) *ScrollableArea {
	return &ScrollableArea{
		width:  width,
		height: height,
		content: []string{},
		scrollOffset: 0,
	}
}

// SetContent sets the content and determines if scrollbar is needed
func (s *ScrollableArea) SetContent(content string) {
	s.content = strings.Split(content, "\n")
	s.showScrollbar = len(s.content) > s.height
	
	// Reset scroll if content changed significantly
	if s.scrollOffset >= len(s.content) {
		s.scrollOffset = 0
	}
}

// ScrollUp scrolls the content up by n lines
func (s *ScrollableArea) ScrollUp(n int) {
	s.scrollOffset -= n
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
}

// ScrollDown scrolls the content down by n lines
func (s *ScrollableArea) ScrollDown(n int) {
	maxOffset := len(s.content) - s.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	
	s.scrollOffset += n
	if s.scrollOffset > maxOffset {
		s.scrollOffset = maxOffset
	}
}

// ScrollToTop scrolls to the beginning
func (s *ScrollableArea) ScrollToTop() {
	s.scrollOffset = 0
}

// ScrollToBottom scrolls to the end
func (s *ScrollableArea) ScrollToBottom() {
	maxOffset := len(s.content) - s.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	s.scrollOffset = maxOffset
}

// ScrollToLine scrolls to make a specific line visible
func (s *ScrollableArea) ScrollToLine(line int) {
	if line < s.scrollOffset {
		s.scrollOffset = line
	} else if line >= s.scrollOffset + s.height {
		s.scrollOffset = line - s.height + 1
	}
	
	// Ensure bounds
	maxOffset := len(s.content) - s.height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.scrollOffset > maxOffset {
		s.scrollOffset = maxOffset
	}
	if s.scrollOffset < 0 {
		s.scrollOffset = 0
	}
}

// Render returns the visible portion of content with scrollbar
func (s *ScrollableArea) Render() string {
	if len(s.content) == 0 {
		return ""
	}
	
	// Calculate visible range
	start := s.scrollOffset
	end := s.scrollOffset + s.height
	if end > len(s.content) {
		end = len(s.content)
	}
	
	// Calculate content width (accounting for scrollbar)
	contentWidth := s.width
	if s.showScrollbar {
		contentWidth = s.width - 2 // Reserve 2 chars for scrollbar
	}
	
	// Build the visible content
	var result strings.Builder
	visibleLines := end - start
	
	for i := 0; i < s.height; i++ {
		if i < visibleLines {
			line := s.content[start+i]
			
			// Truncate or pad line to fit width
			if len(line) > contentWidth {
				line = line[:contentWidth-3] + "..."
			} else {
				// Pad to full width
				line = line + strings.Repeat(" ", contentWidth-lipgloss.Width(line))
			}
			
			// Add scrollbar if needed
			if s.showScrollbar {
				scrollChar := s.getScrollbarChar(i)
				line = line + scrollChar
			}
			
			result.WriteString(line)
		} else {
			// Empty line (for when content is shorter than height)
			if s.showScrollbar {
				line := strings.Repeat(" ", contentWidth) + s.getScrollbarChar(i)
				result.WriteString(line)
			} else {
				result.WriteString(strings.Repeat(" ", s.width))
			}
		}
		
		if i < s.height-1 {
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

// getScrollbarChar returns the appropriate scrollbar character for a given line
func (s *ScrollableArea) getScrollbarChar(lineIndex int) string {
	if !s.showScrollbar {
		return ""
	}
	
	totalLines := len(s.content)
	visibleLines := s.height
	canScrollUp := s.scrollOffset > 0
	canScrollDown := s.scrollOffset+visibleLines < totalLines
	
	// First, handle arrow positions - always show them, but dim when inactive
	if lineIndex == 0 {
		if canScrollUp {
			return dimStyle.Render(" ▲")
		} else {
			// Show dimmed/inactive up arrow
			return dimStyle.Copy().Foreground(lipgloss.Color("#333333")).Render(" ▲")
		}
	}
	if lineIndex == s.height-1 {
		if canScrollDown {
			return dimStyle.Render(" ▼")
		} else {
			// Show dimmed/inactive down arrow
			return dimStyle.Copy().Foreground(lipgloss.Color("#333333")).Render(" ▼")
		}
	}
	
	// For very small heights, just show track
	if s.height <= 4 {
		return dimStyle.Render(" ░")
	}
	
	// Now determine the actual track area for the thumb
	// The thumb can NEVER appear on arrow lines
	// Since arrows are ALWAYS present, track always starts at line 1 and ends at height-1
	trackStart := 1  // Line 0 is always the up arrow
	trackEnd := s.height - 1  // Last line is always the down arrow
	
	// Validate we have space for a track
	if trackStart >= trackEnd {
		return dimStyle.Render(" ░")
	}
	
	trackHeight := trackEnd - trackStart
	
	// Calculate thumb size proportional to visible vs total content
	thumbSize := max(1, (trackHeight * visibleLines) / totalLines)
	if thumbSize > trackHeight {
		thumbSize = trackHeight
	}
	
	// Calculate thumb position
	maxScrollOffset := totalLines - visibleLines
	if maxScrollOffset <= 0 {
		// Content fits entirely - show full track as thumb
		if lineIndex >= trackStart && lineIndex < trackEnd {
			return dimStyle.Render(" █")
		}
		return dimStyle.Render(" ░")
	}
	
	// Calculate where the thumb should be positioned within the track
	// When scrollOffset = 0, thumb is at trackStart
	// When scrollOffset = maxScrollOffset, thumb is at (trackEnd - thumbSize)
	scrollRatio := float64(s.scrollOffset) / float64(maxScrollOffset)
	maxThumbPos := trackEnd - thumbSize
	thumbPos := trackStart + int(scrollRatio * float64(maxThumbPos - trackStart))
	
	// Double-check bounds
	if thumbPos < trackStart {
		thumbPos = trackStart
	}
	if thumbPos + thumbSize > trackEnd {
		thumbPos = trackEnd - thumbSize
	}
	
	// Render the appropriate character for this line
	// CRITICAL: Only render thumb if we're actually within the track area
	if lineIndex >= trackStart && lineIndex < trackEnd {
		// We're in the track area, check if this is thumb or track
		if lineIndex >= thumbPos && lineIndex < thumbPos + thumbSize {
			return dimStyle.Render(" █")
		}
	}
	
	// Default to track (for all non-thumb positions, including outside track area)
	return dimStyle.Render(" ░")
}

// GetScrollPosition returns current position info for display
func (s *ScrollableArea) GetScrollPosition() (current, total int, percentage float64) {
	current = s.scrollOffset + 1
	total = len(s.content)
	if total > 0 {
		percentage = float64(s.scrollOffset+s.height) / float64(total) * 100
		if percentage > 100 {
			percentage = 100
		}
	}
	return current, total, percentage
}

// CanScrollUp returns true if content can be scrolled up
func (s *ScrollableArea) CanScrollUp() bool {
	return s.scrollOffset > 0
}

// CanScrollDown returns true if content can be scrolled down
func (s *ScrollableArea) CanScrollDown() bool {
	return s.scrollOffset + s.height < len(s.content)
}