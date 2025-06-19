package middleware

import (
	"fmt"
	"time"
)

type QueryRangeMiddleware struct{}

func NewQueryRangeMiddleware() *QueryRangeMiddleware {
	return &QueryRangeMiddleware{}
}

func (q *QueryRangeMiddleware) Process(ctx *RequestContext) error {
	return q.validateQueryRange(ctx)
}

func (q *QueryRangeMiddleware) validateQueryRange(ctx *RequestContext) error {
	if !ctx.IsRange || ctx.Step == "" {
		return nil
	}

	stepDuration, err := time.ParseDuration(ctx.Step)
	if err != nil {
		return fmt.Errorf("invalid step format: %v", err)
	}

	var queryDuration time.Duration
	if ctx.StartTime != nil && ctx.EndTime != nil {
		queryDuration = ctx.EndTime.Sub(*ctx.StartTime)
	}

	// 根据step限制查询范围
	var maxDuration time.Duration
	switch {
	case stepDuration >= 10*time.Minute:
		maxDuration = 24 * time.Hour
	case stepDuration >= 5*time.Minute:
		maxDuration = 24 * time.Hour
	case stepDuration >= 2*time.Minute:
		maxDuration = 12 * time.Hour
	case stepDuration >= 1*time.Minute:
		maxDuration = 6 * time.Hour
	default:
		return fmt.Errorf("step must be at least 1 minute")
	}

	if queryDuration > maxDuration {
		return fmt.Errorf("query range %v exceeds maximum allowed %v for step %v",
			queryDuration, maxDuration, stepDuration)
	}

	return nil
}
