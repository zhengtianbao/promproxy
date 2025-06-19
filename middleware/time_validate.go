package middleware

import (
	"fmt"
	"time"
)

type TimeValidateMiddleware struct{}

func NewTimeValidateMiddleware() *TimeValidateMiddleware {
	return &TimeValidateMiddleware{}
}

func (t *TimeValidateMiddleware) Process(ctx *RequestContext) error {
	return t.validateTimeRange(ctx)
}

func (t *TimeValidateMiddleware) validateTimeRange(ctx *RequestContext) error {
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	if ctx.StartTime != nil {
		if ctx.StartTime.Before(twoHoursAgo) {
			return fmt.Errorf("start time must be within 2 hours from now")
		}
	}

	if ctx.Timestamp != nil {
		if ctx.Timestamp.Before(twoHoursAgo) {
			return fmt.Errorf("timestamp must be within 2 hours from now")
		}
	}

	return nil
}
