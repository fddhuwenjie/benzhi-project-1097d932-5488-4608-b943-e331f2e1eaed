package workflow

import (
	"errors"
	"fmt"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	Cause   error  `json:"-"`
}

func NewDetailedError(code, message string, details any) error {
	return &Error{Code: code, Message: message, Details: details}
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func NewError(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func WrapRule(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*Error); ok {
		return err
	}
	var rule *accessibility.RuleError
	if errors.As(err, &rule) {
		return &Error{Code: rule.Code, Message: rule.Message, Cause: err}
	}
	return &Error{Code: "INTERNAL_ERROR", Message: err.Error(), Cause: err}
}

func Code(err error) string {
	var app *Error
	if errors.As(err, &app) {
		return app.Code
	}
	return "INTERNAL_ERROR"
}
