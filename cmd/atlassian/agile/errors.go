package agile

import "errors"

// isErr reports whether err matches target using errors.Is.
// Extracted here so all agile commands share one implementation.
func isErr(err, target error) bool {
	return errors.Is(err, target)
}
