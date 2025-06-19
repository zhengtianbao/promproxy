package middleware

import (
	"net/http"
	"time"

	"github.com/prometheus/prometheus/promql/parser"
)

type RequestContext struct {
	Query     string
	ParsedAST parser.Expr
	StartTime *time.Time
	EndTime   *time.Time
	Timestamp *time.Time
	Step      string
	IsRange   bool
	Request   *http.Request
}
