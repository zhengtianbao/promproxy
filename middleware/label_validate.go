package middleware

import (
	"fmt"
	"slices"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
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
	type foundSpace struct {
		spaceValue string
		matcher    string
		valid      bool
	}

	var foundSpaces []foundSpace

	// 遍历AST查找所有的VectorSelector
	parser.Inspect(ctx.ParsedAST, func(node parser.Node, path []parser.Node) error {
		if vs, ok := node.(*parser.VectorSelector); ok {
			spaceFound := false
			for _, matcher := range vs.LabelMatchers {
				if matcher.Name == "space" {
					spaceFound = true

					if matcher.Type == labels.MatchEqual {
						hasValidSpace := slices.Contains(m.AllowedSpaces, matcher.Value)
						foundSpaces = append(foundSpaces, foundSpace{
							spaceValue: matcher.Value,
							matcher:    matcher.Type.String(),
							valid:      hasValidSpace})
					} else if matcher.Type == labels.MatchRegexp {
						values := strings.Split(matcher.Value, "|")
						allVaild := true
						for _, value := range values {
							if !slices.Contains(m.AllowedSpaces, value) {
								allVaild = false
								break
							}
						}
						foundSpaces = append(foundSpaces, foundSpace{
							spaceValue: matcher.Value,
							matcher:    matcher.Type.String(),
							valid:      allVaild})
					}
				}
			}

			if !spaceFound {
				foundSpaces = append(foundSpaces, foundSpace{
					spaceValue: "<missing>",
					matcher:    " ",
					valid:      false})
			}
		}
		return nil
	})

	if len(foundSpaces) == 0 {
		return fmt.Errorf("query must contain at least one metric with a 'space' label")
	}

	for _, s := range foundSpaces {
		if s.spaceValue == "<missing>" {
			return fmt.Errorf("all metrics in the query must have a 'space' label")
		}
		if !s.valid {
			return fmt.Errorf("space values %v with matcher %v are not allowed", s.spaceValue, s.matcher)
		}
	}

	return nil
}
