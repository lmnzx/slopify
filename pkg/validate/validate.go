package validate

import (
	"github.com/go-playground/validator/v10"
	"github.com/valyala/fasthttp"
)

type CustomValidator struct {
	Validator *validator.Validate
}

func (cv *CustomValidator) Validate(i any) error {
	if err := cv.Validator.Struct(i); err != nil {
		return err
	}
	return nil
}

type ValidatorKeyType struct{}

var ValidatorKey = ValidatorKeyType{}

func ValidatorMiddleware(v *CustomValidator) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			ctx.SetUserValue(ValidatorKey, v)
			next(ctx)
		}
	}
}
