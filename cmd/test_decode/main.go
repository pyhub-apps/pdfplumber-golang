package main

import (
	"fmt"
	"github.com/pyhub-apps/pdfplumber-golang/pkg/pdf"
)

func main() {
	cmapData := `
		beginbfchar
		<0048> <0048>
		<0065> <0065>
		<006C> <006C>
		<006F> <006F>
		endbfchar
	`
	
	cmap := pdf.NewToUnicodeCMap()
	if err := cmap.Parse([]byte(cmapData)); err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}
	
	// Test DecodeHexString
	testHex := "0048006500650065006C006C006F"
	result := cmap.DecodeHexString(testHex)
	fmt.Printf("DecodeHexString(%q) = %q\n", testHex, result)
	
	// Show what we're decoding
	for i := 0; i < len(testHex); i += 4 {
		if i+3 < len(testHex) {
			cidHex := testHex[i:i+4]
			fmt.Printf("  CID %s -> ", cidHex)
			// Try to map it
			var cid uint16
			fmt.Sscanf(cidHex, "%04x", &cid)
			if unicode, ok := cmap.MapCIDToUnicode(cid); ok {
				fmt.Printf("%q (mapped)\n", unicode)
			} else {
				fmt.Printf("not mapped\n")
			}
		}
	}
}