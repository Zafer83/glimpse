/*
Copyright 2026 Zafer Kılıçaslan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crawler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileEntry represents a single collected file with its content and category.
type FileEntry struct {
	Path     string // relative path within the project
	Content  string
	Category string // "doc", "business", or "support"
}

// CollectedContent holds project files grouped by priority tier.
type CollectedContent struct {
	Docs     []FileEntry // documentation files (highest priority)
	Business []FileEntry // core business logic
	Support  []FileEntry // tests, configs, migrations, generated code
}

var codeExtensions = map[string]bool{
	".go":   true,
	".js":   true,
	".ts":   true,
	".jsx":  true,
	".tsx":  true,
	".py":   true,
	".java": true,
	".rb":   true,
	".php":  true,
	".cs":   true,
	".cpp":  true,
	".c":    true,
	".h":    true,
	".json": true,
	".yaml": true,
	".yml":  true,
	".xml":  true,
	".sql":  true,
	".rs":   true,
	".sh":   true,
}

// docExtensions covers plain-text documentation formats.
var docExtensions = map[string]bool{
	".md":  true,
	".mdx": true,
	".txt": true,
	".rst": true,
}

// binaryDocExtensions require special extraction (not plain text).
var binaryDocExtensions = map[string]bool{
	".docx": true,
	".pdf":  true,
}

// docsDirNames are directory names whose contents are treated as documentation
// regardless of file extension. All files within these directories receive equal
// high priority in the documentation tier.
var docsDirNames = map[string]bool{
	"docs":          true,
	"doc":           true,
	"documentation": true,
	"wiki":          true,
	"pages":         true,
}

// skipMediaExtensions lists binary/media file types that cannot be read as text
// and are skipped even when found inside a docs directory.
var skipMediaExtensions = map[string]bool{
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".svg":   true,
	".ico":   true,
	".webp":  true,
	".bmp":   true,
	".mp4":   true,
	".mp3":   true,
	".wav":   true,
	".ogg":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
	".7z":    true,
	".exe":   true,
	".bin":   true,
	".dll":   true,
	".so":    true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
}

// codeInDocsExtensions lists source code file types that should remain in the
// code tier even when found inside a docs directory. React/Vue components,
// scripts, and compiled languages are code — not documentation narrative.
var codeInDocsExtensions = map[string]bool{
	".jsx":  true,
	".tsx":  true,
	".js":   true,
	".ts":   true,
	".py":   true,
	".go":   true,
	".java": true,
	".rb":   true,
	".php":  true,
	".cs":   true,
	".cpp":  true,
	".c":    true,
	".h":    true,
	".rs":   true,
	".sql":  true,
}

// isInDocsDir reports whether the given relative path lives inside a docs
// directory (any path component matching docsDirNames).
func isInDocsDir(relPath string) bool {
	lower := strings.ToLower(filepath.ToSlash(relPath))
	for _, part := range strings.Split(lower, "/") {
		if docsDirNames[part] {
			return true
		}
	}
	return false
}

// isCodeInDocsDir returns true when a file is in a docs directory but has a
// source code extension (e.g. .jsx, .tsx, .py). These files should be classified
// as business/support code, not documentation, so they don't crowd out markdown
// narrative from the docs budget.
func isCodeInDocsDir(relPath string) bool {
	ext := strings.ToLower(filepath.Ext(relPath))
	return isInDocsDir(relPath) && codeInDocsExtensions[ext]
}

// skipDirs is the set of directory names to skip entirely during traversal.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	".idea":        true,
	"vendor":       true,
	"build":        true,
	"__pycache__":  true,
	".next":        true,
	".nuxt":        true,
	"coverage":     true,
	".cache":       true,
	"out":          true,
	".venv":        true,
	"venv":         true,
	"target":       true,
}

// skipFiles is the set of exact filenames to ignore.
var skipFiles = map[string]bool{
	"package-lock.json": true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"go.sum":            true,
}

// expandTilde replaces a leading "~/" or bare "~" with the user's home directory.
func expandTilde(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// docSortPriority returns a sort key for documentation files.
// README files sort first, then files inside docs directories (all equally at
// tier 1), then ARCHITECTURE/DESIGN, then everything else alphabetically.
func docSortPriority(relPath string) string {
	name := filepath.Base(relPath)
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "readme"):
		return "0_" + lower
	case isInDocsDir(relPath):
		// All files inside a docs directory share tier 1 so they are equally
		// prioritized — sorted only alphabetically among themselves.
		return "1_" + strings.ToLower(filepath.ToSlash(relPath))
	case strings.HasPrefix(lower, "architecture") || strings.HasPrefix(lower, "design") || strings.HasPrefix(lower, "overview"):
		return "2_" + lower
	case strings.HasPrefix(lower, "contributing") || strings.HasPrefix(lower, "changelog"):
		return "3_" + lower
	default:
		return "4_" + lower
	}
}

// CollectProject walks the project tree and returns files grouped into docs,
// business logic, and support tiers. Documentation is prioritized for the AI
// to understand the project's purpose before analyzing code.
func CollectProject(root string) (*CollectedContent, error) {
	absPath, err := filepath.Abs(expandTilde(root))
	if err != nil {
		return nil, fmt.Errorf("resolve project path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("project path does not exist: %s", absPath)
	}

	result := &CollectedContent{}
	var warnings []string

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			if strings.HasPrefix(name, ".") || skipDirs[strings.ToLower(name)] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || skipFiles[strings.ToLower(name)] {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		relPath, _ := filepath.Rel(absPath, path)

		// Binary document formats (docx, pdf) are always docs regardless of location.
		if binaryDocExtensions[ext] {
			text, extractErr := extractBinaryDoc(path, ext)
			if extractErr != nil {
				warnings = append(warnings, fmt.Sprintf("Skipping %s: %v", relPath, extractErr))
				return nil
			}
			if strings.TrimSpace(text) != "" {
				result.Docs = append(result.Docs, FileEntry{
					Path:     relPath,
					Content:  text,
					Category: CategoryDoc,
				})
			}
			return nil
		}

		// Files inside a docs directory are treated as documentation regardless of
		// extension. Binary/media files (images, archives, fonts) are skipped.
		// Source code files (.jsx, .tsx, .py, etc.) stay in the code tier even
		// when inside docs/ — they are interactive components, not narrative text.
		if isInDocsDir(relPath) && !skipMediaExtensions[ext] && !isCodeInDocsDir(relPath) {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			text := string(content)
			if strings.TrimSpace(text) != "" {
				result.Docs = append(result.Docs, FileEntry{
					Path:     relPath,
					Content:  text,
					Category: CategoryDoc,
				})
			}
			return nil
		}

		// Plain-text documentation (outside docs directories).
		if docExtensions[ext] {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			text := string(content)
			if strings.TrimSpace(text) != "" {
				result.Docs = append(result.Docs, FileEntry{
					Path:     relPath,
					Content:  text,
					Category: CategoryDoc,
				})
			}
			return nil
		}

		// Code files.
		if codeExtensions[ext] {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			text := string(content)
			if strings.TrimSpace(text) == "" {
				return nil
			}

			cat := classifyCodeFile(relPath, name)
			entry := FileEntry{Path: relPath, Content: text, Category: cat}
			if cat == CategoryBusiness {
				result.Business = append(result.Business, entry)
			} else {
				result.Support = append(result.Support, entry)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Print warnings for skipped binary docs.
	for _, w := range warnings {
		fmt.Printf("  ⚠ %s\n", w)
	}

	// Sort docs: README first, then docs-directory files, then architecture docs,
	// then everything else. Within each priority tier, sort by descending content
	// size so the most comprehensive documents are included first when the AI
	// context budget runs out (greedy fill picks top files).
	sort.Slice(result.Docs, func(i, j int) bool {
		pi := docSortPriority(result.Docs[i].Path)
		pj := docSortPriority(result.Docs[j].Path)
		if pi != pj {
			return pi < pj
		}
		// Same priority tier: larger files first (more content = more comprehensive).
		return len(result.Docs[i].Content) > len(result.Docs[j].Content)
	})
	sort.Slice(result.Business, func(i, j int) bool {
		return result.Business[i].Path < result.Business[j].Path
	})
	sort.Slice(result.Support, func(i, j int) bool {
		return result.Support[i].Path < result.Support[j].Path
	})

	return result, nil
}

// extractBinaryDoc dispatches extraction for binary document types.
func extractBinaryDoc(path, ext string) (string, error) {
	switch ext {
	case ".docx":
		return extractDocxText(path)
	case ".pdf":
		return extractPDFText(path)
	default:
		return "", fmt.Errorf("unsupported binary doc format: %s", ext)
	}
}

// CollectCode is the legacy API that returns a flat concatenated string.
// It wraps CollectProject for backward compatibility.
func CollectCode(root string) (string, error) {
	content, err := CollectProject(root)
	if err != nil {
		return "", err
	}
	return content.Flatten(), nil
}

// Flatten concatenates all tiers into a single string (docs first, then business, then support).
func (c *CollectedContent) Flatten() string {
	var builder strings.Builder
	builder.WriteString("\n\n# PROJECT CONTEXT ORDER: DOCUMENTATION FIRST, CODE SECOND")

	for _, entry := range c.Docs {
		builder.WriteString(fmt.Sprintf("\n\n# --- DOC FILE: %s ---\n%s", entry.Path, entry.Content))
	}
	for _, entry := range c.Business {
		builder.WriteString(fmt.Sprintf("\n\n// --- CODE FILE: %s ---\n%s", entry.Path, entry.Content))
	}
	for _, entry := range c.Support {
		builder.WriteString(fmt.Sprintf("\n\n// --- SUPPORT FILE: %s ---\n%s", entry.Path, entry.Content))
	}
	return builder.String()
}

// Stats returns a summary of collected content counts.
func (c *CollectedContent) Stats() (docs, business, support int) {
	return len(c.Docs), len(c.Business), len(c.Support)
}
