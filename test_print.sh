#!/bin/bash

# Test script for printer application
# Creates some test files and runs the printer app

echo "Creating test files..."

# Create a temporary test directory
TEST_DIR="/tmp/printer_test_$$"
mkdir -p "$TEST_DIR"

# Create various test files
echo "This is test file 1" > "$TEST_DIR/test1.txt"
echo "This is test file 2" > "$TEST_DIR/test2.txt"
echo "This is a PDF test" > "$TEST_DIR/test.pdf"
echo "Document content" > "$TEST_DIR/document.doc"

cat << EOF > "$TEST_DIR/README.txt"
Printer Test Files
==================

This directory contains test files for the printer application.
You can use these files to test:
1. Staging files
2. Printing operations
3. Error handling

Test Instructions:
- Navigate to this directory
- Mark some files for staging (space key)
- Press P to print staged files
- Watch the print operations pane

EOF

echo "Test files created in: $TEST_DIR"
echo ""
echo "Starting printer app in test directory..."
echo ""
echo "Test steps:"
echo "1. Use arrow keys to navigate"
echo "2. Press space to mark files for staging"
echo "3. Press P to send staged files to printer"
echo "4. Press tab to switch between panes"
echo "5. Check the print operations pane for status"
echo ""

# Change to test directory and run the app
cd "$TEST_DIR"
exec "$OLDPWD/printer" add "*.txt"