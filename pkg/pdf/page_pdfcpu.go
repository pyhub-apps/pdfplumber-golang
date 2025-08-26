package pdf

import (
	"fmt"
	"io"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFCPUPage implements the Page interface using pdfcpu
type PDFCPUPage struct {
	ctx        *model.Context
	pageNumber int
	pageDict   types.Dict
	width      float64
	height     float64
	rotation   int
	objects    Objects
	content    []byte
}

// NewPDFCPUPage creates a new page using pdfcpu context
func NewPDFCPUPage(ctx *model.Context, pageNumber int) (*PDFCPUPage, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}

	if pageNumber < 1 || pageNumber > ctx.PageCount {
		return nil, fmt.Errorf("page number %d out of range [1, %d]", pageNumber, ctx.PageCount)
	}

	// Get page dictionary and inherited attributes
	pageDict, _, attrs, err := ctx.PageDict(pageNumber, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get page dict: %w", err)
	}

	// Get page dimensions from MediaBox
	var width, height float64
	if attrs != nil && attrs.MediaBox != nil {
		width = attrs.MediaBox.Width()
		height = attrs.MediaBox.Height()
	} else {
		// Default US Letter size
		width = 612
		height = 792
	}

	page := &PDFCPUPage{
		ctx:        ctx,
		pageNumber: pageNumber,
		pageDict:   pageDict,
		width:      width,
		height:     height,
		rotation:   0, // Will be extracted from attrs or page dict
		objects:    Objects{},
	}
	
	// Extract rotation from inherited attributes first, then from page dict
	if attrs != nil {
		page.rotation = attrs.Rotate
	} else if rot := pageDict["Rotate"]; rot != nil {
		if rotInt, ok := rot.(types.Integer); ok {
			page.rotation = int(rotInt)
		}
	}

	// Extract content stream
	if err := page.extractContent(); err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	return page, nil
}

// extractContent extracts the content stream from the page
func (p *PDFCPUPage) extractContent() error {
	contents := p.pageDict["Contents"]
	if contents == nil {
		return nil // No content
	}

	// fmt.Printf("[DEBUG] Contents type: %T\n", contents)

	var contentStreams [][]byte

	// Handle different content types - also check for value types
	switch v := contents.(type) {
	case *types.IndirectRef:
		// fmt.Println("[DEBUG] Case: *types.IndirectRef (pointer)")
		// Single content stream
		stream, found, err := p.ctx.DereferenceStreamDict(*v)
		if err != nil {
			return fmt.Errorf("failed to dereference content: %w", err)
		}
		if found && stream != nil {
			// fmt.Println("[DEBUG] Successfully got StreamDict via DereferenceStreamDict")
			decoded, err := decodeStream(stream)
			if err != nil {
				return fmt.Errorf("failed to decode stream: %w", err)
			}
			// fmt.Printf("[DEBUG] Decoded %d bytes\n", len(decoded))
			contentStreams = append(contentStreams, decoded)
		} else {
			// fmt.Println("[DEBUG] StreamDict not found")
		}
		
	case types.IndirectRef:
		// fmt.Println("[DEBUG] Case: types.IndirectRef (value)")
		// Single content stream (value type) - use DereferenceStreamDict
		streamDict, _, err := p.ctx.DereferenceStreamDict(v)
		if err != nil {
			return fmt.Errorf("failed to dereference stream: %w", err)
		}
		// fmt.Printf("[DEBUG] DereferenceStreamDict: found=%v, streamDict=%p\n", found, streamDict)
		
		if streamDict != nil {
			// Decode the stream
			if err := streamDict.Decode(); err != nil {
				return fmt.Errorf("failed to decode stream: %w", err)
			}
			// fmt.Printf("[DEBUG] Decoded %d bytes\n", len(streamDict.Content))
			contentStreams = append(contentStreams, streamDict.Content)
		}

	case types.Array:
		// fmt.Printf("[DEBUG] Case: types.Array with %d elements\n", len(v))
		// Multiple content streams
		for _, item := range v {
			// fmt.Printf("[DEBUG] Array element %d type: %T\n", i, item)
			
			// Try pointer type first
			if indRef, ok := item.(*types.IndirectRef); ok {
				// fmt.Printf("[DEBUG]   Element %d is *types.IndirectRef\n", i)
				streamDict, _, err := p.ctx.DereferenceStreamDict(*indRef)
				if err != nil {
					// fmt.Printf("[DEBUG]   Failed to dereference stream: %v\n", err)
					continue
				}
				// fmt.Printf("[DEBUG]   DereferenceStreamDict: found=%v\n", found)
				if streamDict != nil {
					if err := streamDict.Decode(); err != nil {
						// fmt.Printf("[DEBUG]   Failed to decode: %v\n", err)
						continue
					}
					// fmt.Printf("[DEBUG]   Decoded %d bytes\n", len(streamDict.Content))
					contentStreams = append(contentStreams, streamDict.Content)
				}
			} else if indRef, ok := item.(types.IndirectRef); ok {
				// Try value type
				// fmt.Printf("[DEBUG]   Element %d is types.IndirectRef (value)\n", i)
				streamDict, _, err := p.ctx.DereferenceStreamDict(indRef)
				if err != nil {
					// fmt.Printf("[DEBUG]   Failed to dereference stream: %v\n", err)
					continue
				}
				// fmt.Printf("[DEBUG]   DereferenceStreamDict: found=%v\n", found)
				if streamDict != nil {
					if err := streamDict.Decode(); err != nil {
						// fmt.Printf("[DEBUG]   Failed to decode: %v\n", err)
						continue
					}
					// fmt.Printf("[DEBUG]   Decoded %d bytes\n", len(streamDict.Content))
					contentStreams = append(contentStreams, streamDict.Content)
				}
			}
		}
	}

	// Combine all content streams
	// fmt.Printf("[DEBUG] Total content streams collected: %d\n", len(contentStreams))
	if len(contentStreams) > 0 {
		p.content = combineContentStreams(contentStreams)
		// fmt.Printf("[DEBUG] Combined content size: %d bytes\n", len(p.content))
	} else {
		// fmt.Println("[DEBUG] No content streams were extracted!")
	}

	return nil
}

// decodeStream decodes a stream dictionary
func decodeStream(stream *types.StreamDict) ([]byte, error) {
	// If content is already available, return it
	if len(stream.Content) > 0 {
		return stream.Content, nil
	}

	// Decode the stream
	if err := stream.Decode(); err != nil {
		return nil, err
	}

	return stream.Content, nil
}

// combineContentStreams combines multiple content streams
func combineContentStreams(streams [][]byte) []byte {
	var combined []byte
	for _, stream := range streams {
		combined = append(combined, stream...)
		combined = append(combined, '\n')
	}
	return combined
}

// GetPageNumber returns the page number (1-based)
func (p *PDFCPUPage) GetPageNumber() int {
	return p.pageNumber
}

// GetWidth returns the page width
func (p *PDFCPUPage) GetWidth() float64 {
	return p.width
}

// GetHeight returns the page height
func (p *PDFCPUPage) GetHeight() float64 {
	return p.height
}

// GetRotation returns the page rotation in degrees
func (p *PDFCPUPage) GetRotation() int {
	return p.rotation
}

// GetBBox returns the page bounding box
func (p *PDFCPUPage) GetBBox() BoundingBox {
	return BoundingBox{
		X0: 0,
		Y0: 0,
		X1: p.width,
		Y1: p.height,
	}
}

// GetObjects returns all objects on the page
func (p *PDFCPUPage) GetObjects() Objects {
	// Parse content stream if not already done
	// Check if we have parsed content by checking if we have any objects at all
	if len(p.objects.Chars) == 0 && len(p.objects.Lines) == 0 && len(p.objects.Rects) == 0 && len(p.content) > 0 {
		// fmt.Println("[DEBUG] Parsing content stream...")
		parser := NewContentStreamParser(p.ctx, p.pageDict)
		p.objects = parser.Parse(p.content)
		// fmt.Printf("[DEBUG] After parsing: %d chars, %d lines, %d rects\n", 
		//	len(p.objects.Chars), len(p.objects.Lines), len(p.objects.Rects))
	}
	return p.objects
}

// ExtractText extracts text from the page
func (p *PDFCPUPage) ExtractText(opts ...TextExtractionOption) string {
	objects := p.GetObjects()
	
	// Default options
	options := &textExtractionConfig{
		XTolerance: 3,
		YTolerance: 3,
	}
	
	// Apply custom options
	for _, opt := range opts {
		opt(options)
	}
	
	// Extract text from character objects
	var lines []string
	var currentLine []CharObject
	var lastY float64
	
	for _, char := range objects.Chars {
		// Check if we're on a new line
		if len(currentLine) > 0 && abs(char.Y0-lastY) > options.YTolerance {
			// Process current line
			lineText := extractLineText(currentLine, options.XTolerance)
			if lineText != "" {
				lines = append(lines, lineText)
			}
			currentLine = []CharObject{char}
		} else {
			currentLine = append(currentLine, char)
		}
		lastY = char.Y0
	}
	
	// Process last line
	if len(currentLine) > 0 {
		lineText := extractLineText(currentLine, options.XTolerance)
		if lineText != "" {
			lines = append(lines, lineText)
		}
	}
	
	return strings.Join(lines, "\n")
}

// extractLineText extracts text from a line of characters
func extractLineText(chars []CharObject, xTolerance float64) string {
	if len(chars) == 0 {
		return ""
	}
	
	// Sort characters by X position
	sortedChars := make([]CharObject, len(chars))
	copy(sortedChars, chars)
	sortCharsByPosition(sortedChars)
	
	var words []string
	var currentWord []string
	var lastX1 float64
	
	for i, char := range sortedChars {
		if i > 0 && char.X0-lastX1 > xTolerance {
			// Space between words
			if len(currentWord) > 0 {
				words = append(words, strings.Join(currentWord, ""))
				currentWord = []string{}
			}
		}
		currentWord = append(currentWord, char.Text)
		lastX1 = char.X1
	}
	
	// Add last word
	if len(currentWord) > 0 {
		words = append(words, strings.Join(currentWord, ""))
	}
	
	return strings.Join(words, " ")
}

// sortCharsByPosition sorts characters by their position (top-to-bottom, left-to-right)
func sortCharsByPosition(chars []CharObject) {
	// Simple bubble sort for now
	n := len(chars)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if chars[j].Y0 < chars[j+1].Y0 || 
			   (abs(chars[j].Y0-chars[j+1].Y0) < 1 && chars[j].X0 > chars[j+1].X0) {
				chars[j], chars[j+1] = chars[j+1], chars[j]
			}
		}
	}
}

// abs function is already defined in types.go

// ExtractWords extracts individual words from the page
func (p *PDFCPUPage) ExtractWords(opts ...WordExtractionOption) []Word {
	// Default configuration
	config := &wordExtractionConfig{
		XTolerance: 3.0,
		YTolerance: 3.0,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(config)
	}
	
	// Get all character objects
	objects := p.GetObjects()
	if len(objects.Chars) == 0 {
		return []Word{}
	}
	
	// Sort characters by position (Y first, then X)
	chars := make([]CharObject, len(objects.Chars))
	copy(chars, objects.Chars)
	sortCharsByPosition(chars)
	
	// Group characters into words
	var words []Word
	var currentWord []CharObject
	var lastChar *CharObject
	
	for i := range chars {
		char := &chars[i]
		
		// Check if this character belongs to the current word
		if lastChar != nil {
			// Different line?
			if abs(char.Y0-lastChar.Y0) > config.YTolerance {
				// Save current word and start new one
				if len(currentWord) > 0 {
					words = append(words, createWord(currentWord))
					currentWord = []CharObject{}
				}
			} else if char.X0-lastChar.X1 > config.XTolerance {
				// Too far horizontally - new word
				if len(currentWord) > 0 {
					words = append(words, createWord(currentWord))
					currentWord = []CharObject{}
				}
			}
		}
		
		currentWord = append(currentWord, *char)
		lastChar = char
	}
	
	// Add last word
	if len(currentWord) > 0 {
		words = append(words, createWord(currentWord))
	}
	
	return words
}

// createWord creates a Word from a group of characters
func createWord(chars []CharObject) Word {
	if len(chars) == 0 {
		return Word{}
	}
	
	// Calculate bounding box
	minX := chars[0].X0
	minY := chars[0].Y0
	maxX := chars[0].X1
	maxY := chars[0].Y1
	
	text := ""
	for _, char := range chars {
		text += char.Text
		if char.X0 < minX {
			minX = char.X0
		}
		if char.Y0 < minY {
			minY = char.Y0
		}
		if char.X1 > maxX {
			maxX = char.X1
		}
		if char.Y1 > maxY {
			maxY = char.Y1
		}
	}
	
	return Word{
		Text:       text,
		X0:         minX,
		Y0:         minY,
		X1:         maxX,
		Y1:         maxY,
		Characters: chars,
	}
}

// ExtractTables extracts tables from the page
func (p *PDFCPUPage) ExtractTables(opts ...TableExtractionOption) []Table {
	extractor := newTableExtractor(p, opts...)
	return extractor.ExtractTables()
}

// Crop returns a new page cropped to the specified bounding box
func (p *PDFCPUPage) Crop(bbox BoundingBox) Page {
	// TODO: Implement page cropping
	return p
}

// WithinBBox filters objects within a bounding box
func (p *PDFCPUPage) WithinBBox(bbox BoundingBox) Objects {
	objects := p.GetObjects()
	filtered := Objects{}
	
	for _, char := range objects.Chars {
		if char.GetBBox().Intersects(bbox) {
			filtered.Chars = append(filtered.Chars, char)
		}
	}
	
	for _, line := range objects.Lines {
		if line.GetBBox().Intersects(bbox) {
			filtered.Lines = append(filtered.Lines, line)
		}
	}
	
	for _, rect := range objects.Rects {
		if rect.GetBBox().Intersects(bbox) {
			filtered.Rects = append(filtered.Rects, rect)
		}
	}
	
	for _, curve := range objects.Curves {
		if curve.GetBBox().Intersects(bbox) {
			filtered.Curves = append(filtered.Curves, curve)
		}
	}
	
	return filtered
}

// Filter filters objects based on a predicate function
func (p *PDFCPUPage) Filter(predicate func(Object) bool) Objects {
	objects := p.GetObjects()
	filtered := Objects{}
	
	for _, char := range objects.Chars {
		if predicate(char) {
			filtered.Chars = append(filtered.Chars, char)
		}
	}
	
	for _, line := range objects.Lines {
		if predicate(line) {
			filtered.Lines = append(filtered.Lines, line)
		}
	}
	
	for _, rect := range objects.Rects {
		if predicate(rect) {
			filtered.Rects = append(filtered.Rects, rect)
		}
	}
	
	for _, curve := range objects.Curves {
		if predicate(curve) {
			filtered.Curves = append(filtered.Curves, curve)
		}
	}
	
	return filtered
}

// ToImage renders the page to an image (for visual debugging)
func (p *PDFCPUPage) ToImage(opts ...ImageOption) (io.Reader, error) {
	// TODO: Implement page rendering
	return nil, fmt.Errorf("not implemented")
}