#!/bin/bash

# Create test directory structure
echo "Creating test directory structure..."
mkdir -p test_files/subdir
echo "Hello, World!" > test_files/file1.txt
echo "Test content" > test_files/file2.txt
echo "Nested file" > test_files/subdir/file3.txt

# Initialize DSP repository
echo -e "\nInitializing DSP repository..."
dsp init

# Track test files
echo -e "\nTracking test files..."
dsp track test_files

# Create initial snapshot
echo -e "\nCreating initial snapshot..."
dsp snapshot -m "Initial snapshot"

# Make changes
echo -e "\nMaking changes to test files..."
echo "Modified content" > test_files/file1.txt
rm test_files/file2.txt
echo "New file" > test_files/file4.txt

# Create second snapshot
echo -e "\nCreating second snapshot..."
dsp snapshot -m "Second snapshot with changes"

# Test diff functionality
echo -e "\nTesting diff functionality..."
echo "1. Comparing with latest snapshot:"
dsp diff

echo -e "\n2. Showing summary of changes:"
dsp diff -s

echo -e "\n3. Showing detailed changes:"
dsp diff -v

echo -e "\n4. Showing changes for specific file:"
dsp diff -p test_files/file1.txt

# Clean up
echo -e "\nCleaning up..."
rm -rf test_files
rm -rf .dsp

echo -e "\nTest completed!" 