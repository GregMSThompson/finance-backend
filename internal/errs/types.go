package errs

type ErrorMessage struct {
	Message string
}

func (e *ErrorMessage) Error() string { return e.Message }

type NotFoundError struct {
	ErrorMessage
}

type AlreadyExistsError struct {
	ErrorMessage
}

type ValidationError struct {
	ErrorMessage
}

type UnsupportedGroupByError struct {
	ErrorMessage
}

type MalformedFunctionCallError struct {
	ErrorMessage
}

func NewNotFoundError(message string) *NotFoundError {
	return &NotFoundError{
		ErrorMessage: ErrorMessage{Message: message},
	}
}

func NewAlreadyExistsError(message string) *AlreadyExistsError {
	return &AlreadyExistsError{
		ErrorMessage: ErrorMessage{Message: message},
	}
}

func NewValidationError(message string) *ValidationError {
	return &ValidationError{
		ErrorMessage: ErrorMessage{Message: message},
	}
}

func NewUnsupportedGroupByError() *UnsupportedGroupByError {
	return &UnsupportedGroupByError{
		ErrorMessage: ErrorMessage{Message: "unsupported groupBy"},
	}
}

func NewMalformedFunctionCallError() *MalformedFunctionCallError {
	return &MalformedFunctionCallError{
		ErrorMessage: ErrorMessage{Message: "malformed function call"},
	}
}
