#!/usr/bin/env python3
"""
Benchmark script for Python pdfplumber
"""

import sys
import time
import pdfplumber

def main():
    if len(sys.argv) < 2:
        print("Usage: python benchmark_python.py <pdf-file>")
        sys.exit(1)
    
    pdf_path = sys.argv[1]
    
    # Warm-up run
    with pdfplumber.open(pdf_path) as pdf:
        pass
    
    # Benchmark PDF opening
    start = time.time()
    pdf = pdfplumber.open(pdf_path)
    open_time = time.time() - start
    
    print("=== Python pdfplumber Benchmark ===")
    print(f"File: {pdf_path}")
    print(f"Pages: {len(pdf.pages)}")
    print(f"Open time: {open_time:.3f}s")
    
    # Benchmark text extraction
    total_text_len = 0
    start = time.time()
    for page in pdf.pages:
        text = page.extract_text() or ""
        total_text_len += len(text)
    text_time = time.time() - start
    
    print(f"Text extraction time: {text_time:.3f}s")
    print(f"Total text length: {total_text_len} chars")
    if text_time > 0:
        print(f"Text/sec: {total_text_len/text_time:.0f} chars/sec")
    
    # Benchmark table extraction
    total_tables = 0
    start = time.time()
    for page in pdf.pages:
        tables = page.extract_tables()
        total_tables += len(tables)
    table_time = time.time() - start
    
    print(f"Table extraction time: {table_time:.3f}s")
    print(f"Total tables found: {total_tables}")
    
    # Benchmark object extraction
    total_objects = 0
    start = time.time()
    for page in pdf.pages:
        chars = page.chars
        lines = page.lines
        rects = page.rects
        total_objects += len(chars) + len(lines) + len(rects)
    object_time = time.time() - start
    
    print(f"Object extraction time: {object_time:.3f}s")
    print(f"Total objects: {total_objects}")
    if object_time > 0:
        print(f"Objects/sec: {total_objects/object_time:.0f} obj/sec")
    
    # Summary
    total_time = open_time + text_time + table_time + object_time
    print(f"\n=== Summary ===")
    print(f"Total processing time: {total_time:.3f}s")
    if total_time > 0:
        print(f"Pages/sec: {len(pdf.pages)/total_time:.2f}")
    
    pdf.close()

if __name__ == "__main__":
    main()