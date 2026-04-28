package request

// VerifyPinRequest is the DTO for the POST /auth/verify-pin endpoint.
type VerifyPinRequest struct {
	Email   string `json:"email" validate:"required,email"`
	Code    string `json:"code" validate:"required,len=6"`
	Purpose string `json:"purpose" validate:"required,oneof=register forgot_password verify"`
}
