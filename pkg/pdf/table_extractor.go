package pdf

import (
	"math"
	"sort"
	"strings"
)

// TableExtractor handles table extraction from PDF pages
type tableExtractor struct {
	page              Page
	verticalStrategy  string
	horizontalStrategy string
	minTableSize      int
	textTolerance     float64
	snapTolerance     float64
	joinTolerance     float64
	edgeTolerance     float64
}

// newTableExtractor creates a new table extractor with default settings
func newTableExtractor(page Page, opts ...TableExtractionOption) *tableExtractor {
	// Default configuration
	config := &tableExtractionConfig{
		VerticalStrategy:   "lines",
		HorizontalStrategy: "lines", 
		MinTableSize:       3,
		TextTolerance:      3.0,
	}
	
	// Apply options
	for _, opt := range opts {
		opt(config)
	}
	
	return &tableExtractor{
		page:               page,
		verticalStrategy:   config.VerticalStrategy,
		horizontalStrategy: config.HorizontalStrategy,
		minTableSize:       config.MinTableSize,
		textTolerance:      config.TextTolerance,
		snapTolerance:      3.0,
		joinTolerance:      3.0,
		edgeTolerance:      10.0,
	}
}

// ExtractTables extracts tables from the page
func (te *tableExtractor) ExtractTables() []Table {
	tables := []Table{}
	
	// Get all objects from the page
	objects := te.page.GetObjects()
	// fmt.Printf("[DEBUG-TABLE] ExtractTables: Found %d lines, %d rects, %d chars\n",
	//	len(objects.Lines), len(objects.Rects), len(objects.Chars))
	
	// Try line-based table extraction first
	if te.verticalStrategy == "lines" || te.horizontalStrategy == "lines" {
		// fmt.Println("[DEBUG-TABLE] Using line-based extraction strategy")
		lineTables := te.extractLineBasedTables(objects)
		// fmt.Printf("[DEBUG-TABLE] Line-based extraction found %d tables\n", len(lineTables))
		tables = append(tables, lineTables...)
	}
	
	// If no tables found with lines, try text-based detection
	if len(tables) == 0 {
		// fmt.Println("[DEBUG-TABLE] No tables found with lines, trying text-based extraction")
		textTables := te.extractTextBasedTables(objects)
		tables = append(tables, textTables...)
	}
	
	return tables
}

// extractLineBasedTables extracts tables using lines and rectangles
func (te *tableExtractor) extractLineBasedTables(objects Objects) []Table {
	tables := []Table{}
	
	// First try to detect tables from row rectangles
	if len(objects.Rects) > te.minTableSize {
		// fmt.Printf("[DEBUG-TABLE] Checking %d rectangles for row-based table\n", len(objects.Rects))
		rowTable := te.extractTableFromRowRectangles(objects)
		if rowTable != nil && len(rowTable.Rows) >= te.minTableSize {
			// fmt.Printf("[DEBUG-TABLE] Found table with %d rows from rectangles\n", len(rowTable.Rows))
			return []Table{*rowTable}
		}
	}
	
	// Collect all horizontal and vertical lines
	hLines, vLines := te.collectTableLines(objects)
	// fmt.Printf("[DEBUG-TABLE] Collected %d horizontal lines, %d vertical lines\n", len(hLines), len(vLines))
	
	// Also consider rectangles as potential table cells
	for _, rect := range objects.Rects {
		// Add rectangle edges as lines
		// Top edge
		hLines = append(hLines, 
			LineObject{X0: rect.X0, Y0: rect.Y0, X1: rect.X1, Y1: rect.Y0, Width: rect.Width})
		// Bottom edge
		hLines = append(hLines, 
			LineObject{X0: rect.X0, Y0: rect.Y1, X1: rect.X1, Y1: rect.Y1, Width: rect.Width})
		// Left edge
		vLines = append(vLines,
			LineObject{X0: rect.X0, Y0: rect.Y0, X1: rect.X0, Y1: rect.Y1, Width: rect.Width})
		// Right edge
		vLines = append(vLines,
			LineObject{X0: rect.X1, Y0: rect.Y0, X1: rect.X1, Y1: rect.Y1, Width: rect.Width})
	}
	// fmt.Printf("[DEBUG-TABLE] After adding rect edges: %d h-lines, %d v-lines\n", len(hLines), len(vLines))
	
	// Find table regions (intersecting horizontal and vertical lines)
	tableRegions := te.findTableRegions(hLines, vLines)
	// fmt.Printf("[DEBUG-TABLE] Found %d potential table regions\n", len(tableRegions))
	
	// For each table region, extract the table
	for _, region := range tableRegions {
		table := te.extractTableFromRegion(region, objects)
		if len(table.Rows) >= te.minTableSize {
			tables = append(tables, table)
		}
	}
	
	return tables
}

// collectTableLines separates lines into horizontal and vertical
func (te *tableExtractor) collectTableLines(objects Objects) ([]LineObject, []LineObject) {
	var hLines, vLines []LineObject
	
	for _, line := range objects.Lines {
		// Check if line is horizontal or vertical
		if math.Abs(line.Y1-line.Y0) < te.snapTolerance {
			// Horizontal line
			// if i < 3 {
			//	fmt.Printf("[DEBUG-TABLE]   H-Line %d: Y=%.2f, X from %.2f to %.2f\n", i, line.Y0, line.X0, line.X1)
			// }
			hLines = append(hLines, line)
		} else if math.Abs(line.X1-line.X0) < te.snapTolerance {
			// Vertical line
			// if i < 3 {
			//	fmt.Printf("[DEBUG-TABLE]   V-Line %d: X=%.2f, Y from %.2f to %.2f\n", i, line.X0, line.Y0, line.Y1)
			// }
			vLines = append(vLines, line)
		}
	}
	
	return hLines, vLines
}

// tableRegion represents a potential table area
type tableRegion struct {
	BBox      BoundingBox
	HLines    []float64 // Y positions of horizontal lines
	VLines    []float64 // X positions of vertical lines
	Cells     [][]BoundingBox
}

// findTableRegions identifies regions that might contain tables
func (te *tableExtractor) findTableRegions(hLines, vLines []LineObject) []tableRegion {
	regions := []tableRegion{}
	
	// Group lines that are close together
	hGroups := te.groupLines(hLines, true)
	vGroups := te.groupLines(vLines, false)
	// fmt.Printf("[DEBUG-TABLE] Grouped into %d horizontal groups, %d vertical groups\n", len(hGroups), len(vGroups))
	
	// For each group combination, check if it forms a table
	for _, hGroup := range hGroups {
		for _, vGroup := range vGroups {
			// fmt.Printf("[DEBUG-TABLE]   Group %d-%d: %d h-lines, %d v-lines\n", i, j, len(hGroup), len(vGroup))
			if len(hGroup) >= 2 && len(vGroup) >= 2 {
				region := te.createTableRegion(hGroup, vGroup)
				if region != nil {
					// fmt.Printf("[DEBUG-TABLE]     Created region with %d x %d cells\n", len(region.HLines), len(region.VLines))
					regions = append(regions, *region)
				} else {
					// fmt.Println("[DEBUG-TABLE]     Failed to create region")
				}
			}
		}
	}
	
	return regions
}

// groupLines groups lines that are close together
func (te *tableExtractor) groupLines(lines []LineObject, horizontal bool) [][]LineObject {
	if len(lines) == 0 {
		return [][]LineObject{}
	}
	
	// Sort lines by position
	sort.Slice(lines, func(i, j int) bool {
		if horizontal {
			return lines[i].Y0 < lines[j].Y0
		}
		return lines[i].X0 < lines[j].X0
	})
	
	// Group lines that are close together
	groups := [][]LineObject{}
	currentGroup := []LineObject{lines[0]}
	
	for i := 1; i < len(lines); i++ {
		var pos, prevPos float64
		if horizontal {
			pos = lines[i].Y0
			prevPos = lines[i-1].Y0
		} else {
			pos = lines[i].X0
			prevPos = lines[i-1].X0
		}
		
		if math.Abs(pos-prevPos) > 30 { // Gap threshold (was 50, lowered for table detection)
			// Start new group
			groups = append(groups, currentGroup)
			currentGroup = []LineObject{lines[i]}
		} else {
			currentGroup = append(currentGroup, lines[i])
		}
	}
	
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}
	
	return groups
}

// createTableRegion creates a table region from line groups
func (te *tableExtractor) createTableRegion(hLines, vLines []LineObject) *tableRegion {
	// Get unique positions
	hPositions := te.getUniquePositions(hLines, true)
	vPositions := te.getUniquePositions(vLines, false)
	
	// fmt.Printf("[DEBUG-TABLE]       Unique positions: %d h, %d v\n", len(hPositions), len(vPositions))
	
	if len(hPositions) < 2 || len(vPositions) < 2 {
		// fmt.Printf("[DEBUG-TABLE]       Not enough unique positions for a table\n")
		return nil
	}
	
	// Sort positions
	sort.Float64s(hPositions)
	sort.Float64s(vPositions)
	
	// Create cells
	cells := make([][]BoundingBox, len(hPositions)-1)
	for i := 0; i < len(hPositions)-1; i++ {
		cells[i] = make([]BoundingBox, len(vPositions)-1)
		for j := 0; j < len(vPositions)-1; j++ {
			cells[i][j] = BoundingBox{
				X0: vPositions[j],
				Y0: hPositions[i],
				X1: vPositions[j+1],
				Y1: hPositions[i+1],
			}
		}
	}
	
	// Calculate bounding box
	bbox := BoundingBox{
		X0: vPositions[0],
		Y0: hPositions[0],
		X1: vPositions[len(vPositions)-1],
		Y1: hPositions[len(hPositions)-1],
	}
	
	return &tableRegion{
		BBox:   bbox,
		HLines: hPositions,
		VLines: vPositions,
		Cells:  cells,
	}
}

// getUniquePositions gets unique line positions with tolerance
func (te *tableExtractor) getUniquePositions(lines []LineObject, horizontal bool) []float64 {
	posMap := make(map[float64]bool)
	
	for _, line := range lines {
		var pos float64
		if horizontal {
			// For horizontal lines, use Y position
			pos = math.Round(line.Y0/te.snapTolerance) * te.snapTolerance
		} else {
			// For vertical lines, use X position
			pos = math.Round(line.X0/te.snapTolerance) * te.snapTolerance
		}
		posMap[pos] = true
	}
	
	positions := []float64{}
	for pos := range posMap {
		positions = append(positions, pos)
	}
	
	return positions
}

// extractTableFromRegion extracts table data from a region
func (te *tableExtractor) extractTableFromRegion(region tableRegion, objects Objects) Table {
	rows := make([][]string, len(region.Cells))
	
	for i, row := range region.Cells {
		rows[i] = make([]string, len(row))
		for j, cell := range row {
			// Get text within this cell
			cellText := te.extractCellText(cell, objects.Chars)
			rows[i][j] = cellText
		}
	}
	
	return Table{
		Rows: rows,
		BBox: region.BBox,
	}
}

// extractCellText extracts text from a cell
func (te *tableExtractor) extractCellText(cell BoundingBox, chars []CharObject) string {
	var cellChars []CharObject
	
	// Collect characters within the cell
	for _, char := range chars {
		charBBox := char.GetBBox()
		// Check if character center is within cell
		centerX := (charBBox.X0 + charBBox.X1) / 2
		centerY := (charBBox.Y0 + charBBox.Y1) / 2
		
		if centerX >= cell.X0 && centerX <= cell.X1 &&
		   centerY >= cell.Y0 && centerY <= cell.Y1 {
			cellChars = append(cellChars, char)
		}
	}
	
	// Sort characters by position
	sort.Slice(cellChars, func(i, j int) bool {
		// Sort by Y first (top to bottom), then by X (left to right)
		if math.Abs(cellChars[i].Y0-cellChars[j].Y0) > te.textTolerance {
			return cellChars[i].Y0 < cellChars[j].Y0
		}
		return cellChars[i].X0 < cellChars[j].X0
	})
	
	// Build text from characters
	text := ""
	lastY := -1000.0
	lastX := -1000.0
	
	for _, char := range cellChars {
		// Check if we need a newline
		if lastY > 0 && math.Abs(char.Y0-lastY) > te.textTolerance {
			text += "\n"
			lastX = -1000.0
		} else if lastX > 0 && char.X0-lastX > te.textTolerance {
			// Add space between words
			text += " "
		}
		
		text += char.Text
		lastY = char.Y0
		lastX = char.X1
	}
	
	return text
}

// extractTextBasedTables extracts tables using text alignment
func (te *tableExtractor) extractTextBasedTables(objects Objects) []Table {
	tables := []Table{}
	
	// Use words instead of individual characters for better column detection
	words := te.page.ExtractWords()
	if len(words) == 0 {
		return tables
	}
	
	// Group words into lines
	lines := te.groupWordsIntoLines(words)
	
	// Find aligned columns based on word positions
	columns := te.findAlignedColumnsFromWords(lines)
	
	// If we have consistent columns, create a table
	if len(columns) > 1 && len(lines) >= te.minTableSize {
		table := te.createTableFromWordLines(lines, columns)
		if len(table.Rows) >= te.minTableSize {
			tables = append(tables, table)
		}
	}
	
	return tables
}

// textLine represents a line of text with its characters
type textLine struct {
	Chars []CharObject
	BBox  BoundingBox
	Y     float64
}

// wordLine represents a line of words
type wordLine struct {
	Words []Word
	BBox  BoundingBox
	Y     float64
}

// groupCharsIntoLines groups characters into text lines
func (te *tableExtractor) groupCharsIntoLines(chars []CharObject) []textLine {
	if len(chars) == 0 {
		return []textLine{}
	}
	
	// Sort chars by Y position
	sortedChars := make([]CharObject, len(chars))
	copy(sortedChars, chars)
	sort.Slice(sortedChars, func(i, j int) bool {
		return sortedChars[i].Y0 < sortedChars[j].Y0
	})
	
	lines := []textLine{}
	currentLine := textLine{
		Chars: []CharObject{sortedChars[0]},
		Y:     sortedChars[0].Y0,
	}
	
	for i := 1; i < len(sortedChars); i++ {
		if math.Abs(sortedChars[i].Y0-currentLine.Y) < te.textTolerance {
			// Same line
			currentLine.Chars = append(currentLine.Chars, sortedChars[i])
		} else {
			// New line
			lines = append(lines, te.finalizeLine(currentLine))
			currentLine = textLine{
				Chars: []CharObject{sortedChars[i]},
				Y:     sortedChars[i].Y0,
			}
		}
	}
	
	if len(currentLine.Chars) > 0 {
		lines = append(lines, te.finalizeLine(currentLine))
	}
	
	return lines
}

// finalizeLine calculates the bounding box for a text line
func (te *tableExtractor) finalizeLine(line textLine) textLine {
	// Sort chars by X position
	sort.Slice(line.Chars, func(i, j int) bool {
		return line.Chars[i].X0 < line.Chars[j].X0
	})
	
	// Calculate bounding box
	minX := line.Chars[0].X0
	maxX := line.Chars[len(line.Chars)-1].X1
	minY := line.Chars[0].Y0
	maxY := line.Chars[0].Y1
	
	for _, char := range line.Chars {
		minY = min(minY, char.Y0)
		maxY = max(maxY, char.Y1)
	}
	
	line.BBox = BoundingBox{
		X0: minX,
		Y0: minY,
		X1: maxX,
		Y1: maxY,
	}
	
	return line
}

// findAlignedColumns finds vertically aligned text columns
func (te *tableExtractor) findAlignedColumns(lines []textLine) []float64 {
	if len(lines) < 2 {
		return []float64{}
	}
	
	// Collect all unique X positions
	xPositions := make(map[float64]int)
	
	for _, line := range lines {
		for _, char := range line.Chars {
			// Round to snap tolerance
			x := math.Round(char.X0/te.snapTolerance) * te.snapTolerance
			xPositions[x]++
		}
	}
	
	// Find positions that appear in multiple lines
	columns := []float64{}
	minCount := len(lines) / 2 // At least half the lines
	
	for x, count := range xPositions {
		if count >= minCount {
			columns = append(columns, x)
		}
	}
	
	sort.Float64s(columns)
	return columns
}

// extractTableFromRowRectangles extracts table when rectangles represent rows
func (te *tableExtractor) extractTableFromRowRectangles(objects Objects) *Table {
	// Check if rectangles are aligned as table rows
	rects := objects.Rects
	// fmt.Printf("[DEBUG-TABLE]   Checking rectangles: %d rects, min size: %d\n", len(rects), te.minTableSize)
	if len(rects) < te.minTableSize {
		// fmt.Println("[DEBUG-TABLE]   Not enough rectangles for a table")
		return nil
	}
	
	// Check if rectangles are horizontally aligned and vertically stacked
	var minX, maxX float64 = rects[0].X0, rects[0].X1
	// fmt.Printf("[DEBUG-TABLE]   First rect bounds: X from %.2f to %.2f\n", minX, maxX)
	for _, rect := range rects {
		if math.Abs(rect.X0-minX) > te.snapTolerance || math.Abs(rect.X1-maxX) > te.snapTolerance {
			// Rectangles not aligned horizontally
			// fmt.Printf("[DEBUG-TABLE]   Rect %d not aligned: X from %.2f to %.2f\n", i, rect.X0, rect.X1)
			return nil
		}
	}
	// fmt.Println("[DEBUG-TABLE]   All rectangles are aligned horizontally")
	
	// Sort rectangles by Y position (top to bottom in visual order)
	// In PDF coordinates, higher Y values are at the top of the page
	// So we sort in descending order of Y to get top-to-bottom visual order
	sort.Slice(rects, func(i, j int) bool {
		// Use the top edge (Y1) for sorting to ensure consistent ordering
		// Sort in descending order (higher Y first) for top-to-bottom visual order
		return rects[i].Y1 > rects[j].Y1
	})
	
	// Debug: show first few rectangles after sorting
	// for i := 0; i < 3 && i < len(rects); i++ {
	//	fmt.Printf("[DEBUG-TABLE]   Sorted rect %d: Y from %.2f to %.2f\n", i, rects[i].Y0, rects[i].Y1)
	// }
	
	// Find text columns within the rectangles
	columns := te.findTextColumns(objects.Chars, minX, maxX)
	// fmt.Printf("[DEBUG-TABLE]   Found %d column positions from text\n", len(columns))
	// for i, col := range columns {
	//	if i < 10 { // Show first 10 columns
	//		fmt.Printf("[DEBUG-TABLE]     Column %d at X=%.2f\n", i, col)
	//	}
	// }
	if len(columns) < 2 {
		// fmt.Printf("[DEBUG-TABLE]   Not enough columns (need at least 2)\n")
		return nil
	}
	
	// Extract text for each row
	rows := [][]string{}
	for _, rect := range rects {
		row := te.extractRowFromRectangle(rect, objects.Chars, columns)
		if len(row) > 0 {
			// if i < 3 {
			//	fmt.Printf("[DEBUG-TABLE]   Row %d (Y: %.2f-%.2f): %v\n", i, rect.Y0, rect.Y1, row)
			// }
			rows = append(rows, row)
		}
	}
	
	if len(rows) < te.minTableSize {
		return nil
	}
	
	// Remove empty columns
	rows = te.removeEmptyColumns(rows)
	
	return &Table{
		Rows: rows,
		BBox: BoundingBox{
			X0: minX,
			Y0: rects[0].Y0,
			X1: maxX,
			Y1: rects[len(rects)-1].Y1,
		},
	}
}

// findTextColumns finds column positions based on text alignment
func (te *tableExtractor) findTextColumns(chars []CharObject, minX, maxX float64) []float64 {
	// Group characters by X position
	xPositions := make(map[float64]int)
	
	for _, char := range chars {
		if char.X0 >= minX && char.X1 <= maxX {
			// Round to snap tolerance
			x := math.Round(char.X0/te.snapTolerance) * te.snapTolerance
			xPositions[x]++
		}
	}
	
	// Find positions that appear frequently
	columns := []float64{}
	minCount := 3 // At least 3 occurrences to be a column
	
	for x, count := range xPositions {
		if count >= minCount {
			columns = append(columns, x)
		}
	}
	
	sort.Float64s(columns)
	return columns
}

// extractRowFromRectangle extracts text for each column in a row rectangle
func (te *tableExtractor) extractRowFromRectangle(rect RectObject, chars []CharObject, columns []float64) []string {
	row := make([]string, len(columns))
	
	// Collect characters within this rectangle
	rowChars := []CharObject{}
	for _, char := range chars {
		// Check if character center is within rectangle bounds
		charCenterY := (char.Y0 + char.Y1) / 2
		if charCenterY >= rect.Y0 && charCenterY <= rect.Y1 &&
		   char.X0 >= rect.X0-te.snapTolerance && char.X1 <= rect.X1+te.snapTolerance {
			rowChars = append(rowChars, char)
		}
	}
	
	// Assign characters to columns
	for _, char := range rowChars {
		colIdx := te.findColumnIndex(char.X0, columns)
		if colIdx >= 0 && colIdx < len(row) {
			row[colIdx] += char.Text
		}
	}
	
	return row
}

// findColumnIndex finds which column a character belongs to
func (te *tableExtractor) findColumnIndex(x float64, columns []float64) int {
	for i, colX := range columns {
		if i == len(columns)-1 {
			// Last column - anything after this position
			if x >= colX-te.snapTolerance {
				return i
			}
		} else {
			// Check if x is between this column and the next
			nextColX := columns[i+1]
			if x >= colX-te.snapTolerance && x < nextColX-te.snapTolerance {
				return i
			}
		}
	}
	return -1
}

// removeEmptyColumns removes columns that are entirely empty
func (te *tableExtractor) removeEmptyColumns(rows [][]string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	
	// Find columns with any non-empty content
	numCols := len(rows[0])
	hasContent := make([]bool, numCols)
	
	for _, row := range rows {
		for colIdx, cell := range row {
			if colIdx < numCols && strings.TrimSpace(cell) != "" {
				hasContent[colIdx] = true
			}
		}
	}
	
	// Build new rows with only non-empty columns
	newRows := make([][]string, len(rows))
	for rowIdx, row := range rows {
		newRow := []string{}
		for colIdx, cell := range row {
			if colIdx < numCols && hasContent[colIdx] {
				newRow = append(newRow, cell)
			}
		}
		newRows[rowIdx] = newRow
	}
	
	return newRows
}

// createTableFromTextLines creates a table from aligned text lines
func (te *tableExtractor) createTableFromTextLines(lines []textLine, columns []float64) Table {
	rows := make([][]string, len(lines))
	
	// Calculate table bounding box
	var bbox BoundingBox
	if len(lines) > 0 {
		bbox = lines[0].BBox
		for _, line := range lines[1:] {
			bbox.X0 = min(bbox.X0, line.BBox.X0)
			bbox.Y0 = min(bbox.Y0, line.BBox.Y0)
			bbox.X1 = max(bbox.X1, line.BBox.X1)
			bbox.Y1 = max(bbox.Y1, line.BBox.Y1)
		}
	}
	
	// Extract text for each cell
	for i, line := range lines {
		rows[i] = make([]string, len(columns))
		
		// Assign characters to columns
		for _, char := range line.Chars {
			// Find the appropriate column
			colIdx := te.findColumnIndex(char.X0, columns)
			if colIdx >= 0 && colIdx < len(columns) {
				rows[i][colIdx] += char.Text
			}
		}
	}
	
	return Table{
		Rows: rows,
		BBox: bbox,
	}
}


// groupWordsIntoLines groups words into lines based on Y position
func (te *tableExtractor) groupWordsIntoLines(words []Word) []wordLine {
	if len(words) == 0 {
		return []wordLine{}
	}
	
	// Sort words by Y position
	sortedWords := make([]Word, len(words))
	copy(sortedWords, words)
	sort.Slice(sortedWords, func(i, j int) bool {
		return sortedWords[i].Y0 < sortedWords[j].Y0
	})
	
	lines := []wordLine{}
	currentLine := wordLine{
		Words: []Word{sortedWords[0]},
		Y:     sortedWords[0].Y0,
	}
	
	for i := 1; i < len(sortedWords); i++ {
		if math.Abs(sortedWords[i].Y0-currentLine.Y) < te.textTolerance {
			// Same line
			currentLine.Words = append(currentLine.Words, sortedWords[i])
		} else {
			// New line
			lines = append(lines, te.finalizeWordLine(currentLine))
			currentLine = wordLine{
				Words: []Word{sortedWords[i]},
				Y:     sortedWords[i].Y0,
			}
		}
	}
	
	if len(currentLine.Words) > 0 {
		lines = append(lines, te.finalizeWordLine(currentLine))
	}
	
	return lines
}

// finalizeWordLine calculates the bounding box for a word line
func (te *tableExtractor) finalizeWordLine(line wordLine) wordLine {
	// Sort words by X position
	sort.Slice(line.Words, func(i, j int) bool {
		return line.Words[i].X0 < line.Words[j].X0
	})
	
	// Calculate bounding box
	if len(line.Words) > 0 {
		minX := line.Words[0].X0
		maxX := line.Words[len(line.Words)-1].X1
		minY := line.Words[0].Y0
		maxY := line.Words[0].Y1
		
		for _, word := range line.Words {
			minY = min(minY, word.Y0)
			maxY = max(maxY, word.Y1)
		}
		
		line.BBox = BoundingBox{
			X0: minX,
			Y0: minY,
			X1: maxX,
			Y1: maxY,
		}
	}
	
	return line
}

// findAlignedColumnsFromWords finds vertically aligned columns from word positions
func (te *tableExtractor) findAlignedColumnsFromWords(lines []wordLine) []float64 {
	if len(lines) < 2 {
		return []float64{}
	}
	
	// Collect all unique X positions of word starts
	xPositions := make(map[float64]int)
	
	for _, line := range lines {
		for _, word := range line.Words {
			// Round to snap tolerance
			x := math.Round(word.X0/te.snapTolerance) * te.snapTolerance
			xPositions[x]++
		}
	}
	
	// Find positions that appear in multiple lines (at least 30% of lines)
	columns := []float64{}
	minCount := max(2.0, float64(len(lines)*3/10)) // At least 2 or 30% of lines
	
	for x, count := range xPositions {
		if float64(count) >= minCount {
			columns = append(columns, x)
		}
	}
	
	sort.Float64s(columns)
	return columns
}

// createTableFromWordLines creates a table from aligned word lines
func (te *tableExtractor) createTableFromWordLines(lines []wordLine, columns []float64) Table {
	rows := make([][]string, len(lines))
	
	// Calculate table bounding box
	var bbox BoundingBox
	if len(lines) > 0 {
		bbox = lines[0].BBox
		for _, line := range lines[1:] {
			bbox.X0 = min(bbox.X0, line.BBox.X0)
			bbox.Y0 = min(bbox.Y0, line.BBox.Y0)
			bbox.X1 = max(bbox.X1, line.BBox.X1)
			bbox.Y1 = max(bbox.Y1, line.BBox.Y1)
		}
	}
	
	// Extract text for each cell
	for i, line := range lines {
		rows[i] = make([]string, len(columns))
		
		// Assign words to columns based on their X position
		for _, word := range line.Words {
			// Find the appropriate column for this word
			colIdx := te.findWordColumn(word.X0, columns)
			if colIdx >= 0 && colIdx < len(columns) {
				// Add word text to the appropriate cell
				if rows[i][colIdx] != "" {
					rows[i][colIdx] += " "
				}
				rows[i][colIdx] += word.Text
			}
		}
	}
	
	return Table{
		Rows: rows,
		BBox: bbox,
	}
}

// findWordColumn finds which column a word belongs to
func (te *tableExtractor) findWordColumn(wordX float64, columns []float64) int {
	// Find the closest column that is less than or equal to wordX
	bestCol := -1
	minDist := math.MaxFloat64
	
	for i, colX := range columns {
		dist := math.Abs(wordX - colX)
		if dist < minDist && dist < te.snapTolerance*3 { // Within reasonable distance
			minDist = dist
			bestCol = i
		}
	}
	
	return bestCol
}