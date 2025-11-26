package pdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

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

	// Be very aggressive about keeping sections together
	// Calculate threshold: if we're in the bottom portion of the page, start new page
	// For level 2 headings, be more conservative since level 3 might follow
	// For subsections (level 3+), be even more aggressive
	var thresholdY float64
	var minSpaceNeeded float64

	if level == 2 {
		// Level 2: be EXTREMELY aggressive to leave room for potential level 3 subsections
		// Break early (at 40% of page) to ensure parent+child stay together
		// This prevents orphaned level 2 headings when level 3 needs a new page
		thresholdY = pageHeight * 0.40 // 40% down - break very early
		minSpaceNeeded = 120.0         // Need lots of space for level 2 + at least one level 3 + content
	} else if level == 3 {
		// Level 3: be aggressive, and check if parent level 2 is on same page
		thresholdY = pageHeight * 0.55 // 55% down for subsections
		minSpaceNeeded = 70.0          // Subsections need substantial space

		// Check if parent level 2 is on the same page and near bottom
		// Only apply aggressive breaking if we actually don't have enough space
		currentPage := w.pdf.PageNo()
		if w.lastLevel2Page == currentPage && w.lastLevel2Y > 0 {
			// First check if we actually need to break (not enough space)
			needsBreak := y > thresholdY || remainingSpace < minSpaceNeeded

			if needsBreak {
				// We need to break - now check if parent is low on page
				// If parent is in bottom 40% of page, be more aggressive to keep them together
				if w.lastLevel2Y > pageHeight*0.40 {
					// Parent is very low on page, break immediately to keep parent+child together
					thresholdY = 0.0 // Break immediately
					minSpaceNeeded = 80.0
				} else if w.lastLevel2Y > pageHeight*0.30 {
					// Parent is low on page, be more aggressive
					thresholdY = pageHeight * 0.30 // Break earlier
					minSpaceNeeded = 85.0
				}
			}
			// If we have enough space, don't apply aggressive breaking - just use normal thresholds
		}
	} else if level == 1 {
		thresholdY = pageHeight * 0.60
		minSpaceNeeded = 90.0
	} else {
		thresholdY = pageHeight * 0.60
		minSpaceNeeded = 50.0
	}

	// Add spacing before heading (except for first heading)
	if y > 40 { // Not at the very top
		// Start new page if:
		// 1. We're past the threshold position on the page (or threshold is 0 for immediate break), OR
		// 2. We don't have enough remaining space
		shouldBreak := false
		if thresholdY == 0.0 {
			// Immediate break requested (for level 3 when parent is orphaned)
			shouldBreak = true
		} else {
			shouldBreak = y > thresholdY || remainingSpace < minSpaceNeeded
		}

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

func (w *Writer) Save(path string) error {
	err := w.pdf.OutputFileAndClose(path)
	// Clean up temporary font files
	for _, tempFile := range w.tempFiles {
		os.Remove(tempFile)
	}
	return err
}
