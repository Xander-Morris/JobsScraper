package database

import (
	"errors"
	"fmt"
)

// ErrInvalidInput marks a request the caller can fix by sending different
// values. Handlers map it to 400 and surface its message; anything not
// matching it is a server-side failure they log and turn into a 500.
var ErrInvalidInput = errors.New("invalid input")

// invalidInputError carries only the caller-facing message, so a 400 body
// reads "skill is required" rather than "invalid input: skill is required".
type invalidInputError struct {
	msg string
}

func (e invalidInputError) Error() string { return e.msg }

func (e invalidInputError) Unwrap() error { return ErrInvalidInput }

func invalidInput(format string, args ...any) error {
	return invalidInputError{msg: fmt.Sprintf(format, args...)}
}
