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
	// errors.As 沿 Unwrap 链向下探测，既能识别直接的 *Error，
	// 也能识别被 fmt.Errorf("%w", ...) 包装过的 *Error，从而保留
	// 原始稳定错误码（如 NOT_FOUND），避免被误判为 INTERNAL_ERROR。
	var app *Error
	if errors.As(err, &app) {
		return err
	}
	var rule *accessibility.RuleError
	if errors.As(err, &rule) {
		return &Error{Code: rule.Code, Message: rule.Message, Cause: err}
	}
	// 将 cause 重新编码为文本会切断调用方的 errors.Unwrap/errors.Is 链。
	return &Error{Code: "INTERNAL_ERROR", Message: err.Error(), Cause: fmt.Errorf("%s", err)}
}

func Code(err error) string {
	var app *Error
	if errors.As(err, &app) {
		return app.Code
	}
	return "INTERNAL_ERROR"
}
