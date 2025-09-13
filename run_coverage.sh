#!/bin/bash
cd /home/pavel/STORAGE/Projects/my_projects/wtf_cli

echo "=== Running Go Test Coverage Analysis ==="
echo

# Run tests with coverage for all packages
echo "1. Basic coverage summary:"
go test -cover ./...

echo
echo "2. Detailed coverage by package:"
go test -coverprofile=coverage.out ./...

echo
echo "3. Coverage breakdown by function:"
go tool cover -func=coverage.out

echo
echo "4. Total coverage summary:"
go tool cover -func=coverage.out | grep total

echo
echo "Coverage profile saved to: coverage.out"
echo "To view HTML coverage report, run: go tool cover -html=coverage.out"