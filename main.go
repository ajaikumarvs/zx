package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	//"strings"
	"time"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// ANSI color codes for highlighting
const (
	Reset   = "\033[0m"
	Red     = "\033[1;31m"
	Green   = "\033[1;32m"
	Yellow  = "\033[1;33m"
	Cyan    = "\033[1;36m"
	Magenta = "\033[1;35m"
)

// searchInFile searches for a pattern in a file and prints matches with details.
func searchInFile(pattern, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s[ERROR] Unable to open file:%s %s\n", Red, Reset, filePath)
		return
	}
	defer file.Close()

	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s[ERROR] Invalid regex pattern:%s %s\n", Red, Reset, err)
		return
	}

	// Get file info
	fileInfo, _ := file.Stat()
	fmt.Printf("\n%s📂 File:%s %s (%d bytes, Last Modified: %s)\n", Cyan, Reset, filePath, fileInfo.Size(), fileInfo.ModTime().Format(time.RFC822))

	scanner := bufio.NewScanner(file)
	lineNum := 1
	var matchFound bool
	var suggestions []string

	for scanner.Scan() {
		line := scanner.Text()

		// Check for exact match
		if re.MatchString(line) {
			matchFound = true
			fmt.Printf("  %s%d |%s %s\n", Yellow, lineNum, Reset, highlightMatch(line, re))
		} else {
			// Store potential similar matches for suggestions
			if levenshtein.DistanceForStrings([]rune(line), []rune(pattern), levenshtein.DefaultOptions) <= 3 {
				suggestions = append(suggestions, line)
			}
		}
		lineNum++
	}

	if !matchFound {
		fmt.Printf("  %s[INFO] No exact matches found in:%s %s\n", Magenta, Reset, filePath)
	}

	// Show suggested matches if no exact match found
	if len(suggestions) > 0 {
		fmt.Printf("  %s🔍 Suggested Matches:%s\n", Green, Reset)
		for _, suggestion := range suggestions {
			fmt.Printf("    - %s%s%s\n", Cyan, suggestion, Reset)
		}
	}
}

// highlightMatch adds red color to matched text.
func highlightMatch(text string, re *regexp.Regexp) string {
	return re.ReplaceAllString(text, Red+"$0"+Reset)
}

// searchInFolder recursively searches all files in a directory.
func searchInFolder(pattern, folderPath string) {
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s[ERROR] Unable to access:%s %s\n", Red, Reset, path)
			return nil
		}

		// Skip directories
		if !info.IsDir() {
			searchInFile(pattern, path)
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s[ERROR] Failed to scan folder:%s %s\n", Red, Reset, folderPath)
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "%sUsage:%s %s <pattern> <file|folder>\n", Yellow, Reset, os.Args[0])
		os.Exit(1)
	}

	pattern := os.Args[1]
	target := os.Args[2]

	// Check if target is a file or folder
	fileInfo, err := os.Stat(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s[ERROR] File or folder not found:%s %s\n", Red, Reset, target)
		os.Exit(1)
	}

	if fileInfo.IsDir() {
		fmt.Printf("\n%s📂 Searching folder:%s %s\n", Cyan, Reset, target)
		searchInFolder(pattern, target)
	} else {
		fmt.Printf("\n%s📄 Searching file:%s %s\n", Cyan, Reset, target)
		searchInFile(pattern, target)
	}
}
