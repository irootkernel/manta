package model

import "fmt"

type ErrorCode int

const (
	ExitCodeConfigError   ErrorCode = 2
	ExitCodeArtifactError ErrorCode = 3
	ExitCodeParserError   ErrorCode = 4
	ExitCodeTimeout       ErrorCode = 124
)

type MantaError struct {
	Code ErrorCode
	Op   string
	Err  error
}

func (e *MantaError) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *MantaError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewMantaError(code ErrorCode, op string, err error) error {
	if err == nil {
		return nil
	}
	return &MantaError{Code: code, Op: op, Err: err}
}

func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var mantaErr *MantaError
	if ok := As(err, &mantaErr); ok {
		return int(mantaErr.Code)
	}
	return int(ExitCodeParserError)
}

func As(err error, target any) bool {
	type unwrapper interface{ Unwrap() error }
	for err != nil {
		switch t := target.(type) {
		case **MantaError:
			if v, ok := err.(*MantaError); ok {
				*t = v
				return true
			}
		}
		u, ok := err.(unwrapper)
		if !ok {
			break
		}
		err = u.Unwrap()
	}
	return false
}
