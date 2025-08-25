#!/usr/bin/env python3
"""Test Python pdfplumber to understand exact behavior"""

import pdfplumber
import json

def test_pdfplumber(pdf_path):
    """Test pdfplumber text extraction with various options"""
    
    with pdfplumber.open(pdf_path) as pdf:
        print(f"PDF Info:")
        print(f"  Pages: {len(pdf.pages)}")
        print(f"  Metadata: {pdf.metadata}")
        print()
        
        for i, page in enumerate(pdf.pages):
            print(f"Page {i+1}:")
            print(f"  Width: {page.width}")
            print(f"  Height: {page.height}")
            print(f"  Bbox: {page.bbox}")
            print()
            
            # Extract characters
            chars = page.chars
            print(f"  Total characters: {len(chars)}")
            
            if chars:
                # Show first 5 characters with details
                print("  First 5 characters:")
                for j, char in enumerate(chars[:5]):
                    print(f"    {j+1}. Text: '{char['text']}'")
                    print(f"       Position: x0={char['x0']:.2f}, y0={char['top']:.2f}, x1={char['x1']:.2f}, y1={char['bottom']:.2f}")
                    print(f"       Font: {char.get('fontname', 'N/A')}, Size: {char.get('size', 'N/A')}")
                    print()
            
            # Test different extraction methods
            print("  Text extraction methods:")
            
            # 1. Default extract_text
            text_default = page.extract_text()
            print(f"    1. Default: {repr(text_default[:100] if text_default else 'None')}")
            
            # 2. With x_tolerance
            text_x_tol = page.extract_text(x_tolerance=1)
            print(f"    2. x_tolerance=1: {repr(text_x_tol[:100] if text_x_tol else 'None')}")
            
            # 3. With y_tolerance
            text_y_tol = page.extract_text(y_tolerance=1)
            print(f"    3. y_tolerance=1: {repr(text_y_tol[:100] if text_y_tol else 'None')}")
            
            # 4. With layout
            text_layout = page.extract_text(layout=True)
            print(f"    4. layout=True: {repr(text_layout[:100] if text_layout else 'None')}")
            
            # Extract words
            words = page.extract_words()
            print(f"\n  Words extracted: {len(words)}")
            if words:
                print("  First 3 words:")
                for j, word in enumerate(words[:3]):
                    print(f"    {j+1}. '{word['text']}' at ({word['x0']:.2f}, {word['top']:.2f})")
            
            print("-" * 50)
            
            # Only process first page for now
            break

if __name__ == "__main__":
    import sys
    if len(sys.argv) < 2:
        print("Usage: python test_python_pdfplumber.py <pdf_file>")
        sys.exit(1)
    
    test_pdfplumber(sys.argv[1])