package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// PrintOperation tracks a print operation initiated by the user
type PrintOperation struct {
	ID        string
	FilePath  string
	FileName  string
	Status    PrintStatus
	Error     error
	StartedAt time.Time
	UpdatedAt time.Time
	SystemJobID string // The actual system print job ID if successfully submitted
}


func (m *model) formatPrintFileName(op PrintOperation) string {
	// Show relative path from current directory
	relPath, err := filepath.Rel(m.currentDir, op.FilePath)
	if err != nil {
		// If we can't get relative path, just show the filename
		return op.FileName
	}
	
	// If file is in current directory, just show the name
	if !strings.Contains(relPath, string(filepath.Separator)) && !strings.HasPrefix(relPath, "..") {
		return relPath
	}
	
	// Otherwise show the relative path
	return relPath
}

func (m *model) formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	
	if duration < time.Second {
		return "now"
	} else if duration < time.Minute {
		return fmt.Sprintf("%ds ago", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	}
}

