package dto

type CreateUserRequest struct {
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
}

type SetFCMTokenRequest struct {
	Token string `json:"token"`
}
