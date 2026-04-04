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

func shouldSkipPath(path string) bool {
	lowPath := strings.ToLower(path)
	return strings.Contains(lowPath, "node_modules") ||
		strings.Contains(lowPath, ".git") ||
		strings.Contains(lowPath, "dist") ||
		strings.Contains(lowPath, ".idea") ||
		strings.Contains(lowPath, "vendor") ||
		strings.Contains(lowPath, "build") ||
		strings.HasSuffix(lowPath, "package-lock.json") ||
		strings.HasSuffix(lowPath, "pnpm-lock.yaml") ||
		strings.HasSuffix(lowPath, "yarn.lock") ||
		strings.HasPrefix(filepath.Base(path), ".")
}

// CollectCode walks the project tree and concatenates docs + source code.
// Documentation is placed first so the model can infer intent/story before deep code details.
func CollectCode(root string) (string, error) {
	absPath, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}

	var docs []string
	var code []string

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if shouldSkipPath(path) {
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
