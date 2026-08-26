package accessibility

import "fmt"

type RuleError struct {
	Code    string
	Message string
}

func (e *RuleError) Error() string { return e.Message }

func NewRuleError(code, format string, args ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCode(err error) string {
	if e, ok := err.(*RuleError); ok {
		return e.Code
	}
	return "INTERNAL_ERROR"
}
