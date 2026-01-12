package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// PrintStatus represents the status of a print operation
type PrintStatus string

const (
	StatusPending  PrintStatus = "pending"
	StatusSending  PrintStatus = "sending"
	StatusSent     PrintStatus = "sent"
	StatusFailed   PrintStatus = "failed"
	StatusCanceled PrintStatus = "canceled"
)

// PrintStatusMsg is sent when a print job status changes
type PrintStatusMsg struct {
	FileID      string
	Status      PrintStatus
	SystemJobID string // CUPS job ID (e.g., "216") for matching with lpq
	Error       error
}

// parseJobIDFromLpOutput extracts job number from lp output
// Input: "request id is EPSON_ET_2810_Series-216 (1 file(s))"
// Output: "216"
func parseJobIDFromLpOutput(output string) string {
	const prefix = "request id is "
	idx := strings.Index(output, prefix)
	if idx == -1 {
		return ""
	}

	remaining := output[idx+len(prefix):]
	// Find end of job ID (space or parenthesis)
	end := strings.IndexAny(remaining, " (")
	if end == -1 {
		end = len(remaining)
	}

	fullID := remaining[:end] // e.g., "EPSON_ET_2810_Series-216"

	// Extract just the number after the last hyphen
	lastHyphen := strings.LastIndex(fullID, "-")
	if lastHyphen != -1 && lastHyphen < len(fullID)-1 {
		return fullID[lastHyphen+1:] // e.g., "216"
	}

	return fullID
}

// submitPrintJobCmd creates a command that sends a file to the printer
func submitPrintJobCmd(opID string, filePath string) tea.Cmd {
	return func() tea.Msg {
		// Add a small random delay to stagger concurrent submissions
		// This helps prevent overwhelming the print spooler
		delay := time.Duration(100 + (time.Now().UnixNano()%200)) * time.Millisecond
		time.Sleep(delay)

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return PrintStatusMsg{
				FileID: opID,
				Status: StatusFailed,
				Error:  fmt.Errorf("file does not exist: %s", filePath),
			}
		}

		// Execute lp command with -t to set job title (filename)
		// This makes the filename visible in lpq output
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fileName := filepath.Base(filePath)
		cmd := exec.CommandContext(ctx, "lp", "-t", fileName, filePath)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return PrintStatusMsg{
					FileID: opID,
					Status: StatusFailed,
					Error:  fmt.Errorf("print command timed out after 10 seconds"),
				}
			}
			return PrintStatusMsg{
				FileID: opID,
				Status: StatusFailed,
				Error:  fmt.Errorf("failed to print: %v - %s", err, stderr.String()),
			}
		}

		// Parse job ID from lp output: "request id is PRINTER-123 (1 file(s))"
		jobID := parseJobIDFromLpOutput(stdout.String())

		return PrintStatusMsg{
			FileID:      opID,
			Status:      StatusSent,
			SystemJobID: jobID,
			Error:       nil,
		}
	}
}

// submitPrintBatchCmd sends multiple files with delays between them
func submitPrintBatchCmd(operations []PrintOperation) tea.Cmd {
	return func() tea.Msg {
		// Send status updates as a batch process begins
		for i, op := range operations {
			// Add delay between jobs (except for the first one)
			if i > 0 {
				time.Sleep(500 * time.Millisecond)
			}
			
			// Process this print job synchronously within this goroutine
			// but the whole batch runs async to the UI
			_ = processPrintJob(op.ID, op.FilePath)
			
			// We can't send intermediate messages directly in tea.Cmd
			// So we'll process all and return a batch complete message
		}
		
		// Return first job status to trigger update
		if len(operations) > 0 {
			return processPrintJob(operations[0].ID, operations[0].FilePath)
		}
		
		return nil
	}
}

// processPrintJob handles a single print job (helper function)
func processPrintJob(opID string, filePath string) PrintStatusMsg {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return PrintStatusMsg{
			FileID: opID,
			Status: StatusFailed,
			Error:  fmt.Errorf("file does not exist: %s", filePath),
		}
	}

	// Execute lp command with -t to set job title (filename)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fileName := filepath.Base(filePath)
	cmd := exec.CommandContext(ctx, "lp", "-t", fileName, filePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return PrintStatusMsg{
				FileID: opID,
				Status: StatusFailed,
				Error:  fmt.Errorf("print command timed out after 10 seconds"),
			}
		}
		return PrintStatusMsg{
			FileID: opID,
			Status: StatusFailed,
			Error:  fmt.Errorf("failed to print: %v - %s", err, stderr.String()),
		}
	}

	return PrintStatusMsg{
		FileID: opID,
		Status: StatusSent,
		Error:  nil,
	}
}


// CheckPrinterAvailable checks if a printer is configured and available
func CheckPrinterAvailable() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lpstat", "-p")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, fmt.Errorf("printer check timed out")
		}
		return false, err
	}

	// If we have any output, we have at least one printer
	return len(output) > 0, nil
}

// sequentialPrintCmd creates individual print commands that will run one after another
func sequentialPrintCmd(operations []PrintOperation) tea.Cmd {
	if len(operations) == 0 {
		return nil
	}
	
	// Take the first operation
	first := operations[0]
	remaining := operations[1:]
	
	return tea.Sequence(
		// First, update status to "sending"
		func() tea.Msg {
			return PrintStatusMsg{
				FileID: first.ID,
				Status: StatusSending,
			}
		},
		// Then submit the print job
		submitPrintJobCmd(first.ID, first.FilePath),
		// Then wait a bit
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return nil // Just for delay
		}),
		// Then process the rest recursively
		func() tea.Msg {
			if len(remaining) > 0 {
				// This will trigger another sequence
				return sequentialPrintCmd(remaining)
			}
			return nil
		},
	)
}