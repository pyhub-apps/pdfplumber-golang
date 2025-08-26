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
	ToUnicodeCMap *ToUnicodeCMap // Added for proper text decoding
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
			CTM:         IdentityMatrix(),
			LineWidth:   1.0,
			MiterLimit:  10.0,
			StrokeColor: PDFColor{R: 0, G: 0, B: 0, ColorSpace: "Gray"}, // Default black
			FillColor:   PDFColor{R: 0, G: 0, B: 0, ColorSpace: "Gray"}, // Default black
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
	// fmt.Println("[DEBUG-FONT] extractFonts called")
	if p.resources == nil {
		// fmt.Println("[DEBUG-FONT] No resources")
		return
	}
	
	fontDict := p.resources["Font"]
	if fontDict == nil {
		// fmt.Println("[DEBUG-FONT] No Font in resources")
		return
	}
	
	// Try to dereference Font dictionary
	var fonts types.Dict
	
	if indRef, ok := fontDict.(types.IndirectRef); ok {
		// fmt.Println("[DEBUG-FONT] Font is IndirectRef, using DereferenceDict...")
		dict, err := p.ctx.DereferenceDict(indRef)
		if err != nil {
			// fmt.Printf("[DEBUG-FONT] Failed to DereferenceDict: %v\n", err)
			return
		}
		if dict != nil {
			fonts = dict
			// fmt.Printf("[DEBUG-FONT] Successfully got font dict with %d entries\n", len(fonts))
		} else {
			// fmt.Println("[DEBUG-FONT] DereferenceDict returned nil")
			return
		}
	} else if dict, ok := fontDict.(types.Dict); ok {
		fonts = dict
		// fmt.Printf("[DEBUG-FONT] Font is already a Dict with %d entries\n", len(fonts))
	} else {
		// fmt.Printf("[DEBUG-FONT] Font is unexpected type: %T\n", fontDict)
		return
	}
	
	// fmt.Printf("[DEBUG-FONT] Found %d fonts\n", len(fonts))
	
	for name, fontRef := range fonts {
		// fmt.Printf("[DEBUG-FONT] Processing font %s, type: %T\n", name, fontRef)
		fontObj := fontRef
		
		// Dereference font object
		if indRef, ok := fontRef.(types.IndirectRef); ok {
			// fmt.Printf("[DEBUG-FONT] Font %s is IndirectRef, using DereferenceDict...\n", name)
			dict, err := p.ctx.DereferenceDict(indRef)
			if err != nil {
				// fmt.Printf("[DEBUG-FONT] Failed to DereferenceDict font %s: %v\n", name, err)
				continue
			}
			if dict != nil {
				fontObj = dict
			}
		} else if indRef, ok := fontRef.(*types.IndirectRef); ok {
			// fmt.Printf("[DEBUG-FONT] Font %s is *IndirectRef, using DereferenceDict...\n", name)
			dict, err := p.ctx.DereferenceDict(*indRef)
			if err != nil {
				// fmt.Printf("[DEBUG-FONT] Failed to DereferenceDict font %s: %v\n", name, err)
				continue
			}
			if dict != nil {
				fontObj = dict
			}
		}
		
		// fmt.Printf("[DEBUG-FONT] After dereference, font %s type: %T\n", name, fontObj)
		if fontDict, ok := fontObj.(types.Dict); ok {
			// fmt.Printf("[DEBUG-FONT] Font %s is a Dict\n", name)
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
			
			// Extract ToUnicode CMap
			if toUnicode := fontDict["ToUnicode"]; toUnicode != nil {
				// fmt.Printf("[DEBUG-FONT] Font %s has ToUnicode, type: %T\n", name, toUnicode)
				
				// Try to dereference ToUnicode stream
				var cmapData []byte
				
				if indRef, ok := toUnicode.(types.IndirectRef); ok {
					streamDict, _, err := p.ctx.DereferenceStreamDict(indRef)
					if err == nil && streamDict != nil {
						if err := streamDict.Decode(); err == nil {
							cmapData = streamDict.Content
							// fmt.Printf("[DEBUG-FONT] Got ToUnicode CMap data for %s: %d bytes\n", name, len(cmapData))
						}
					}
				} else if indRef, ok := toUnicode.(*types.IndirectRef); ok {
					streamDict, _, err := p.ctx.DereferenceStreamDict(*indRef)
					if err == nil && streamDict != nil {
						if err := streamDict.Decode(); err == nil {
							cmapData = streamDict.Content
							// fmt.Printf("[DEBUG-FONT] Got ToUnicode CMap data for %s: %d bytes\n", name, len(cmapData))
						}
					}
				}
				
				// Parse CMap if we got data
				if len(cmapData) > 0 {
					cmap := NewToUnicodeCMap()
					if err := cmap.Parse(cmapData); err == nil {
						fontInfo.ToUnicodeCMap = cmap
						// fmt.Printf("[DEBUG-FONT] Successfully parsed CMap for %s: %d mappings\n", name, cmap.GetMappingCount())
					} else {
						// fmt.Printf("[DEBUG-FONT] Failed to parse CMap for %s: %v\n", name, err)
					}
				}
			}
			
			p.fonts[name] = fontInfo
			// fmt.Printf("[DEBUG-FONT] Added font %s: %+v\n", name, fontInfo)
		}
	}
}

// Parse parses a content stream and returns extracted objects
func (p *ContentStreamParser) Parse(content []byte) Objects {
	// Tokenize the content stream
	tokens := p.tokenize(content)
	
	// Process tokens
	operands := []string{}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		
		// Check if it's an operator
		if p.isOperator(token) {
			// Process the operator with accumulated operands
			p.processOperator(token, operands)
			
			// Clear operands for next operator
			operands = []string{}
		} else {
			// Accumulate operands
			operands = append(operands, token)
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


// processOperator processes a PDF operator with its operands
func (p *ContentStreamParser) processOperator(operator string, operands []string) {
	switch operator {
	// Text object operators
	case "BT":
		// fmt.Println("[DEBUG-TEXT] BT (begin text) operator found")
		p.beginText()
	case "ET":
		// fmt.Println("[DEBUG-TEXT] ET (end text) operator found")
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
		// fmt.Printf("[DEBUG-TEXT] Tj (show text) operator with operands: %v\n", operands)
		p.showText(operands)
	case "TJ":
		// fmt.Printf("[DEBUG-TEXT] TJ (show text array) operator with operands: %v\n", operands)
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
		// fmt.Printf("[DEBUG-TEXT] Tf (set font) operator with operands: %v\n", operands)
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
		
	// Line width and style
	case "w":
		p.setLineWidth(operands)
	case "J":
		p.setLineCap(operands)
	case "j":
		p.setLineJoin(operands)
	case "M":
		p.setMiterLimit(operands)
	case "d":
		p.setDashPattern(operands)
		
	// Color operators
	case "RG":
		p.setStrokeColorRGB(operands)
	case "rg":
		p.setFillColorRGB(operands)
	case "G":
		p.setStrokeColorGray(operands)
	case "g":
		p.setFillColorGray(operands)
	case "K":
		p.setStrokeColorCMYK(operands)
	case "k":
		p.setFillColorCMYK(operands)
	case "CS":
		p.setStrokeColorSpace(operands)
	case "cs":
		p.setFillColorSpace(operands)
	case "SC", "SCN":
		p.setStrokeColor(operands)
	case "sc", "scn":
		p.setFillColor(operands)
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
	
	// Add rectangle path for later stroke/fill
	// Convert rectangle to path
	p.currentPath = append(p.currentPath, 
		PathElement{Type: "moveto", Points: []PDFPoint{{X: x, Y: y}}},
		PathElement{Type: "lineto", Points: []PDFPoint{{X: x + width, Y: y}}},
		PathElement{Type: "lineto", Points: []PDFPoint{{X: x + width, Y: y + height}}},
		PathElement{Type: "lineto", Points: []PDFPoint{{X: x, Y: y + height}}},
		PathElement{Type: "close"},
	)
}

// Path painting operators

func (p *ContentStreamParser) stroke() {
	p.createLineFromPath()
	p.currentPath = nil
}

func (p *ContentStreamParser) fill() {
	p.createFilledPath()
	p.currentPath = nil
}

func (p *ContentStreamParser) fillAndStroke() {
	// First create filled object, then stroked lines
	p.createFilledPath()
	p.createLineFromPath()
	p.currentPath = nil
}

// createFilledPath creates filled rectangles or shapes from the current path
func (p *ContentStreamParser) createFilledPath() {
	if len(p.currentPath) < 3 {
		return
	}
	
	// Check if path forms a rectangle
	if p.isRectanglePath() {
		// Extract rectangle bounds
		minX, minY, maxX, maxY := p.getPathBounds()
		
		// Apply transformation
		x0, y0 := p.transformPoint(minX, minY)
		x1, y1 := p.transformPoint(maxX, maxY)
		
		fillColor := p.convertPDFColorToColor(p.graphicsState.FillColor)
		
		rect := RectObject{
			X0:          min(x0, x1),
			Y0:          min(y0, y1),
			X1:          max(x0, x1),
			Y1:          max(y0, y1),
			Width:       0, // Filled rectangle has no stroke width
			FillColor:   fillColor,
			NonStroking: true, // This is a filled (non-stroking) rectangle
		}
		
		p.objects.Rects = append(p.objects.Rects, rect)
	}
	// For complex paths, we could create a more general filled shape object
}

// isRectanglePath checks if the current path forms a rectangle
func (p *ContentStreamParser) isRectanglePath() bool {
	// Simple heuristic: 4 lines with close
	lineCount := 0
	hasClose := false
	
	for _, elem := range p.currentPath {
		if elem.Type == "lineto" {
			lineCount++
		} else if elem.Type == "close" {
			hasClose = true
		}
	}
	
	return lineCount == 3 && hasClose // 3 lineto + 1 implicit line from close
}

// getPathBounds returns the bounding box of the current path
func (p *ContentStreamParser) getPathBounds() (minX, minY, maxX, maxY float64) {
	first := true
	
	for _, elem := range p.currentPath {
		for _, pt := range elem.Points {
			if first {
				minX, maxX = pt.X, pt.X
				minY, maxY = pt.Y, pt.Y
				first = false
			} else {
				minX = min(minX, pt.X)
				maxX = max(maxX, pt.X)
				minY = min(minY, pt.Y)
				maxY = max(maxY, pt.Y)
			}
		}
	}
	
	return
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

func (p *ContentStreamParser) setLineCap(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.graphicsState.LineCap = parseInt(operands[0])
}

func (p *ContentStreamParser) setLineJoin(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.graphicsState.LineJoin = parseInt(operands[0])
}

func (p *ContentStreamParser) setMiterLimit(operands []string) {
	if len(operands) < 1 {
		return
	}
	p.graphicsState.MiterLimit = parseFloat(operands[0])
}

func (p *ContentStreamParser) setDashPattern(operands []string) {
	// Format: [array] phase d
	// For now, just store whether dashed or not
	if len(operands) >= 2 {
		// Parse array and phase
		// Simplified: just check if we have a dash pattern
		p.graphicsState.DashPattern = []float64{1} // Placeholder
	}
}

// Color operators

func (p *ContentStreamParser) setStrokeColorRGB(operands []string) {
	if len(operands) < 3 {
		return
	}
	p.graphicsState.StrokeColor = PDFColor{
		R:          parseFloat(operands[0]),
		G:          parseFloat(operands[1]),
		B:          parseFloat(operands[2]),
		ColorSpace: "RGB",
	}
}

func (p *ContentStreamParser) setFillColorRGB(operands []string) {
	if len(operands) < 3 {
		return
	}
	p.graphicsState.FillColor = PDFColor{
		R:          parseFloat(operands[0]),
		G:          parseFloat(operands[1]),
		B:          parseFloat(operands[2]),
		ColorSpace: "RGB",
	}
}

func (p *ContentStreamParser) setStrokeColorGray(operands []string) {
	if len(operands) < 1 {
		return
	}
	gray := parseFloat(operands[0])
	p.graphicsState.StrokeColor = PDFColor{
		R:          gray,
		G:          gray,
		B:          gray,
		ColorSpace: "Gray",
	}
}

func (p *ContentStreamParser) setFillColorGray(operands []string) {
	if len(operands) < 1 {
		return
	}
	gray := parseFloat(operands[0])
	p.graphicsState.FillColor = PDFColor{
		R:          gray,
		G:          gray,
		B:          gray,
		ColorSpace: "Gray",
	}
}

func (p *ContentStreamParser) setStrokeColorCMYK(operands []string) {
	if len(operands) < 4 {
		return
	}
	// Convert CMYK to RGB (simplified)
	c := parseFloat(operands[0])
	m := parseFloat(operands[1])
	y := parseFloat(operands[2])
	k := parseFloat(operands[3])
	
	p.graphicsState.StrokeColor = PDFColor{
		R:          (1 - c) * (1 - k),
		G:          (1 - m) * (1 - k),
		B:          (1 - y) * (1 - k),
		ColorSpace: "CMYK",
	}
}

func (p *ContentStreamParser) setFillColorCMYK(operands []string) {
	if len(operands) < 4 {
		return
	}
	// Convert CMYK to RGB (simplified)
	c := parseFloat(operands[0])
	m := parseFloat(operands[1])
	y := parseFloat(operands[2])
	k := parseFloat(operands[3])
	
	p.graphicsState.FillColor = PDFColor{
		R:          (1 - c) * (1 - k),
		G:          (1 - m) * (1 - k),
		B:          (1 - y) * (1 - k),
		ColorSpace: "CMYK",
	}
}

func (p *ContentStreamParser) setStrokeColorSpace(operands []string) {
	// Store color space name for later use
	// For now, simplified implementation
}

func (p *ContentStreamParser) setFillColorSpace(operands []string) {
	// Store color space name for later use
	// For now, simplified implementation
}

func (p *ContentStreamParser) setStrokeColor(operands []string) {
	// Generic color setting based on current color space
	// Simplified: treat as grayscale or RGB
	if len(operands) == 1 {
		p.setStrokeColorGray(operands)
	} else if len(operands) >= 3 {
		p.setStrokeColorRGB(operands)
	}
}

func (p *ContentStreamParser) setFillColor(operands []string) {
	// Generic color setting based on current color space
	// Simplified: treat as grayscale or RGB
	if len(operands) == 1 {
		p.setFillColorGray(operands)
	} else if len(operands) >= 3 {
		p.setFillColorRGB(operands)
	}
}

// Helper functions

func (p *ContentStreamParser) addTextChars(text string) {
	// fmt.Printf("[DEBUG-TEXT] addTextChars called with text: %q, Font: %v\n", text, p.textState.Font)
	if text == "" {
		// fmt.Println("[DEBUG-TEXT] Empty text, skipping")
		return
	}
	if p.textState.Font == nil {
		// fmt.Println("[DEBUG-TEXT] Font is nil, skipping")
		return
	}
	
	// Process each character individually for better positioning
	for _, runeValue := range text {
		charStr := string(runeValue)
		
		// Calculate character width (simplified - should use font metrics)
		// For now use a better approximation based on character type
		charWidth := p.getCharWidth(charStr) * p.textState.FontSize
		
		// Transform coordinates - apply both text matrix and CTM
		textX, textY := p.textMatrix.E, p.textMatrix.F
		
		// Apply CTM transformation to get actual page coordinates
		ctm := p.graphicsState.CTM
		x := ctm.A*textX + ctm.C*textY + ctm.E
		y := ctm.B*textX + ctm.D*textY + ctm.F
		
		// Create character object
		char := CharObject{
			Text:     charStr,
			Font:     p.textState.Font.Name,
			FontSize: p.textState.FontSize,
			X0:       x,
			Y0:       y,
			X1:       x + charWidth,
			Y1:       y + p.textState.FontSize,
			Width:    charWidth,
			Height:   p.textState.FontSize,
		}
		
		p.objects.Chars = append(p.objects.Chars, char)
		
		// Update text matrix for next character
		// Include character spacing and word spacing if it's a space
		displacement := charWidth
		if charStr == " " {
			displacement += p.textState.WordSpace
		}
		displacement += p.textState.CharSpace
		
		// Apply horizontal scaling
		displacement *= p.textState.Scale / 100.0
		
		// Update text matrix (move horizontally)
		p.textMatrix.E += displacement * p.textMatrix.A
		p.textMatrix.F += displacement * p.textMatrix.B
	}
}

// getCharWidth returns an approximate width factor for a character
func (p *ContentStreamParser) getCharWidth(char string) float64 {
	// This is a simplified approximation
	// In reality, we should use font metrics from the font dictionary
	switch char {
	case " ":
		return 0.25
	case "i", "l", "I", "!", ".", ",", ";", ":", "'", "\"":
		return 0.3
	case "m", "M", "W", "w":
		return 0.8
	default:
		return 0.5
	}
}

func (p *ContentStreamParser) createLineFromPath() {
	if len(p.currentPath) < 2 {
		return
	}
	
	// Convert stroke color to Color type for LineObject
	strokeColor := p.convertPDFColorToColor(p.graphicsState.StrokeColor)
	
	// Track current position
	var currentX, currentY float64
	var pathStartX, pathStartY float64
	
	for i := 0; i < len(p.currentPath); i++ {
		elem := p.currentPath[i]
		
		switch elem.Type {
		case "moveto":
			if len(elem.Points) > 0 {
				currentX = elem.Points[0].X
				currentY = elem.Points[0].Y
				pathStartX = currentX
				pathStartY = currentY
			}
			
		case "lineto":
			if len(elem.Points) > 0 {
				endX := elem.Points[0].X
				endY := elem.Points[0].Y
				
				// Apply CTM transformation
				startX, startY := p.transformPoint(currentX, currentY)
				endXTransformed, endYTransformed := p.transformPoint(endX, endY)
				
				line := LineObject{
					X0:          startX,
					Y0:          startY,
					X1:          endXTransformed,
					Y1:          endYTransformed,
					Width:       p.graphicsState.LineWidth,
					StrokeColor: strokeColor,
				}
				
				p.objects.Lines = append(p.objects.Lines, line)
				
				currentX = endX
				currentY = endY
			}
			
		case "close":
			// Draw line back to path start
			if currentX != pathStartX || currentY != pathStartY {
				startX, startY := p.transformPoint(currentX, currentY)
				endX, endY := p.transformPoint(pathStartX, pathStartY)
				
				line := LineObject{
					X0:          startX,
					Y0:          startY,
					X1:          endX,
					Y1:          endY,
					Width:       p.graphicsState.LineWidth,
					StrokeColor: strokeColor,
				}
				
				p.objects.Lines = append(p.objects.Lines, line)
			}
			
		case "curveto":
			// For now, approximate curves as lines (could be improved)
			if len(elem.Points) >= 3 {
				// Use the end point of the curve
				endX := elem.Points[2].X
				endY := elem.Points[2].Y
				
				startX, startY := p.transformPoint(currentX, currentY)
				endXTransformed, endYTransformed := p.transformPoint(endX, endY)
				
				// Create a curve object instead of line
				cp1X, cp1Y := p.transformPoint(elem.Points[0].X, elem.Points[0].Y)
				cp2X, cp2Y := p.transformPoint(elem.Points[1].X, elem.Points[1].Y)
				
				curve := CurveObject{
					Points: []Point{
						{X: startX, Y: startY},
						{X: cp1X, Y: cp1Y},
						{X: cp2X, Y: cp2Y},
						{X: endXTransformed, Y: endYTransformed},
					},
					StrokeColor: strokeColor,
					Width:       p.graphicsState.LineWidth,
				}
				
				p.objects.Curves = append(p.objects.Curves, curve)
				
				currentX = endX
				currentY = endY
			}
		}
	}
}

// transformPoint applies the current transformation matrix to a point
func (p *ContentStreamParser) transformPoint(x, y float64) (float64, float64) {
	ctm := p.graphicsState.CTM
	newX := ctm.A*x + ctm.C*y + ctm.E
	newY := ctm.B*x + ctm.D*y + ctm.F
	return newX, newY
}

// convertPDFColorToColor converts PDFColor to Color type
func (p *ContentStreamParser) convertPDFColorToColor(pdfColor PDFColor) Color {
	// Convert float RGB values (0-1) to uint8 (0-255)
	return Color{
		R: uint8(pdfColor.R * 255),
		G: uint8(pdfColor.G * 255),
		B: uint8(pdfColor.B * 255),
		A: 255, // Full opacity
	}
}

func (p *ContentStreamParser) extractString(str string) string {
	if strings.HasPrefix(str, "(") && strings.HasSuffix(str, ")") {
		// String literal
		str = strings.TrimPrefix(str, "(")
		str = strings.TrimSuffix(str, ")")
		
		// Handle escape sequences
		str = p.unescapeString(str)
		
		// Apply font encoding if available
		if p.textState.Font != nil {
			str = p.decodeString(str)
		}
		
		return str
	} else if strings.HasPrefix(str, "<") && strings.HasSuffix(str, ">") {
		// Hex string
		str = strings.TrimPrefix(str, "<")
		str = strings.TrimSuffix(str, ">")
		
		// Convert hex to string
		decoded := p.decodeHexString(str)
		
		// Apply font encoding if available
		if p.textState.Font != nil {
			decoded = p.decodeString(decoded)
		}
		
		return decoded
	}
	return str
}

// unescapeString handles PDF escape sequences
func (p *ContentStreamParser) unescapeString(str string) string {
	var result strings.Builder
	for i := 0; i < len(str); i++ {
		if str[i] == '\\' && i+1 < len(str) {
			switch str[i+1] {
			case 'n':
				result.WriteByte('\n')
				i++
			case 'r':
				result.WriteByte('\r')
				i++
			case 't':
				result.WriteByte('\t')
				i++
			case 'b':
				result.WriteByte('\b')
				i++
			case 'f':
				result.WriteByte('\f')
				i++
			case '(':
				result.WriteByte('(')
				i++
			case ')':
				result.WriteByte(')')
				i++
			case '\\':
				result.WriteByte('\\')
				i++
			default:
				// Check for octal escape sequence
				if i+3 < len(str) && str[i+1] >= '0' && str[i+1] <= '7' {
					// Try to parse octal
					endIdx := i + 4
					if endIdx > len(str) {
						endIdx = len(str)
					}
					octalStr := str[i+1:endIdx]
					if val, err := strconv.ParseInt(octalStr, 8, 16); err == nil {
						result.WriteByte(byte(val))
						i += len(octalStr)
					} else {
						result.WriteByte(str[i])
					}
				} else {
					result.WriteByte(str[i])
				}
			}
		} else {
			result.WriteByte(str[i])
		}
	}
	return result.String()
}

// decodeHexString converts hex string to bytes
func (p *ContentStreamParser) decodeHexString(hexStr string) string {
	var result strings.Builder
	for i := 0; i < len(hexStr); i += 2 {
		if i+1 < len(hexStr) {
			if val, err := strconv.ParseInt(hexStr[i:i+2], 16, 16); err == nil {
				result.WriteByte(byte(val))
			}
		} else {
			// Handle odd number of hex digits
			if val, err := strconv.ParseInt(hexStr[i:i+1]+"0", 16, 16); err == nil {
				result.WriteByte(byte(val))
			}
		}
	}
	return result.String()
}

// decodeString applies font encoding to decode the string
func (p *ContentStreamParser) decodeString(str string) string {
	// Check if we have a font with ToUnicode CMap
	if p.textState.Font != nil && p.textState.Font.ToUnicodeCMap != nil {
		// Convert string to bytes for CID extraction
		// Assuming the string is already in binary form (from hex or literal)
		// We need to process it as 2-byte CIDs
		
		var result strings.Builder
		data := []byte(str)
		
		// Process based on encoding type
		if p.textState.Font.Encoding == "Identity-H" || p.textState.Font.Encoding == "Identity-V" {
			// Identity encoding - 2-byte CIDs
			for i := 0; i < len(data); i += 2 {
				if i+1 >= len(data) {
					// Odd byte at the end
					cid := uint16(data[i])
					if unicode, ok := p.textState.Font.ToUnicodeCMap.MapCIDToUnicode(cid); ok {
						result.WriteString(unicode)
					} else {
						result.WriteByte(data[i])
					}
				} else {
					// Extract 2-byte CID
					cid := uint16(data[i])<<8 | uint16(data[i+1])
					
					if unicode, ok := p.textState.Font.ToUnicodeCMap.MapCIDToUnicode(cid); ok {
						result.WriteString(unicode)
					} else {
						// Try single-byte CIDs as fallback
						if unicode1, ok := p.textState.Font.ToUnicodeCMap.MapCIDToUnicode(uint16(data[i])); ok {
							result.WriteString(unicode1)
						} else {
							result.WriteByte(data[i])
						}
						if unicode2, ok := p.textState.Font.ToUnicodeCMap.MapCIDToUnicode(uint16(data[i+1])); ok {
							result.WriteString(unicode2)
						} else {
							result.WriteByte(data[i+1])
						}
					}
				}
			}
			return result.String()
		} else {
			// Other encodings - try single-byte CIDs
			for _, b := range data {
				cid := uint16(b)
				if unicode, ok := p.textState.Font.ToUnicodeCMap.MapCIDToUnicode(cid); ok {
					result.WriteString(unicode)
				} else {
					result.WriteByte(b)
				}
			}
			return result.String()
		}
	}
	
	// No ToUnicode CMap, return as-is
	return str
}

func (p *ContentStreamParser) parseTextArray(arrayStr string) []string {
	var elements []string
	var current strings.Builder
	inString := false
	inHexString := false
	parenDepth := 0
	
	for i := 0; i < len(arrayStr); i++ {
		ch := arrayStr[i]
		
		// Handle whitespace when not in string
		if !inString && !inHexString && isWhitespace(ch) {
			if current.Len() > 0 {
				elements = append(elements, current.String())
				current.Reset()
			}
			continue
		}
		
		if ch == '(' && !inHexString {
			if !inString {
				inString = true
				parenDepth = 1
				current.WriteByte(ch)
			} else {
				// Check if escaped
				if i > 0 && arrayStr[i-1] == '\\' {
					// Already added the backslash, just add paren
					current.WriteByte(ch)
				} else {
					parenDepth++
					current.WriteByte(ch)
				}
			}
		} else if ch == ')' && inString && !inHexString {
			// Check if escaped
			if i > 0 && arrayStr[i-1] == '\\' {
				current.WriteByte(ch)
			} else {
				parenDepth--
				current.WriteByte(ch)
				if parenDepth == 0 {
					inString = false
					elements = append(elements, current.String())
					current.Reset()
				}
			}
		} else if ch == '<' && !inString {
			if !inHexString {
				inHexString = true
				current.WriteByte(ch)
			}
		} else if ch == '>' && inHexString {
			current.WriteByte(ch)
			inHexString = false
			elements = append(elements, current.String())
			current.Reset()
		} else if ch == '-' && !inString && !inHexString {
			// Check if it's the start of a negative number
			if i+1 < len(arrayStr) && (arrayStr[i+1] >= '0' && arrayStr[i+1] <= '9' || arrayStr[i+1] == '.') {
				current.WriteByte(ch)
			} else {
				// It's just a minus sign, treat as separator
				if current.Len() > 0 {
					elements = append(elements, current.String())
					current.Reset()
				}
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