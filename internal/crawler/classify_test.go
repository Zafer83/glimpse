package crawler

import "testing"

func TestClassifyCodeFile(t *testing.T) {
	tests := []struct {
		relPath  string
		fileName string
		want     string
	}{
		// Business logic
		{"internal/ai/ai.go", "ai.go", CategoryBusiness},
		{"cmd/main.go", "main.go", CategoryBusiness},
		{"src/services/auth.ts", "auth.ts", CategoryBusiness},
		{"app/models/user.py", "user.py", CategoryBusiness},

		// Test files by naming convention
		{"internal/ai/ai_test.go", "ai_test.go", CategorySupport},
		{"src/auth.test.ts", "auth.test.ts", CategorySupport},
		{"src/auth.spec.js", "auth.spec.js", CategorySupport},
		{"tests/test_utils.py", "test_utils.py", CategorySupport},
		{"tests/user_test.py", "user_test.py", CategorySupport},

		// Files in test directories
		{"tests/helper.go", "helper.go", CategorySupport},
		{"__tests__/setup.ts", "setup.ts", CategorySupport},
		{"spec/factories.rb", "factories.rb", CategorySupport},
		{"test/fixtures.go", "fixtures.go", CategorySupport},

		// Mock/stub directories
		{"mocks/service.go", "service.go", CategorySupport},
		{"internal/fake/client.go", "client.go", CategorySupport},

		// Generated code
		{"gen/proto/api.go", "api.go", CategorySupport},
		{"generated/models.ts", "models.ts", CategorySupport},

		// Migration directories
		{"migrations/001_create_users.sql", "001_create_users.sql", CategorySupport},
		{"db/seeds/data.go", "data.go", CategorySupport},

		// Config files
		{"Dockerfile", "Dockerfile", CategorySupport},
		{"Makefile", "Makefile", CategorySupport},
		{"tsconfig.json", "tsconfig.json", CategorySupport},
		{"docker-compose.yml", "docker-compose.yml", CategorySupport},

		// Config/scripting extensions
		{"config/app.yaml", "app.yaml", CategorySupport},
		{"scripts/deploy.sh", "deploy.sh", CategorySupport},
		{"settings.json", "settings.json", CategorySupport},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			got := classifyCodeFile(tt.relPath, tt.fileName)
			if got != tt.want {
				t.Errorf("classifyCodeFile(%q, %q) = %q, want %q", tt.relPath, tt.fileName, got, tt.want)
			}
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"ai_test.go", true},
		{"auth.test.ts", true},
		{"auth.spec.js", true},
		{"test_utils.py", true},
		{"user_test.py", true},
		{"usertest.java", true},
		{"main.go", false},
		{"service.ts", false},
		{"testing.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTestFile(tt.name); got != tt.want {
				t.Errorf("isTestFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsInSupportDir(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"tests/helper.go", true},
		{"__tests__/setup.ts", true},
		{"mocks/service.go", true},
		{"generated/api.go", true},
		{"migrations/001.sql", true},
		{"internal/ai/ai.go", false},
		{"cmd/main.go", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isInSupportDir(tt.path); got != tt.want {
				t.Errorf("isInSupportDir(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
