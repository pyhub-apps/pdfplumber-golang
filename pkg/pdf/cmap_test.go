package pdf

import (
	"testing"
)

func TestNewToUnicodeCMap(t *testing.T) {
	cmap := NewToUnicodeCMap()
	if cmap == nil {
		t.Fatal("NewToUnicodeCMap() returned nil")
	}
	if cmap.cidToUnicode == nil {
		t.Error("cidToUnicode map not initialized")
	}
	if cmap.ranges == nil {
		t.Error("ranges slice not initialized")
	}
}

func TestParseBeginBFChar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[uint16]string
	}{
		{
			name: "Single mapping",
			input: `
				beginbfchar
				<0001> <0041>
				endbfchar
			`,
			expected: map[uint16]string{
				0x0001: "A", // 0x41 is 'A'
			},
		},
		{
			name: "Multiple mappings",
			input: `
				beginbfchar
				<0001> <0041>
				<0002> <0042>
				<0003> <0043>
				endbfchar
			`,
			expected: map[uint16]string{
				0x0001: "A",
				0x0002: "B",
				0x0003: "C",
			},
		},
		{
			name: "Korean characters",
			input: `
				beginbfchar
				<0001> <AC00>
				<0002> <AC01>
				endbfchar
			`,
			expected: map[uint16]string{
				0x0001: "가", // U+AC00
				0x0002: "각", // U+AC01
			},
		},
		{
			name: "UTF-16 encoded strings",
			input: `
				beginbfchar
				<0001> <FEFF0041>
				<0002> <FEFF0042>
				endbfchar
			`,
			expected: map[uint16]string{
				0x0001: "A",
				0x0002: "B",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmap := NewToUnicodeCMap()
			err := cmap.Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			for cid, expectedUnicode := range tt.expected {
				unicode, ok := cmap.MapCIDToUnicode(cid)
				if !ok {
					t.Errorf("CID %04X not found in mapping", cid)
					continue
				}
				if unicode != expectedUnicode {
					t.Errorf("CID %04X: expected %q, got %q", cid, expectedUnicode, unicode)
				}
			}
		})
	}
}

func TestParseBeginBFRange(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		testCIDs map[uint16]string
	}{
		{
			name: "Contiguous range",
			input: `
				beginbfrange
				<0001> <0005> <0041>
				endbfrange
			`,
			testCIDs: map[uint16]string{
				0x0001: "A",
				0x0002: "B",
				0x0003: "C",
				0x0004: "D",
				0x0005: "E",
			},
		},
		{
			name: "Array mapping",
			input: `
				beginbfrange
				<0001> <0003> [<0041> <0043> <0045>]
				endbfrange
			`,
			testCIDs: map[uint16]string{
				0x0001: "A",
				0x0002: "C",
				0x0003: "E",
			},
		},
		{
			name: "Multiple ranges",
			input: `
				beginbfrange
				<0001> <0003> <0041>
				<0010> <0012> <0061>
				endbfrange
			`,
			testCIDs: map[uint16]string{
				0x0001: "A",
				0x0002: "B",
				0x0003: "C",
				0x0010: "a",
				0x0011: "b",
				0x0012: "c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmap := NewToUnicodeCMap()
			err := cmap.Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			for cid, expectedUnicode := range tt.testCIDs {
				unicode, ok := cmap.MapCIDToUnicode(cid)
				if !ok {
					t.Errorf("CID %04X not found in mapping", cid)
					continue
				}
				if unicode != expectedUnicode {
					t.Errorf("CID %04X: expected %q, got %q", cid, expectedUnicode, unicode)
				}
			}
		})
	}
}

func TestDecode(t *testing.T) {
	// Create a CMap with some mappings
	cmapData := `
		beginbfchar
		<0048> <0048>
		<0065> <0065>
		<006C> <006C>
		<006F> <006F>
		endbfchar
		beginbfrange
		<0020> <007E> <0020>
		endbfrange
	`
	
	cmap := NewToUnicodeCMap()
	if err := cmap.Parse([]byte(cmapData)); err != nil {
		t.Fatalf("Failed to parse CMap: %v", err)
	}

	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "ASCII text",
			input:    []byte{0x00, 0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F},
			expected: "Hello",
		},
		{
			name:     "Single byte fallback",
			input:    []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F},
			expected: "Hello",
		},
		{
			name:     "Mixed mapped and unmapped",
			input:    []byte{0x00, 0x48, 0xFF, 0xFF, 0x00, 0x65},
			expected: "H\xff\xffe", // Unmapped bytes preserved
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmap.Decode(tt.input)
			if result != tt.expected {
				t.Errorf("Decode() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDecodeHexString(t *testing.T) {
	// Create a CMap with some mappings
	cmapData := `
		beginbfchar
		<0048> <0048>
		<0065> <0065>
		<006C> <006C>
		<006F> <006F>
		endbfchar
	`
	
	cmap := NewToUnicodeCMap()
	if err := cmap.Parse([]byte(cmapData)); err != nil {
		t.Fatalf("Failed to parse CMap: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Hex string without brackets",
			input:    "004800650065006C006C006F",
			expected: "Heeello", // 00 48, 00 65, 00 65, 00 6C, 00 6C, 00 6F
		},
		{
			name:     "Hex string with brackets",
			input:    "<0048006500650065006C006C006F>",
			expected: "Heeeello",
		},
		{
			name:     "Invalid hex",
			input:    "GGGG",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmap.DecodeHexString(tt.input)
			if result != tt.expected {
				t.Errorf("DecodeHexString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetMappingCount(t *testing.T) {
	cmap := NewToUnicodeCMap()
	
	// Initially empty
	if count := cmap.GetMappingCount(); count != 0 {
		t.Errorf("Empty CMap has %d mappings, expected 0", count)
	}
	
	// Add some mappings
	cmapData := `
		beginbfchar
		<0001> <0041>
		<0002> <0042>
		<0003> <0043>
		endbfchar
		beginbfrange
		<0010> <0015> <0061>
		endbfrange
	`
	
	if err := cmap.Parse([]byte(cmapData)); err != nil {
		t.Fatalf("Failed to parse CMap: %v", err)
	}
	
	// 3 direct mappings + 6 range mappings (0010-0015 inclusive)
	expectedCount := 3 + 6
	if count := cmap.GetMappingCount(); count != expectedCount {
		t.Errorf("CMap has %d mappings, expected %d", count, expectedCount)
	}
}

func TestComplexRealWorldCMap(t *testing.T) {
	// Test with a more complex real-world-like CMap
	cmapData := `
		/CIDInit /ProcSet findresource begin
		12 dict begin
		begincmap
		/CIDSystemInfo
		<< /Registry (Adobe)
		/Ordering (UCS)
		/Supplement 0
		>> def
		/CMapName /Adobe-Identity-UCS def
		/CMapType 2 def
		1 begincodespacerange
		<0000> <FFFF>
		endcodespacerange
		3 beginbfchar
		<0003> <0020>
		<0048> <AC00>
		<0049> <AC01>
		endbfchar
		2 beginbfrange
		<004A> <004C> <AC02>
		<0050> <0052> [<AC10> <AC11> <AC12>]
		endbfrange
		endcmap
		CMapName currentdict /CMap defineresource pop
		end
		end
	`
	
	cmap := NewToUnicodeCMap()
	if err := cmap.Parse([]byte(cmapData)); err != nil {
		t.Fatalf("Failed to parse complex CMap: %v", err)
	}
	
	// Test specific mappings
	tests := map[uint16]string{
		0x0003: " ",  // Space
		0x0048: "가", // Korean character
		0x0049: "각",
		0x004A: "간",
		0x004B: "갇",
		0x004C: "갈",
		0x0050: "감",
		0x0051: "갑",
		0x0052: "값",
	}
	
	for cid, expected := range tests {
		unicode, ok := cmap.MapCIDToUnicode(cid)
		if !ok {
			t.Errorf("CID %04X not found", cid)
			continue
		}
		if unicode != expected {
			t.Errorf("CID %04X: expected %q, got %q", cid, expected, unicode)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	cmapData := []byte(`
		beginbfchar
		<0001> <0041>
		<0002> <0042>
		<0003> <0043>
		endbfchar
		beginbfrange
		<0010> <00FF> <0061>
		endbfrange
	`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmap := NewToUnicodeCMap()
		_ = cmap.Parse(cmapData)
	}
}

func BenchmarkMapCIDToUnicode(b *testing.B) {
	cmap := NewToUnicodeCMap()
	_ = cmap.Parse([]byte(`
		beginbfchar
		<0001> <0041>
		endbfchar
		beginbfrange
		<0010> <00FF> <0061>
		endbfrange
	`))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test both direct and range mappings
		_, _ = cmap.MapCIDToUnicode(0x0001)
		_, _ = cmap.MapCIDToUnicode(0x0050)
	}
}

func BenchmarkDecode(b *testing.B) {
	cmap := NewToUnicodeCMap()
	_ = cmap.Parse([]byte(`
		beginbfrange
		<0020> <007E> <0020>
		endbfrange
	`))
	
	data := []byte{0x00, 0x48, 0x00, 0x65, 0x00, 0x6C, 0x00, 0x6C, 0x00, 0x6F}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cmap.Decode(data)
	}
}