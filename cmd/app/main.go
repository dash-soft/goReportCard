package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"report/internal/markdown"
	"report/internal/pdf"

	"github.com/yuin/goldmark/ast"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: report <input.md> <output.pdf>")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	// Read markdown
	mdBytes, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("Failed to read markdown file: %v\n", err)
		os.Exit(1)
	}

	// Extract metadata variables from markdown
	mdContent := string(mdBytes)

	// Extract __author__, __date__, __project__ from the content
	var author, date, project string

	// Use regex to find the variables
	authorRegex := regexp.MustCompile(`__author__\s*:\s*(.+)`)
	dateRegex := regexp.MustCompile(`__date__\s*:\s*(.+)`)
	projectRegex := regexp.MustCompile(`__project__\s*:\s*(.+)`)

	if matches := authorRegex.FindStringSubmatch(mdContent); len(matches) > 1 {
		author = strings.TrimSpace(matches[1])
	}
	if matches := dateRegex.FindStringSubmatch(mdContent); len(matches) > 1 {
		date = strings.TrimSpace(matches[1])
	}
	if matches := projectRegex.FindStringSubmatch(mdContent); len(matches) > 1 {
		project = strings.TrimSpace(matches[1])
	}

	// Convert back to []byte for parsing
	mdBytes = []byte(mdContent)

	// Parse markdown AST
	doc, err := markdown.ParseMarkdown(mdBytes)
	if err != nil {
		fmt.Printf("Markdown parsing error: %v\n", err)
		os.Exit(1)
	}

	// Type check: ensure doc is an AST document
	_, ok := doc.(*ast.Document)
	if !ok {
		fmt.Println("Parsed markdown root node is not a Document")
		os.Exit(1)
	}

	// Prepare PDF writer
	w := pdf.NewWriter()

	// Set PDF metadata
	w.SetMetadata(author, date, project)

	// Render markdown → PDF
	err = markdown.RenderToPDF(doc, w, mdBytes)
	if err != nil {
		fmt.Printf("PDF rendering error: %v\n", err)
		os.Exit(1)
	}

	// Save final PDF
	if err := w.Save(outputPath); err != nil {
		fmt.Printf("Failed to save PDF: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("PDF generated:", filepath.Base(outputPath))
}
