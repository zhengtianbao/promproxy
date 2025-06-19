package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/prometheus/promql/parser"
)

type FunctionValidateMiddleware struct{}

func NewFunctionValidateMiddleware() *FunctionValidateMiddleware {
	return &FunctionValidateMiddleware{}
}

func (f *FunctionValidateMiddleware) Process(ctx *RequestContext) error {
	return f.validatePromQLFunctions(ctx)
}

func (f *FunctionValidateMiddleware) validatePromQLFunctions(ctx *RequestContext) error {
	var errors []string

	// 遍历AST查找函数调用
	parser.Inspect(ctx.ParsedAST, func(node parser.Node, path []parser.Node) error {
		if call, ok := node.(*parser.Call); ok {
			switch call.Func.Name {
			case "increase":
				if err := f.validateIncreaseFunction(call); err != nil {
					errors = append(errors, err.Error())
				}
			default:
				// 检查_over_time函数
				if strings.HasSuffix(call.Func.Name, "_over_time") {
					if err := f.validateOverTimeFunction(call); err != nil {
						errors = append(errors, err.Error())
					}
				}
			}
		}
		return nil
	})

	if len(errors) > 0 {
		return fmt.Errorf("function validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (f *FunctionValidateMiddleware) validateIncreaseFunction(call *parser.Call) error {
	if len(call.Args) == 0 {
		return fmt.Errorf("increase function requires arguments")
	}

	// 检查第一个参数是否为MatrixSelector
	if ms, ok := call.Args[0].(*parser.MatrixSelector); ok {
		duration := time.Duration(ms.Range)
		if duration > 24*time.Hour {
			return fmt.Errorf("increase function time range %v cannot exceed 24h", duration)
		}
	}

	return nil
}

func (f *FunctionValidateMiddleware) validateOverTimeFunction(call *parser.Call) error {
	if len(call.Args) == 0 {
		return fmt.Errorf("%s function requires arguments", call.Func.Name)
	}

	// 检查第一个参数是否为MatrixSelector
	if ms, ok := call.Args[0].(*parser.MatrixSelector); ok {
		duration := time.Duration(ms.Range)
		if duration > 24*time.Hour {
			return fmt.Errorf("%s function time range %v cannot exceed 24h", call.Func.Name, duration)
		}
	}

	return nil
}
