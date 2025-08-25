package pdf

import (
	"time"
)

// ObjectType represents the type of PDF object
type ObjectType string

const (
	ObjectTypeChar  ObjectType = "char"
	ObjectTypeLine  ObjectType = "line"
	ObjectTypeRect  ObjectType = "rect"
	ObjectTypeCurve ObjectType = "curve"
	ObjectTypeImage ObjectType = "image"
	ObjectTypeAnno  ObjectType = "annotation"
)

// BoundingBox represents a rectangular area with coordinates
type BoundingBox struct {
	X0 float64 // Left
	Y0 float64 // Top
	X1 float64 // Right
	Y1 float64 // Bottom
}

// Width returns the width of the bounding box
func (b BoundingBox) Width() float64 {
	return b.X1 - b.X0
}

// Height returns the height of the bounding box
func (b BoundingBox) Height() float64 {
	return b.Y1 - b.Y0
}

// Contains checks if a point is within the bounding box
func (b BoundingBox) Contains(x, y float64) bool {
	return x >= b.X0 && x <= b.X1 && y >= b.Y0 && y <= b.Y1
}

// Intersects checks if two bounding boxes intersect
func (b BoundingBox) Intersects(other BoundingBox) bool {
	return !(b.X1 < other.X0 || b.X0 > other.X1 || b.Y1 < other.Y0 || b.Y0 > other.Y1)
}

// Metadata represents PDF document metadata
type Metadata struct {
	Title        string
	Author       string
	Subject      string
	Keywords     string
	Creator      string
	Producer     string
	CreationDate time.Time
	ModDate      time.Time
	Trapped      string
}

// Objects represents a collection of PDF objects
type Objects struct {
	Chars  []CharObject
	Lines  []LineObject
	Rects  []RectObject
	Curves []CurveObject
	Images []ImageObject
	Annos  []AnnotationObject
}

// CharObject represents a character in the PDF
type CharObject struct {
	Text     string
	Font     string
	FontSize float64
	X0       float64
	Y0       float64
	X1       float64
	Y1       float64
	Width    float64
	Height   float64
	Color    Color
	Matrix   TransformMatrix
}

// GetType returns the object type
func (c CharObject) GetType() ObjectType {
	return ObjectTypeChar
}

// GetBBox returns the character's bounding box
func (c CharObject) GetBBox() BoundingBox {
	return BoundingBox{X0: c.X0, Y0: c.Y0, X1: c.X1, Y1: c.Y1}
}

// GetProperties returns character properties
func (c CharObject) GetProperties() map[string]interface{} {
	return map[string]interface{}{
		"text":      c.Text,
		"font":      c.Font,
		"font_size": c.FontSize,
		"color":     c.Color,
	}
}

// LineObject represents a line in the PDF
type LineObject struct {
	X0         float64
	Y0         float64
	X1         float64
	Y1         float64
	Width      float64
	StrokeColor Color
	NonStroking bool
}

// GetType returns the object type
func (l LineObject) GetType() ObjectType {
	return ObjectTypeLine
}

// GetBBox returns the line's bounding box
func (l LineObject) GetBBox() BoundingBox {
	return BoundingBox{
		X0: min(l.X0, l.X1),
		Y0: min(l.Y0, l.Y1),
		X1: max(l.X0, l.X1),
		Y1: max(l.Y0, l.Y1),
	}
}

// GetProperties returns line properties
func (l LineObject) GetProperties() map[string]interface{} {
	return map[string]interface{}{
		"width":        l.Width,
		"stroke_color": l.StrokeColor,
		"non_stroking": l.NonStroking,
	}
}

// RectObject represents a rectangle in the PDF
type RectObject struct {
	X0          float64
	Y0          float64
	X1          float64
	Y1          float64
	Width       float64
	StrokeColor Color
	FillColor   Color
	NonStroking bool
}

// GetType returns the object type
func (r RectObject) GetType() ObjectType {
	return ObjectTypeRect
}

// GetBBox returns the rectangle's bounding box
func (r RectObject) GetBBox() BoundingBox {
	return BoundingBox{X0: r.X0, Y0: r.Y0, X1: r.X1, Y1: r.Y1}
}

// GetProperties returns rectangle properties
func (r RectObject) GetProperties() map[string]interface{} {
	return map[string]interface{}{
		"width":        r.Width,
		"stroke_color": r.StrokeColor,
		"fill_color":   r.FillColor,
		"non_stroking": r.NonStroking,
	}
}

// CurveObject represents a curve in the PDF
type CurveObject struct {
	Points      []Point
	StrokeColor Color
	FillColor   Color
	Width       float64
}

// GetType returns the object type
func (c CurveObject) GetType() ObjectType {
	return ObjectTypeCurve
}

// GetBBox returns the curve's bounding box
func (c CurveObject) GetBBox() BoundingBox {
	if len(c.Points) == 0 {
		return BoundingBox{}
	}
	
	minX, minY := c.Points[0].X, c.Points[0].Y
	maxX, maxY := minX, minY
	
	for _, p := range c.Points[1:] {
		minX = min(minX, p.X)
		minY = min(minY, p.Y)
		maxX = max(maxX, p.X)
		maxY = max(maxY, p.Y)
	}
	
	return BoundingBox{X0: minX, Y0: minY, X1: maxX, Y1: maxY}
}

// GetProperties returns curve properties
func (c CurveObject) GetProperties() map[string]interface{} {
	return map[string]interface{}{
		"points":       c.Points,
		"stroke_color": c.StrokeColor,
		"fill_color":   c.FillColor,
		"width":        c.Width,
	}
}

// ImageObject represents an image in the PDF
type ImageObject struct {
	X0         float64
	Y0         float64
	X1         float64
	Y1         float64
	Width      int
	Height     int
	ColorSpace string
	BitsPerComponent int
}

// GetType returns the object type
func (i ImageObject) GetType() ObjectType {
	return ObjectTypeImage
}

// GetBBox returns the image's bounding box
func (i ImageObject) GetBBox() BoundingBox {
	return BoundingBox{X0: i.X0, Y0: i.Y0, X1: i.X1, Y1: i.Y1}
}

// GetProperties returns image properties
func (i ImageObject) GetProperties() map[string]interface{} {
	return map[string]interface{}{
		"width":              i.Width,
		"height":             i.Height,
		"color_space":        i.ColorSpace,
		"bits_per_component": i.BitsPerComponent,
	}
}

// AnnotationObject represents an annotation in the PDF
type AnnotationObject struct {
	Type     string
	X0       float64
	Y0       float64
	X1       float64
	Y1       float64
	Contents string
	URL      string
}

// GetType returns the object type
func (a AnnotationObject) GetType() ObjectType {
	return ObjectTypeAnno
}

// GetBBox returns the annotation's bounding box
func (a AnnotationObject) GetBBox() BoundingBox {
	return BoundingBox{X0: a.X0, Y0: a.Y0, X1: a.X1, Y1: a.Y1}
}

// GetProperties returns annotation properties
func (a AnnotationObject) GetProperties() map[string]interface{} {
	return map[string]interface{}{
		"type":     a.Type,
		"contents": a.Contents,
		"url":      a.URL,
	}
}

// Color represents an RGB color
type Color struct {
	R, G, B uint8
	A       uint8 // Alpha channel
}

// Point represents a 2D point
type Point struct {
	X, Y float64
}

// TransformMatrix represents a 2D transformation matrix
type TransformMatrix struct {
	A, B, C, D, E, F float64
}

// Table represents an extracted table
type Table struct {
	Rows [][]string
	BBox BoundingBox
}

// TextExtractionOption is a function that modifies text extraction behavior
type TextExtractionOption func(*textExtractionConfig)

type textExtractionConfig struct {
	Layout      bool
	XTolerance  float64
	YTolerance  float64
	UnicodeNorm string
}

// WithLayout enables layout-aware text extraction
func WithLayout(enabled bool) TextExtractionOption {
	return func(c *textExtractionConfig) {
		c.Layout = enabled
	}
}

// WithXTolerance sets the horizontal tolerance for text grouping
func WithXTolerance(tolerance float64) TextExtractionOption {
	return func(c *textExtractionConfig) {
		c.XTolerance = tolerance
	}
}

// WithYTolerance sets the vertical tolerance for text grouping
func WithYTolerance(tolerance float64) TextExtractionOption {
	return func(c *textExtractionConfig) {
		c.YTolerance = tolerance
	}
}

// TableExtractionOption is a function that modifies table extraction behavior
type TableExtractionOption func(*tableExtractionConfig)

type tableExtractionConfig struct {
	VerticalStrategy   string
	HorizontalStrategy string
	MinTableSize       int
	TextTolerance      float64
}

// ImageOption is a function that modifies image rendering behavior
type ImageOption func(*imageConfig)

type imageConfig struct {
	Resolution int
	Format     string
}

// Helper functions
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