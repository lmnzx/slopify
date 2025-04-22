package response

type StandardResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func ErrorResponse(message string) StandardResponse {
	return StandardResponse{
		Success: false,
		Error:   message,
	}
}

func SuccessResponse(data any) StandardResponse {
	return StandardResponse{
		Success: true,
		Data:    data,
	}
}

func Empty() StandardResponse {
	return StandardResponse{
		Success: true,
	}
}
