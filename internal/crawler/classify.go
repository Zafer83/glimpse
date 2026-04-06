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
	"path/filepath"
	"strings"
)

// Category constants for file classification.
const (
	CategoryDoc      = "doc"
	CategoryBusiness = "business"
	CategorySupport  = "support"
)

// testDirs are directory names that indicate test/fixture content.
var testDirs = map[string]bool{
	"test":       true,
	"tests":      true,
	"__tests__":  true,
	"spec":       true,
	"specs":      true,
	"testdata":   true,
	"fixtures":   true,
	"mock":       true,
	"mocks":      true,
	"fake":       true,
	"fakes":      true,
	"stub":       true,
	"stubs":      true,
	"testutil":   true,
	"testutils":  true,
	"test_utils": true,
	"test-utils": true,
}

// generatedDirs are directory names indicating generated/compiled code.
var generatedDirs = map[string]bool{
	"generated": true,
	"gen":       true,
	"proto":     true,
	"pb":        true,
	"swagger":   true,
	"openapi":   true,
}

// migrationDirs indicate database migration/seed content.
var migrationDirs = map[string]bool{
	"migrations": true,
	"migrate":    true,
	"seeds":      true,
	"seeders":    true,
}

// configFiles are filenames treated as support (config/build tooling).
var configFiles = map[string]bool{
	"dockerfile":          true,
	"docker-compose.yml":  true,
	"docker-compose.yaml": true,
	"makefile":            true,
	"jenkinsfile":         true,
	"procfile":            true,
	"rakefile":            true,
	"gruntfile.js":        true,
	"gulpfile.js":         true,
	"tsconfig.json":       true,
	"jsconfig.json":       true,
	"babel.config.js":     true,
	"babel.config.json":   true,
	".babelrc":            true,
	"webpack.config.js":   true,
	"webpack.config.ts":   true,
	"vite.config.js":      true,
	"vite.config.ts":      true,
	"rollup.config.js":    true,
	"jest.config.js":      true,
	"jest.config.ts":      true,
	"vitest.config.ts":    true,
	".eslintrc.js":        true,
	".eslintrc.json":      true,
	".prettierrc":         true,
	".prettierrc.json":    true,
	"go.mod":              true,
	"setup.py":            true,
	"setup.cfg":           true,
	"pyproject.toml":      true,
	"cargo.toml":          true,
	"gemfile":             true,
	"build.gradle":        true,
	"pom.xml":             true,
}

// supportExtensions are file types that are typically config/scripting, not business logic.
var supportExtensions = map[string]bool{
	".json": true,
	".yaml": true,
	".yml":  true,
	".xml":  true,
	".toml": true,
	".ini":  true,
	".cfg":  true,
	".conf": true,
	".sh":   true,
	".bash": true,
	".ps1":  true,
	".bat":  true,
	".cmd":  true,
}

// classifyCodeFile determines whether a code file is "business" (core logic) or
// "support" (tests, configs, generated, migrations). relPath is relative to the
// project root; fileName is the base name of the file.
func classifyCodeFile(relPath, fileName string) string {
	lower := strings.ToLower(fileName)
	lowerPath := strings.ToLower(relPath)

	// Test files by naming convention.
	if isTestFile(lower) {
		return CategorySupport
	}

	// Files inside test/mock/generated/migration directories.
	if isInSupportDir(lowerPath) {
		return CategorySupport
	}

	// Known config/build files by exact name.
	if configFiles[lower] {
		return CategorySupport
	}

	// Config/scripting extensions are support.
	ext := strings.ToLower(filepath.Ext(fileName))
	if supportExtensions[ext] {
		return CategorySupport
	}

	return CategoryBusiness
}

// isTestFile checks common test file naming patterns across languages.
func isTestFile(lower string) bool {
	// Go: *_test.go
	if strings.HasSuffix(lower, "_test.go") {
		return true
	}
	// JS/TS: *.test.js, *.test.ts, *.test.jsx, *.test.tsx
	// JS/TS: *.spec.js, *.spec.ts, *.spec.jsx, *.spec.tsx
	for _, pattern := range []string{".test.", ".spec."} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	// Python: test_*.py, *_test.py
	if strings.HasSuffix(lower, ".py") {
		base := strings.TrimSuffix(lower, ".py")
		if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test") {
			return true
		}
	}
	// Java: *Test.java, *Tests.java, *Spec.java
	if strings.HasSuffix(lower, "test.java") || strings.HasSuffix(lower, "tests.java") || strings.HasSuffix(lower, "spec.java") {
		return true
	}
	return false
}

// isInSupportDir checks if any path component matches test/mock/generated/migration dirs.
func isInSupportDir(lowerPath string) bool {
	parts := strings.Split(filepath.ToSlash(lowerPath), "/")
	for _, part := range parts {
		if testDirs[part] || generatedDirs[part] || migrationDirs[part] {
			return true
		}
	}
	return false
}
