package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// TokenType represents the type of a PDF token
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenNumber
	TokenString
	TokenHexString
	TokenName
	TokenKeyword
	TokenArrayStart
	TokenArrayEnd
	TokenDictStart
	TokenDictEnd
	TokenRef
)

func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenNumber:
		return "Number"
	case TokenString:
		return "String"
	case TokenHexString:
		return "HexString"
	case TokenName:
		return "Name"
	case TokenKeyword:
		return "Keyword"
	case TokenArrayStart:
		return "ArrayStart"
	case TokenArrayEnd:
		return "ArrayEnd"
	case TokenDictStart:
		return "DictStart"
	case TokenDictEnd:
		return "DictEnd"
	case TokenRef:
		return "Ref"
	default:
		return "Unknown"
	}
}

// Token represents a PDF token
type Token struct {
	Type  TokenType
	Value interface{}
}

// Lexer tokenizes PDF content
type Lexer struct {
	reader       *bufio.Reader
	buffer       []byte
	pos          int64
	pushedTokens []*Token // For unread support
}

// NewLexer creates a new PDF lexer
func NewLexer(r io.Reader) *Lexer {
	return &Lexer{
		reader:       bufio.NewReader(r),
		buffer:       make([]byte, 0, 1024),
		pushedTokens: make([]*Token, 0),
	}
}

// UnreadToken pushes a token back to be read again
func (l *Lexer) UnreadToken(token *Token) {
	l.pushedTokens = append(l.pushedTokens, token)
}

// Position returns the current position in the stream
func (l *Lexer) Position() int64 {
	return l.pos
}

// NextToken returns the next token from the stream
func (l *Lexer) NextToken() (*Token, error) {
	// Check if we have pushed tokens
	if len(l.pushedTokens) > 0 {
		token := l.pushedTokens[len(l.pushedTokens)-1]
		l.pushedTokens = l.pushedTokens[:len(l.pushedTokens)-1]
		return token, nil
	}
	
	// Skip whitespace and comments
	if err := l.skipWhitespaceAndComments(); err != nil {
		if err == io.EOF {
			return &Token{Type: TokenEOF}, nil
		}
		return nil, err
	}

	// Peek at the next character
	ch, err := l.peekByte()
	if err != nil {
		if err == io.EOF {
			return &Token{Type: TokenEOF}, nil
		}
		return nil, err
	}

	// Identify token type based on first character
	switch ch {
	case '[':
		l.readByte()
		return &Token{Type: TokenArrayStart}, nil
	case ']':
		l.readByte()
		return &Token{Type: TokenArrayEnd}, nil
	case '<':
		// Could be dictionary or hex string
		l.readByte()
		next, err := l.peekByte()
		if err != nil {
			return nil, err
		}
		if next == '<' {
			l.readByte()
			return &Token{Type: TokenDictStart}, nil
		}
		// Hex string
		return l.readHexString()
	case '>':
		// Should be >>
		l.readByte()
		next, err := l.readByte()
		if err != nil {
			return nil, err
		}
		if next != '>' {
			return nil, fmt.Errorf("expected >>, got >%c", next)
		}
		return &Token{Type: TokenDictEnd}, nil
	case '(':
		return l.readString()
	case '/':
		return l.readName()
	case '+', '-', '.', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return l.readNumber()
	default:
		// Must be a keyword
		return l.readKeyword()
	}
}

// skipWhitespaceAndComments skips whitespace and comments
func (l *Lexer) skipWhitespaceAndComments() error {
	for {
		ch, err := l.peekByte()
		if err != nil {
			return err
		}

		// Skip whitespace
		if isWhitespace(ch) {
			l.readByte()
			continue
		}

		// Skip comments
		if ch == '%' {
			l.readByte()
			// Read until end of line
			for {
				ch, err := l.readByte()
				if err != nil {
					return err
				}
				if ch == '\n' || ch == '\r' {
					break
				}
			}
			continue
		}

		break
	}
	return nil
}

// peekByte peeks at the next byte without consuming it
func (l *Lexer) peekByte() (byte, error) {
	b, err := l.reader.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// readByte reads and consumes the next byte
func (l *Lexer) readByte() (byte, error) {
	b, err := l.reader.ReadByte()
	if err == nil {
		l.pos++
	}
	return b, err
}

// readNumber reads a number token
func (l *Lexer) readNumber() (*Token, error) {
	l.buffer = l.buffer[:0]
	
	for {
		ch, err := l.peekByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if ch == '+' || ch == '-' || ch == '.' || (ch >= '0' && ch <= '9') {
			l.readByte()
			l.buffer = append(l.buffer, ch)
		} else {
			break
		}
	}

	str := string(l.buffer)
	
	// Check if it's a float
	if bytes.ContainsAny(l.buffer, ".") {
		f, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float: %s", str)
		}
		return &Token{Type: TokenNumber, Value: PDFFloat(f)}, nil
	}

	// It's an integer
	i, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid integer: %s", str)
	}
	return &Token{Type: TokenNumber, Value: PDFInt(i)}, nil
}

// readString reads a string token
func (l *Lexer) readString() (*Token, error) {
	l.buffer = l.buffer[:0]
	l.readByte() // consume opening (

	parenCount := 1
	escaped := false

	for parenCount > 0 {
		ch, err := l.readByte()
		if err != nil {
			return nil, fmt.Errorf("unterminated string: %v", err)
		}

		if escaped {
			// Handle escape sequences
			switch ch {
			case 'n':
				l.buffer = append(l.buffer, '\n')
			case 'r':
				l.buffer = append(l.buffer, '\r')
			case 't':
				l.buffer = append(l.buffer, '\t')
			case 'b':
				l.buffer = append(l.buffer, '\b')
			case 'f':
				l.buffer = append(l.buffer, '\f')
			case '(', ')', '\\':
				l.buffer = append(l.buffer, ch)
			default:
				// Octal escape sequence
				if ch >= '0' && ch <= '7' {
					octal := []byte{ch}
					for i := 0; i < 2; i++ {
						ch2, err := l.peekByte()
						if err != nil {
							break
						}
						if ch2 >= '0' && ch2 <= '7' {
							l.readByte()
							octal = append(octal, ch2)
						} else {
							break
						}
					}
					val, _ := strconv.ParseInt(string(octal), 8, 16)
					l.buffer = append(l.buffer, byte(val))
				} else {
					l.buffer = append(l.buffer, ch)
				}
			}
			escaped = false
		} else {
			switch ch {
			case '\\':
				escaped = true
			case '(':
				parenCount++
				l.buffer = append(l.buffer, ch)
			case ')':
				parenCount--
				if parenCount > 0 {
					l.buffer = append(l.buffer, ch)
				}
			default:
				l.buffer = append(l.buffer, ch)
			}
		}
	}

	return &Token{Type: TokenString, Value: PDFString(l.buffer)}, nil
}

// readHexString reads a hexadecimal string token
func (l *Lexer) readHexString() (*Token, error) {
	l.buffer = l.buffer[:0]

	for {
		ch, err := l.readByte()
		if err != nil {
			return nil, fmt.Errorf("unterminated hex string: %v", err)
		}

		if ch == '>' {
			break
		}

		if isHexDigit(ch) {
			l.buffer = append(l.buffer, ch)
		} else if !isWhitespace(ch) {
			return nil, fmt.Errorf("invalid character in hex string: %c", ch)
		}
	}

	// Convert hex to bytes
	if len(l.buffer)%2 != 0 {
		l.buffer = append(l.buffer, '0')
	}

	result := make([]byte, len(l.buffer)/2)
	for i := 0; i < len(result); i++ {
		val, err := strconv.ParseInt(string(l.buffer[i*2:i*2+2]), 16, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid hex string: %v", err)
		}
		result[i] = byte(val)
	}

	return &Token{Type: TokenHexString, Value: PDFString(result)}, nil
}

// readName reads a name token
func (l *Lexer) readName() (*Token, error) {
	l.buffer = l.buffer[:0]
	l.readByte() // consume /

	for {
		ch, err := l.peekByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if isDelimiter(ch) || isWhitespace(ch) {
			break
		}

		l.readByte()
		
		// Handle # escape sequences
		if ch == '#' {
			hex1, err := l.readByte()
			if err != nil {
				return nil, err
			}
			hex2, err := l.readByte()
			if err != nil {
				return nil, err
			}
			val, err := strconv.ParseInt(string([]byte{hex1, hex2}), 16, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid hex escape in name: %c%c", hex1, hex2)
			}
			l.buffer = append(l.buffer, byte(val))
		} else {
			l.buffer = append(l.buffer, ch)
		}
	}

	return &Token{Type: TokenName, Value: PDFName(l.buffer)}, nil
}

// readKeyword reads a keyword token
func (l *Lexer) readKeyword() (*Token, error) {
	l.buffer = l.buffer[:0]

	for {
		ch, err := l.peekByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if isDelimiter(ch) || isWhitespace(ch) {
			break
		}

		l.readByte()
		l.buffer = append(l.buffer, ch)
	}

	keyword := string(l.buffer)
	
	// Check for special keywords
	switch keyword {
	case "true":
		return &Token{Type: TokenKeyword, Value: PDFBool(true)}, nil
	case "false":
		return &Token{Type: TokenKeyword, Value: PDFBool(false)}, nil
	case "null":
		return &Token{Type: TokenKeyword, Value: PDFNull{}}, nil
	case "R":
		return &Token{Type: TokenRef}, nil
	default:
		return &Token{Type: TokenKeyword, Value: keyword}, nil
	}
}

// Helper functions
func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '\f' || ch == 0
}

func isDelimiter(ch byte) bool {
	return ch == '(' || ch == ')' || ch == '<' || ch == '>' || 
		ch == '[' || ch == ']' || ch == '{' || ch == '}' || 
		ch == '/' || ch == '%'
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'F') || (ch >= 'a' && ch <= 'f')
}