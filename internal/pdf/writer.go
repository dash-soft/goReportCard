package pdf

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/jung-kurt/gofpdf"
)

type Writer struct {
	pdf              *gofpdf.Fpdf
	logoOpt          gofpdf.ImageOptions
	logoWidth        float64
	logoHeight       float64
	tempFiles        []string // Track temp files for cleanup
	lastHeadingLevel int      // Track last heading level to detect section boundaries
	lastLevel2Y      float64  // Track Y position of last level 2 heading
	lastLevel2Page   int      // Track page number of last level 2 heading
	// PDF metadata
	author  string
	date    string
	project string
}

func NewWriter() *Writer {
	p := gofpdf.New("P", "mm", "A4", "")
	var tempFiles []string

	// Register embedded fonts - must use custom fonts only, never default fonts
	// Write TTF to temp files since AddUTF8Font requires file paths
	// Create temp file for Italic font
	italicTempFile, err := os.CreateTemp("", "MapleMono-Italic-*.ttf")
	if err == nil {
		italicFile := italicTempFile.Name()
		if _, err := italicTempFile.Write(FontItalic); err == nil {
			italicTempFile.Close()
			// Get absolute path to ensure gofpdf can find it
			italicFile, _ = filepath.Abs(italicFile)
			// Set font location to the directory containing the font
			p.SetFontLocation(filepath.Dir(italicFile))
			p.AddUTF8Font("Mono-Italic", "", filepath.Base(italicFile))
			tempFiles = append(tempFiles, italicFile)
		} else {
			italicTempFile.Close()
			os.Remove(italicFile)
		}
	}

	// Create temp file for BoldItalic font
	boldItalicTempFile, err := os.CreateTemp("", "MapleMono-BoldItalic-*.ttf")
	if err == nil {
		boldItalicFile := boldItalicTempFile.Name()
		if _, err := boldItalicTempFile.Write(FontBoldItalic); err == nil {
			boldItalicTempFile.Close()
			// Get absolute path
			boldItalicFile, _ = filepath.Abs(boldItalicFile)
			// Font location already set above, just add the font
			p.AddUTF8Font("Mono-BoldItalic", "", filepath.Base(boldItalicFile))
			tempFiles = append(tempFiles, boldItalicFile)
		} else {
			boldItalicTempFile.Close()
			os.Remove(boldItalicFile)
		}
	}

	// Set default font to custom font
	p.SetFont("Mono-Italic", "", 12)

	// Set margins: left, top, right
	p.SetMargins(20, 30, 20)

	// Register logo image once
	r := bytes.NewReader(Logo)
	opt := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	p.RegisterImageOptionsReader("logo", opt, r)

	// Get logo dimensions for header placement
	logoWidth := 40.0 // Width in mm
	logoHeight := 0.0 // Auto height

	// Set header function to draw logo on every page
	p.SetHeaderFunc(func() {
		// Get page width
		pageWidth, _ := p.GetPageSize()
		// Position logo in upper right corner with margin
		logoX := pageWidth - 20 - logoWidth
		logoY := 10.0
		p.ImageOptions("logo", logoX, logoY, logoWidth, logoHeight, false, opt, 0, "")
	})

	// Get system metadata for footer
	systemInfo := getSystemMetadata()

	// Set footer function to display system metadata on every page
	p.SetFooterFunc(func() {
		// Use custom font - never default fonts
		p.SetFont("Mono-Italic", "", 9)

		// Get page dimensions
		pageWidth, pageHeight := p.GetPageSize()

		// Position footer text at bottom center
		footerY := pageHeight - 15.0 // 15mm from bottom
		footerText := "Report generated on: " + systemInfo + " - " + time.Now().Format("02.01.2006")

		// Center the text
		p.SetXY(0, footerY)
		p.CellFormat(pageWidth, 5, footerText, "", 0, "C", false, 0, "")
	})

	// Add first page
	p.AddPage()

	return &Writer{
		pdf:        p,
		logoOpt:    opt,
		logoWidth:  logoWidth,
		logoHeight: logoHeight,
		tempFiles:  tempFiles,
	}
}

func (w *Writer) WriteHeading(level int, text string) {
	if text == "" {
		return
	}

	// Use different font sizes for different heading levels
	// Level 2 (##) should be noticeably larger than level 3 (###)
	var size float64
	switch level {
	case 1:
		size = 20.0 // Largest for main sections
	case 2:
		size = 16.0 // Medium-large for major subsections
	case 3:
		size = 14.0 // Medium for sub-subsections
	case 4:
		size = 13.0
	case 5:
		size = 12.5
	default:
		size = 12.0
	}

	// Use custom font - never default fonts
	w.pdf.SetFont("Mono-BoldItalic", "", size)

	// Check if we need a new page to avoid splitting sections
	_, y := w.pdf.GetXY()
	_, pageHeight := w.pdf.GetPageSize()
	marginBottom := 20.0 // Bottom margin
	remainingSpace := pageHeight - y - marginBottom

	// Calculate minimum space needed for the heading and its content
	// Only break when we actually don't have enough space, not just based on position
	var minSpaceNeeded float64

	if level == 2 {
		// Level 2: need space for heading + potential level 3 subsection + content
		minSpaceNeeded = 100.0 // Conservative estimate
	} else if level == 3 {
		// Level 3: need space for heading + content
		minSpaceNeeded = 60.0
	} else if level == 1 {
		minSpaceNeeded = 80.0
	} else {
		minSpaceNeeded = 40.0
	}

	// Check if parent level 2 is on same page (for level 3 headings)
	// If parent is very low on page and we need to break, be more aggressive
	var shouldBreak bool
	if level == 3 {
		currentPage := w.pdf.PageNo()
		if w.lastLevel2Page == currentPage && w.lastLevel2Y > 0 {
			// Check if we actually need to break
			needsBreakBySpace := remainingSpace < minSpaceNeeded

			if needsBreakBySpace {
				// We need to break - check if parent is low on page
				// If parent is in bottom 50% of page, break to keep them together
				if w.lastLevel2Y > pageHeight*0.50 {
					// Parent is low, definitely break to keep parent+child together
					shouldBreak = true
				} else if w.lastLevel2Y > pageHeight*0.40 && remainingSpace < minSpaceNeeded*1.2 {
					// Parent is getting low and space is tight, break
					shouldBreak = true
				} else {
					// Parent is fine, just break if we need space
					shouldBreak = remainingSpace < minSpaceNeeded
				}
			} else {
				// We have enough space, don't break
				shouldBreak = false
			}
		} else {
			// No parent on this page, just check space
			shouldBreak = remainingSpace < minSpaceNeeded
		}
	} else {
		// For other levels, only break if we don't have enough space
		// Use position as a secondary check only when space is borderline
		shouldBreak = remainingSpace < minSpaceNeeded

		// For level 2, also check position if space is borderline (within 20% of min needed)
		if level == 2 && remainingSpace < minSpaceNeeded*1.2 {
			// Space is tight - check if we're in bottom portion of page
			if y > pageHeight*0.65 {
				// We're in bottom 35% and space is tight, break
				shouldBreak = true
			}
		}
	}

	// Add spacing before heading (except for first heading)
	if y > 40 { // Not at the very top
		if shouldBreak {
			w.pdf.AddPage()
		} else {
			w.pdf.Ln(4)
		}
	}

	// Track level 2 heading position BEFORE writing (for parent-child relationship)
	if level == 2 {
		_, y := w.pdf.GetXY()
		w.lastLevel2Y = y
		w.lastLevel2Page = w.pdf.PageNo()
	}

	w.pdf.CellFormat(0, 12, text, "", 1, "L", false, 0, "")
	w.pdf.Ln(3)

	// Track the heading level for content spacing decisions
	w.lastHeadingLevel = level
}

func (w *Writer) WriteParagraph(text string) {
	if text == "" {
		return
	}

	// Use custom font - never default fonts
	w.pdf.SetFont("Mono-Italic", "", 12)

	// Check if paragraph fits on current page, if not, add page break
	_, y := w.pdf.GetXY()
	_, pageHeight := w.pdf.GetPageSize()
	marginBottom := 20.0
	remainingSpace := pageHeight - y - marginBottom

	// Estimate height needed for paragraph (rough estimate: 6mm per line, assume 3-4 lines minimum)
	estimatedHeight := 24.0

	// If this paragraph follows a heading, be more aggressive about page breaks
	// Check if we're in the bottom portion of the page
	thresholdY := pageHeight * 0.65 // 65% down the page
	if w.lastHeadingLevel > 0 {
		// If we just wrote a heading and we're past threshold or don't have enough space, new page
		if y > thresholdY || remainingSpace < estimatedHeight+20.0 {
			w.pdf.AddPage()
		}
	} else if remainingSpace < estimatedHeight {
		w.pdf.AddPage()
	}

	w.pdf.MultiCell(0, 6, text, "", "L", false)
	w.pdf.Ln(4)

	// Reset heading level tracking after writing content
	w.lastHeadingLevel = 0
}

func (w *Writer) WriteText(text string) {
	if text == "" {
		return
	}

	// Use custom font - never default fonts
	w.pdf.SetFont("Mono-Italic", "", 12)
	w.pdf.Write(6, text)
}

func (w *Writer) WriteCode(code string) {
	if code == "" {
		return
	}

	// Use custom font - never default fonts like Courier
	w.pdf.SetFont("Mono-Italic", "", 11)

	// Check if code block fits on current page
	_, y := w.pdf.GetXY()
	_, pageHeight := w.pdf.GetPageSize()
	marginBottom := 20.0
	remainingSpace := pageHeight - y - marginBottom

	// Estimate height needed (rough estimate)
	estimatedHeight := 20.0
	if remainingSpace < estimatedHeight {
		w.pdf.AddPage()
	}

	w.pdf.SetFillColor(240, 240, 240)
	w.pdf.MultiCell(0, 6, code, "", "L", true)
	w.pdf.Ln(3)
}

func (w *Writer) WriteInlineCode(code string) {
	if code == "" {
		return
	}

	// Save current position
	x, y := w.pdf.GetXY()

	// Use monospace font for inline code
	w.pdf.SetFont("Mono-Italic", "", 11)

	// Light gray background for inline code
	w.pdf.SetFillColor(245, 245, 245)
	w.pdf.SetTextColor(0, 0, 0) // Ensure text is black

	// Calculate width of the code text
	width := w.pdf.GetStringWidth(code) + 4 // Add some padding

	// Draw background rectangle
	w.pdf.Rect(x, y-1, width, 13, "F")

	// Draw the code text
	w.pdf.Text(x+2, y, code)

	// Move cursor forward
	w.pdf.SetXY(x+width, y)

	// Restore to default paragraph font
	w.pdf.SetFont("Mono-Italic", "", 12)
}

// WriteThematicBreak renders a horizontal rule with subtle styling (like Microsoft Word)
func (w *Writer) WriteThematicBreak() {
	pageWidth, _ := w.pdf.GetPageSize()

	// Add some spacing before the rule
	w.pdf.Ln(6)
	_, y := w.pdf.GetXY()

	// Draw a subtle line (like Word's page break indicator)
	// Use a light gray color
	w.pdf.SetDrawColor(200, 200, 200)
	w.pdf.SetLineWidth(0.2)

	// Draw line with margins
	marginLeft := 20.0
	marginRight := 20.0
	lineY := y
	w.pdf.Line(marginLeft, lineY, pageWidth-marginRight, lineY)

	// Add spacing after
	w.pdf.Ln(6)
}

func (w *Writer) WriteListItem(text string, marker byte, index int) {
	if text == "" {
		return
	}

	// Use custom font - never default fonts
	w.pdf.SetFont("Mono-Italic", "", 12)

	// Check if list item fits on current page
	_, y := w.pdf.GetXY()
	_, pageHeight := w.pdf.GetPageSize()
	marginBottom := 20.0
	remainingSpace := pageHeight - y - marginBottom

	// Estimate height needed (at least one line: 6mm, but be conservative)
	estimatedHeight := 10.0

	// If this list item follows a heading, be more aggressive about page breaks
	thresholdY := pageHeight * 0.70 // 70% down the page
	if w.lastHeadingLevel > 0 && (y > thresholdY || remainingSpace < estimatedHeight) {
		w.pdf.AddPage()
	} else if remainingSpace < estimatedHeight {
		w.pdf.AddPage()
	}

	// Determine bullet/number prefix
	var prefix string
	if marker == '-' || marker == '+' || marker == '*' {
		prefix = "• "
	} else if marker == '.' || marker == ')' {
		// Ordered list - use number
		if index > 0 {
			prefix = fmt.Sprintf("%d. ", index)
		} else {
			prefix = "• " // Fallback if index not provided
		}
	} else {
		prefix = "• "
	}

	// Write bullet and text with proper indentation
	fullText := prefix + text
	w.pdf.MultiCell(0, 6, fullText, "", "L", false)
	w.pdf.Ln(2)

	// Reset heading level tracking after writing list content
	w.lastHeadingLevel = 0
}

func (w *Writer) WriteImageBytes(title string, path []byte) {
	// not implemented — later we can allow inline Base64 images
}

func (w *Writer) WriteHighlightedCode(code string, language string) error {
	if code == "" {
		return nil
	}

	// Use custom font - never default fonts like Courier
	w.pdf.SetFont("Mono-Italic", "", 11)

	// Check if code block fits on current page
	_, y := w.pdf.GetXY()
	_, pageHeight := w.pdf.GetPageSize()
	marginBottom := 20.0
	remainingSpace := pageHeight - y - marginBottom

	// Estimate height needed (rough estimate)
	estimatedHeight := 20.0
	if remainingSpace < estimatedHeight {
		w.pdf.AddPage()
	}

	// Apply syntax highlighting using chroma
	return w.renderSyntaxHighlightedCode(code, language)
}

// renderSyntaxHighlightedCode uses chroma to highlight code and render with colors
func (w *Writer) renderSyntaxHighlightedCode(code string, language string) error {
	// Get lexer for the language
	lexer := lexers.Get(language)
	if lexer == nil {
		// Fallback to auto-detection
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		// Ultimate fallback - treat as plain text
		lexer = lexers.Fallback
	}

	// Use GitHub-style theme
	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
	}

	// Tokenize the code
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		// Fallback to plain text rendering
		w.pdf.SetTextColor(0, 0, 0)
		w.pdf.MultiCell(0, 6, code, "", "L", false)
		return nil
	}

	// Render each token with appropriate color
	for {
		token := iterator()
		if token == chroma.EOF {
			break
		}

		// Get color for token type
		r, g, b := w.getChromaColor(style, token.Type)

		// Set color
		w.pdf.SetTextColor(r, g, b)

		// Write token value
		value := string(token.Value)
		if value == "\n" {
			w.pdf.Ln(6)
		} else {
			w.pdf.Write(6, value)
		}
	}

	return nil
}

// getChromaColor returns RGB color for a chroma token type
func (w *Writer) getChromaColor(style *chroma.Style, tokenType chroma.TokenType) (r, g, b int) {
	// Get the style entry for this token type
	entry := style.Get(tokenType)
	if entry.Colour.IsSet() {
		// Convert chroma.Color to RGB values
		color := entry.Colour
		r = int(color.Red())
		g = int(color.Green())
		b = int(color.Blue())
		return r, g, b
	}

	// Fallback colors based on token type
	switch tokenType {
	case chroma.Keyword:
		return 0, 0, 255 // Blue
	case chroma.String:
		return 0, 128, 0 // Green
	case chroma.Number:
		return 255, 0, 0 // Red
	case chroma.Comment, chroma.CommentSingle, chroma.CommentMultiline:
		return 128, 128, 128 // Gray
	case chroma.NameFunction:
		return 0, 0, 128 // Dark blue
	case chroma.NameClass:
		return 0, 100, 200 // Light blue
	case chroma.NameVariable:
		return 139, 69, 19 // Brown
	case chroma.Literal:
		return 255, 69, 0 // Orange red
	case chroma.NameBuiltin:
		return 0, 100, 0 // Dark green
	default:
		return 0, 0, 0 // Black
	}
}


// SetMetadata sets the PDF metadata fields
func (w *Writer) SetMetadata(author, date, project string) {
	w.author = author
	w.date = date
	w.project = project
}

func (w *Writer) Save(path string) error {
	// Set PDF metadata before saving
	if w.author != "" {
		w.pdf.SetAuthor(w.author, true)
	}
	if w.date != "" {
		w.pdf.SetCreationDate(time.Now())
		// Note: gofpdf doesn't have a direct SetDate method, but we can use SetTitle to include date info
	}
	if w.project != "" {
		w.pdf.SetTitle(w.project, true)
		w.pdf.SetSubject(fmt.Sprintf("Project: %s", w.project), true)
	}

	err := w.pdf.OutputFileAndClose(path)
	// Clean up temporary font files
	for _, tempFile := range w.tempFiles {
		os.Remove(tempFile)
	}
	return err
}

// getSystemMetadata returns OS-specific system information for the footer
func getSystemMetadata() string {
	switch runtime.GOOS {
	case "darwin":
		return getMacOSMetadata()
	case "linux":
		return getLinuxMetadata()
	case "windows":
		return "Microsoft Windows"
	default:
		return runtime.GOOS
	}
}

// getMacOSMetadata returns macOS version and Mac model
func getMacOSMetadata() string {
	var version, model string

	// Get macOS version using sw_vers
	if cmd := exec.Command("sw_vers", "-productVersion"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			version = strings.TrimSpace(string(output))
		}
	}

	// Get Mac model using system_profiler
	if cmd := exec.Command("system_profiler", "SPHardwareDataType"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Model Name:") || strings.Contains(line, "Model Identifier:") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						model = strings.TrimSpace(parts[1])
						// Prefer Model Name over Model Identifier
						if strings.Contains(line, "Model Name:") {
							break
						}
					}
				}
			}
		}
	}

	// Fallback if model not found
	if model == "" {
		if cmd := exec.Command("sysctl", "-n", "hw.model"); cmd != nil {
			if output, err := cmd.Output(); err == nil {
				model = strings.TrimSpace(string(output))
			}
		}
	}

	if version != "" && model != "" {
		return fmt.Sprintf("macOS %s %s", version, model)
	} else if version != "" {
		return fmt.Sprintf("macOS %s", version)
	} else if model != "" {
		return fmt.Sprintf("macOS on %s", model)
	}
	return "macOS"
}

// getLinuxMetadata returns Linux distribution information
func getLinuxMetadata() string {
	// Try to read /etc/os-release first (most common)
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		var name, version string
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				value := strings.TrimPrefix(line, "PRETTY_NAME=")
				value = strings.Trim(value, "\"")
				return value
			}
			if strings.HasPrefix(line, "NAME=") {
				name = strings.TrimPrefix(line, "NAME=")
				name = strings.Trim(name, "\"")
			}
			if strings.HasPrefix(line, "VERSION=") {
				version = strings.TrimPrefix(line, "VERSION=")
				version = strings.Trim(version, "\"")
			}
		}
		if name != "" {
			if version != "" {
				return fmt.Sprintf("%s %s", name, version)
			}
			return name
		}
	}

	// Fallback to /etc/issue
	if data, err := os.ReadFile("/etc/issue"); err == nil {
		line := strings.TrimSpace(string(data))
		// Remove escape sequences and newlines
		line = strings.ReplaceAll(line, "\\n", "")
		line = strings.ReplaceAll(line, "\\l", "")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	// Last resort
	return "Linux"
}
