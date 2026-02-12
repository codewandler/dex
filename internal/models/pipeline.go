package models

import "time"

// PipelineSummary represents a pipeline in list views
type PipelineSummary struct {
	ID        int
	IID       int
	ProjectID int
	Status    string
	Source    string
	Ref       string
	SHA       string
	User      string
	WebURL    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PipelineDetail contains full pipeline information
type PipelineDetail struct {
	ID             int
	IID            int
	ProjectID      int
	Status         string
	Source         string
	Ref            string
	SHA            string
	BeforeSHA      string
	Tag            bool
	YamlErrors     string
	User           string
	WebURL         string
	Duration       int // seconds
	QueuedDuration int // seconds
	Coverage       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	Jobs           []PipelineJob // populated when fetching with jobs
}

// PipelineJob represents a job/build within a pipeline
type PipelineJob struct {
	ID             int
	Name           string
	Stage          string
	Status         string
	Ref            string
	Tag            bool
	AllowFailure   bool
	Duration       float64 // seconds
	QueuedDuration float64
	FailureReason  string
	WebURL         string
	User           string
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
}
