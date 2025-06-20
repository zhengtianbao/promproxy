package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/prometheus/promql/parser"
	"github.com/zhengtianbao/promproxy/config"
	"github.com/zhengtianbao/promproxy/middleware"
)

type ProxyServer struct {
	config      *config.Config
	middlewares []middleware.Middleware
	semaphore   chan struct{}
	client      *http.Client
}

func NewProxyServer(config *config.Config) *ProxyServer {
	server := &ProxyServer{
		config:    config,
		semaphore: make(chan struct{}, config.Server.MaxConcurrency),
		client:    &http.Client{Timeout: 30 * time.Second},
	}

	return server
}

func (p *ProxyServer) RegisterMiddlewares(middlewares ...middleware.Middleware) {
	p.middlewares = append(p.middlewares, middlewares...)
}

func (p *ProxyServer) processMiddlewares(ctx *middleware.RequestContext) error {
	for _, middleware := range p.middlewares {
		if err := middleware.Process(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *ProxyServer) proxyToPrometheus(w http.ResponseWriter, r *http.Request) error {
	targetURL := p.config.Prometheus.URL + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		return err
	}

	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	resp, err := p.client.Do(proxyReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	return err
}

func parseTimeParam(timeStr string) (time.Time, error) {
	if timestamp, err := strconv.ParseFloat(timeStr, 64); err == nil {
		return time.Unix(int64(timestamp), 0), nil
	}

	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format")
}

func (p *ProxyServer) parseRequestContext(r *http.Request) (*middleware.RequestContext, error) {
	ctx := &middleware.RequestContext{Request: r}

	query := r.URL.Query()
	ctx.Query = query.Get("query")

	if ctx.Query == "" {
		return nil, fmt.Errorf("missing query parameter")
	}

	expr, err := parser.ParseExpr(ctx.Query)
	if err != nil {
		return nil, fmt.Errorf("invalid PromQL syntax: %v", err)
	}
	ctx.ParsedAST = expr

	path := strings.TrimPrefix(r.URL.Path, "/")
	ctx.IsRange = strings.Contains(path, "query_range")

	if startStr := query.Get("start"); startStr != "" {
		if startTime, err := parseTimeParam(startStr); err == nil {
			ctx.StartTime = &startTime
		}
	}

	if endStr := query.Get("end"); endStr != "" {
		if endTime, err := parseTimeParam(endStr); err == nil {
			ctx.EndTime = &endTime
		}
	}

	if timeStr := query.Get("time"); timeStr != "" {
		if timeVal, err := parseTimeParam(timeStr); err == nil {
			ctx.Timestamp = &timeVal
		}
	}

	ctx.Step = query.Get("step")

	return ctx, nil
}

func (p *ProxyServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	select {
	case p.semaphore <- struct{}{}:
		defer func() {
			<-p.semaphore
			log.Printf("Processed request: Host: %s, Method: %s, Path: %s, Query: %s, duration: %s",
				r.RemoteAddr, r.Method, r.URL.Path, r.URL.Query(), time.Since(start).String())
		}()
	case <-r.Context().Done():
		http.Error(w, "Request cancelled", http.StatusRequestTimeout)
		return
	}

	query := r.URL.Query()

	if query.Get("query") != "" {
		ctx, err := p.parseRequestContext(r)
		if err != nil {
			log.Printf("Parse request error: %v, query: %s", err, r.URL.Query().Get("query"))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := p.processMiddlewares(ctx); err != nil {
			log.Printf("Validation error: %v, query: %s", err, ctx.Query)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err := p.proxyToPrometheus(w, r); err != nil {
		log.Printf("Error proxying to Prometheus: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (p *ProxyServer) defaultProxyHandler(w http.ResponseWriter, r *http.Request) {
	if err := p.proxyToPrometheus(w, r); err != nil {
		log.Printf("Error proxying to Prometheus: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (p *ProxyServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/query", p.handleQuery)
	mux.HandleFunc("/api/v1/query_range", p.handleQuery)
	mux.HandleFunc("/select/0/prometheus/api/v1/query", p.handleQuery)
	mux.HandleFunc("/select/0/prometheus/api/v1/query_range", p.handleQuery)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("/", p.defaultProxyHandler)

	mux.HandleFunc("/debug/parse", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Error(w, "missing query parameter", http.StatusBadRequest)
			return
		}

		expr, err := parser.ParseExpr(query)
		if err != nil {
			http.Error(w, fmt.Sprintf("parse error: %v", err), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Query: %s\n\n", query)

		fmt.Fprintf(w, "Simplified Expression Tree:\n")
		fmt.Fprintf(w, strings.Repeat("=", 30) + "\n")
		printExpressionTree(w, expr, 0)
	})

	addr := fmt.Sprintf(":%d", p.config.Server.Port)
	log.Printf("Starting PromQL proxy server on %s", addr)
	log.Printf("Max concurrency: %d", p.config.Server.MaxConcurrency)
	log.Printf("Prometheus backend: %s", p.config.Prometheus.URL)
	log.Printf("Allowed spaces: %v", p.config.Rules.AllowedSpaces)

	return http.ListenAndServe(addr, mux)
}

func printExpressionTree(w http.ResponseWriter, expr parser.Expr, depth int) {
	indent := strings.Repeat("  ", depth)
	
	switch e := expr.(type) {
	case *parser.VectorSelector:
		fmt.Fprintf(w, "%sVectorSelector: %s", indent, e.Name)
		if len(e.LabelMatchers) > 0 {
			fmt.Fprintf(w, "{")
			for i, m := range e.LabelMatchers {
				if i > 0 {
					fmt.Fprintf(w, ", ")
				}
				fmt.Fprintf(w, "%s%s%q", m.Name, m.Type, m.Value)
			}
			fmt.Fprintf(w, "}")
		}
		fmt.Fprintf(w, "\n")
		
	case *parser.MatrixSelector:
		fmt.Fprintf(w, "%sMatrixSelector[%v]\n", indent, time.Duration(e.Range))
		if e.VectorSelector != nil {
			printExpressionTree(w, e.VectorSelector, depth+1)
		}
		
	case *parser.Call:
		fmt.Fprintf(w, "%sCall: %s()\n", indent, e.Func.Name)
		for _, arg := range e.Args {
			printExpressionTree(w, arg, depth+1)
		}
		
	case *parser.AggregateExpr:
		groupInfo := ""
		if len(e.Grouping) > 0 {
			groupType := "by"
			if e.Without {
				groupType = "without"
			}
			groupInfo = fmt.Sprintf(" %s (%s)", groupType, strings.Join(e.Grouping, ", "))
		}
		fmt.Fprintf(w, "%sAggregateExpr: %s%s\n", indent, e.Op, groupInfo)
		if e.Expr != nil {
			printExpressionTree(w, e.Expr, depth+1)
		}
		if e.Param != nil {
			printExpressionTree(w, e.Param, depth+1)
		}
		
	case *parser.BinaryExpr:
		fmt.Fprintf(w, "%sBinaryExpr: %s\n", indent, e.Op)
		if e.LHS != nil {
			fmt.Fprintf(w, "%s  LHS:\n", indent)
			printExpressionTree(w, e.LHS, depth+2)
		}
		if e.RHS != nil {
			fmt.Fprintf(w, "%s  RHS:\n", indent)
			printExpressionTree(w, e.RHS, depth+2)
		}
		
	case *parser.UnaryExpr:
		fmt.Fprintf(w, "%sUnaryExpr: %s\n", indent, e.Op)
		if e.Expr != nil {
			printExpressionTree(w, e.Expr, depth+1)
		}
		
	case *parser.ParenExpr:
		fmt.Fprintf(w, "%sParenExpr\n", indent)
		if e.Expr != nil {
			printExpressionTree(w, e.Expr, depth+1)
		}
		
	case *parser.NumberLiteral:
		fmt.Fprintf(w, "%sNumberLiteral: %g\n", indent, e.Val)
		
	case *parser.StringLiteral:
		fmt.Fprintf(w, "%sStringLiteral: %q\n", indent, e.Val)
		
	case *parser.SubqueryExpr:
		fmt.Fprintf(w, "%sSubqueryExpr[%v:%v]\n", indent, time.Duration(e.Range), time.Duration(e.Step))
		if e.Expr != nil {
			printExpressionTree(w, e.Expr, depth+1)
		}
		
	default:
		fmt.Fprintf(w, "%s%T\n", indent, expr)
	}
}
