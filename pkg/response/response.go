package response

import (
	"encoding/json"

	"github.com/valyala/fasthttp"
)

type ResponseSender struct{}

func NewResponseSender() *ResponseSender {
	return &ResponseSender{}
}

func (rs *ResponseSender) Send(ctx *fasthttp.RequestCtx, statusCode int, payload any) {
	ctx.Response.Header.Set("Content-Type", "application/json")
	ctx.Response.SetStatusCode(statusCode)

	if payload != nil {
		responseBody, err := json.Marshal(payload)
		if err != nil {
			ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.Response.SetBodyString(`{"success":false,"error":"Internal server error"}`)
			return
		}
		ctx.Response.SetBody(responseBody)
	}
}

func (rs *ResponseSender) SendError(ctx *fasthttp.RequestCtx, statusCode int, errorMessage string) {
	rs.Send(ctx, statusCode, ErrorResponse(errorMessage))
}

func (rs *ResponseSender) SendSuccess(ctx *fasthttp.RequestCtx, statusCode int, data any) {
	rs.Send(ctx, statusCode, SuccessResponse(data))
}

func (rs *ResponseSender) SendEmpty(ctx *fasthttp.RequestCtx, statusCode int) {
	rs.Send(ctx, statusCode, Empty())
}
