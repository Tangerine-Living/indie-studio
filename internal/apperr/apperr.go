// Package apperr defines errors that cross the Wails boundary.
//
// Every error carries a stable machine code and a Chinese user-facing message.
// The frontend can split the code prefix (before the first colon) to react to
// specific failures while still showing a readable message.
package apperr

import (
	"errors"
	"fmt"
)

// Error is a user-facing application error.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
}

// New creates an error with a code and a Chinese message.
func New(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf creates an error with a formatted Chinese message.
func Newf(code, message string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(message, args...)}
}

// WithDetail attaches technical detail that is safe to show in an expandable
// section or a log file.
func (e *Error) WithDetail(detail string) *Error {
	e.Detail = detail
	return e
}

// Wrap preserves an existing *Error; otherwise it wraps err with a code and a
// Chinese message.
func Wrap(code, message string, err error) *Error {
	if err == nil {
		return nil
	}
	var existing *Error
	if errors.As(err, &existing) {
		return existing
	}
	return &Error{Code: code, Message: message, Detail: err.Error()}
}

// Is reports whether err is an *Error with the given code.
func Is(err error, code string) bool {
	var target *Error
	return errors.As(err, &target) && target.Code == code
}
