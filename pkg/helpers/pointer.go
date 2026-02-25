package helpers

// Ptr returns a pointer to the provided value.
func Ptr[T any](val T) *T {
	return &val
}

// Value returns the dereferenced value or the zero value if nil.
func Value[T any](val *T) T {
	if val == nil {
		var zero T
		return zero
	}
	return *val
}

// ValueOr returns the dereferenced value or the provided default if nil.
func ValueOr[T any](val *T, fallback T) T {
	if val == nil {
		return fallback
	}
	return *val
}
