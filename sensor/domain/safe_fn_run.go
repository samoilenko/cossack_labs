package domain

import (
	"fmt"
)

// SafeFunctionRun executes the provided function with panic recovery.
// It catches any panic that occurs during function execution, logs it,
// and returns it as a regular error instead of crashing the program.
func SafeFunctionRun(fn func() error, logger Logger) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic: %v", rec)
			logger.Error("panic: %s", fmt.Sprintf("%v", rec))
		}
	}()
	err = fn()
	return
}
