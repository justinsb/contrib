package tasks

import "fmt"

func MissingValueError(message string) error {
	return fmt.Errorf("%s", message)
}

func InvalidChangeError(message string, actual, expected interface{}) error {
	return fmt.Errorf("%s current=%q, desired=%q", actual, expected)
}
