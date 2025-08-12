package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// tickMsg is sent periodically to refresh the print queue
type tickMsg time.Time

// jobsRefreshedMsg contains the refreshed list of print jobs
type jobsRefreshedMsg struct {
	jobs []PrintJob
}

// tickCmd returns a command that sends a tickMsg every second
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// refreshJobsCmd runs lpstat asynchronously and returns the jobs
func refreshJobsCmd() tea.Cmd {
	return func() tea.Msg {
		// This runs in a background goroutine, not blocking the UI
		jobs := getSystemPrintJobs()
		return jobsRefreshedMsg{jobs: jobs}
	}
}

// getSystemPrintJobs retrieves the current print queue from the system
func getSystemPrintJobs() []PrintJob {
	// Add timeout to prevent hanging when print spooler is stuck
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "lpstat", "-o")
	output, err := cmd.Output()
	if err != nil {
		// Log the error but don't block - return empty list
		// This allows the app to continue working even if lpstat fails
		if ctx.Err() == context.DeadlineExceeded {
			// Print spooler is not responding
			// Could add an error message to the UI here
		}
		return []PrintJob{}
	}

	var jobs []PrintJob
	lines := strings.Split(string(output), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse lpstat output format: "printer-jobid user size date time"
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Extract job ID from the first field (format: printer-jobid)
		jobParts := strings.Split(parts[0], "-")
		if len(jobParts) < 2 {
			continue
		}
		jobID := jobParts[len(jobParts)-1]

		// Get file size (third field)
		size, _ := strconv.ParseInt(parts[2], 10, 64)

		// Only use tracker, don't call getJobFileName which does another lpstat
		fileName := ""
		if info, ok := tracker.GetJob(parts[0]); ok {
			fileName = info.FileName
		}
		if fileName == "" {
			fileName = fmt.Sprintf("Job %s", jobID)
		}

		job := PrintJob{
			ID:       parts[0],
			FileName: fileName,
			FilePath: getJobFilePath(parts[0]),
			Size:     size,
			Status:   "Queued",
		}

		jobs = append(jobs, job)
	}

	return jobs
}

// getJobFileName attempts to retrieve the original filename for a print job
func getJobFileName(jobID string) string {
	// First check our tracker
	if info, ok := tracker.GetJob(jobID); ok {
		return info.FileName
	}
	
	// Fallback to trying to extract from lpstat
	cmd := exec.Command("lpstat", "-W", "completed", "-o", jobID)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse output to find filename
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Look for filename patterns in the output
		if strings.Contains(line, "/") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, "/") {
					return filepath.Base(part)
				}
			}
		}
	}

	return ""
}

// getJobFilePath attempts to retrieve the original file path for a print job
func getJobFilePath(jobID string) string {
	// Check our tracker
	if info, ok := tracker.GetJob(jobID); ok {
		return info.FilePath
	}
	return ""
}

// cancelPrintJob cancels a specific print job
func cancelPrintJob(jobID string) error {
	cmd := exec.Command("cancel", jobID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to cancel job %s: %v - %s", jobID, err, stderr.String())
	}
	
	return nil
}

// addToPrintQueue adds a file to the print queue
func addToPrintQueue(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Send to printer using lpr and capture the job ID
	cmd := exec.Command("lpr", filePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to print %s: %v - %s", filePath, err, stderr.String())
	}
	
	// Get the latest job ID (this is a bit hacky but works)
	// We look for the newest job right after submission
	time.Sleep(100 * time.Millisecond)
	jobs := getSystemPrintJobs()
	if len(jobs) > 0 {
		// Track this job
		tracker.AddJob(jobs[0].ID, filePath)
	}
	
	return nil
}

// openFile opens a file with the default application
func openFile(filePath string) error {
	if filePath == "" {
		return nil
	}
	
	cmd := exec.Command("open", filePath)
	return cmd.Start()
}

// openFolder opens the containing folder of a file
func openFolder(filePath string) error {
	if filePath == "" {
		return nil
	}
	
	dir := filepath.Dir(filePath)
	cmd := exec.Command("open", dir)
	return cmd.Start()
}