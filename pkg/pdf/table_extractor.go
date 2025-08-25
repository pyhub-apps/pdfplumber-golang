package pdf

import (
	"math"
	"sort"
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
	
	// Try line-based table extraction first
	if te.verticalStrategy == "lines" || te.horizontalStrategy == "lines" {
		lineTables := te.extractLineBasedTables(objects)
		tables = append(tables, lineTables...)
	}
	
	// If no tables found with lines, try text-based detection
	if len(tables) == 0 && (te.verticalStrategy == "text" || te.horizontalStrategy == "text") {
		textTables := te.extractTextBasedTables(objects)
		tables = append(tables, textTables...)
	}
	
	return tables
}

// extractLineBasedTables extracts tables using lines and rectangles
func (te *tableExtractor) extractLineBasedTables(objects Objects) []Table {
	tables := []Table{}
	
	// Collect all horizontal and vertical lines
	hLines, vLines := te.collectTableLines(objects)
	
	// Also consider rectangles as potential table cells
	for _, rect := range objects.Rects {
		// Add rectangle edges as lines
		hLines = append(hLines, 
			LineObject{X0: rect.X0, Y0: rect.Y0, X1: rect.X1, Y1: rect.Y0, Width: rect.Width},
			LineObject{X0: rect.X0, Y0: rect.Y1, X1: rect.X1, Y1: rect.Y1, Width: rect.Width},
		)
		vLines = append(vLines,
			LineObject{X0: rect.X0, Y0: rect.Y0, X1: rect.X0, Y1: rect.Y1, Width: rect.Width},
			LineObject{X0: rect.X1, Y0: rect.Y0, X1: rect.X1, Y1: rect.Y1, Width: rect.Width},
		)
	}
	
	// Find table regions (intersecting horizontal and vertical lines)
	tableRegions := te.findTableRegions(hLines, vLines)
	
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
			hLines = append(hLines, line)
		} else if math.Abs(line.X1-line.X0) < te.snapTolerance {
			// Vertical line
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
	
	// For each group combination, check if it forms a table
	for _, hGroup := range hGroups {
		for _, vGroup := range vGroups {
			if len(hGroup) >= 2 && len(vGroup) >= 2 {
				region := te.createTableRegion(hGroup, vGroup)
				if region != nil {
					regions = append(regions, *region)
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
		
		if math.Abs(pos-prevPos) > 50 { // Gap threshold
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
	
	if len(hPositions) < 2 || len(vPositions) < 2 {
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
			pos = math.Round(line.Y0/te.snapTolerance) * te.snapTolerance
		} else {
			pos = math.Round(line.X0/te.snapTolerance) * te.snapTolerance
		}
		posMap[pos] = true
		
		// Also add the end position
		if horizontal {
			pos = math.Round(line.Y1/te.snapTolerance) * te.snapTolerance
		} else {
			pos = math.Round(line.X1/te.snapTolerance) * te.snapTolerance
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
	
	if len(objects.Chars) == 0 {
		return tables
	}
	
	// Group characters into lines
	lines := te.groupCharsIntoLines(objects.Chars)
	
	// Find aligned columns
	columns := te.findAlignedColumns(lines)
	
	// If we have consistent columns, create a table
	if len(columns) > 1 {
		table := te.createTableFromTextLines(lines, columns)
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

// findColumnIndex finds which column a character belongs to
func (te *tableExtractor) findColumnIndex(x float64, columns []float64) int {
	for i := 0; i < len(columns)-1; i++ {
		if x >= columns[i] && x < columns[i+1] {
			return i
		}
	}
	
	// Check last column
	if len(columns) > 0 && x >= columns[len(columns)-1] {
		return len(columns) - 1
	}
	
	return -1
}