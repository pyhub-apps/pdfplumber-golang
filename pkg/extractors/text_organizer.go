package extractors

import (
	"sort"
	"strings"

	"github.com/pyhub-apps/pdfplumber-golang/pkg/pdf"
)

// TextOrganizer organizes text objects into lines and words
type TextOrganizer struct {
	xTolerance float64 // Horizontal tolerance for grouping characters into words
	yTolerance float64 // Vertical tolerance for grouping characters into lines
}

// NewTextOrganizer creates a new text organizer with default tolerances
func NewTextOrganizer() *TextOrganizer {
	return &TextOrganizer{
		xTolerance: 3.0,  // Default horizontal tolerance
		yTolerance: 3.0,  // Default vertical tolerance
	}
}

// SetTolerances sets the tolerances for text grouping
func (to *TextOrganizer) SetTolerances(xTol, yTol float64) {
	to.xTolerance = xTol
	to.yTolerance = yTol
}

// OrganizeText organizes character objects into structured text
func (to *TextOrganizer) OrganizeText(chars []pdf.CharObject) string {
	if len(chars) == 0 {
		return ""
	}
	
	// Sort characters by position (top to bottom, left to right)
	sortedChars := to.sortCharacters(chars)
	
	// Group characters into lines
	lines := to.groupIntoLines(sortedChars)
	
	// Group characters within lines into words
	var result strings.Builder
	for i, line := range lines {
		lineText := to.extractLineText(line)
		result.WriteString(lineText)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

// sortCharacters sorts characters by their position on the page
func (to *TextOrganizer) sortCharacters(chars []pdf.CharObject) []pdf.CharObject {
	sorted := make([]pdf.CharObject, len(chars))
	copy(sorted, chars)
	
	sort.Slice(sorted, func(i, j int) bool {
		// First sort by Y position (top to bottom)
		if abs(sorted[i].Y0-sorted[j].Y0) > to.yTolerance {
			return sorted[i].Y0 > sorted[j].Y0 // PDF coordinates: Y increases upward
		}
		// Then sort by X position (left to right)
		return sorted[i].X0 < sorted[j].X0
	})
	
	return sorted
}

// groupIntoLines groups characters into lines based on Y position
func (to *TextOrganizer) groupIntoLines(chars []pdf.CharObject) [][]pdf.CharObject {
	if len(chars) == 0 {
		return nil
	}
	
	var lines [][]pdf.CharObject
	var currentLine []pdf.CharObject
	
	currentY := chars[0].Y0
	
	for _, char := range chars {
		// Check if this character is on a new line
		if abs(char.Y0-currentY) > to.yTolerance {
			if len(currentLine) > 0 {
				lines = append(lines, currentLine)
			}
			currentLine = []pdf.CharObject{char}
			currentY = char.Y0
		} else {
			currentLine = append(currentLine, char)
		}
	}
	
	// Add the last line
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}
	
	return lines
}

// extractLineText extracts text from a line of characters
func (to *TextOrganizer) extractLineText(lineChars []pdf.CharObject) string {
	if len(lineChars) == 0 {
		return ""
	}
	
	// Sort characters by X position within the line
	sort.Slice(lineChars, func(i, j int) bool {
		return lineChars[i].X0 < lineChars[j].X0
	})
	
	var result strings.Builder
	var lastX float64
	
	for i, char := range lineChars {
		if i > 0 {
			// Check if there's a significant gap (indicating a space)
			gap := char.X0 - lastX
			if gap > to.xTolerance {
				// Add space if gap is large enough
				if gap > char.Width*0.5 { // If gap is more than half character width
					result.WriteString(" ")
				}
			}
		}
		result.WriteString(char.Text)
		lastX = char.X1
	}
	
	return result.String()
}

// ExtractWords extracts individual words from character objects
func (to *TextOrganizer) ExtractWords(chars []pdf.CharObject) []Word {
	if len(chars) == 0 {
		return nil
	}
	
	// Sort and group into lines
	sortedChars := to.sortCharacters(chars)
	lines := to.groupIntoLines(sortedChars)
	
	var words []Word
	
	for _, line := range lines {
		lineWords := to.extractWordsFromLine(line)
		words = append(words, lineWords...)
	}
	
	return words
}

// extractWordsFromLine extracts words from a single line of characters
func (to *TextOrganizer) extractWordsFromLine(lineChars []pdf.CharObject) []Word {
	if len(lineChars) == 0 {
		return nil
	}
	
	// Sort by X position
	sort.Slice(lineChars, func(i, j int) bool {
		return lineChars[i].X0 < lineChars[j].X0
	})
	
	var words []Word
	var currentWord []pdf.CharObject
	
	for i, char := range lineChars {
		if i == 0 {
			currentWord = []pdf.CharObject{char}
		} else {
			// Check if this character starts a new word
			gap := char.X0 - lineChars[i-1].X1
			if gap > to.xTolerance || gap > char.Width*0.3 {
				// Save current word and start new one
				if len(currentWord) > 0 {
					words = append(words, to.createWord(currentWord))
				}
				currentWord = []pdf.CharObject{char}
			} else {
				currentWord = append(currentWord, char)
			}
		}
	}
	
	// Add the last word
	if len(currentWord) > 0 {
		words = append(words, to.createWord(currentWord))
	}
	
	return words
}

// createWord creates a Word from a group of characters
func (to *TextOrganizer) createWord(chars []pdf.CharObject) Word {
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
		Text: text.String(),
		BBox: pdf.BoundingBox{
			X0: minX,
			Y0: minY,
			X1: maxX,
			Y1: maxY,
		},
		Characters: chars,
	}
}

// Word represents a word extracted from PDF
type Word struct {
	Text       string
	BBox       pdf.BoundingBox
	Characters []pdf.CharObject
}

// Line represents a line of text
type Line struct {
	Text  string
	BBox  pdf.BoundingBox
	Words []Word
}

// ExtractLines extracts structured lines with words
func (to *TextOrganizer) ExtractLines(chars []pdf.CharObject) []Line {
	if len(chars) == 0 {
		return nil
	}
	
	sortedChars := to.sortCharacters(chars)
	lineGroups := to.groupIntoLines(sortedChars)
	
	var lines []Line
	
	for _, lineChars := range lineGroups {
		words := to.extractWordsFromLine(lineChars)
		if len(words) == 0 {
			continue
		}
		
		// Calculate line bounding box
		minX, minY := words[0].BBox.X0, words[0].BBox.Y0
		maxX, maxY := words[0].BBox.X1, words[0].BBox.Y1
		
		var lineText strings.Builder
		for i, word := range words {
			if i > 0 {
				lineText.WriteString(" ")
			}
			lineText.WriteString(word.Text)
			
			minX = min(minX, word.BBox.X0)
			minY = min(minY, word.BBox.Y0)
			maxX = max(maxX, word.BBox.X1)
			maxY = max(maxY, word.BBox.Y1)
		}
		
		lines = append(lines, Line{
			Text: lineText.String(),
			BBox: pdf.BoundingBox{
				X0: minX,
				Y0: minY,
				X1: maxX,
				Y1: maxY,
			},
			Words: words,
		})
	}
	
	return lines
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}