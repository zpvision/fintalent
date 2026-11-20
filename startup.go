package main

import (
	"context"
	"errors"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

// Only fixed stage names and elapsed times are logged: no errors, SQL, arguments
// or environment values are passed to this logger. Timers add no DB/network I/O.
func startupTimer(stage string) func() {
	started := time.Now()
	log.Printf("startup stage=%s event=begin", stage)
	return func() {
		log.Printf("startup stage=%s event=end duration_ms=%.3f", stage, float64(time.Since(started).Microseconds())/1000)
	}
}

// This diagnostic-only release supports the existing synchronous auto path.
// Rejecting other values prevents an accidental assumption that verify is active.
func validateStartupMode() error {
	mode := strings.TrimSpace(os.Getenv("DB_STARTUP_MODE"))
	if mode != "" && mode != "auto" {
		return errors.New("this release supports only DB_STARTUP_MODE=auto")
	}
	return nil
}

var startupSQLStatePattern = regexp.MustCompile(`^[0-9A-Z]{5}$`)

// Do not log err.Error(): connection errors can contain a DSN or credentials.
func startupFailure(stage string, err error) {
	code := "operation_failed"
	var state interface{ SQLState() string }
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code = "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		code = "canceled"
	case errors.As(err, &state):
		if value := state.SQLState(); startupSQLStatePattern.MatchString(value) {
			code = value
		}
	}
	log.Printf("startup stage=%s event=failed error_code=%s", stage, code)
}
