package content

// GraphicsState represents the PDF graphics state
type GraphicsState struct {
	CTM         Matrix  // Current Transformation Matrix
	TextMatrix  Matrix  // Text matrix
	TextLineMatrix Matrix // Text line matrix
	CharSpace   float64 // Character spacing
	WordSpace   float64 // Word spacing
	HScale      float64 // Horizontal scaling
	Leading     float64 // Text leading
	FontName    string  // Current font name
	FontSize    float64 // Current font size
	TextRise    float64 // Text rise
	RenderMode  int     // Text rendering mode
	
	// Graphics state
	LineWidth   float64
	LineCap     int
	LineJoin    int
	MiterLimit  float64
	DashPattern []float64
	DashPhase   float64
	
	// Color state
	StrokeColor []float64
	FillColor   []float64
	ColorSpace  string
	
	// Path state
	CurrentPath []PathElement
	CurrentPoint Point
}

// NewGraphicsState creates a new graphics state with defaults
func NewGraphicsState() *GraphicsState {
	return &GraphicsState{
		CTM:           IdentityMatrix(),
		TextMatrix:    IdentityMatrix(),
		TextLineMatrix: IdentityMatrix(),
		CharSpace:     0,
		WordSpace:     0,
		HScale:        100,
		Leading:       0,
		FontSize:      0,
		TextRise:      0,
		RenderMode:    0,
		LineWidth:     1,
		LineCap:       0,
		LineJoin:      0,
		MiterLimit:    10,
		StrokeColor:   []float64{0},     // Black
		FillColor:     []float64{0},     // Black
		ColorSpace:    "DeviceGray",
		CurrentPath:   []PathElement{},
	}
}

// Clone creates a copy of the graphics state
func (gs *GraphicsState) Clone() *GraphicsState {
	newState := *gs
	
	// Deep copy slices
	if gs.DashPattern != nil {
		newState.DashPattern = make([]float64, len(gs.DashPattern))
		copy(newState.DashPattern, gs.DashPattern)
	}
	
	if gs.StrokeColor != nil {
		newState.StrokeColor = make([]float64, len(gs.StrokeColor))
		copy(newState.StrokeColor, gs.StrokeColor)
	}
	
	if gs.FillColor != nil {
		newState.FillColor = make([]float64, len(gs.FillColor))
		copy(newState.FillColor, gs.FillColor)
	}
	
	if gs.CurrentPath != nil {
		newState.CurrentPath = make([]PathElement, len(gs.CurrentPath))
		copy(newState.CurrentPath, gs.CurrentPath)
	}
	
	return &newState
}

// Matrix represents a 2D transformation matrix
type Matrix struct {
	A, B, C, D, E, F float64
}

// IdentityMatrix returns an identity matrix
func IdentityMatrix() Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0}
}

// Multiply multiplies two matrices
func (m Matrix) Multiply(other Matrix) Matrix {
	return Matrix{
		A: m.A*other.A + m.B*other.C,
		B: m.A*other.B + m.B*other.D,
		C: m.C*other.A + m.D*other.C,
		D: m.C*other.B + m.D*other.D,
		E: m.E*other.A + m.F*other.C + other.E,
		F: m.E*other.B + m.F*other.D + other.F,
	}
}

// Transform applies the matrix transformation to a point
func (m Matrix) Transform(x, y float64) (float64, float64) {
	newX := m.A*x + m.C*y + m.E
	newY := m.B*x + m.D*y + m.F
	return newX, newY
}

// Scale creates a scaling matrix
func Scale(sx, sy float64) Matrix {
	return Matrix{A: sx, B: 0, C: 0, D: sy, E: 0, F: 0}
}

// Translate creates a translation matrix
func Translate(tx, ty float64) Matrix {
	return Matrix{A: 1, B: 0, C: 0, D: 1, E: tx, F: ty}
}

// Point represents a 2D point
type Point struct {
	X, Y float64
}

// PathElement represents an element in a path
type PathElement struct {
	Type   string  // "move", "line", "curve", "close"
	Points []Point
}

// StateStack manages graphics state stack for save/restore operations
type StateStack struct {
	states []*GraphicsState
}

// NewStateStack creates a new state stack
func NewStateStack() *StateStack {
	return &StateStack{
		states: []*GraphicsState{NewGraphicsState()},
	}
}

// Current returns the current graphics state
func (s *StateStack) Current() *GraphicsState {
	if len(s.states) == 0 {
		return NewGraphicsState()
	}
	return s.states[len(s.states)-1]
}

// Save saves the current graphics state
func (s *StateStack) Save() {
	current := s.Current()
	s.states = append(s.states, current.Clone())
}

// Restore restores the previous graphics state
func (s *StateStack) Restore() {
	if len(s.states) > 1 {
		s.states = s.states[:len(s.states)-1]
	}
}