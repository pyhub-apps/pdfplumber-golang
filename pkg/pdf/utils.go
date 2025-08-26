package pdf

import (
	"math"
	"sort"
)

// Tolerance for floating point comparisons
const FloatTolerance = 0.1

// DeduplicateLines removes duplicate lines based on coordinates
func DeduplicateLines(lines []LineObject) []LineObject {
	if len(lines) == 0 {
		return lines
	}

	// Sort lines for consistent ordering
	sort.Slice(lines, func(i, j int) bool {
		if math.Abs(lines[i].Y0-lines[j].Y0) > FloatTolerance {
			return lines[i].Y0 < lines[j].Y0
		}
		if math.Abs(lines[i].X0-lines[j].X0) > FloatTolerance {
			return lines[i].X0 < lines[j].X0
		}
		if math.Abs(lines[i].Y1-lines[j].Y1) > FloatTolerance {
			return lines[i].Y1 < lines[j].Y1
		}
		return lines[i].X1 < lines[j].X1
	})

	// Remove duplicates
	result := []LineObject{lines[0]}
	for i := 1; i < len(lines); i++ {
		last := result[len(result)-1]
		curr := lines[i]
		
		// Check if lines are essentially the same
		if !linesEqual(last, curr) {
			result = append(result, curr)
		}
	}

	return result
}

// linesEqual checks if two lines are essentially the same
func linesEqual(a, b LineObject) bool {
	// Check both directions (lines might be reversed)
	sameDirection := math.Abs(a.X0-b.X0) < FloatTolerance &&
		math.Abs(a.Y0-b.Y0) < FloatTolerance &&
		math.Abs(a.X1-b.X1) < FloatTolerance &&
		math.Abs(a.Y1-b.Y1) < FloatTolerance

	reversedDirection := math.Abs(a.X0-b.X1) < FloatTolerance &&
		math.Abs(a.Y0-b.Y1) < FloatTolerance &&
		math.Abs(a.X1-b.X0) < FloatTolerance &&
		math.Abs(a.Y1-b.Y0) < FloatTolerance

	return sameDirection || reversedDirection
}

// FilterPageBorderLines removes lines that are at page borders
func FilterPageBorderLines(lines []LineObject, pageWidth, pageHeight float64) []LineObject {
	result := []LineObject{}
	
	for _, line := range lines {
		// Check if line is at page edge (with small tolerance)
		atLeftEdge := math.Abs(line.X0) < 1 && math.Abs(line.X1) < 1
		atRightEdge := math.Abs(line.X0-pageWidth) < 1 && math.Abs(line.X1-pageWidth) < 1
		atTopEdge := math.Abs(line.Y0-pageHeight) < 1 && math.Abs(line.Y1-pageHeight) < 1
		atBottomEdge := math.Abs(line.Y0) < 1 && math.Abs(line.Y1) < 1
		
		// Keep line if it's not at any edge
		if !atLeftEdge && !atRightEdge && !atTopEdge && !atBottomEdge {
			result = append(result, line)
		}
	}
	
	return result
}

// FilterTableLines extracts lines that are likely part of tables
func FilterTableLines(lines []LineObject) []LineObject {
	result := []LineObject{}
	
	for _, line := range lines {
		// Table lines are usually:
		// - Horizontal or vertical (not diagonal)
		// - Within reasonable page margins
		// - Have consistent width/style
		
		isHorizontal := math.Abs(line.Y0-line.Y1) < FloatTolerance
		isVertical := math.Abs(line.X0-line.X1) < FloatTolerance
		
		// Check if within reasonable margins (not too close to edges)
		inMargins := line.X0 > 20 && line.X1 > 20 && 
		            line.X0 < 575 && line.X1 < 575 &&
		            line.Y0 > 20 && line.Y1 > 20
		
		if (isHorizontal || isVertical) && inMargins {
			result = append(result, line)
		}
	}
	
	return result
}

// ConsolidateTableLines merges overlapping or nearly overlapping lines
func ConsolidateTableLines(lines []LineObject) []LineObject {
	if len(lines) == 0 {
		return lines
	}
	
	// Separate horizontal and vertical lines
	var horizontal, vertical []LineObject
	
	for _, line := range lines {
		if math.Abs(line.Y0-line.Y1) < FloatTolerance {
			// Horizontal line
			horizontal = append(horizontal, line)
		} else if math.Abs(line.X0-line.X1) < FloatTolerance {
			// Vertical line
			vertical = append(vertical, line)
		}
	}
	
	// Consolidate horizontal lines
	horizontal = consolidateHorizontalLines(horizontal)
	
	// Consolidate vertical lines
	vertical = consolidateVerticalLines(vertical)
	
	// Combine results
	result := append(horizontal, vertical...)
	return result
}

func consolidateHorizontalLines(lines []LineObject) []LineObject {
	if len(lines) == 0 {
		return lines
	}
	
	// Sort by Y position, then X
	sort.Slice(lines, func(i, j int) bool {
		if math.Abs(lines[i].Y0-lines[j].Y0) > FloatTolerance {
			return lines[i].Y0 < lines[j].Y0
		}
		return lines[i].X0 < lines[j].X0
	})
	
	result := []LineObject{}
	current := lines[0]
	
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		
		// Check if this line is on the same Y level and overlaps or touches
		if math.Abs(line.Y0-current.Y0) < FloatTolerance &&
		   math.Abs(line.Y1-current.Y1) < FloatTolerance {
			// Check if lines overlap or are very close
			if line.X0 <= current.X1+1 && line.X1 >= current.X0-1 {
				// Merge lines
				current.X0 = math.Min(current.X0, line.X0)
				current.X1 = math.Max(current.X1, line.X1)
				// Keep the thicker line width
				if line.Width > current.Width {
					current.Width = line.Width
				}
				continue
			}
		}
		
		// Different line, save current and start new
		result = append(result, current)
		current = line
	}
	
	// Add the last line
	result = append(result, current)
	
	return result
}

func consolidateVerticalLines(lines []LineObject) []LineObject {
	if len(lines) == 0 {
		return lines
	}
	
	// Sort by X position, then Y
	sort.Slice(lines, func(i, j int) bool {
		if math.Abs(lines[i].X0-lines[j].X0) > FloatTolerance {
			return lines[i].X0 < lines[j].X0
		}
		return lines[i].Y0 < lines[j].Y0
	})
	
	result := []LineObject{}
	current := lines[0]
	
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		
		// Check if this line is on the same X level and overlaps or touches
		if math.Abs(line.X0-current.X0) < FloatTolerance &&
		   math.Abs(line.X1-current.X1) < FloatTolerance {
			// Check if lines overlap or are very close
			if line.Y0 <= current.Y1+1 && line.Y1 >= current.Y0-1 {
				// Merge lines
				current.Y0 = math.Min(current.Y0, line.Y0)
				current.Y1 = math.Max(current.Y1, line.Y1)
				// Keep the thicker line width
				if line.Width > current.Width {
					current.Width = line.Width
				}
				continue
			}
		}
		
		// Different line, save current and start new
		result = append(result, current)
		current = line
	}
	
	// Add the last line
	result = append(result, current)
	
	return result
}

// DeduplicateRectangles removes duplicate rectangles
func DeduplicateRectangles(rects []RectObject) []RectObject {
	if len(rects) == 0 {
		return rects
	}

	// Sort rectangles for consistent ordering
	sort.Slice(rects, func(i, j int) bool {
		if math.Abs(rects[i].Y0-rects[j].Y0) > FloatTolerance {
			return rects[i].Y0 < rects[j].Y0
		}
		return rects[i].X0 < rects[j].X0
	})

	// Remove duplicates
	result := []RectObject{rects[0]}
	for i := 1; i < len(rects); i++ {
		last := result[len(result)-1]
		curr := rects[i]
		
		// Check if rectangles are essentially the same
		if !rectsEqual(last, curr) {
			result = append(result, curr)
		}
	}

	return result
}

// rectsEqual checks if two rectangles are essentially the same
func rectsEqual(a, b RectObject) bool {
	return math.Abs(a.X0-b.X0) < FloatTolerance &&
		math.Abs(a.Y0-b.Y0) < FloatTolerance &&
		math.Abs(a.X1-b.X1) < FloatTolerance &&
		math.Abs(a.Y1-b.Y1) < FloatTolerance
}