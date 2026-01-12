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

// PrinterInfo holds the default printer name and status
type PrinterInfo struct {
	Name   string
	Status string // "idle", "printing", etc.
}

// getDefaultPrinter returns info about the default printer
func getDefaultPrinter() PrinterInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lpstat", "-p", "-d")
	output, err := cmd.Output()
	if err != nil {
		return PrinterInfo{Name: "Unknown", Status: ""}
	}

	info := PrinterInfo{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "printer ") {
			// "printer EPSON_ET_2810_Series is idle.  enabled since..."
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				info.Name = parts[1]
				info.Status = parts[3]
				// Remove trailing period
				info.Status = strings.TrimSuffix(info.Status, ".")
			}
		}
	}
	if info.Name == "" {
		info.Name = "No printer"
	}
	return info
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
// Uses lpq which shows job titles (filenames) set via lp -t
func getSystemPrintJobs() []PrintJob {
	// Add timeout to prevent hanging when print spooler is stuck
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lpq", "-a")
	output, err := cmd.Output()
	if err != nil {
		return []PrintJob{}
	}

	var jobs []PrintJob
	lines := strings.Split(string(output), "\n")

	// lpq output format:
	// Rank    Owner   Job     File(s)                         Total Size
	// active  adrian  210     filename.pdf                    155648 bytes
	// 1st     adrian  212     another.pdf                     1024 bytes

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header line
		if strings.HasPrefix(line, "Rank") {
			continue
		}

		// Parse: rank owner job filename size "bytes"
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}

		rank := parts[0]
		// Skip if not a job line (active, 1st, 2nd, etc.)
		if rank != "active" && !strings.HasSuffix(rank, "st") &&
		   !strings.HasSuffix(rank, "nd") && !strings.HasSuffix(rank, "rd") &&
		   !strings.HasSuffix(rank, "th") {
			continue
		}

		jobNum := parts[2]

		// Filename is everything between job number and size
		// Find the "bytes" suffix to locate size
		bytesIdx := -1
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == "bytes" {
				bytesIdx = i
				break
			}
		}

		if bytesIdx < 4 {
			continue
		}

		// Size is the field before "bytes"
		size, _ := strconv.ParseInt(parts[bytesIdx-1], 10, 64)

		// Filename is fields 3 to (bytesIdx-2)
		fileName := strings.Join(parts[3:bytesIdx-1], " ")
		if fileName == "" {
			fileName = fmt.Sprintf("Job %s", jobNum)
		}

		job := PrintJob{
			ID:       jobNum,
			FileName: fileName,
			Size:     size,
			Status:   rank,
		}

		jobs = append(jobs, job)
	}

	return jobs
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

	// Send to printer using lp with -t to set job title (filename)
	fileName := filepath.Base(filePath)
	cmd := exec.Command("lp", "-t", fileName, filePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to print %s: %v - %s", filePath, err, stderr.String())
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