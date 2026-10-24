// Package validator provides a small, dependency-free input-validation helper
// used across the HTTP layer of several microservices.
package validator

import (
	"regexp"
)

// EmailRX is a permissive email-address pattern suitable for form validation.
//
//nolint:lll // a regex literal doesn't wrap cleanly across lines without risking a transcription error.
var EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// Validator accumulates per-field validation errors.
type Validator struct {
	Errors map[string]string
}

// New builds an empty Validator.
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

// Valid reports whether no errors have been recorded.
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError records message against key, keeping only the first error per key.
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// Check records message against key when ok is false.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// PermittedValue reports whether value is one of permittedValues.
func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	for i := range permittedValues {
		if value == permittedValues[i] {
			return true
		}
	}

	return false
}

// Matches reports whether value matches rx.
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// Unique reports whether values contains no duplicate elements.
func Unique[T comparable](values []T) bool {
	uniqueValues := make(map[T]bool)

	for _, value := range values {
		uniqueValues[value] = true
	}

	return len(values) == len(uniqueValues)
}
