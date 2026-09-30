package exitcode

import "fmt"

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) ExitCode() int {
	return e.Code
}

func Resolve(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	priorityOrder := []int{int(CodeInvalidArgs), int(CodeNetworkError), int(CodeTimeout), int(CodeParseError)}
	for _, pCode := range priorityOrder {
		for _, err := range errs {
			if appErr, ok := err.(*AppError); ok && appErr.Code == pCode {
				return appErr
			}
		}
	}

	return &AppError{Code: int(CodeNetworkError), Message: "multiple errors occurred"}
}
