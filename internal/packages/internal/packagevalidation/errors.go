// Partially: Copyright 2022 The Go Authors. All rights reserved.
// Slightly modified errors.Join implementation.

package packagevalidation

import (
	"strings"
)

// joinErrorsReadable returns an error that wraps the given errors.
// Any nil error values are discarded.
// Join returns nil if every value in errs is nil.
// The error formats as the concatenation of the `-`-prefixed strings obtained
// by calling the Error method of each element of errs, with a newline
// between each string.
// It also optionally prefixes the whole error string with a newline,
// which can be useful when wrapping this error and printing it directly.
//
// A non-nil error returned by Join implements the Unwrap() []error method.
func joinErrorsReadable(prefixNewline bool, errs ...error) error {
	n := 0
	for _, err := range errs {
		if err != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	e := &readableJoinError{
		prefixNewline: prefixNewline,
		errs:          make([]error, 0, n),
	}
	for _, err := range errs {
		if err != nil {
			e.errs = append(e.errs, err)
		}
	}
	return e
}

// Never instantiate readableJoinError directly!
// Use `joinErrorsReadable(...)` instead.
type readableJoinError struct {
	prefixNewline bool
	errs          []error
}

func (e *readableJoinError) Error() string {
	// Since Join returns nil if every value in errs is nil,
	// e.errs cannot be empty.
	sb := strings.Builder{}
	if e.prefixNewline {
		sb.WriteByte('\n')
	}
	sb.WriteString("- " + e.errs[0].Error())

	for _, err := range e.errs[1:] {
		sb.WriteByte('\n')
		sb.WriteString("- " + err.Error())
	}
	return sb.String()
}

func (e *readableJoinError) Unwrap() []error {
	return e.errs
}
