package middleware

import (
	"fmt"
	"slices"

	"github.com/prometheus/prometheus/promql/parser"
)

type LabelValidateMiddleware struct {
	AllowedSpaces []string
}

func NewLabelValidateMiddleware(allowedSpaces []string) *LabelValidateMiddleware {
	return &LabelValidateMiddleware{
		AllowedSpaces: allowedSpaces,
	}
}

func (m *LabelValidateMiddleware) Process(ctx *RequestContext) error {
	return m.validateSpaceLabel(ctx)
}

func (m *LabelValidateMiddleware) validateSpaceLabel(ctx *RequestContext) error {
	var hasValidSpace bool
	var foundSpaces []string

	// 遍历AST查找所有的VectorSelector
	parser.Inspect(ctx.ParsedAST, func(node parser.Node, path []parser.Node) error {
		if vs, ok := node.(*parser.VectorSelector); ok {
			spaceFound := false
			for _, matcher := range vs.LabelMatchers {
				if matcher.Name == "space" {
					spaceFound = true
					foundSpaces = append(foundSpaces, matcher.Value)

					hasValidSpace = slices.Contains(m.AllowedSpaces, matcher.Value)
				}
			}

			if !spaceFound {
				foundSpaces = append(foundSpaces, "<missing>")
			}
		}
		return nil
	})

	if len(foundSpaces) == 0 {
		return fmt.Errorf("query must contain at least one metric with a 'space' label")
	}

	// 检查是否有缺失space标签的情况
	for _, space := range foundSpaces {
		if space == "<missing>" {
			return fmt.Errorf("all metrics in the query must have a 'space' label")
		}
	}

	if !hasValidSpace {
		return fmt.Errorf("space values %v are not allowed", foundSpaces)
	}

	return nil
}
