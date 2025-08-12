package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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
	FileID  string
	Status  PrintStatus
	Error   error
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

		// Execute lpr command with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "lpr", filePath)
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
		
		// Successfully sent to printer
		// Try to track the job ID after a brief delay
		time.Sleep(200 * time.Millisecond)
		if jobID := tryGetLatestJobID(); jobID != "" {
			tracker.AddJob(jobID, filePath)
		}

		return PrintStatusMsg{
			FileID: opID,
			Status: StatusSent,
			Error:  nil,
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

	// Execute lpr command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lpr", filePath)
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

	// Successfully sent to printer
	// Try to track the job ID
	if jobID := tryGetLatestJobID(); jobID != "" {
		tracker.AddJob(jobID, filePath)
	}

	return PrintStatusMsg{
		FileID: opID,
		Status: StatusSent,
		Error:  nil,
	}
}

// tryGetLatestJobID attempts to get the latest print job ID
func tryGetLatestJobID() string {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lpstat", "-o")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse the first job from output
	lines := bytes.Split(output, []byte("\n"))
	if len(lines) > 0 && len(lines[0]) > 0 {
		parts := bytes.Fields(lines[0])
		if len(parts) > 0 {
			return string(parts[0])
		}
	}

	return ""
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