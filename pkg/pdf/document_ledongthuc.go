package pdf

import (
	"fmt"
	"io"
	"sort"
	"strings"

	lpdf "github.com/ledongthuc/pdf"
)

// LedongthucDocument implements the Document interface using ledongthuc/pdf library
type LedongthucDocument struct {
	file     io.Closer
	reader   *lpdf.Reader
	filepath string
	pages    []Page
	metadata Metadata
}

// OpenWithLedongthuc opens a PDF file using the ledongthuc/pdf library
func OpenWithLedongthuc(filepath string) (Document, error) {
	f, r, err := lpdf.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF with ledongthuc: %w", err)
	}
	
	doc := &LedongthucDocument{
		file:     f,
		reader:   r,
		filepath: filepath,
	}
	
	// Extract metadata
	doc.extractMetadata()
	
	// Initialize pages
	if err := doc.initializePages(); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to initialize pages: %w", err)
	}
	
	return doc, nil
}

// extractMetadata extracts PDF metadata
func (d *LedongthucDocument) extractMetadata() {
	// Extract metadata from trailer/info dictionary if available
	d.metadata = Metadata{}
	
	// The ledongthuc/pdf library may have metadata in trailer
	// This needs to be implemented based on the library's capabilities
}

// initializePages initializes all pages in the document
func (d *LedongthucDocument) initializePages() error {
	pageCount := d.reader.NumPage()
	d.pages = make([]Page, pageCount)
	
	for i := 1; i <= pageCount; i++ {
		page, err := NewLedongthucPage(d.reader, i)
		if err != nil {
			return fmt.Errorf("failed to initialize page %d: %w", i, err)
		}
		d.pages[i-1] = page
	}
	
	return nil
}

// GetMetadata returns the PDF metadata
func (d *LedongthucDocument) GetMetadata() Metadata {
	return d.metadata
}

// GetPages returns all pages in the document
func (d *LedongthucDocument) GetPages() []Page {
	return d.pages
}

// GetPage returns a specific page by index (0-based)
func (d *LedongthucDocument) GetPage(index int) (Page, error) {
	if index < 0 || index >= len(d.pages) {
		return nil, fmt.Errorf("page index %d out of range [0, %d)", index, len(d.pages))
	}
	return d.pages[index], nil
}

// PageCount returns the total number of pages
func (d *LedongthucDocument) PageCount() int {
	return len(d.pages)
}

// Close releases resources associated with the document
func (d *LedongthucDocument) Close() error {
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

// LedongthucPage implements the Page interface using ledongthuc/pdf
type LedongthucPage struct {
	reader     *lpdf.Reader
	pageNumber int
	page       lpdf.Page
	width      float64
	height     float64
	bbox       BoundingBox
	objects    Objects
}

// NewLedongthucPage creates a new page using ledongthuc/pdf
func NewLedongthucPage(reader *lpdf.Reader, pageNumber int) (Page, error) {
	if pageNumber < 1 || pageNumber > reader.NumPage() {
		return nil, fmt.Errorf("invalid page number: %d", pageNumber)
	}
	
	page := reader.Page(pageNumber)
	
	// Get page dimensions from MediaBox
	width := 612.0  // Default to US Letter
	height := 792.0
	
	mediaBox := page.V.Key("MediaBox")
	if mediaBox.Kind() == lpdf.Array && mediaBox.Len() == 4 {
		// MediaBox is [x0, y0, x1, y1]
		x0 := mediaBox.Index(0).Float64()
		y0 := mediaBox.Index(1).Float64()
		x1 := mediaBox.Index(2).Float64()
		y1 := mediaBox.Index(3).Float64()
		width = x1 - x0
		height = y1 - y0
	}
	
	p := &LedongthucPage{
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
func (p *LedongthucPage) extractObjects() error {
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
func (p *LedongthucPage) extractTextObjects(content lpdf.Content) {
	for _, text := range content.Text {
		// For pdfplumber compatibility, we need to:
		// 1. Invert Y coordinates (PDF uses bottom-left, pdfplumber uses top-left)
		// 2. Extract individual characters with their positions
		
		// Calculate Y position with inverted coordinates
		// In PDF: Y increases upward from bottom
		// In pdfplumber: Y increases downward from top
		// text.Y is the baseline position in PDF
		// We need to calculate the top of the character for pdfplumber
		// Use actual font size from PDF
		fontSize := text.FontSize
		fontHeight := fontSize
		y_baseline_pdf := text.Y
		y_top_pdf := y_baseline_pdf + fontHeight*0.8 // Baseline is typically at 80% of font height
		y0_plumber := p.height - y_top_pdf
		
		// If text contains multiple characters, we need to split them
		// For now, treat each text item as a single unit
		// In a real implementation, we'd need to parse the font metrics
		// to get individual character positions
		
		chars := []rune(text.S)
		if len(chars) == 0 {
			continue
		}
		
		// Calculate approximate character width
		charWidth := text.W / float64(len(chars))
		x := text.X
		
		for _, ch := range chars {
			// Skip space characters as they're used for word separation
			if ch != ' ' {
				char := CharObject{
					Text:     string(ch),
					Font:     text.Font,
					FontSize: fontSize, // Use actual font size from PDF
					X0:       x,
					Y0:       y0_plumber,
					X1:       x + charWidth,
					Y1:       y0_plumber + fontHeight,
					Width:    charWidth,
					Height:   fontHeight,
					Color:    Color{R: 0, G: 0, B: 0, A: 255},
				}
				
				p.objects.Chars = append(p.objects.Chars, char)
			}
			x += charWidth
		}
	}
}

// GetPageNumber returns the page number (1-based)
func (p *LedongthucPage) GetPageNumber() int {
	return p.pageNumber
}

// GetWidth returns the page width
func (p *LedongthucPage) GetWidth() float64 {
	return p.width
}

// GetHeight returns the page height
func (p *LedongthucPage) GetHeight() float64 {
	return p.height
}

// GetRotation returns the page rotation in degrees
func (p *LedongthucPage) GetRotation() int {
	// Check for Rotate key in page dictionary
	rotate := p.page.V.Key("Rotate")
	if rotate.Kind() == lpdf.Integer {
		return int(rotate.Int64())
	}
	return 0
}

// GetBBox returns the page bounding box
func (p *LedongthucPage) GetBBox() BoundingBox {
	return p.bbox
}

// GetObjects returns all objects on the page
func (p *LedongthucPage) GetObjects() Objects {
	return p.objects
}

// ExtractText extracts text from the page
func (p *LedongthucPage) ExtractText(opts ...TextExtractionOption) string {
	// Apply options
	config := &textExtractionConfig{
		Layout:     false,
		XTolerance: 3.0,
		YTolerance: 3.0,
	}
	for _, opt := range opts {
		opt(config)
	}
	
	// Simple text extraction from content
	content := p.page.Content()
	
	var text strings.Builder
	for _, item := range content.Text {
		text.WriteString(item.S)
		// ledongthuc/pdf already handles spacing properly
	}
	
	return text.String()
}

// ExtractTables extracts tables from the page
func (p *LedongthucPage) ExtractTables(opts ...TableExtractionOption) []Table {
	// TODO: Implement table extraction
	return []Table{}
}

// Crop returns a new page cropped to the specified bounding box
func (p *LedongthucPage) Crop(bbox BoundingBox) Page {
	// Create a new page with cropped dimensions
	croppedPage := &LedongthucPage{
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
func (p *LedongthucPage) WithinBBox(bbox BoundingBox) Objects {
	return p.filterObjectsInBBox(bbox)
}

// Filter filters objects based on a predicate function
func (p *LedongthucPage) Filter(predicate func(Object) bool) Objects {
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
func (p *LedongthucPage) ExtractWords(opts ...WordExtractionOption) []Word {
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
func (p *LedongthucPage) extractWordsFromLine(lineChars []CharObject, xTolerance float64) []Word {
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
func (p *LedongthucPage) createWord(chars []CharObject) Word {
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
func (p *LedongthucPage) ToImage(opts ...ImageOption) (io.Reader, error) {
	return nil, fmt.Errorf("image rendering not yet implemented")
}

// filterObjectsInBBox filters objects that are within the given bounding box
func (p *LedongthucPage) filterObjectsInBBox(bbox BoundingBox) Objects {
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