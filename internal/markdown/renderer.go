package markdown

import (
	"bytes"

	"report/internal/pdf"

	"github.com/yuin/goldmark/ast"
)

func RenderToPDF(n ast.Node, p *pdf.Writer, src []byte) error {
	return walk(n, p, src)
}

// extractText recursively extracts all text from a node and its children
// This handles nested structures like emphasis, strong, links, etc.
func extractText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	extractTextRecursive(n, &buf, src)
	return buf.String()
}

func extractTextRecursive(n ast.Node, buf *bytes.Buffer, src []byte) {
	switch node := n.(type) {
	case *ast.Text:
		buf.Write(node.Segment.Value(src))
	case *ast.String:
		buf.Write(node.Value)
	case *ast.CodeSpan:
		// Extract text from code span children
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			extractTextRecursive(child, buf, src)
		}
	case *ast.Link:
		// Extract text from link content
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			extractTextRecursive(child, buf, src)
		}
	case *ast.Emphasis:
		// Extract text from emphasis content
		// Level 1 = emphasis (italic), Level 2 = strong (bold)
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			extractTextRecursive(child, buf, src)
		}
	default:
		// For other node types, recursively process children
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			extractTextRecursive(child, buf, src)
		}
	}
}

func walk(n ast.Node, p *pdf.Writer, src []byte) error {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch node := child.(type) {
		case *ast.Heading:
			// Extract all text including nested structures
			text := extractText(node, src)
			if text != "" {
				p.WriteHeading(node.Level, text)
			}
			// Don't recurse into heading children - we've already extracted all text
			continue

		case *ast.Paragraph:
			// Extract all text including nested structures
			text := extractText(node, src)
			if text != "" {
				p.WriteParagraph(text)
			}
			// Don't recurse into paragraph children - we've already extracted all text
			continue

		case *ast.Image:
			// Images are handled but not implemented yet
			p.WriteImageBytes(
				string(node.Title),
				node.Destination,
			)

		case *ast.CodeBlock:
			// Extract code block content using Lines() method
			var codeBuf bytes.Buffer
			lines := node.Lines()
			if lines != nil {
				for i := 0; i < lines.Len(); i++ {
					segment := lines.At(i)
					codeBuf.Write(segment.Value(src))
				}
			}
			code := codeBuf.String()
			if code != "" {
				p.WriteCode(code)
			}
			// Don't recurse into code block - we've already extracted all content
			continue

		case *ast.ThematicBreak:
			// Horizontal rule - render with subtle styling (like Microsoft Word)
			p.WriteThematicBreak()
			// Don't recurse - thematic breaks have no children
			continue

		case *ast.List:
			// Process list items - iterate through children and render each item
			itemIndex := 0
			if node.IsOrdered() {
				itemIndex = node.Start
			}
			for item := node.FirstChild(); item != nil; item = item.NextSibling() {
				if listItem, ok := item.(*ast.ListItem); ok {
					// Extract all text from list item (including nested paragraphs, etc.)
					itemText := extractText(listItem, src)
					if itemText != "" {
						p.WriteListItem(itemText, node.Marker, itemIndex)
						if node.IsOrdered() {
							itemIndex++
						}
					}
				}
			}
			// Don't recurse - we've processed all list items
			continue

		case *ast.ListItem:
			// List items are handled within List nodes, but if we encounter one standalone,
			// extract and render it
			itemText := extractText(node, src)
			if itemText != "" {
				p.WriteListItem(itemText, '-', 0) // Default to bullet
			}
			// Don't recurse - we've extracted all text
			continue
		}

		// Recursively process children for nested structures
		// This ensures we don't miss any content in complex nodes
		if err := walk(child, p, src); err != nil {
			return err
		}
	}

	return nil
}
