package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// Insert metadata if needed
	mdContent := string(mdBytes)

	if !strings.Contains(mdContent, "{{name}}") {
		mdContent = "Name: Automatically Inserted\n\n" + mdContent
	}
	if !strings.Contains(mdContent, "{{date}}") {
		mdContent = "Datum: " + time.Now().Format("02.01.2006") + "\n\n" + mdContent
	}

	// Replace tags
	userName := os.Getenv("USER")
	if userName == "" {
		userName = "Unknown User"
	}

	mdContent = strings.ReplaceAll(mdContent, "{{name}}", userName)
	mdContent = strings.ReplaceAll(mdContent, "{{date}}", time.Now().Format("02.01.2006"))

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
