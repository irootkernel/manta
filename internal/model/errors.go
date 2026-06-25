package model

import "fmt"

type ErrorCode int

const (
	ExitCodeConfigError   ErrorCode = 2
	ExitCodeArtifactError ErrorCode = 3
	ExitCodeParserError   ErrorCode = 4
	ExitCodeTimeout       ErrorCode = 124
)

type KATError struct {
	Code ErrorCode
	Op   string
	Err  error
}

func (e *KATError) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *KATError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewKATError(code ErrorCode, op string, err error) error {
	if err == nil {
		return nil
	}
	return &KATError{Code: code, Op: op, Err: err}
}

func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var katErr *KATError
	if ok := As(err, &katErr); ok {
		return int(katErr.Code)
	}
	return int(ExitCodeParserError)
}

func As(err error, target any) bool {
	type unwrapper interface{ Unwrap() error }
	for err != nil {
		switch t := target.(type) {
		case **KATError:
			if v, ok := err.(*KATError); ok {
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
