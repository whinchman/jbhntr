// Package models defines the core data types for jobhuntr.
package models

import "time"

// JobStatus represents the lifecycle state of a job listing.
type JobStatus string

const (
	StatusDiscovered JobStatus = "discovered"
	StatusNotified   JobStatus = "notified"
	StatusApproved   JobStatus = "approved"
	StatusRejected   JobStatus = "rejected"
	StatusGenerating JobStatus = "generating"
	StatusComplete   JobStatus = "complete"
	StatusFailed     JobStatus = "failed"
)

// Valid reports whether s is a recognised JobStatus constant.
func (s JobStatus) Valid() bool {
	switch s {
	case StatusDiscovered, StatusNotified, StatusApproved,
		StatusRejected, StatusGenerating, StatusComplete, StatusFailed:
		return true
	}
	return false
}

// ApplicationStatus represents the user-facing application pipeline stage for
// a job that has been approved (status in approved/generating/complete/failed).
type ApplicationStatus string

const (
	AppStatusApplied      ApplicationStatus = "applied"
	AppStatusInterviewing ApplicationStatus = "interviewing"
	AppStatusLost         ApplicationStatus = "lost"
	AppStatusWon          ApplicationStatus = "won"
)

// Valid reports whether s is a recognised ApplicationStatus constant.
// An empty string is not valid.
func (s ApplicationStatus) Valid() bool {
	switch s {
	case AppStatusApplied, AppStatusInterviewing, AppStatusLost, AppStatusWon:
		return true
	}
	return false
}

// Job represents a single job listing stored in the database.
type Job struct {
	ID                int64
	UserID            int64 // references users.id; 0 = legacy/unassigned
	ExternalID        string
	Source            string
	Title             string
	Company           string
	Location          string
	Description       string
	Salary            string
	ApplyURL          string
	Status            JobStatus
	Summary           string
	ExtractedSalary   string
	ResumeHTML        string
	CoverHTML         string
	ResumeMarkdown    string
	CoverMarkdown     string
	ResumePDF         string
	CoverPDF          string
	ErrorMsg          string
	ApplicationStatus ApplicationStatus
	AppliedAt         *time.Time
	InterviewingAt    *time.Time
	LostAt            *time.Time
	WonAt             *time.Time
	DiscoveredAt      time.Time
	UpdatedAt         time.Time
}

// SearchFilter represents a single job search query configuration.
type SearchFilter struct {
	Keywords  string `yaml:"keywords"`
	Location  string `yaml:"location"`
	MinSalary int    `yaml:"min_salary"`
	MaxSalary int    `yaml:"max_salary"`
	Title     string `yaml:"title"`
}
