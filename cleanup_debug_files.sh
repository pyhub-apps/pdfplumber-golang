#!/bin/bash
# Safe cleanup script for debug and test files

echo "Creating backup directory..."
mkdir -p _archived_debug

echo "Archiving debug files..."
# Archive debug directories
for dir in cmd/debug_*; do
    if [ -d "$dir" ]; then
        echo "Archiving $dir..."
        tar -czf "_archived_debug/$(basename $dir).tar.gz" "$dir" 2>/dev/null
    fi
done

# Archive test directories (except test_extraction)
for dir in cmd/test_*; do
    if [ -d "$dir" ] && [ "$dir" != "cmd/test_extraction" ]; then
        echo "Archiving $dir..."
        tar -czf "_archived_debug/$(basename $dir).tar.gz" "$dir" 2>/dev/null
    fi
done

# Archive other utility commands
for dir in cmd/dump_* cmd/show_* cmd/extract_cmap; do
    if [ -d "$dir" ]; then
        echo "Archiving $dir..."
        tar -czf "_archived_debug/$(basename $dir).tar.gz" "$dir" 2>/dev/null
    fi
done

echo ""
echo "Archives created in _archived_debug/"
echo "To remove original files, run: bash cleanup_debug_files.sh --remove"
echo ""

if [ "$1" == "--remove" ]; then
    echo "Removing original debug files..."
    rm -rf cmd/debug_*
    # Remove test directories except test_extraction
    for dir in cmd/test_*; do
        if [ "$dir" != "cmd/test_extraction" ]; then
            rm -rf "$dir"
        fi
    done
    rm -rf cmd/dump_* cmd/show_* cmd/extract_cmap
    echo "Cleanup complete!"
    
    # Count remaining cmd directories
    echo ""
    echo "Remaining cmd directories:"
    ls -1d cmd/*/ | wc -l
    ls -1d cmd/*/
fi