package markdown

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
)

func ParseMarkdown(src []byte) (ast.Node, error) {
	reader := text.NewReader(src)
	doc := md.Parser().Parse(reader)
	return doc, nil
}
