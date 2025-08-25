package pdf

import (
	"fmt"
	"io"
	"sort"
	"strings"

	gopdf "github.com/dslipak/pdf"
)

// DsliPakDocument implements the Document interface using dslipak/pdf library
type DsliPakDocument struct {
	reader   *gopdf.Reader
	filepath string
	pages    []Page
	metadata Metadata
}

// OpenWithDslipak opens a PDF file using the dslipak/pdf library
func OpenWithDslipak(filepath string) (Document, error) {
	r, err := gopdf.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF with dslipak: %w", err)
	}
	
	doc := &DsliPakDocument{
		reader:   r,
		filepath: filepath,
	}
	
	// Extract metadata
	doc.extractMetadata()
	
	// Initialize pages
	if err := doc.initializePages(); err != nil {
		return nil, fmt.Errorf("failed to initialize pages: %w", err)
	}
	
	return doc, nil
}

// extractMetadata extracts PDF metadata
func (d *DsliPakDocument) extractMetadata() {
	// The dslipak/pdf library doesn't directly expose metadata
	// We'll leave this empty for now
	d.metadata = Metadata{}
	
	// In a real implementation, we would parse the PDF's info dictionary
	// which contains metadata like Title, Author, etc.
}

// initializePages initializes all pages in the document
func (d *DsliPakDocument) initializePages() error {
	pageCount := d.reader.NumPage()
	d.pages = make([]Page, pageCount)
	
	for i := 1; i <= pageCount; i++ {
		page, err := NewDsliPakPage(d.reader, i)
		if err != nil {
			return fmt.Errorf("failed to initialize page %d: %w", i, err)
		}
		d.pages[i-1] = page
	}
	
	return nil
}

// GetMetadata returns the PDF metadata
func (d *DsliPakDocument) GetMetadata() Metadata {
	return d.metadata
}

// GetPages returns all pages in the document
func (d *DsliPakDocument) GetPages() []Page {
	return d.pages
}

// GetPage returns a specific page by index (0-based)
func (d *DsliPakDocument) GetPage(index int) (Page, error) {
	if index < 0 || index >= len(d.pages) {
		return nil, fmt.Errorf("page index %d out of range [0, %d)", index, len(d.pages))
	}
	return d.pages[index], nil
}

// PageCount returns the total number of pages
func (d *DsliPakDocument) PageCount() int {
	return len(d.pages)
}

// Close releases resources associated with the document
func (d *DsliPakDocument) Close() error {
	d.reader = nil
	d.pages = nil
	return nil
}

// DsliPakPage implements the Page interface using dslipak/pdf
type DsliPakPage struct {
	reader     *gopdf.Reader
	pageNumber int
	page       gopdf.Page
	width      float64
	height     float64
	bbox       BoundingBox
	objects    Objects
}

// NewDsliPakPage creates a new page using dslipak/pdf
func NewDsliPakPage(reader *gopdf.Reader, pageNumber int) (Page, error) {
	if pageNumber < 1 || pageNumber > reader.NumPage() {
		return nil, fmt.Errorf("invalid page number: %d", pageNumber)
	}
	
	page := reader.Page(pageNumber)
	
	// Get page dimensions - default to US Letter if not available
	// The dslipak/pdf library doesn't expose MediaBox directly
	width := 612.0  // 8.5 inches in points
	height := 792.0 // 11 inches in points
	
	p := &DsliPakPage{
		reader:     reader,
		pageNumber: pageNumber,
		page:       page,
		width:      width,
		height:     height,
		bbox: BoundingBox{
			X0: 0,
			Y0: 0,
			X1: width,
			Y1: height,
		},
	}
	
	// Extract objects from the page
	if err := p.extractObjects(); err != nil {
		return nil, fmt.Errorf("failed to extract objects: %w", err)
	}
	
	return p, nil
}

// extractObjects extracts all objects from the page
func (p *DsliPakPage) extractObjects() error {
	p.objects = Objects{
		Chars:  []CharObject{},
		Lines:  []LineObject{},
		Rects:  []RectObject{},
		Curves: []CurveObject{},
		Images: []ImageObject{},
		Annos:  []AnnotationObject{},
	}
	
	// Extract text content
	content := p.page.Content()
	p.extractTextObjects(content)
	
	return nil
}

// extractTextObjects extracts text objects from page content
func (p *DsliPakPage) extractTextObjects(content gopdf.Content) {
	for _, text := range content.Text {
		// Convert each text item to CharObjects
		x := text.X
		y := text.Y
		fontSize := text.FontSize // Use actual font size from PDF
		fontHeight := fontSize    // Approximate height as font size
		
		for _, ch := range text.S {
			if ch == ' ' || ch == '\n' || ch == '\r' {
				x += text.W / float64(len(text.S))
				continue
			}
			
			charWidth := text.W / float64(len(text.S))
			
			char := CharObject{
				Text:     string(ch),
				Font:     text.Font,
				FontSize: fontSize, // Use actual font size from PDF
				X0:       x,
				Y0:       y,
				X1:       x + charWidth,
				Y1:       y + fontHeight,
				Width:    charWidth,
				Height:   fontHeight,
				Color:    Color{R: 0, G: 0, B: 0, A: 255},
			}
			
			p.objects.Chars = append(p.objects.Chars, char)
			x += charWidth
		}
	}
}

// GetPageNumber returns the page number (1-based)
func (p *DsliPakPage) GetPageNumber() int {
	return p.pageNumber
}

// GetWidth returns the page width
func (p *DsliPakPage) GetWidth() float64 {
	return p.width
}

// GetHeight returns the page height
func (p *DsliPakPage) GetHeight() float64 {
	return p.height
}

// GetRotation returns the page rotation in degrees
func (p *DsliPakPage) GetRotation() int {
	return 0 // TODO: Extract from page.Rotate if available
}

// GetBBox returns the page bounding box
func (p *DsliPakPage) GetBBox() BoundingBox {
	return p.bbox
}

// GetObjects returns all objects on the page
func (p *DsliPakPage) GetObjects() Objects {
	return p.objects
}

// ExtractText extracts text from the page
func (p *DsliPakPage) ExtractText(opts ...TextExtractionOption) string {
	// Apply options
	config := &textExtractionConfig{
		Layout:     false,
		XTolerance: 3.0,
		YTolerance: 3.0,
	}
	for _, opt := range opts {
		opt(config)
	}
	
	// If layout mode is enabled, use the text organizer
	if config.Layout && len(p.objects.Chars) > 0 {
		// This would use the TextOrganizer to preserve layout
		// For now, we'll use the simple extraction
	}
	
	// Simple text extraction from content
	content := p.page.Content()
	
	var text strings.Builder
	for _, item := range content.Text {
		text.WriteString(item.S)
		if !strings.HasSuffix(item.S, " ") && !strings.HasSuffix(item.S, "\n") {
			text.WriteString(" ")
		}
	}
	
	return text.String()
}

// ExtractTables extracts tables from the page
func (p *DsliPakPage) ExtractTables(opts ...TableExtractionOption) []Table {
	// TODO: Implement table extraction
	return []Table{}
}

// Crop returns a new page cropped to the specified bounding box
func (p *DsliPakPage) Crop(bbox BoundingBox) Page {
	// Create a new page with cropped dimensions
	croppedPage := &DsliPakPage{
		reader:     p.reader,
		pageNumber: p.pageNumber,
		page:       p.page,
		width:      bbox.Width(),
		height:     bbox.Height(),
		bbox:       bbox,
		objects:    p.filterObjectsInBBox(bbox),
	}
	
	return croppedPage
}

// WithinBBox filters objects within a bounding box
func (p *DsliPakPage) WithinBBox(bbox BoundingBox) Objects {
	return p.filterObjectsInBBox(bbox)
}

// Filter filters objects based on a predicate function
func (p *DsliPakPage) Filter(predicate func(Object) bool) Objects {
	filtered := Objects{
		Chars:  []CharObject{},
		Lines:  []LineObject{},
		Rects:  []RectObject{},
		Curves: []CurveObject{},
		Images: []ImageObject{},
		Annos:  []AnnotationObject{},
	}
	
	for _, obj := range p.objects.Chars {
		if predicate(obj) {
			filtered.Chars = append(filtered.Chars, obj)
		}
	}
	
	for _, obj := range p.objects.Lines {
		if predicate(obj) {
			filtered.Lines = append(filtered.Lines, obj)
		}
	}
	
	for _, obj := range p.objects.Rects {
		if predicate(obj) {
			filtered.Rects = append(filtered.Rects, obj)
		}
	}
	
	return filtered
}

// ExtractWords extracts individual words from the page
func (p *DsliPakPage) ExtractWords(opts ...WordExtractionOption) []Word {
	// Apply options
	config := &wordExtractionConfig{
		XTolerance: 3.0,
		YTolerance: 3.0,
	}
	for _, opt := range opts {
		opt(config)
	}
	
	if len(p.objects.Chars) == 0 {
		return nil
	}
	
	// Sort characters by position (top to bottom, left to right)
	sortedChars := make([]CharObject, len(p.objects.Chars))
	copy(sortedChars, p.objects.Chars)
	
	sort.Slice(sortedChars, func(i, j int) bool {
		// First sort by Y position (top to bottom)
		if abs(sortedChars[i].Y0-sortedChars[j].Y0) > config.YTolerance {
			return sortedChars[i].Y0 < sortedChars[j].Y0
		}
		// Then sort by X position (left to right)
		return sortedChars[i].X0 < sortedChars[j].X0
	})
	
	// Group characters into lines
	var lines [][]CharObject
	var currentLine []CharObject
	currentY := sortedChars[0].Y0
	
	for _, char := range sortedChars {
		// Check if this character is on a new line
		if abs(char.Y0-currentY) > config.YTolerance {
			if len(currentLine) > 0 {
				lines = append(lines, currentLine)
			}
			currentLine = []CharObject{char}
			currentY = char.Y0
		} else {
			currentLine = append(currentLine, char)
		}
	}
	
	// Add the last line
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}
	
	// Extract words from each line
	var words []Word
	for _, line := range lines {
		lineWords := p.extractWordsFromLine(line, config.XTolerance)
		words = append(words, lineWords...)
	}
	
	return words
}

// extractWordsFromLine extracts words from a single line of characters
func (p *DsliPakPage) extractWordsFromLine(lineChars []CharObject, xTolerance float64) []Word {
	if len(lineChars) == 0 {
		return nil
	}
	
	// Sort by X position
	sort.Slice(lineChars, func(i, j int) bool {
		return lineChars[i].X0 < lineChars[j].X0
	})
	
	var words []Word
	var currentWord []CharObject
	
	for i, char := range lineChars {
		if i == 0 {
			currentWord = []CharObject{char}
		} else {
			// Check if this character starts a new word
			gap := char.X0 - lineChars[i-1].X1
			if gap > xTolerance || gap > char.Width*0.3 {
				// Save current word and start new one
				if len(currentWord) > 0 {
					words = append(words, p.createWord(currentWord))
				}
				currentWord = []CharObject{char}
			} else {
				currentWord = append(currentWord, char)
			}
		}
	}
	
	// Add the last word
	if len(currentWord) > 0 {
		words = append(words, p.createWord(currentWord))
	}
	
	return words
}

// createWord creates a Word from a group of characters
func (p *DsliPakPage) createWord(chars []CharObject) Word {
	var text strings.Builder
	minX, minY := chars[0].X0, chars[0].Y0
	maxX, maxY := chars[0].X1, chars[0].Y1
	
	for _, char := range chars {
		text.WriteString(char.Text)
		minX = min(minX, char.X0)
		minY = min(minY, char.Y0)
		maxX = max(maxX, char.X1)
		maxY = max(maxY, char.Y1)
	}
	
	return Word{
		Text:       text.String(),
		X0:         minX,
		Y0:         minY,
		X1:         maxX,
		Y1:         maxY,
		Characters: chars,
	}
}


// ToImage renders the page to an image (for visual debugging)
func (p *DsliPakPage) ToImage(opts ...ImageOption) (io.Reader, error) {
	return nil, fmt.Errorf("image rendering not yet implemented")
}

// filterObjectsInBBox filters objects that are within the given bounding box
func (p *DsliPakPage) filterObjectsInBBox(bbox BoundingBox) Objects {
	filtered := Objects{
		Chars:  []CharObject{},
		Lines:  []LineObject{},
		Rects:  []RectObject{},
		Curves: []CurveObject{},
		Images: []ImageObject{},
		Annos:  []AnnotationObject{},
	}
	
	for _, obj := range p.objects.Chars {
		if bbox.Intersects(obj.GetBBox()) {
			filtered.Chars = append(filtered.Chars, obj)
		}
	}
	
	for _, obj := range p.objects.Lines {
		if bbox.Intersects(obj.GetBBox()) {
			filtered.Lines = append(filtered.Lines, obj)
		}
	}
	
	for _, obj := range p.objects.Rects {
		if bbox.Intersects(obj.GetBBox()) {
			filtered.Rects = append(filtered.Rects, obj)
		}
	}
	
	return filtered
}