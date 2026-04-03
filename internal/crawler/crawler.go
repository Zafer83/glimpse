package crawler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func CollectCode(root string) (string, error) {
	var builder strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.Contains(path, "node_modules") || strings.Contains(path, ".git") {
			return nil
		}

		ext := filepath.Ext(path)
		// Erweitere die Liste der unterstützten Dateien
		supported := map[string]bool{".ts": true, ".js": true, ".vue": true, ".go": true, ".py": true}

		if supported[ext] {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			builder.WriteString(fmt.Sprintf("\n\n// --- FILE: %s ---\n", path))
			builder.Write(content)
		}
		return nil
	})
	return builder.String(), err
}
