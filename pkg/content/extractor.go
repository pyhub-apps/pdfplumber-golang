package content

import (
	"fmt"
	"math"
	"strconv"

	"github.com/pyhub-apps/pdfplumber-golang/pkg/parser"
	"github.com/pyhub-apps/pdfplumber-golang/pkg/pdf"
)

// ContentExtractor extracts content from PDF pages
type ContentExtractor struct {
	page       *parser.PDFPage
	stateStack *StateStack
	chars      []pdf.CharObject
	lines      []pdf.LineObject
	rects      []pdf.RectObject
	curves     []pdf.CurveObject
	fonts      map[string]*FontInfo
}

// FontInfo holds font information
type FontInfo struct {
	Name     string
	BaseFont string
	Encoding string
	ToUnicode *pdf.ToUnicodeCMap
}

// NewContentExtractor creates a new content extractor
func NewContentExtractor(page *parser.PDFPage) *ContentExtractor {
	return &ContentExtractor{
		page:       page,
		stateStack: NewStateStack(),
		chars:      []pdf.CharObject{},
		lines:      []pdf.LineObject{},
		rects:      []pdf.RectObject{},
		curves:     []pdf.CurveObject{},
		fonts:      make(map[string]*FontInfo),
	}
}

// Extract extracts all content from the page
func (e *ContentExtractor) Extract() error {
	// Load fonts from resources
	if e.page.Resources != nil {
		if fontDict, ok := e.page.Resources.GetDict(parser.PDFName("Font")); ok {
			e.loadFonts(fontDict)
		}
	}

	// Process each content stream
	for i := range e.page.Contents {
		if err := e.processContentStream(e.page.Contents[i].Data); err != nil {
			return fmt.Errorf("error processing content stream: %v", err)
		}
	}

	return nil
}

// loadFonts loads font information from resources
func (e *ContentExtractor) loadFonts(fontDict parser.PDFDict) {
	for name, fontRef := range fontDict {
		fontInfo := &FontInfo{
			Name: string(name),
		}
		
		// Get the font object
		if ref, ok := fontRef.(parser.ObjectRef); ok {
			if fontObj, err := e.page.Document.GetObject(ref); err == nil {
				if font, ok := fontObj.(parser.PDFDict); ok {
					// Get BaseFont
					if baseFont := font.Get(parser.PDFName("BaseFont")); baseFont != nil {
						fontInfo.BaseFont = string(baseFont.(parser.PDFName))
					}
					
					// Get Encoding
					if encoding := font.Get(parser.PDFName("Encoding")); encoding != nil {
						if encName, ok := encoding.(parser.PDFName); ok {
							fontInfo.Encoding = string(encName)
						}
					}
					
					// Get ToUnicode CMap
					if toUnicode := font.Get(parser.PDFName("ToUnicode")); toUnicode != nil {
						if toUnicodeRef, ok := toUnicode.(parser.ObjectRef); ok {
							if toUnicodeObj, err := e.page.Document.GetObject(toUnicodeRef); err == nil {
								if stream, ok := toUnicodeObj.(*parser.PDFStream); ok {
									cmap := pdf.NewToUnicodeCMap()
									if err := cmap.Parse(stream.Data); err == nil {
										fontInfo.ToUnicode = cmap
									}
								}
							}
						}
					}
				}
			}
		}
		
		e.fonts[string(name)] = fontInfo
	}
}

// processContentStream processes a PDF content stream
func (e *ContentExtractor) processContentStream(data []byte) error {
	lexer := NewContentLexer(data)
	operands := []interface{}{}

	for {
		token, err := lexer.NextToken()
		if err != nil {
			break
		}

		if token.Type == TokenOperator {
			// Process operator with accumulated operands
			if err := e.processOperator(token.Value.(string), operands); err != nil {
				// Log error but continue processing
				fmt.Printf("Error processing operator %s: %v\n", token.Value, err)
			}
			operands = []interface{}{}
		} else {
			// Accumulate operand
			operands = append(operands, token.Value)
		}
	}

	return nil
}

// processOperator processes a PDF operator with its operands
func (e *ContentExtractor) processOperator(op string, operands []interface{}) error {
	state := e.stateStack.Current()

	switch op {
	// Graphics state operators
	case "q": // Save graphics state
		e.stateStack.Save()
		
	case "Q": // Restore graphics state
		e.stateStack.Restore()
		
	case "cm": // Concatenate matrix
		if len(operands) == 6 {
			matrix := Matrix{
				A: toFloat(operands[0]),
				B: toFloat(operands[1]),
				C: toFloat(operands[2]),
				D: toFloat(operands[3]),
				E: toFloat(operands[4]),
				F: toFloat(operands[5]),
			}
			state.CTM = state.CTM.Multiply(matrix)
		}

	// Path construction operators
	case "m": // Move to
		if len(operands) == 2 {
			x, y := toFloat(operands[0]), toFloat(operands[1])
			x, y = state.CTM.Transform(x, y)
			state.CurrentPoint = Point{X: x, Y: y}
			state.CurrentPath = append(state.CurrentPath, PathElement{
				Type:   "move",
				Points: []Point{{X: x, Y: y}},
			})
		}

	case "l": // Line to
		if len(operands) == 2 {
			x, y := toFloat(operands[0]), toFloat(operands[1])
			x, y = state.CTM.Transform(x, y)
			
			if len(state.CurrentPath) > 0 {
				e.lines = append(e.lines, pdf.LineObject{
					X0:          state.CurrentPoint.X,
					Y0:          state.CurrentPoint.Y,
					X1:          x,
					Y1:          y,
					Width:       state.LineWidth,
					StrokeColor: makeColor(state.StrokeColor),
				})
			}
			
			state.CurrentPoint = Point{X: x, Y: y}
			state.CurrentPath = append(state.CurrentPath, PathElement{
				Type:   "line",
				Points: []Point{{X: x, Y: y}},
			})
		}

	case "re": // Rectangle
		if len(operands) == 4 {
			x, y := toFloat(operands[0]), toFloat(operands[1])
			w, h := toFloat(operands[2]), toFloat(operands[3])
			
			// Transform corners
			x0, y0 := state.CTM.Transform(x, y)
			x1, y1 := state.CTM.Transform(x+w, y+h)
			
			// Ensure correct ordering
			if x0 > x1 {
				x0, x1 = x1, x0
			}
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			
			e.rects = append(e.rects, pdf.RectObject{
				X0:          x0,
				Y0:          y0,
				X1:          x1,
				Y1:          y1,
				Width:       state.LineWidth,
				StrokeColor: makeColor(state.StrokeColor),
				FillColor:   makeColor(state.FillColor),
			})
		}

	case "h": // Close path
		// Close current subpath
		state.CurrentPath = append(state.CurrentPath, PathElement{
			Type: "close",
		})
		
		// Check if this forms a rectangle
		if rect := e.detectRectangleFromPath(state.CurrentPath); rect != nil {
			// Apply current transformation matrix
			x0, y0 := state.CTM.Transform(rect.X0, rect.Y0)
			x1, y1 := state.CTM.Transform(rect.X1, rect.Y1)
			
			// Ensure correct ordering
			if x0 > x1 {
				x0, x1 = x1, x0
			}
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			
			e.rects = append(e.rects, pdf.RectObject{
				X0:          x0,
				Y0:          y0,
				X1:          x1,
				Y1:          y1,
				Width:       state.LineWidth,
				StrokeColor: makeColor(state.StrokeColor),
				FillColor:   makeColor(state.FillColor),
			})
		}

	// Path painting operators
	case "S", "s": // Stroke path
		// Path has been constructed, stroking creates visible lines
		if op == "s" {
			// Close path first
			state.CurrentPath = append(state.CurrentPath, PathElement{
				Type: "close",
			})
		}
		state.CurrentPath = []PathElement{}

	case "f", "F", "f*": // Fill path
		// Check if this is a rectangle being filled
		if rect := e.detectRectangleFromPath(state.CurrentPath); rect != nil {
			// Apply current transformation matrix
			x0, y0 := state.CTM.Transform(rect.X0, rect.Y0)
			x1, y1 := state.CTM.Transform(rect.X1, rect.Y1)
			
			// Ensure correct ordering
			if x0 > x1 {
				x0, x1 = x1, x0
			}
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			
			// Mark as filled rectangle
			e.rects = append(e.rects, pdf.RectObject{
				X0:          x0,
				Y0:          y0,
				X1:          x1,
				Y1:          y1,
				Width:       0, // Filled rectangles typically have no stroke width
				StrokeColor: makeColor(state.StrokeColor),
				FillColor:   makeColor(state.FillColor),
				Filled:      true,
			})
		}
		state.CurrentPath = []PathElement{}

	case "B", "B*", "b", "b*": // Fill and stroke
		// Check if this is a rectangle being filled and stroked
		if rect := e.detectRectangleFromPath(state.CurrentPath); rect != nil {
			// Apply current transformation matrix
			x0, y0 := state.CTM.Transform(rect.X0, rect.Y0)
			x1, y1 := state.CTM.Transform(rect.X1, rect.Y1)
			
			// Ensure correct ordering
			if x0 > x1 {
				x0, x1 = x1, x0
			}
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			
			// Mark as filled and stroked rectangle
			e.rects = append(e.rects, pdf.RectObject{
				X0:          x0,
				Y0:          y0,
				X1:          x1,
				Y1:          y1,
				Width:       state.LineWidth,
				StrokeColor: makeColor(state.StrokeColor),
				FillColor:   makeColor(state.FillColor),
				Filled:      true,
				Stroked:     true,
			})
		}
		state.CurrentPath = []PathElement{}

	case "n": // End path without fill or stroke
		state.CurrentPath = []PathElement{}

	// Text state operators
	case "BT": // Begin text
		state.TextMatrix = IdentityMatrix()
		state.TextLineMatrix = IdentityMatrix()

	case "ET": // End text
		// Reset text matrices

	case "Tf": // Set font and size
		if len(operands) == 2 {
			state.FontName = toString(operands[0])
			state.FontSize = toFloat(operands[1])
		}

	case "Td": // Move text position
		if len(operands) == 2 {
			tx, ty := toFloat(operands[0]), toFloat(operands[1])
			state.TextLineMatrix = state.TextLineMatrix.Multiply(Translate(tx, ty))
			state.TextMatrix = state.TextLineMatrix
		}

	case "TD": // Move text position and set leading
		if len(operands) == 2 {
			tx, ty := toFloat(operands[0]), toFloat(operands[1])
			state.Leading = -ty
			state.TextLineMatrix = state.TextLineMatrix.Multiply(Translate(tx, ty))
			state.TextMatrix = state.TextLineMatrix
		}

	case "Tm": // Set text matrix
		if len(operands) == 6 {
			state.TextMatrix = Matrix{
				A: toFloat(operands[0]),
				B: toFloat(operands[1]),
				C: toFloat(operands[2]),
				D: toFloat(operands[3]),
				E: toFloat(operands[4]),
				F: toFloat(operands[5]),
			}
			state.TextLineMatrix = state.TextMatrix
		}

	case "T*": // Move to next line
		state.TextLineMatrix = state.TextLineMatrix.Multiply(Translate(0, -state.Leading))
		state.TextMatrix = state.TextLineMatrix

	case "Tj": // Show text
		if len(operands) == 1 {
			text := toBytes(operands[0])
			e.showText(string(text), state)
		}

	case "TJ": // Show text with positioning
		if len(operands) == 1 {
			if array, ok := operands[0].([]interface{}); ok {
				for _, item := range array {
					switch v := item.(type) {
					case []byte:
						e.showText(string(v), state)
					case string:
						e.showText(v, state)
					case float64:
						// Adjust text position
						adjustment := -v / 1000 * state.FontSize
						state.TextMatrix = state.TextMatrix.Multiply(Translate(adjustment, 0))
					}
				}
			}
		}

	case "'": // Move to next line and show text
		state.TextLineMatrix = state.TextLineMatrix.Multiply(Translate(0, -state.Leading))
		state.TextMatrix = state.TextLineMatrix
		if len(operands) == 1 {
			text := toBytes(operands[0])
			e.showText(string(text), state)
		}

	case "\"": // Set spacing, move to next line, and show text
		if len(operands) == 3 {
			state.WordSpace = toFloat(operands[0])
			state.CharSpace = toFloat(operands[1])
			state.TextLineMatrix = state.TextLineMatrix.Multiply(Translate(0, -state.Leading))
			state.TextMatrix = state.TextLineMatrix
			text := toBytes(operands[2])
			e.showText(string(text), state)
		}

	// Color operators
	case "g": // Set gray fill color
		if len(operands) == 1 {
			gray := toFloat(operands[0])
			state.FillColor = []float64{gray}
			state.ColorSpace = "DeviceGray"
		}

	case "G": // Set gray stroke color
		if len(operands) == 1 {
			gray := toFloat(operands[0])
			state.StrokeColor = []float64{gray}
		}

	case "rg": // Set RGB fill color
		if len(operands) == 3 {
			state.FillColor = []float64{
				toFloat(operands[0]),
				toFloat(operands[1]),
				toFloat(operands[2]),
			}
			state.ColorSpace = "DeviceRGB"
		}

	case "RG": // Set RGB stroke color
		if len(operands) == 3 {
			state.StrokeColor = []float64{
				toFloat(operands[0]),
				toFloat(operands[1]),
				toFloat(operands[2]),
			}
		}

	// Line style operators
	case "w": // Set line width
		if len(operands) == 1 {
			state.LineWidth = toFloat(operands[0])
		}

	case "J": // Set line cap
		if len(operands) == 1 {
			state.LineCap = int(toFloat(operands[0]))
		}

	case "j": // Set line join
		if len(operands) == 1 {
			state.LineJoin = int(toFloat(operands[0]))
		}
	}

	return nil
}

// detectRectangleFromPath checks if a path forms a rectangle
func (e *ContentExtractor) detectRectangleFromPath(path []PathElement) *pdf.RectObject {
	if len(path) < 4 {
		return nil
	}
	
	// Look for pattern: move, line, line, line, [line], close
	// Should have 4-5 line segments forming a closed rectangle
	
	var points []Point
	for _, elem := range path {
		if elem.Type == "move" && len(elem.Points) > 0 {
			points = []Point{elem.Points[0]}
		} else if elem.Type == "line" && len(elem.Points) > 0 {
			points = append(points, elem.Points[0])
		}
	}
	
	// Need at least 4 points for a rectangle
	if len(points) < 4 {
		return nil
	}
	
	// Check if it forms a rectangle (4 or 5 points, last might be same as first)
	if len(points) == 4 || len(points) == 5 {
		// Find min/max coordinates
		minX, maxX := points[0].X, points[0].X
		minY, maxY := points[0].Y, points[0].Y
		
		for _, p := range points {
			if p.X < minX {
				minX = p.X
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
		
		// Check if all points are at corners of the bounding box
		tolerance := 0.1
		isRect := true
		for _, p := range points {
			atCorner := (math.Abs(p.X-minX) < tolerance || math.Abs(p.X-maxX) < tolerance) &&
			           (math.Abs(p.Y-minY) < tolerance || math.Abs(p.Y-maxY) < tolerance)
			if !atCorner {
				isRect = false
				break
			}
		}
		
		if isRect {
			return &pdf.RectObject{
				X0: minX,
				Y0: minY,
				X1: maxX,
				Y1: maxY,
			}
		}
	}
	
	return nil
}

// showText processes text showing operations
func (e *ContentExtractor) showText(text string, state *GraphicsState) {
	if state.FontSize == 0 {
		return
	}

	// Decode text using font's CMap if available
	decodedText := text
	if fontInfo, ok := e.fonts[state.FontName]; ok && fontInfo.ToUnicode != nil {
		// Convert string to bytes for CMap decoding
		decodedText = fontInfo.ToUnicode.Decode([]byte(text))
	}

	// Calculate text position
	x, y := state.CTM.Transform(state.TextMatrix.E, state.TextMatrix.F)
	
	// Calculate approximate width (simplified)
	width := float64(len(decodedText)) * state.FontSize * 0.5
	
	// Create character object
	char := pdf.CharObject{
		Text:     decodedText,
		Font:     state.FontName,
		FontSize: state.FontSize,
		X0:       x,
		Y0:       y - state.FontSize,
		X1:       x + width,
		Y1:       y,
		Width:    width,
		Height:   state.FontSize,
		Color:    makeColor(state.FillColor),
	}
	
	e.chars = append(e.chars, char)
	
	// Advance text position
	state.TextMatrix = state.TextMatrix.Multiply(Translate(width, 0))
}

// GetCharacters returns extracted characters
func (e *ContentExtractor) GetCharacters() []pdf.CharObject {
	return e.chars
}

// GetLines returns extracted lines
func (e *ContentExtractor) GetLines() []pdf.LineObject {
	return e.lines
}

// GetLinesFiltered returns deduplicated and filtered lines
func (e *ContentExtractor) GetLinesFiltered() []pdf.LineObject {
	// Get page dimensions
	pageWidth := 595.0  // Default A4
	pageHeight := 841.0
	if e.page != nil && len(e.page.MediaBox) >= 4 {
		pageWidth = e.page.MediaBox[2] - e.page.MediaBox[0]
		pageHeight = e.page.MediaBox[3] - e.page.MediaBox[1]
	}
	
	// Filter and deduplicate
	filtered := pdf.FilterPageBorderLines(e.lines, pageWidth, pageHeight)
	filtered = pdf.FilterTableLines(filtered)
	filtered = pdf.DeduplicateLines(filtered)
	filtered = pdf.ConsolidateTableLines(filtered)
	
	return filtered
}

// GetRectangles returns extracted rectangles
func (e *ContentExtractor) GetRectangles() []pdf.RectObject {
	return e.rects
}

// GetRectanglesFiltered returns deduplicated rectangles
func (e *ContentExtractor) GetRectanglesFiltered() []pdf.RectObject {
	return pdf.DeduplicateRectangles(e.rects)
}

// GetFilledRectangles returns only filled rectangles (like pdfplumber)
func (e *ContentExtractor) GetFilledRectangles() []pdf.RectObject {
	var filled []pdf.RectObject
	for _, rect := range e.rects {
		if rect.Filled && !rect.Stroked {
			filled = append(filled, rect)
		}
	}
	return pdf.DeduplicateRectangles(filled)
}

// GetCurves returns extracted curves
func (e *ContentExtractor) GetCurves() []pdf.CurveObject {
	return e.curves
}

// Helper functions

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toBytes(v interface{}) []byte {
	switch val := v.(type) {
	case []byte:
		return val
	case string:
		return []byte(val)
	default:
		return []byte(fmt.Sprintf("%v", val))
	}
}

func makeColor(values []float64) pdf.Color {
	if len(values) == 1 {
		// Grayscale
		gray := uint8(values[0] * 255)
		return pdf.Color{R: gray, G: gray, B: gray, A: 255}
	} else if len(values) == 3 {
		// RGB
		return pdf.Color{
			R: uint8(values[0] * 255),
			G: uint8(values[1] * 255),
			B: uint8(values[2] * 255),
			A: 255,
		}
	}
	// Default to black
	return pdf.Color{R: 0, G: 0, B: 0, A: 255}
}

// ContentLexer tokenizes PDF content streams
type ContentLexer struct {
	data []byte
	pos  int
}

// TokenType for content streams
type TokenType int

const (
	TokenOperator TokenType = iota
	TokenOperand
)

// Token represents a content stream token
type Token struct {
	Type  TokenType
	Value interface{}
}

// NewContentLexer creates a new content lexer
func NewContentLexer(data []byte) *ContentLexer {
	return &ContentLexer{data: data, pos: 0}
}

// NextToken returns the next token from the content stream
func (l *ContentLexer) NextToken() (*Token, error) {
	// Skip whitespace
	l.skipWhitespace()
	
	if l.pos >= len(l.data) {
		return nil, fmt.Errorf("EOF")
	}
	
	ch := l.data[l.pos]
	
	// Check for different token types
	switch {
	case ch == '(':
		// String
		return l.readString()
	case ch == '<':
		// Hex string or dictionary
		if l.pos+1 < len(l.data) && l.data[l.pos+1] == '<' {
			// Dictionary start
			l.pos += 2
			return &Token{Type: TokenOperand, Value: "<<"}, nil
		}
		return l.readHexString()
	case ch == '[':
		// Array - parse the entire array
		return l.readArray()
	case ch == '/':
		// Name
		return l.readName()
	case ch == '+' || ch == '-' || ch == '.' || (ch >= '0' && ch <= '9'):
		// Number
		return l.readNumber()
	default:
		// Operator or keyword
		return l.readOperator()
	}
}

// skipWhitespace skips whitespace characters
func (l *ContentLexer) skipWhitespace() {
	for l.pos < len(l.data) {
		ch := l.data[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '\f' {
			l.pos++
		} else {
			break
		}
	}
}

// readString reads a string literal
func (l *ContentLexer) readString() (*Token, error) {
	l.pos++ // Skip (
	start := l.pos
	parenCount := 1
	escaped := false
	
	for l.pos < len(l.data) && parenCount > 0 {
		ch := l.data[l.pos]
		if escaped {
			escaped = false
		} else {
			switch ch {
			case '\\':
				escaped = true
			case '(':
				parenCount++
			case ')':
				parenCount--
			}
		}
		l.pos++
	}
	
	if parenCount > 0 {
		return nil, fmt.Errorf("unterminated string")
	}
	
	// Process escape sequences
	text := l.data[start : l.pos-1]
	processed := processEscapes(text)
	
	return &Token{Type: TokenOperand, Value: processed}, nil
}

// readHexString reads a hexadecimal string
func (l *ContentLexer) readHexString() (*Token, error) {
	l.pos++ // Skip <
	start := l.pos
	
	for l.pos < len(l.data) && l.data[l.pos] != '>' {
		l.pos++
	}
	
	if l.pos >= len(l.data) {
		return nil, fmt.Errorf("unterminated hex string")
	}
	
	hex := l.data[start:l.pos]
	l.pos++ // Skip >
	
	// Convert hex to bytes - skip whitespace in hex
	cleanHex := make([]byte, 0, len(hex))
	for _, b := range hex {
		if (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F') {
			cleanHex = append(cleanHex, b)
		}
	}
	
	// Convert hex to bytes
	result := make([]byte, 0, len(cleanHex)/2)
	for i := 0; i < len(cleanHex); i += 2 {
		if i+1 < len(cleanHex) {
			val, _ := strconv.ParseUint(string(cleanHex[i:i+2]), 16, 8)
			result = append(result, byte(val))
		} else if i < len(cleanHex) {
			// Handle odd number of hex digits
			val, _ := strconv.ParseUint(string(cleanHex[i:i+1])+"0", 16, 8)
			result = append(result, byte(val))
		}
	}
	
	return &Token{Type: TokenOperand, Value: result}, nil
}

// readArray reads an array from the content stream
func (l *ContentLexer) readArray() (*Token, error) {
	l.pos++ // Skip [
	array := []interface{}{}
	
	for l.pos < len(l.data) {
		l.skipWhitespace()
		
		if l.pos >= len(l.data) {
			break
		}
		
		ch := l.data[l.pos]
		
		if ch == ']' {
			l.pos++ // Skip ]
			break
		}
		
		// Parse array element
		switch {
		case ch == '(':
			// String
			token, err := l.readString()
			if err != nil {
				return nil, err
			}
			array = append(array, token.Value)
			
		case ch == '<':
			// Hex string
			if l.pos+1 < len(l.data) && l.data[l.pos+1] == '<' {
				// Dictionary - shouldn't appear in TJ arrays
				return nil, fmt.Errorf("unexpected dictionary in array")
			}
			token, err := l.readHexString()
			if err != nil {
				return nil, err
			}
			array = append(array, token.Value)
			
		case ch == '/':
			// Name
			token, err := l.readName()
			if err != nil {
				return nil, err
			}
			array = append(array, token.Value)
			
		case ch == '+' || ch == '-' || ch == '.' || (ch >= '0' && ch <= '9'):
			// Number
			token, err := l.readNumber()
			if err != nil {
				return nil, err
			}
			array = append(array, token.Value)
			
		default:
			// Unknown element
			return nil, fmt.Errorf("unexpected character in array: %c", ch)
		}
	}
	
	return &Token{Type: TokenOperand, Value: array}, nil
}

// readName reads a name object
func (l *ContentLexer) readName() (*Token, error) {
	l.pos++ // Skip /
	start := l.pos
	
	for l.pos < len(l.data) {
		ch := l.data[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '\f' ||
			ch == '(' || ch == ')' || ch == '<' || ch == '>' || ch == '[' || ch == ']' ||
			ch == '/' || ch == '%' {
			break
		}
		l.pos++
	}
	
	name := string(l.data[start:l.pos])
	return &Token{Type: TokenOperand, Value: name}, nil
}

// readNumber reads a numeric value
func (l *ContentLexer) readNumber() (*Token, error) {
	start := l.pos
	hasDecimal := false
	
	for l.pos < len(l.data) {
		ch := l.data[l.pos]
		if ch == '.' {
			if hasDecimal {
				break
			}
			hasDecimal = true
		} else if ch == '+' || ch == '-' {
			if l.pos != start {
				break
			}
		} else if ch < '0' || ch > '9' {
			break
		}
		l.pos++
	}
	
	numStr := string(l.data[start:l.pos])
	if hasDecimal {
		val, _ := strconv.ParseFloat(numStr, 64)
		return &Token{Type: TokenOperand, Value: val}, nil
	} else {
		val, _ := strconv.ParseInt(numStr, 10, 64)
		return &Token{Type: TokenOperand, Value: float64(val)}, nil
	}
}

// readOperator reads an operator
func (l *ContentLexer) readOperator() (*Token, error) {
	start := l.pos
	
	for l.pos < len(l.data) {
		ch := l.data[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '\f' ||
			ch == '(' || ch == '<' || ch == '[' || ch == '/' ||
			(ch >= '0' && ch <= '9') || ch == '+' || ch == '-' || ch == '.' {
			break
		}
		l.pos++
	}
	
	op := string(l.data[start:l.pos])
	return &Token{Type: TokenOperator, Value: op}, nil
}

// processEscapes processes escape sequences in a string
func processEscapes(text []byte) []byte {
	var result []byte
	escaped := false
	
	for i := 0; i < len(text); i++ {
		if escaped {
			switch text[i] {
			case 'n':
				result = append(result, '\n')
			case 'r':
				result = append(result, '\r')
			case 't':
				result = append(result, '\t')
			case 'b':
				result = append(result, '\b')
			case 'f':
				result = append(result, '\f')
			case '\\', '(', ')':
				result = append(result, text[i])
			default:
				// Octal escape or literal
				if text[i] >= '0' && text[i] <= '7' {
					// Try to read up to 3 octal digits
					octal := string(text[i:min(i+3, len(text))])
					if val, err := strconv.ParseUint(octal, 8, 8); err == nil {
						result = append(result, byte(val))
						i += len(octal) - 1
					} else {
						result = append(result, text[i])
					}
				} else {
					result = append(result, text[i])
				}
			}
			escaped = false
		} else if text[i] == '\\' {
			escaped = true
		} else {
			result = append(result, text[i])
		}
	}
	
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}