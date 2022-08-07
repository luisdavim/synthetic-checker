package api

import (
	"context"
	"time"
)

// Check defines the api for implementing a checker
type Check interface {
	// Checkers must implement an Execute function that runs the check and returns the status
	Execute(ctx context.Context) (bool, error)
	// Checkers must implement an Invoke function that indicates how often the check should run
	Interval() time.Duration
}

// Status represents the state of what is being checked
type Status struct {
	// OK indicates if the last check passed
	OK bool `json:"ok,omitempty"`
	// Error holds an error message explaining why the check failed
	Error string `json:"error,omitempty"`
	// Timestamp indicates when the check was last run
	Timestamp time.Time `json:"timestamp"`
	// Duration indicates how long the last check took to run
	Duration time.Duration `json:"duration,omitempty"`
	// ContiguousFailures indicates the number of failures that occurred in a row
	ContiguousFailures int `json:"contiguousFailures"`
	// TimeOfFirstFailure indicates when the first failure occurred
	TimeOfFirstFailure time.Time `json:"timeOfFirstFailure"`
}
