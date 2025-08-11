package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JobTracker keeps track of print jobs and their original file paths
type JobTracker struct {
	mu      sync.RWMutex
	jobs    map[string]JobInfo
	dbPath  string
}

// JobInfo stores information about a print job
type JobInfo struct {
	JobID    string    `json:"job_id"`
	FilePath string    `json:"file_path"`
	FileName string    `json:"file_name"`
	AddedAt  time.Time `json:"added_at"`
}

var tracker *JobTracker

func init() {
	// Use XDG_DATA_HOME if set, otherwise fall back to ~/.local/share
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, _ := os.UserHomeDir()
		dataHome = filepath.Join(home, ".local", "share")
	}
	
	dbPath := filepath.Join(dataHome, "printer", "jobs.json")
	
	// Ensure directory exists
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	
	tracker = &JobTracker{
		jobs:   make(map[string]JobInfo),
		dbPath: dbPath,
	}
	
	tracker.load()
}

// load reads the job database from disk
func (t *JobTracker) load() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	data, err := os.ReadFile(t.dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	return json.Unmarshal(data, &t.jobs)
}

// save writes the job database to disk
func (t *JobTracker) save() error {
	t.mu.RLock()
	data, err := json.MarshalIndent(t.jobs, "", "  ")
	t.mu.RUnlock()
	
	if err != nil {
		return err
	}
	
	return os.WriteFile(t.dbPath, data, 0644)
}

// AddJob records a new print job
func (t *JobTracker) AddJob(jobID, filePath string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.jobs[jobID] = JobInfo{
		JobID:    jobID,
		FilePath: filePath,
		FileName: filepath.Base(filePath),
		AddedAt:  time.Now(),
	}
	
	t.save()
}

// GetJob retrieves information about a print job
func (t *JobTracker) GetJob(jobID string) (JobInfo, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	info, ok := t.jobs[jobID]
	return info, ok
}

// RemoveJob removes a job from tracking
func (t *JobTracker) RemoveJob(jobID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	delete(t.jobs, jobID)
	t.save()
}

// CleanOldJobs removes jobs older than 24 hours
func (t *JobTracker) CleanOldJobs() {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	cutoff := time.Now().Add(-24 * time.Hour)
	
	for id, job := range t.jobs {
		if job.AddedAt.Before(cutoff) {
			delete(t.jobs, id)
		}
	}
	
	t.save()
}