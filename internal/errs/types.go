package errs

type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string { return e.Message }

type AlreadyExistsError struct {
	Message string
}

func (e *AlreadyExistsError) Error() string { return e.Message }

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
