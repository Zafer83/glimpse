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

var docExtensions = map[string]bool{
	".md":  true,
	".mdx": true,
}

// skipDirs is the set of directory names to skip entirely during traversal.
// Checked against the exact directory name (not a substring of the full path)
// to avoid false positives like "distribute" or home dirs with "build" in the name.
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
	"target":       true, // Rust / Maven
}

// skipFiles is the set of exact filenames to ignore.
var skipFiles = map[string]bool{
	"package-lock.json": true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"go.sum":            true,
}

// expandTilde replaces a leading "~/" or bare "~" with the user's home directory.
// filepath.Abs does not handle tilde, so we do it explicitly.
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

// CollectCode walks the project tree and concatenates docs + source code.
// Documentation is placed first so the model can infer intent/story before deep code details.
func CollectCode(root string) (string, error) {
	absPath, err := filepath.Abs(expandTilde(root))
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("project path does not exist: %s", absPath)
	}

	var docs []string
	var code []string

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := info.Name()
		if info.IsDir() {
			// Skip hidden directories and known noise directories by exact name.
			if strings.HasPrefix(name, ".") || skipDirs[strings.ToLower(name)] {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip hidden files and known lock/generated files by exact name.
		if strings.HasPrefix(name, ".") || skipFiles[strings.ToLower(name)] {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !docExtensions[ext] && !codeExtensions[ext] {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		text := string(content)
		if strings.TrimSpace(text) == "" {
			return nil
		}

		if docExtensions[ext] {
			docs = append(docs, fmt.Sprintf("\n\n# --- DOC FILE: %s ---\n%s", path, text))
		} else {
			code = append(code, fmt.Sprintf("\n\n// --- CODE FILE: %s ---\n%s", path, text))
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(docs)
	sort.Strings(code)

	var builder strings.Builder
	builder.WriteString("\n\n# PROJECT CONTEXT ORDER: DOCUMENTATION FIRST, CODE SECOND")
	for _, part := range docs {
		builder.WriteString(part)
	}
	for _, part := range code {
		builder.WriteString(part)
	}
	return builder.String(), nil
}
