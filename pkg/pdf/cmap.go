package pdf

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// ToUnicodeCMap represents a PDF ToUnicode CMap that maps CIDs to Unicode values
type ToUnicodeCMap struct {
	// Direct character mappings (from beginbfchar sections)
	cidToUnicode map[uint16]string
	
	// Range mappings (from beginbfrange sections)
	ranges []cmapRange
	
	// Raw CMap data for debugging
	rawData []byte
}

// cmapRange represents a contiguous range mapping from beginbfrange
type cmapRange struct {
	startCID     uint16
	endCID       uint16
	startUnicode uint16
	unicodeArray []string // For non-contiguous mappings
}

// NewToUnicodeCMap creates a new ToUnicode CMap parser
func NewToUnicodeCMap() *ToUnicodeCMap {
	return &ToUnicodeCMap{
		cidToUnicode: make(map[uint16]string),
		ranges:       []cmapRange{},
	}
}

// Parse parses a ToUnicode CMap stream
func (cmap *ToUnicodeCMap) Parse(data []byte) error {
	cmap.rawData = data
	
	// Convert to string for easier processing
	content := string(data)
	
	// Parse beginbfchar sections
	if err := cmap.parseBeginBFChar(content); err != nil {
		return fmt.Errorf("failed to parse beginbfchar: %w", err)
	}
	
	// Parse beginbfrange sections
	if err := cmap.parseBeginBFRange(content); err != nil {
		return fmt.Errorf("failed to parse beginbfrange: %w", err)
	}
	
	return nil
}

// parseBeginBFChar parses beginbfchar...endbfchar sections
func (cmap *ToUnicodeCMap) parseBeginBFChar(content string) error {
	// Regular expression to find beginbfchar sections
	// Format: N beginbfchar
	//         <src> <dst>
	//         ...
	//         endbfchar
	re := regexp.MustCompile(`(\d+)\s+beginbfchar\s*((?:<[0-9A-Fa-f]+>\s*<[0-9A-Fa-f]+>\s*)+)endbfchar`)
	
	matches := re.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		
		// Parse each mapping
		mappingStr := match[2]
		mappingRe := regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>`)
		mappings := mappingRe.FindAllStringSubmatch(mappingStr, -1)
		
		for _, mapping := range mappings {
			if len(mapping) < 3 {
				continue
			}
			
			// Parse source CID
			srcBytes, err := hex.DecodeString(mapping[1])
			if err != nil {
				continue
			}
			
			// Convert to CID (assuming 2-byte CIDs for now)
			var srcCID uint16
			if len(srcBytes) == 1 {
				srcCID = uint16(srcBytes[0])
			} else if len(srcBytes) >= 2 {
				srcCID = uint16(srcBytes[0])<<8 | uint16(srcBytes[1])
			} else {
				continue
			}
			
			// Parse destination Unicode
			dstBytes, err := hex.DecodeString(mapping[2])
			if err != nil {
				continue
			}
			
			// Convert to Unicode string
			unicodeStr := cmap.bytesToUnicode(dstBytes)
			
			// Store mapping
			cmap.cidToUnicode[srcCID] = unicodeStr
		}
	}
	
	return nil
}

// parseBeginBFRange parses beginbfrange...endbfrange sections
func (cmap *ToUnicodeCMap) parseBeginBFRange(content string) error {
	// Regular expression to find beginbfrange sections
	// Format: N beginbfrange
	//         <srcStart> <srcEnd> <dst>
	//         ...
	//         endbfrange
	re := regexp.MustCompile(`(\d+)\s+beginbfrange\s*((?:<[0-9A-Fa-f]+>\s*<[0-9A-Fa-f]+>\s*(?:<[0-9A-Fa-f]+>|\[[^\]]+\])\s*)+)endbfrange`)
	
	matches := re.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		
		// Parse each range
		rangeStr := match[2]
		// Handle both single Unicode values and arrays
		rangeRe := regexp.MustCompile(`<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>\s*(<[0-9A-Fa-f]+>|\[([^\]]+)\])`)
		ranges := rangeRe.FindAllStringSubmatch(rangeStr, -1)
		
		for _, r := range ranges {
			if len(r) < 4 {
				continue
			}
			
			// Parse start CID
			startBytes, err := hex.DecodeString(r[1])
			if err != nil {
				continue
			}
			var startCID uint16
			if len(startBytes) == 1 {
				startCID = uint16(startBytes[0])
			} else if len(startBytes) >= 2 {
				startCID = uint16(startBytes[0])<<8 | uint16(startBytes[1])
			}
			
			// Parse end CID
			endBytes, err := hex.DecodeString(r[2])
			if err != nil {
				continue
			}
			var endCID uint16
			if len(endBytes) == 1 {
				endCID = uint16(endBytes[0])
			} else if len(endBytes) >= 2 {
				endCID = uint16(endBytes[0])<<8 | uint16(endBytes[1])
			}
			
			// Parse destination
			if strings.HasPrefix(r[3], "<") {
				// Single starting Unicode value (contiguous range)
				hexStr := strings.Trim(r[3], "<>")
				dstBytes, err := hex.DecodeString(hexStr)
				if err != nil {
					continue
				}
				
				var startUnicode uint16
				if len(dstBytes) == 1 {
					startUnicode = uint16(dstBytes[0])
				} else if len(dstBytes) >= 2 {
					startUnicode = uint16(dstBytes[0])<<8 | uint16(dstBytes[1])
				}
				
				cmap.ranges = append(cmap.ranges, cmapRange{
					startCID:     startCID,
					endCID:       endCID,
					startUnicode: startUnicode,
				})
			} else if strings.HasPrefix(r[3], "[") {
				// Array of Unicode values
				// TODO: Handle array format
				// For now, skip array format
			}
		}
	}
	
	return nil
}

// bytesToUnicode converts bytes to Unicode string
func (cmap *ToUnicodeCMap) bytesToUnicode(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	
	// Handle different byte lengths
	if len(data) == 1 {
		// Single byte - direct ASCII
		return string(rune(data[0]))
	} else if len(data) == 2 {
		// Two bytes - UTF-16BE
		codePoint := uint16(data[0])<<8 | uint16(data[1])
		return string(rune(codePoint))
	} else if len(data) == 4 {
		// Four bytes - UTF-16BE surrogate pair or direct UTF-32
		// Try UTF-16 surrogate pair first
		high := uint16(data[0])<<8 | uint16(data[1])
		low := uint16(data[2])<<8 | uint16(data[3])
		
		if high >= 0xD800 && high <= 0xDBFF && low >= 0xDC00 && low <= 0xDFFF {
			// Valid surrogate pair
			codePoint := 0x10000 + ((uint32(high)&0x3FF)<<10) + (uint32(low) & 0x3FF)
			return string(rune(codePoint))
		} else {
			// Treat as UTF-32BE
			codePoint := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
			return string(rune(codePoint))
		}
	}
	
	// Fallback: interpret as UTF-8
	return string(data)
}

// MapCIDToUnicode maps a CID to its Unicode string
func (cmap *ToUnicodeCMap) MapCIDToUnicode(cid uint16) (string, bool) {
	// First check direct mappings
	if unicode, ok := cmap.cidToUnicode[cid]; ok {
		return unicode, true
	}
	
	// Then check ranges
	for _, r := range cmap.ranges {
		if cid >= r.startCID && cid <= r.endCID {
			if len(r.unicodeArray) > 0 {
				// Array mapping
				index := int(cid - r.startCID)
				if index < len(r.unicodeArray) {
					return r.unicodeArray[index], true
				}
			} else {
				// Contiguous range mapping
				offset := cid - r.startCID
				unicodePoint := r.startUnicode + offset
				return string(rune(unicodePoint)), true
			}
		}
	}
	
	return "", false
}

// DecodeHexString decodes a hex string using this CMap
func (cmap *ToUnicodeCMap) DecodeHexString(hexStr string) string {
	// Remove angle brackets if present
	hexStr = strings.Trim(hexStr, "<>")
	
	// Decode hex to bytes
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return ""
	}
	
	var result strings.Builder
	
	// Process 2 bytes at a time (assuming 2-byte CIDs)
	for i := 0; i < len(data); i += 2 {
		if i+1 >= len(data) {
			// Handle odd byte at the end
			cid := uint16(data[i])
			if unicode, ok := cmap.MapCIDToUnicode(cid); ok {
				result.WriteString(unicode)
			} else {
				// Fallback: treat as ASCII
				result.WriteByte(data[i])
			}
		} else {
			// Extract 2-byte CID
			cid := uint16(data[i])<<8 | uint16(data[i+1])
			
			if unicode, ok := cmap.MapCIDToUnicode(cid); ok {
				result.WriteString(unicode)
			} else {
				// Fallback: try single-byte CIDs
				if unicode1, ok := cmap.MapCIDToUnicode(uint16(data[i])); ok {
					result.WriteString(unicode1)
				} else {
					result.WriteByte(data[i])
				}
				if unicode2, ok := cmap.MapCIDToUnicode(uint16(data[i+1])); ok {
					result.WriteString(unicode2)
				} else {
					result.WriteByte(data[i+1])
				}
			}
		}
	}
	
	return result.String()
}

// GetMappingCount returns the total number of mappings in this CMap
func (cmap *ToUnicodeCMap) GetMappingCount() int {
	count := len(cmap.cidToUnicode)
	
	for _, r := range cmap.ranges {
		if len(r.unicodeArray) > 0 {
			count += len(r.unicodeArray)
		} else {
			count += int(r.endCID - r.startCID + 1)
		}
	}
	
	return count
}

// String returns a string representation of the CMap for debugging
func (cmap *ToUnicodeCMap) String() string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("ToUnicodeCMap:\n"))
	sb.WriteString(fmt.Sprintf("  Direct mappings: %d\n", len(cmap.cidToUnicode)))
	sb.WriteString(fmt.Sprintf("  Range mappings: %d\n", len(cmap.ranges)))
	sb.WriteString(fmt.Sprintf("  Total mappings: %d\n", cmap.GetMappingCount()))
	
	// Show first few direct mappings
	count := 0
	for cid, unicode := range cmap.cidToUnicode {
		if count >= 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more direct mappings\n", len(cmap.cidToUnicode)-5))
			break
		}
		sb.WriteString(fmt.Sprintf("  CID %04X -> %q\n", cid, unicode))
		count++
	}
	
	// Show ranges
	for i, r := range cmap.ranges {
		if i >= 3 {
			sb.WriteString(fmt.Sprintf("  ... and %d more ranges\n", len(cmap.ranges)-3))
			break
		}
		if len(r.unicodeArray) > 0 {
			sb.WriteString(fmt.Sprintf("  Range: CID %04X-%04X -> [array of %d values]\n",
				r.startCID, r.endCID, len(r.unicodeArray)))
		} else {
			sb.WriteString(fmt.Sprintf("  Range: CID %04X-%04X -> U+%04X-U+%04X\n",
				r.startCID, r.endCID, r.startUnicode, r.startUnicode+r.endCID-r.startCID))
		}
	}
	
	return sb.String()
}