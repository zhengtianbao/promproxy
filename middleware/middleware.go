package middleware

type Middleware interface {
	Process(ctx *RequestContext) error
}

type MiddlewareFunc func(ctx *RequestContext) error

func (f MiddlewareFunc) Process(ctx *RequestContext) error {
	return f(ctx)
}
