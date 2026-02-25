package errs

type ErrorMessage struct {
	Message string
	Cause   error // wrapped original error
}

func (e *ErrorMessage) Error() string { return e.Message }

func (e *ErrorMessage) Unwrap() error { return e.Cause }

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

type DatabaseError struct {
	ErrorMessage
	Operation string // "create", "read", "update", "delete"
}

type ExternalServiceError struct {
	ErrorMessage
	Service   string // "plaid", "vertex", "firestore", "kms"
	Transient bool   // true if retry might succeed
}

type EncryptionError struct {
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

func NewDatabaseError(operation, message string, cause error) *DatabaseError {
	return &DatabaseError{
		ErrorMessage: ErrorMessage{
			Message: message,
			Cause:   cause,
		},
		Operation: operation,
	}
}

func NewExternalServiceError(service, message string, transient bool, cause error) *ExternalServiceError {
	return &ExternalServiceError{
		ErrorMessage: ErrorMessage{
			Message: message,
			Cause:   cause,
		},
		Service:   service,
		Transient: transient,
	}
}

func NewEncryptionError(message string, cause error) *EncryptionError {
	return &EncryptionError{
		ErrorMessage: ErrorMessage{
			Message: message,
			Cause:   cause,
		},
	}
}
