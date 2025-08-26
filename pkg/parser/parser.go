package parser

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// PDFParser is the main PDF parser
type PDFParser struct {
	reader   io.ReaderAt
	size     int64
	xref     *XRefTable
	trailer  PDFDict
	catalog  PDFDict
	objects  map[ObjectRef]PDFObject
}

// NewPDFParser creates a new PDF parser
func NewPDFParser(reader io.ReaderAt, size int64) *PDFParser {
	return &PDFParser{
		reader:  reader,
		size:    size,
		objects: make(map[ObjectRef]PDFObject),
	}
}

// Parse parses the PDF document
func (p *PDFParser) Parse() (*PDFDocument, error) {
	// Verify PDF header
	if err := p.verifyHeader(); err != nil {
		return nil, fmt.Errorf("invalid PDF header: %v", err)
	}

	// Find and parse xref table
	xrefOffset, err := p.findXRefOffset()
	if err != nil {
		return nil, fmt.Errorf("failed to find xref offset: %v", err)
	}

	if err := p.parseXRef(xrefOffset); err != nil {
		return nil, fmt.Errorf("failed to parse xref: %v", err)
	}

	// Get catalog
	if root, ok := p.trailer[PDFName("Root")]; ok {
		if ref, ok := root.(ObjectRef); ok {
			cat, err := p.GetObject(ref)
			if err != nil {
				return nil, fmt.Errorf("failed to get catalog: %v", err)
			}
			if dict, ok := cat.(PDFDict); ok {
				p.catalog = dict
			} else {
				return nil, fmt.Errorf("catalog is not a dictionary")
			}
		}
	} else {
		return nil, fmt.Errorf("no Root in trailer")
	}

	// Parse pages
	pages, err := p.parsePages()
	if err != nil {
		return nil, fmt.Errorf("failed to parse pages: %v", err)
	}

	// Get version
	version, _ := p.getVersion()

	doc := &PDFDocument{
		Version:  version,
		XRef:     p.xref,
		Trailer:  p.trailer,
		Catalog:  p.catalog,
		Pages:    pages,
		Objects:  p.objects,
		parser:   p,
	}
	
	// Set document reference in pages
	for _, page := range pages {
		page.Document = doc
	}
	
	return doc, nil
}

// verifyHeader verifies the PDF header
func (p *PDFParser) verifyHeader() error {
	header := make([]byte, 8)
	n, err := p.reader.ReadAt(header, 0)
	if err != nil || n < 8 {
		return fmt.Errorf("failed to read header")
	}

	if !bytes.HasPrefix(header, []byte("%PDF-")) {
		return fmt.Errorf("not a PDF file")
	}

	return nil
}

// getVersion gets the PDF version
func (p *PDFParser) getVersion() (string, error) {
	header := make([]byte, 16)
	n, err := p.reader.ReadAt(header, 0)
	if err != nil || n < 8 {
		return "", fmt.Errorf("failed to read header")
	}

	if bytes.HasPrefix(header, []byte("%PDF-")) {
		// Find end of version
		for i := 5; i < len(header); i++ {
			if header[i] == '\r' || header[i] == '\n' || header[i] == ' ' {
				return string(header[5:i]), nil
			}
		}
	}

	return "", fmt.Errorf("invalid PDF version")
}

// findXRefOffset finds the offset of the xref table
func (p *PDFParser) findXRefOffset() (int64, error) {
	// Read last 1024 bytes
	bufSize := int64(1024)
	if p.size < bufSize {
		bufSize = p.size
	}

	buf := make([]byte, bufSize)
	_, err := p.reader.ReadAt(buf, p.size-bufSize)
	if err != nil {
		return 0, err
	}

	// Find startxref
	idx := bytes.LastIndex(buf, []byte("startxref"))
	if idx < 0 {
		return 0, fmt.Errorf("startxref not found")
	}

	// Parse the offset
	lexer := NewLexer(bytes.NewReader(buf[idx+9:]))
	lexer.skipWhitespaceAndComments()
	
	token, err := lexer.NextToken()
	if err != nil {
		return 0, err
	}

	switch v := token.Value.(type) {
	case PDFInt:
		return int64(v), nil
	case PDFFloat:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("invalid xref offset")
	}
}

// parseXRef parses the cross-reference table
func (p *PDFParser) parseXRef(offset int64) error {
	p.xref = NewXRefTable()

	// Read xref section
	buf := make([]byte, 65536) // Start with 64KB buffer
	n, err := p.reader.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return err
	}
	buf = buf[:n]

	lexer := NewLexer(bytes.NewReader(buf))
	
	// Read "xref" keyword
	token, err := lexer.NextToken()
	if err != nil {
		return err
	}
	if kw, ok := token.Value.(string); !ok || kw != "xref" {
		return fmt.Errorf("expected 'xref', got %v", token.Value)
	}

	// Parse xref subsections
	for {
		// Read first object number or "trailer" keyword
		token, err = lexer.NextToken()
		if err != nil {
			return err
		}

		// Check if we've reached the trailer
		if kw, ok := token.Value.(string); ok && kw == "trailer" {
			break
		}

		// Must be first object number
		var firstObj int64
		switch v := token.Value.(type) {
		case PDFInt:
			firstObj = int64(v)
		case PDFFloat:
			firstObj = int64(v)
		default:
			return fmt.Errorf("expected object number or 'trailer', got %T: %v", token.Value, token.Value)
		}

		// Read count
		token, err = lexer.NextToken()
		if err != nil {
			return err
		}
		
		var count int64
		switch v := token.Value.(type) {
		case PDFInt:
			count = int64(v)
		case PDFFloat:
			count = int64(v)
		default:
			return fmt.Errorf("expected count, got %T: %v", token.Value, token.Value)
		}

		// Read entries
		for i := 0; i < int(count); i++ {
			// Read offset
			token, err = lexer.NextToken()
			if err != nil {
				return err
			}
			
			var offsetVal int64
			switch v := token.Value.(type) {
			case PDFInt:
				offsetVal = int64(v)
			case PDFFloat:
				offsetVal = int64(v)
			default:
				return fmt.Errorf("expected offset for entry %d, got %T: %v", i, token.Value, token.Value)
			}

			// Read generation
			token, err = lexer.NextToken()
			if err != nil {
				return err
			}
			
			var gen int64
			switch v := token.Value.(type) {
			case PDFInt:
				gen = int64(v)
			case PDFFloat:
				gen = int64(v)
			default:
				return fmt.Errorf("expected generation for entry %d, got %T: %v", i, token.Value, token.Value)
			}

			// Read flag (n or f)
			token, err = lexer.NextToken()
			if err != nil {
				return err
			}
			flag, ok := token.Value.(string)
			if !ok {
				return fmt.Errorf("expected flag for entry %d, got %T: %v", i, token.Value, token.Value)
			}

			ref := ObjectRef{
				Number:     int(firstObj) + i,
				Generation: int(gen),
			}

			entry := &XRefEntry{
				Offset:     offsetVal,
				Generation: int(gen),
				InUse:      flag == "n",
			}

			p.xref.Add(ref, entry)
		}
	}

	// Parse trailer dictionary
	trailer, err := p.parseObject(lexer)
	if err != nil {
		return fmt.Errorf("failed to parse trailer: %v", err)
	}

	dict, ok := trailer.(PDFDict)
	if !ok {
		return fmt.Errorf("trailer is not a dictionary")
	}
	p.trailer = dict

	return nil
}

// GetObject retrieves an object by reference
func (p *PDFParser) GetObject(ref ObjectRef) (PDFObject, error) {
	// Check cache
	if obj, ok := p.objects[ref]; ok {
		return obj, nil
	}

	// Get xref entry
	entry, ok := p.xref.Get(ref)
	if !ok || !entry.InUse {
		return PDFNull{}, nil
	}

	// Read object from file - use a larger buffer to ensure we get the full object
	buf := make([]byte, 131072) // 128KB buffer
	n, err := p.reader.ReadAt(buf, entry.Offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	buf = buf[:n]

	lexer := NewLexer(bytes.NewReader(buf))

	// Read object number
	token, err := lexer.NextToken()
	if err != nil {
		return nil, err
	}
	objNum, ok := token.Value.(PDFInt)
	if !ok || int(objNum) != ref.Number {
		return nil, fmt.Errorf("object number mismatch")
	}

	// Read generation number
	token, err = lexer.NextToken()
	if err != nil {
		return nil, err
	}
	genNum, ok := token.Value.(PDFInt)
	if !ok || int(genNum) != ref.Generation {
		return nil, fmt.Errorf("generation number mismatch")
	}

	// Read "obj" keyword
	token, err = lexer.NextToken()
	if err != nil {
		return nil, err
	}
	if kw, ok := token.Value.(string); !ok || kw != "obj" {
		return nil, fmt.Errorf("expected 'obj', got %v", token.Value)
	}

	// Parse the object
	obj, err := p.parseObject(lexer)
	if err != nil {
		return nil, err
	}

	// Check for stream
	if dict, ok := obj.(PDFDict); ok {
		token, _ = lexer.NextToken()
		if kw, ok := token.Value.(string); ok && kw == "stream" {
			// Read stream data
			stream, err := p.readStream(lexer, dict, entry.Offset+lexer.Position())
			if err != nil {
				return nil, err
			}
			obj = stream
		}
	}

	// Cache the object
	p.objects[ref] = obj

	return obj, nil
}

// parseObject parses a PDF object
func (p *PDFParser) parseObject(lexer *Lexer) (PDFObject, error) {
	token, err := lexer.NextToken()
	if err != nil {
		return nil, err
	}

	switch token.Type {
	case TokenEOF:
		return nil, io.EOF
	case TokenNumber:
		// Could be a number or an indirect reference
		num1 := token.Value.(PDFObject)
		
		// Try to read the next token
		token2, err := lexer.NextToken()
		if err != nil {
			// Just a number
			return num1, nil
		}
		
		// Check if it's a number (potential generation number)
		if num2, ok := token2.Value.(PDFInt); ok {
			// Try to read the third token
			token3, err := lexer.NextToken()
			if err != nil {
				// Not a reference, push token2 back
				lexer.UnreadToken(token2)
				return num1, nil
			}
			
			if token3.Type == TokenRef {
				// It's an indirect reference!
				return ObjectRef{
					Number:     int(num1.(PDFInt)),
					Generation: int(num2),
				}, nil
			} else {
				// Not a reference - push back token3 and token2
				lexer.UnreadToken(token3)
				lexer.UnreadToken(token2)
				return num1, nil
			}
		} else {
			// token2 is not a number, so num1 is just a number
			// Push token2 back
			lexer.UnreadToken(token2)
			return num1, nil
		}

	case TokenString, TokenHexString:
		return token.Value.(PDFString), nil
	case TokenName:
		return token.Value.(PDFName), nil
	case TokenKeyword:
		// Keywords can be bool, null, or string
		if obj, ok := token.Value.(PDFObject); ok {
			return obj, nil
		}
		// String keyword
		return PDFName(token.Value.(string)), nil
	case TokenArrayStart:
		return p.parseArray(lexer)
	case TokenDictStart:
		return p.parseDict(lexer)
	default:
		return nil, fmt.Errorf("unexpected token type: %v", token.Type)
	}
}

// parseArray parses a PDF array
func (p *PDFParser) parseArray(lexer *Lexer) (PDFArray, error) {
	array := PDFArray{}

	for {
		// Use parseObject for each array element
		// But first check for array end
		token, err := lexer.NextToken()
		if err != nil {
			return nil, err
		}

		if token.Type == TokenArrayEnd {
			break
		}

		// Put token back and parse as object
		lexer.UnreadToken(token)
		obj, err := p.parseObject(lexer)
		if err != nil {
			return nil, err
		}
		
		array = append(array, obj)
	}

	return array, nil
}

// parseDict parses a PDF dictionary
func (p *PDFParser) parseDict(lexer *Lexer) (PDFDict, error) {
	dict := make(PDFDict)

	for {
		// Read key (should be a name)
		token, err := lexer.NextToken()
		if err != nil {
			return nil, err
		}

		if token.Type == TokenDictEnd {
			break
		}

		if token.Type != TokenName {
			return nil, fmt.Errorf("expected name for dict key, got %v (value: %v)", token.Type, token.Value)
		}
		key := token.Value.(PDFName)

		// Read value
		value, err := p.parseObject(lexer)
		if err != nil {
			return nil, fmt.Errorf("error parsing value for key %s: %v", key, err)
		}

		dict[key] = value
	}

	return dict, nil
}

// handleToken handles a token that's already been read
func (p *PDFParser) handleToken(token *Token, lexer *Lexer) (PDFObject, error) {
	switch token.Type {
	case TokenString, TokenHexString:
		return token.Value.(PDFString), nil
	case TokenName:
		return token.Value.(PDFName), nil
	case TokenKeyword:
		// Keywords can be bool, null, or string
		if obj, ok := token.Value.(PDFObject); ok {
			return obj, nil
		}
		// String keyword
		return PDFName(token.Value.(string)), nil
	case TokenArrayStart:
		return p.parseArray(lexer)
	case TokenDictStart:
		return p.parseDict(lexer)
	case TokenNumber:
		return token.Value.(PDFObject), nil
	default:
		if obj, ok := token.Value.(PDFObject); ok {
			return obj, nil
		}
		return nil, fmt.Errorf("unexpected token value type: %T (token type: %v, value: %v)", token.Value, token.Type, token.Value)
	}
}

// readStream reads stream data
func (p *PDFParser) readStream(lexer *Lexer, dict PDFDict, offset int64) (*PDFStream, error) {
	// Get stream length
	lengthObj := dict.Get(PDFName("Length"))
	if lengthObj == nil {
		return nil, fmt.Errorf("stream missing Length")
	}

	var length int64
	switch v := lengthObj.(type) {
	case PDFInt:
		length = int64(v)
	case ObjectRef:
		// Resolve indirect reference
		obj, err := p.GetObject(v)
		if err != nil {
			return nil, err
		}
		if i, ok := obj.(PDFInt); ok {
			length = int64(i)
		} else {
			return nil, fmt.Errorf("invalid stream length")
		}
	default:
		return nil, fmt.Errorf("invalid stream length type")
	}

	// Skip whitespace after "stream" keyword
	// PDF spec requires either LF or CRLF after "stream"
	skipBytes := 0
	peekBuf := make([]byte, 2)
	p.reader.ReadAt(peekBuf, offset)
	if peekBuf[0] == '\r' && peekBuf[1] == '\n' {
		skipBytes = 2
	} else if peekBuf[0] == '\n' {
		skipBytes = 1
	} else if peekBuf[0] == '\r' {
		skipBytes = 1
	}
	
	// Read stream data
	data := make([]byte, length)
	n, err := p.reader.ReadAt(data, offset+int64(skipBytes))
	if err != nil && err != io.EOF {
		return nil, err
	}
	data = data[:n]

	// Decode if necessary
	if filter := dict.Get(PDFName("Filter")); filter != nil {
		data, err = p.decodeStream(data, filter)
		if err != nil {
			return nil, err
		}
	}

	return &PDFStream{
		Dict: dict,
		Data: data,
	}, nil
}

// decodeStream decodes stream data based on filter
func (p *PDFParser) decodeStream(data []byte, filter PDFObject) ([]byte, error) {
	var filters []PDFName

	switch f := filter.(type) {
	case PDFName:
		filters = []PDFName{f}
	case PDFArray:
		for _, item := range f {
			if name, ok := item.(PDFName); ok {
				filters = append(filters, name)
			}
		}
	default:
		return data, nil
	}

	// Apply filters in order
	for _, f := range filters {
		var err error
		switch string(f) {
		case "FlateDecode":
			data, err = p.flateDecode(data)
		case "ASCIIHexDecode":
			data, err = p.asciiHexDecode(data)
		case "ASCII85Decode":
			data, err = p.ascii85Decode(data)
		default:
			// Unknown filter, return data as-is
			return data, nil
		}
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// flateDecode decodes FlateDecode (zlib) compressed data
func (p *PDFParser) flateDecode(data []byte) ([]byte, error) {
	// Try zlib first (with header)
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err == nil {
		defer reader.Close()
		return io.ReadAll(reader)
	}
	
	// If zlib fails, try raw DEFLATE (without header)
	// This is common in PDF files
	reader2 := flate.NewReader(bytes.NewReader(data))
	defer reader2.Close()
	return io.ReadAll(reader2)
}

// asciiHexDecode decodes ASCIIHexDecode data
func (p *PDFParser) asciiHexDecode(data []byte) ([]byte, error) {
	// Remove whitespace and >
	clean := []byte{}
	for _, b := range data {
		if (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f') {
			clean = append(clean, b)
		}
	}

	// Pad if odd length
	if len(clean)%2 != 0 {
		clean = append(clean, '0')
	}

	// Convert hex to bytes
	result := make([]byte, len(clean)/2)
	for i := 0; i < len(result); i++ {
		val, err := strconv.ParseInt(string(clean[i*2:i*2+2]), 16, 16)
		if err != nil {
			return nil, err
		}
		result[i] = byte(val)
	}

	return result, nil
}

// ascii85Decode decodes ASCII85Decode data
func (p *PDFParser) ascii85Decode(data []byte) ([]byte, error) {
	// Simple implementation - can be improved
	// For now, return data as-is
	return data, nil
}

// parsePages parses all pages from the document
func (p *PDFParser) parsePages() ([]*PDFPage, error) {
	// Get Pages dictionary from catalog
	pagesRef, ok := p.catalog[PDFName("Pages")].(ObjectRef)
	if !ok {
		return nil, fmt.Errorf("invalid Pages reference in catalog")
	}

	pagesObj, err := p.GetObject(pagesRef)
	if err != nil {
		return nil, err
	}

	pagesDict, ok := pagesObj.(PDFDict)
	if !ok {
		return nil, fmt.Errorf("Pages object is not a dictionary")
	}

	// Parse page tree recursively
	var pages []*PDFPage
	pageNum := 1
	err = p.parsePageTree(pagesDict, &pages, &pageNum)
	if err != nil {
		return nil, err
	}

	return pages, nil
}

// parsePageTree recursively parses the page tree
func (p *PDFParser) parsePageTree(node PDFDict, pages *[]*PDFPage, pageNum *int) error {
	return p.parsePageTreeWithInheritance(node, pages, pageNum, nil)
}

// parsePageTreeWithInheritance recursively parses the page tree with inheritance
func (p *PDFParser) parsePageTreeWithInheritance(node PDFDict, pages *[]*PDFPage, pageNum *int, inherited PDFDict) error {
	nodeType, ok := node.GetName(PDFName("Type"))
	if !ok {
		return fmt.Errorf("missing Type in page tree node")
	}

	// Create inherited properties for children
	childInherited := make(PDFDict)
	if inherited != nil {
		// Copy inherited properties
		for k, v := range inherited {
			childInherited[k] = v
		}
	}
	
	// Add inheritable properties from this node
	inheritableKeys := []PDFName{
		PDFName("Resources"),
		PDFName("MediaBox"),
		PDFName("CropBox"),
		PDFName("Rotate"),
	}
	
	for _, key := range inheritableKeys {
		if val := node.Get(key); val != nil {
			childInherited[key] = val
		}
	}

	switch string(nodeType) {
	case "Pages":
		// Internal node - process children
		kids, ok := node.GetArray(PDFName("Kids"))
		if !ok {
			return fmt.Errorf("missing Kids in Pages node")
		}

		for _, kidRef := range kids {
			if ref, ok := kidRef.(ObjectRef); ok {
				kidObj, err := p.GetObject(ref)
				if err != nil {
					return err
				}
				if kidDict, ok := kidObj.(PDFDict); ok {
					err = p.parsePageTreeWithInheritance(kidDict, pages, pageNum, childInherited)
					if err != nil {
						return err
					}
				}
			}
		}

	case "Page":
		// Leaf node - create page with inherited properties
		// Merge inherited properties with page properties (page properties override)
		mergedDict := make(PDFDict)
		if inherited != nil {
			for k, v := range inherited {
				mergedDict[k] = v
			}
		}
		for k, v := range node {
			mergedDict[k] = v
		}
		
		page := &PDFPage{
			Number: *pageNum,
			Dict:   mergedDict,
		}
		*pageNum++

		// Get resources from merged dictionary (includes inherited)
		if res := mergedDict.Get(PDFName("Resources")); res != nil {
			switch r := res.(type) {
			case PDFDict:
				page.Resources = r
			case ObjectRef:
				resObj, err := p.GetObject(r)
				if err == nil {
					if resDict, ok := resObj.(PDFDict); ok {
						page.Resources = resDict
					}
				}
			}
		}

		// Get content streams from merged dictionary
		if contents := mergedDict.Get(PDFName("Contents")); contents != nil {
			switch c := contents.(type) {
			case ObjectRef:
				obj, err := p.GetObject(c)
				if err != nil {
					fmt.Printf("Error getting content stream %s: %v\n", c.String(), err)
				} else {
					if stream, ok := obj.(*PDFStream); ok {
						page.Contents = []PDFStream{*stream}
					} else {
						fmt.Printf("Content object %s is not a stream: %T\n", c.String(), obj)
					}
				}
			case PDFArray:
				for _, item := range c {
					if ref, ok := item.(ObjectRef); ok {
						obj, err := p.GetObject(ref)
						if err != nil {
							fmt.Printf("Error getting content stream %s: %v\n", ref.String(), err)
						} else {
							if stream, ok := obj.(*PDFStream); ok {
								page.Contents = append(page.Contents, *stream)
							} else {
								fmt.Printf("Content object %s is not a stream: %T\n", ref.String(), obj)
							}
						}
					}
				}
			}
		}

		// Get MediaBox from merged dictionary
		if mediaBox := p.getPageBox(mergedDict, PDFName("MediaBox")); mediaBox != nil {
			page.MediaBox = mediaBox
		}

		// Get CropBox from merged dictionary
		if cropBox := p.getPageBox(mergedDict, PDFName("CropBox")); cropBox != nil {
			page.CropBox = cropBox
		} else {
			page.CropBox = page.MediaBox
		}

		*pages = append(*pages, page)
	}

	return nil
}

// getPageBox gets a page box (MediaBox, CropBox, etc.)
func (p *PDFParser) getPageBox(page PDFDict, boxName PDFName) []float64 {
	if box := page.Get(boxName); box != nil {
		switch b := box.(type) {
		case PDFArray:
			if len(b) == 4 {
				result := make([]float64, 4)
				for i, val := range b {
					switch v := val.(type) {
					case PDFInt:
						result[i] = float64(v)
					case PDFFloat:
						result[i] = float64(v)
					}
				}
				return result
			}
		case ObjectRef:
			obj, err := p.GetObject(b)
			if err == nil {
				if arr, ok := obj.(PDFArray); ok && len(arr) == 4 {
					result := make([]float64, 4)
					for i, val := range arr {
						switch v := val.(type) {
						case PDFInt:
							result[i] = float64(v)
						case PDFFloat:
							result[i] = float64(v)
						}
					}
					return result
				}
			}
		}
	}
	return nil
}

// GetPageCount returns the number of pages in the document
func (d *PDFDocument) GetPageCount() int {
	return len(d.Pages)
}

// GetPage returns a specific page by index (0-based)
func (d *PDFDocument) GetPage(index int) (*PDFPage, error) {
	if index < 0 || index >= len(d.Pages) {
		return nil, fmt.Errorf("page index out of range")
	}
	return d.Pages[index], nil
}

// GetObject returns an object from the document
func (d *PDFDocument) GetObject(ref ObjectRef) (PDFObject, error) {
	// Check if it's already loaded
	if obj, ok := d.Objects[ref]; ok {
		return obj, nil
	}
	
	// Try to load it using the parser
	if d.parser != nil {
		if p, ok := d.parser.(*PDFParser); ok {
			obj, err := p.GetObject(ref)
			if err == nil {
				// Cache it
				d.Objects[ref] = obj
				return obj, nil
			}
			return nil, err
		}
	}
	
	// If not found and no parser available
	return nil, fmt.Errorf("object %s not found in document", ref.String())
}

// GetContentString returns the content stream as a string (for debugging)
func (p *PDFPage) GetContentString() string {
	var result strings.Builder
	for _, stream := range p.Contents {
		result.Write(stream.Data)
		result.WriteString("\n")
	}
	return result.String()
}