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
	"strings"
)

var supportedExtensions = map[string]bool{
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

// CollectCode walks the project tree and concatenates supported source files.
func CollectCode(root string) (string, error) {
	absPath, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project path: %w", err)
	}
	var builder strings.Builder

	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		lowPath := strings.ToLower(path)
		// Skip large/generated/sensitive folders to keep prompts focused and fast.
		if strings.Contains(lowPath, "node_modules") ||
			strings.Contains(lowPath, ".git") ||
			strings.Contains(lowPath, "dist") ||
			strings.Contains(lowPath, ".idea") ||
			strings.Contains(lowPath, "vendor") ||
			strings.Contains(lowPath, "build") ||
			strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExtensions[ext] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		builder.WriteString(fmt.Sprintf("\n\n// --- FILE: %s ---\n", path))
		builder.Write(content)

		return nil
	})

	return builder.String(), err
}
