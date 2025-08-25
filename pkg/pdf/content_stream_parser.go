package pdf

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// ContentStreamParser parses PDF content streams and extracts objects
type ContentStreamParser struct {
	ctx      *model.Context
	pageDict types.Dict
	objects  Objects
	
	// Graphics state
	graphicsState *GraphicsState
	stateStack    []*GraphicsState
	
	// Text state
	textState     *TextState
	textMatrix    Matrix
	lineMatrix    Matrix
	
	// Current path
	currentPath   []PathElement
	
	// Resources
	resources     types.Dict
	fonts         map[string]*FontInfo
}

// GraphicsState represents the PDF graphics state
type GraphicsState struct {
	CTM           Matrix  // Current transformation matrix
	StrokeColor   PDFColor
	FillColor     PDFColor
	LineWidth     float64
	LineCap       int
	LineJoin      int
	MiterLimit    float64
	DashPattern   []float64
	DashPhase     float64
}

// TextState represents the PDF text state
type TextState struct {
	Font         *FontInfo
	FontSize     float64
	CharSpace    float64
	WordSpace    float64
	Scale        float64
	Leading      float64
	Rise         float64
	RenderMode   int
}

// FontInfo represents font information
type FontInfo struct {
	Name         string
	BaseFont     string
	Encoding     string
	IsVertical   bool
	SpaceWidth   float64
	FontMatrix   Matrix
}

// Matrix represents a 2D transformation matrix
type Matrix struct {
	A, B, C, D, E, F float64
}

// PDFColor represents a color in PDF (renamed to avoid conflict with types.go)
type PDFColor struct {
	R, G, B float64
	ColorSpace string
}

// PathElement represents an element in a path
type PathElement struct {
	Type   string  // moveto, lineto, curveto, close
	Points []PDFPoint
}

// PDFPoint represents a 2D point (renamed to avoid conflict with types.go)
type PDFPoint struct {
	X, Y float64
}

// NewContentStreamParser creates a new content stream parser
func NewContentStreamParser(ctx *model.Context, pageDict types.Dict) *ContentStreamParser {
	parser := &ContentStreamParser{
		ctx:      ctx,
		pageDict: pageDict,
		objects:  Objects{},
		graphicsState: &GraphicsState{
			CTM:        IdentityMatrix(),
			LineWidth:  1.0,
			MiterLimit: 10.0,
		},
		textState: &TextState{
			FontSize:   12,
			Scale:      100,
			RenderMode: 0,
		},
		textMatrix: IdentityMatrix(),
		lineMatrix: IdentityMatrix(),
		fonts:      make(map[string]*FontInfo),
	}
	
	// Extract resources
	if res := pageDict["Resources"]; res != nil {
		if resDict, ok := res.(types.Dict); ok {
			parser.resources = resDict
			parser.extractFonts()
		}
	}
	
	return parser
}

// extractFonts extracts font information from resources
func (p *ContentStreamParser) extractFonts() {
	if p.resources == nil {
		return
	}
	
	fontDict := p.resources["Font"]
	if fontDict == nil {
		return
	}
	
	fonts, ok := fontDict.(types.Dict)
	if !ok {
		return
	}
	
	for name, fontRef := range fonts {
		fontObj := fontRef
		
		// Dereference if needed
		if indRef, ok := fontRef.(*types.IndirectRef); ok {
			dereferenced, err := p.ctx.Dereference(indRef)
			if err != nil {
				continue
			}
			fontObj = dereferenced
		}
		
		if fontDict, ok := fontObj.(types.Dict); ok {
			fontInfo := &FontInfo{
				Name:       name,
				FontMatrix: Matrix{A: 0.001, B: 0, C: 0, D: 0.001, E: 0, F: 0}, // Default
				SpaceWidth: 0.25, // Default estimate
			}
			
			// Extract BaseFont
			if baseFont := fontDict["BaseFont"]; baseFont != nil {
				if bf, ok := baseFont.(types.Name); ok {
					fontInfo.BaseFont = string(bf)
				}
			}
			
			// Extract Encoding
			if encoding := fontDict["Encoding"]; encoding != nil {
				if enc, ok := encoding.(types.Name); ok {
					fontInfo.Encoding = string(enc)
				}
			}
			
			p.fonts[name] = fontInfo
		}
	}
}

// Parse parses a content stream and returns extracted objects
func (p *ContentStreamParser) Parse(content []byte) Objects {
	// Tokenize the content stream
	tokens := p.tokenize(content)
	
	// Process tokens
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		
		// Check if it's an operator
		if p.isOperator(token) {
			// Get operands (all tokens before the operator)
			operands := p.getOperands(tokens, i)
			
			// Process the operator
			p.processOperator(token, operands)
			
			// Clear operands from token list
			tokens = tokens[i+1:]
			i = -1 // Reset counter
		}
	}
	
	return p.objects
}

// tokenize splits content stream into tokens
func (p *ContentStreamParser) tokenize(content []byte) []string {
	var tokens []string
	reader := bytes.NewReader(content)
	
	for reader.Len() > 0 {
		// Skip whitespace
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		
		if isWhitespace(b) {
			continue
		}
		
		// Handle different token types
		switch b {
		case '(':
			// String literal
			str := p.readStringLiteral(reader)
			tokens = append(tokens, "("+str+")")
			
		case '<':
			// Hex string or dictionary
			next, _ := reader.ReadByte()
			if next == '<' {
				tokens = append(tokens, "<<")
			} else {
				reader.UnreadByte()
				hex := p.readHexString(reader)
				tokens = append(tokens, "<"+hex+">")
			}
			
		case '>':
			// Dictionary end
			next, _ := reader.ReadByte()
			if next == '>' {
				tokens = append(tokens, ">>")
			} else {
				reader.UnreadByte()
			}
			
		case '[':
			tokens = append(tokens, "[")
			
		case ']':
			tokens = append(tokens, "]")
			
		case '/':
			// Name
			name := p.readName(reader)
			tokens = append(tokens, "/"+name)
			
		case '%':
			// Comment - skip to end of line
			p.skipComment(reader)
			
		default:
			// Number or operator
			reader.UnreadByte()
			token := p.readToken(reader)
			if token != "" {
				tokens = append(tokens, token)
			}
		}
	}
	
	return tokens
}

// readStringLiteral reads a string literal from the reader
func (p *ContentStreamParser) readStringLiteral(reader *bytes.Reader) string {
	var result []byte
	depth := 1
	
	for reader.Len() > 0 {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		
		if b == '\\' {
			// Escape sequence
			next, _ := reader.ReadByte()
			result = append(result, '\\', next)
		} else if b == '(' {
			depth++
			result = append(result, b)
		} else if b == ')' {
			depth--
			if depth == 0 {
				break
			}
			result = append(result, b)
		} else {
			result = append(result, b)
		}
	}
	
	return string(result)
}

// readHexString reads a hex string from the reader
func (p *ContentStreamParser) readHexString(reader *bytes.Reader) string {
	var result []byte
	
	for reader.Len() > 0 {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		
		if b == '>' {
			break
		}
		
		if !isWhitespace(b) {
			result = append(result, b)
		}
	}
	
	return string(result)
}

// readName reads a name from the reader
func (p *ContentStreamParser) readName(reader *bytes.Reader) string {
	var result []byte
	
	for reader.Len() > 0 {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		
		if isDelimiter(b) || isWhitespace(b) {
			reader.UnreadByte()
			break
		}
		
		result = append(result, b)
	}
	
	return string(result)
}

// readToken reads a general token from the reader
func (p *ContentStreamParser) readToken(reader *bytes.Reader) string {
	var result []byte
	
	for reader.Len() > 0 {
		b, err := reader.ReadByte()
		if err != nil {
			break
		}
		
		if isDelimiter(b) || isWhitespace(b) {
			reader.UnreadByte()
			break
		}
		
		result = append(result, b)
	}
	
	return string(result)
}

// skipComment skips a comment line
func (p *ContentStreamParser) skipComment(reader *bytes.Reader) {
	for reader.Len() > 0 {
		b, _ := reader.ReadByte()
		if b == '\n' || b == '\r' {
			break
		}
	}
}

// isWhitespace checks if a byte is whitespace
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == 0
}

// isDelimiter checks if a byte is a delimiter
func isDelimiter(b byte) bool {
	return b == '(' || b == ')' || b == '<' || b == '>' || b == '[' || b == ']' ||
		b == '{' || b == '}' || b == '/' || b == '%'
}

// isOperator checks if a token is a PDF operator
func (p *ContentStreamParser) isOperator(token string) bool {
	// Common PDF operators
	operators := []string{
		// Text operators
		"BT", "ET", "Td", "TD", "Tm", "T*", "Tj", "TJ", "'", "\"",
		"Tc", "Tw", "Tz", "TL", "Tf", "Tr", "Ts",
		// Graphics state
		"q", "Q", "cm", "w", "J", "j", "M", "d", "ri", "i", "gs",
		// Path construction
		"m", "l", "c", "v", "y", "h", "re",
		// Path painting
		"S", "s", "f", "F", "f*", "B", "B*", "b", "b*", "n",
		// Color
		"CS", "cs", "SC", "SCN", "sc", "scn", "G", "g", "RG", "rg", "K", "k",
		// Other
		"W", "W*", "BX", "EX", "Do", "MP", "DP", "BMC", "BDC", "EMC",
	}
	
	for _, op := range operators {
		if token == op {
			return true
		}
	}
	
	return false
}

// getOperands extracts operands before an operator
func (p *ContentStreamParser) getOperands(tokens []string, operatorIndex int) []string {
	if operatorIndex == 0 {
		return []string{}
	}
	
	return tokens[:operatorIndex]
}

// processOperator processes a PDF operator with its operands
func (p *ContentStreamParser) processOperator(operator string, operands []string) {
	switch operator {
	// Text object operators
	case "BT":
		p.beginText()
	case "ET":
		p.endText()
		
	// Text positioning
	case "Td":
		p.textMoveBy(operands)
	case "TD":
		p.textMoveByWithLeading(operands)
	case "Tm":
		p.setTextMatrix(operands)
	case "T*":
		p.textNextLine()
		
	// Text showing
	case "Tj":
		p.showText(operands)
	case "TJ":
		p.showTextArray(operands)
	case "'":
		p.textNextLineShow(operands)
	case "\"":
		p.textNextLineShowWithSpacing(operands)
		
	// Text state
	case "Tc":
		p.setCharSpace(operands)
	case "Tw":
		p.setWordSpace(operands)
	case "Tz":
		p.setHorizontalScale(operands)
	case "TL":
		p.setTextLeading(operands)
	case "Tf":
		p.setFont(operands)
	case "Tr":
		p.setTextRenderMode(operands)
	case "Ts":
		p.setTextRise(operands)
		
	// Graphics state
	case "q":
		p.saveGraphicsState()
	case "Q":
		p.restoreGraphicsState()
	case "cm":
		p.concatenateMatrix(operands)
		
	// Path construction
	case "m":
		p.moveTo(operands)
	case "l":
		p.lineTo(operands)
	case "c":
		p.curveTo(operands)
	case "v":
		p.curveToV(operands)
	case "y":
		p.curveToY(operands)
	case "h":
		p.closePath()
	case "re":
		p.rectangle(operands)
		
	// Path painting
	case "S", "s":
		p.stroke()
	case "f", "F", "f*":
		p.fill()
	case "B", "B*", "b", "b*":
		p.fillAndStroke()
	case "n":
		p.endPath()
		
	// Line width
	case "w":
		p.setLineWidth(operands)
	}
}

// Text object operators

func (p *ContentStreamParser) beginText() {
	p.textMatrix = IdentityMatrix()
	p.lineMatrix = IdentityMatrix()
}

func (p *ContentStreamParser) endText() {
	// Text object ended
}

// Text positioning operators

func (p *ContentStreamParser) textMoveBy(operands []string) {
	if len(operands) < 2 {
		return
	}
	
	tx := parseFloat(operands[0])
	ty := parseFloat(operands[1])
	
	translation := TranslationMatrix(tx, ty)
	p.lineMatrix = MultiplyMatrix(translation, p.lineMatrix)
	p.textMatrix = p.lineMatrix
}

func (p *ContentStreamParser) textMoveByWithLeading(operands []string) {
	if len(operands) < 2 {
		return
	}
	
	// tx := parseFloat(operands[0]) // Not used directly, passed through textMoveBy
	ty := parseFloat(operands[1])
	
	p.textState.Leading = -ty
	p.textMoveBy(operands)
}

func (p *ContentStreamParser) setTextMatrix(operands []string) {
	if len(operands) < 6 {
		return
	}
	
	p.textMatrix = Matrix{
		A: parseFloat(operands[0]),
		B: parseFloat(operands[1]),
		C: parseFloat(operands[2]),
		D: parseFloat(operands[3]),
		E: parseFloat(operands[4]),
		F: parseFloat(operands[5]),
	}
	p.lineMatrix = p.textMatrix
}

func (p *ContentStreamParser) textNextLine() {
	p.textMoveBy([]string{"0", fmt.Sprintf("%f", -p.textState.Leading)})
}

// Text showing operators

func (p *ContentStreamParser) showText(operands []string) {
	if len(operands) < 1 {
		return
	}
	
	text := p.extractString(operands[0])
	p.addTextChars(text)
}

func (p *ContentStreamParser) showTextArray(operands []string) {
	if len(operands) < 1 {
		return
	}
	
	// Parse array
	arrayStr := strings.Join(operands, " ")
	if !strings.HasPrefix(arrayStr, "[") || !strings.HasSuffix(arrayStr, "]") {
		return
	}
	
	// Remove brackets and parse elements
	arrayStr = strings.TrimPrefix(arrayStr, "[")
	arrayStr = strings.TrimSuffix(arrayStr, "]")
	
	elements := p.parseTextArray(arrayStr)
	
	for _, elem := range elements {
		if strings.HasPrefix(elem, "(") || strings.HasPrefix(elem, "<") {
			text := p.extractString(elem)
			p.addTextChars(text)
		} else {
			// It's a number (spacing adjustment)
			spacing := parseFloat(elem) / 1000.0 * p.textState.FontSize
			p.textMatrix.E -= spacing * p.textMatrix.A
		}
	}
}

func (p *ContentStreamParser) textNextLineShow(operands []string) {
	p.textNextLine()
	p.showText(operands)
}

func (p *ContentStreamParser) textNextLineShowWithSpacing(operands []string) {
	if len(operands) < 3 {
		return
	}
	
	p.setWordSpace(operands[:1])
	p.setCharSpace(operands[1:2])
	p.showText(operands[2:])
}

// Text state operators

func (p *ContentStreamParser) setCharSpace(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.textState.CharSpace = parseFloat(operands[0])
}

func (p *ContentStreamParser) setWordSpace(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.textState.WordSpace = parseFloat(operands[0])
}

func (p *ContentStreamParser) setHorizontalScale(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.textState.Scale = parseFloat(operands[0])
}

func (p *ContentStreamParser) setTextLeading(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.textState.Leading = parseFloat(operands[0])
}

func (p *ContentStreamParser) setFont(operands []string) {
	if len(operands) < 2 {
		return
	}
	
	fontName := strings.TrimPrefix(operands[0], "/")
	fontSize := parseFloat(operands[1])
	
	if font, ok := p.fonts[fontName]; ok {
		p.textState.Font = font
	}
	p.textState.FontSize = fontSize
}

func (p *ContentStreamParser) setTextRenderMode(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.textState.RenderMode = parseInt(operands[0])
}

func (p *ContentStreamParser) setTextRise(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.textState.Rise = parseFloat(operands[0])
}

// Graphics state operators

func (p *ContentStreamParser) saveGraphicsState() {
	// Save current state
	stateCopy := *p.graphicsState
	p.stateStack = append(p.stateStack, &stateCopy)
}

func (p *ContentStreamParser) restoreGraphicsState() {
	if len(p.stateStack) > 0 {
		p.graphicsState = p.stateStack[len(p.stateStack)-1]
		p.stateStack = p.stateStack[:len(p.stateStack)-1]
	}
}

func (p *ContentStreamParser) concatenateMatrix(operands []string) {
	if len(operands) < 6 {
		return
	}
	
	m := Matrix{
		A: parseFloat(operands[0]),
		B: parseFloat(operands[1]),
		C: parseFloat(operands[2]),
		D: parseFloat(operands[3]),
		E: parseFloat(operands[4]),
		F: parseFloat(operands[5]),
	}
	
	p.graphicsState.CTM = MultiplyMatrix(m, p.graphicsState.CTM)
}

// Path construction operators

func (p *ContentStreamParser) moveTo(operands []string) {
	if len(operands) < 2 {
		return
	}
	
	x := parseFloat(operands[0])
	y := parseFloat(operands[1])
	
	p.currentPath = append(p.currentPath, PathElement{
		Type:   "moveto",
		Points: []PDFPoint{{X: x, Y: y}},
	})
}

func (p *ContentStreamParser) lineTo(operands []string) {
	if len(operands) < 2 {
		return
	}
	
	x := parseFloat(operands[0])
	y := parseFloat(operands[1])
	
	p.currentPath = append(p.currentPath, PathElement{
		Type:   "lineto",
		Points: []PDFPoint{{X: x, Y: y}},
	})
}

func (p *ContentStreamParser) curveTo(operands []string) {
	if len(operands) < 6 {
		return
	}
	
	p.currentPath = append(p.currentPath, PathElement{
		Type: "curveto",
		Points: []PDFPoint{
			{X: parseFloat(operands[0]), Y: parseFloat(operands[1])},
			{X: parseFloat(operands[2]), Y: parseFloat(operands[3])},
			{X: parseFloat(operands[4]), Y: parseFloat(operands[5])},
		},
	})
}

func (p *ContentStreamParser) curveToV(operands []string) {
	if len(operands) < 4 {
		return
	}
	
	// Use current point as first control point
	p.currentPath = append(p.currentPath, PathElement{
		Type: "curveto",
		Points: []PDFPoint{
			{X: 0, Y: 0}, // Will be filled with current point
			{X: parseFloat(operands[0]), Y: parseFloat(operands[1])},
			{X: parseFloat(operands[2]), Y: parseFloat(operands[3])},
		},
	})
}

func (p *ContentStreamParser) curveToY(operands []string) {
	if len(operands) < 4 {
		return
	}
	
	// Use third point as second control point
	p.currentPath = append(p.currentPath, PathElement{
		Type: "curveto",
		Points: []PDFPoint{
			{X: parseFloat(operands[0]), Y: parseFloat(operands[1])},
			{X: parseFloat(operands[2]), Y: parseFloat(operands[3])},
			{X: parseFloat(operands[2]), Y: parseFloat(operands[3])},
		},
	})
}

func (p *ContentStreamParser) closePath() {
	p.currentPath = append(p.currentPath, PathElement{
		Type: "close",
	})
}

func (p *ContentStreamParser) rectangle(operands []string) {
	if len(operands) < 4 {
		return
	}
	
	x := parseFloat(operands[0])
	y := parseFloat(operands[1])
	width := parseFloat(operands[2])
	height := parseFloat(operands[3])
	
	// Add rectangle to objects
	rect := RectObject{
		X0:    x,
		Y0:    y,
		X1:    x + width,
		Y1:    y + height,
		Width: p.graphicsState.LineWidth,
	}
	
	p.objects.Rects = append(p.objects.Rects, rect)
}

// Path painting operators

func (p *ContentStreamParser) stroke() {
	p.createLineFromPath()
	p.currentPath = nil
}

func (p *ContentStreamParser) fill() {
	// TODO: Handle filled paths
	p.currentPath = nil
}

func (p *ContentStreamParser) fillAndStroke() {
	p.createLineFromPath()
	p.currentPath = nil
}

func (p *ContentStreamParser) endPath() {
	p.currentPath = nil
}

func (p *ContentStreamParser) setLineWidth(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.graphicsState.LineWidth = parseFloat(operands[0])
}

// Helper functions

func (p *ContentStreamParser) addTextChars(text string) {
	if text == "" || p.textState.Font == nil {
		return
	}
	
	// Calculate text width (simplified)
	textWidth := float64(len(text)) * p.textState.FontSize * 0.5
	
	// Transform coordinates
	x, y := p.textMatrix.E, p.textMatrix.F
	
	// Create character object
	char := CharObject{
		Text:     text,
		Font:     p.textState.Font.Name,
		FontSize: p.textState.FontSize,
		X0:       x,
		Y0:       y,
		X1:       x + textWidth,
		Y1:       y + p.textState.FontSize,
		Width:    textWidth,
		Height:   p.textState.FontSize,
	}
	
	p.objects.Chars = append(p.objects.Chars, char)
	
	// Update text matrix for next character
	p.textMatrix.E += textWidth
}

func (p *ContentStreamParser) createLineFromPath() {
	if len(p.currentPath) < 2 {
		return
	}
	
	// Create line objects from path
	for i := 0; i < len(p.currentPath)-1; i++ {
		if p.currentPath[i].Type == "moveto" && i+1 < len(p.currentPath) && p.currentPath[i+1].Type == "lineto" {
			start := p.currentPath[i].Points[0]
			end := p.currentPath[i+1].Points[0]
			
			line := LineObject{
				X0:    start.X,
				Y0:    start.Y,
				X1:    end.X,
				Y1:    end.Y,
				Width: p.graphicsState.LineWidth,
			}
			
			p.objects.Lines = append(p.objects.Lines, line)
		}
	}
}

func (p *ContentStreamParser) extractString(str string) string {
	if strings.HasPrefix(str, "(") && strings.HasSuffix(str, ")") {
		// String literal
		str = strings.TrimPrefix(str, "(")
		str = strings.TrimSuffix(str, ")")
		// TODO: Handle escape sequences
		return str
	} else if strings.HasPrefix(str, "<") && strings.HasSuffix(str, ">") {
		// Hex string
		str = strings.TrimPrefix(str, "<")
		str = strings.TrimSuffix(str, ">")
		// TODO: Convert hex to string
		return str
	}
	return str
}

func (p *ContentStreamParser) parseTextArray(arrayStr string) []string {
	var elements []string
	var current strings.Builder
	inString := false
	parenDepth := 0
	
	for i := 0; i < len(arrayStr); i++ {
		ch := arrayStr[i]
		
		if !inString && isWhitespace(ch) {
			if current.Len() > 0 {
				elements = append(elements, current.String())
				current.Reset()
			}
			continue
		}
		
		if ch == '(' {
			if !inString {
				inString = true
				parenDepth = 1
			} else {
				parenDepth++
			}
			current.WriteByte(ch)
		} else if ch == ')' && inString {
			parenDepth--
			current.WriteByte(ch)
			if parenDepth == 0 {
				inString = false
				elements = append(elements, current.String())
				current.Reset()
			}
		} else if ch == '<' && !inString {
			// Start of hex string
			start := i
			for i < len(arrayStr) && arrayStr[i] != '>' {
				i++
			}
			if i < len(arrayStr) {
				elements = append(elements, arrayStr[start:i+1])
			}
		} else {
			current.WriteByte(ch)
		}
	}
	
	if current.Len() > 0 {
		elements = append(elements, current.String())
	}
	
	return elements
}

// Utility functions

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// min and max functions are already defined in types.go

// Matrix operations

func IdentityMatrix() Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}
}

func TranslationMatrix(tx, ty float64) Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: tx, F: ty}
}

func MultiplyMatrix(m1, m2 Matrix) Matrix {
	return Matrix{
		A: m1.A*m2.A + m1.B*m2.C,
		B: m1.A*m2.B + m1.B*m2.D,
		C: m1.C*m2.A + m1.D*m2.C,
		D: m1.C*m2.B + m1.D*m2.D,
		E: m1.E*m2.A + m1.F*m2.C + m2.E,
		F: m1.E*m2.B + m1.F*m2.D + m2.F,
	}
}