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

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"

	"strings"
	"syscall"
	"time"

	"github.com/Zafer83/glimpse/internal/ai"
	"github.com/Zafer83/glimpse/internal/config"
	"github.com/Zafer83/glimpse/internal/crawler"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
	"github.com/peterh/liner"
	"golang.org/x/term"
)

// --- Constants ---
const (
	loaderTickInterval    = 110 * time.Millisecond
	plainLoaderInterval   = 300 * time.Millisecond
	progressBarWidth      = 26
	progressInitialPct    = 8.0
	progressMaxPct        = 97.0
	progressCurveExponent = 10.0
	defaultLocalLLMURL    = "http://localhost:8080"
)

var (
	ColorBlue    = "\033[34m"
	ColorCyan    = "\033[36m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorGray    = "\033[90m"
	ColorRed     = "\033[31m"
	ColorMagenta = "\033[35m"
	ColorReset   = "\033[0m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
	ansiEnabled  = true
)

// version should follow Major.Minor.Build and can be overridden at build time:
// go build -ldflags "-X main.version=1.2.3"
var version = "0.0.0"

// forcePlainMode can be set at build time for platform-specific fallback:
// go build -ldflags "-X main.forcePlainMode=1"
var forcePlainMode = "0"

var semverTripletRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// Banner gradient colors ("Cyberpunk Night" theme).
var (
	bannerStartHex = "#00f2fe"
	bannerEndHex   = "#7117ea"
)

// --- Version helpers ---
func normalizeTripletVersion(raw string) (string, bool) {
	m := semverTripletRe.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) != 4 {
		return "", false
	}
	return fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3]), true
}

func resolvedVersion() string {
	if v, ok := normalizeTripletVersion(version); ok {
		return v
	}
	return "0.0.0"
}

func shortBannerVersion(full string) string {
	parts := strings.Split(full, ".")
	if len(parts) >= 2 {
		return fmt.Sprintf("v%s.%s", parts[0], parts[1])
	}
	return "v0.0"
}

// --- Terminal setup ---
func initTerminalAppearance() {
	if forcePlainMode == "1" {
		ansiEnabled = false
	}
	if os.Getenv("NO_COLOR") != "" {
		ansiEnabled = false
	}
	if runtime.GOOS == "windows" && os.Getenv("FORCE_ANSI") != "1" {
		ansiEnabled = false
	}
	if os.Getenv("GLIMPSE_NO_ANSI") == "1" {
		ansiEnabled = false
	}
	if !ansiEnabled {
		ColorBlue = ""
		ColorCyan = ""
		ColorGreen = ""
		ColorYellow = ""
		ColorGray = ""
		ColorRed = ""
		ColorMagenta = ""
		ColorReset = ""
		ColorBold = ""
		ColorDim = ""
	}
}

func setupInterruptHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		exitOnInterrupt()
	}()
}

func exitOnInterrupt() {
	fmt.Println()
	fmt.Println(ColorRed + "[!] Aborted by user (Ctrl+C)." + ColorReset)
	os.Exit(130)
}

func isInterruptErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "interrupted")
}

// --- User input ---
func askBuffered(reader *bufio.Reader, prompt, defaultValue string) string {
	fullPrompt := fmt.Sprintf("%s%s%s %s[%s]%s: ", ColorBold, ColorCyan, prompt, ColorYellow, defaultValue, ColorReset)
	fmt.Print(fullPrompt)

	input, err := reader.ReadString('\n')
	if isInterruptErr(err) {
		exitOnInterrupt()
	}
	if err != nil && err != io.EOF {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

func askLine(line *liner.State, prompt, defaultValue string) string {
	var fullPrompt string
	if defaultValue == "" {
		fullPrompt = fmt.Sprintf("%s: ", prompt)
	} else {
		fullPrompt = fmt.Sprintf("%s [%s]: ", prompt, defaultValue)
	}

	input, err := line.Prompt(fullPrompt)
	if errors.Is(err, liner.ErrPromptAborted) {
		exitOnInterrupt()
	}
	if err != nil {
		return defaultValue
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	line.AppendHistory(input)
	return input
}

func ask(line *liner.State, reader *bufio.Reader, prompt, defaultValue string) string {
	if line != nil {
		return askLine(line, prompt, defaultValue)
	}
	return askBuffered(reader, prompt, defaultValue)
}

func supportsLineEditing() bool {
	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	if strings.Contains(termProgram, "jetbrains") {
		return false
	}
	if os.Getenv("TERM") == "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func promptInputReader() (*bufio.Reader, io.Closer) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return bufio.NewReader(os.Stdin), nil
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil {
		return bufio.NewReader(tty), tty
	}

	return bufio.NewReader(os.Stdin), nil
}

// --- Slidev theme helpers ---
func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func resolveThemeCandidates(theme string) ([]string, bool) {
	t := strings.TrimSpace(strings.ToLower(theme))
	switch t {
	case "default", "seriph":
		return []string{t}, true
	}

	if strings.HasPrefix(t, "@slidev/theme-slidev-theme-") {
		trimmed := strings.TrimPrefix(t, "@slidev/theme-")
		return uniqueStrings([]string{trimmed, "@slidev/theme-" + strings.TrimPrefix(trimmed, "slidev-theme-")}), false
	}

	if strings.HasPrefix(t, "@") || strings.Contains(t, "/") {
		candidates := []string{t}
		if strings.HasPrefix(t, "@slidev/theme-") {
			name := strings.TrimPrefix(t, "@slidev/theme-")
			if strings.HasPrefix(name, "slidev-theme-") {
				candidates = append(candidates, name)
			} else {
				candidates = append(candidates, "slidev-theme-"+name)
			}
		}
		return uniqueStrings(candidates), false
	}

	return uniqueStrings([]string{
		"@slidev/theme-" + t,
		"slidev-theme-" + t,
	}), false
}

// glimpseWorkDir returns the persistent cache directory for glimpse's Slidev
// infrastructure (node_modules, vite config). Uses the OS user cache dir so
// files survive reboots and never pollute the user's working directory.
func glimpseWorkDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "glimpse")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create glimpse work dir: %w", err)
	}
	return dir, nil
}

// initWorkDir writes the minimal package.json and vite.config.ts into workDir
// if they do not already exist.
func initWorkDir(workDir string) error {
	pkgJSON := filepath.Join(workDir, "package.json")
	if _, err := os.Stat(pkgJSON); os.IsNotExist(err) {
		content := `{"private":true,"dependencies":{}}` + "\n"
		if err := os.WriteFile(pkgJSON, []byte(content), 0644); err != nil {
			return fmt.Errorf("cannot write package.json: %w", err)
		}
	}
	viteCfg := filepath.Join(workDir, "vite.config.ts")
	if _, err := os.Stat(viteCfg); os.IsNotExist(err) {
		content := "import { defineConfig } from 'vite'\n\nexport default defineConfig({\n  server: { fs: { strict: false } },\n})\n"
		if err := os.WriteFile(viteCfg, []byte(content), 0644); err != nil {
			return fmt.Errorf("cannot write vite.config.ts: %w", err)
		}
	}
	return nil
}

func npmPackageExists(pkg string) bool {
	cmd := exec.Command("npm", "view", pkg, "name", "--silent")
	return cmd.Run() == nil
}

func npmPackageInstalled(pkg, workDir string) bool {
	parts := strings.Split(pkg, "/")
	path := filepath.Join(append([]string{workDir, "node_modules"}, parts...)...)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func ensureSlidevTheme(theme, workDir string) error {
	candidates, builtin := resolveThemeCandidates(theme)
	if builtin {
		return nil
	}

	var pkg string
	for _, c := range candidates {
		if npmPackageExists(c) {
			pkg = c
			break
		}
	}
	if pkg == "" {
		return fmt.Errorf("theme %q is not a valid Slidev theme package. tried: %s", theme, strings.Join(candidates, ", "))
	}
	if npmPackageInstalled(pkg, workDir) {
		return nil
	}

	fmt.Printf("%s📦 Installing Slidev theme package: %s%s\n", ColorCyan, pkg, ColorReset)
	cmd := exec.Command("npm", "install", "-D", pkg)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install theme package %s: %w", pkg, err)
	}
	return nil
}

// --- Progress loader ---
func startFancyLoader(message string, profile termenv.Profile, startColor, endColor colorful.Color) func(doneText string) {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		if !isTTY || !ansiEnabled {
			fmt.Printf("%s%s%s\n", ColorMagenta, message, ColorReset)
			start := time.Now()
			ticker := time.NewTicker(plainLoaderInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					elapsed := time.Since(start).Round(time.Second)
					fmt.Printf("\rProgress (estimated): running | Elapsed %s", elapsed)
				}
			}
		}

		start := time.Now()
		ticker := time.NewTicker(loaderTickInterval)
		defer ticker.Stop()

		render := func() {
			elapsed := time.Since(start)
			sec := elapsed.Seconds()

			displayProgress := progressInitialPct + (progressMaxPct-progressInitialPct)*(1.0-1.0/(1.0+sec/progressCurveExponent))
			if displayProgress > progressMaxPct {
				displayProgress = progressMaxPct
			}

			filled := int(displayProgress / 100.0 * float64(progressBarWidth))
			if filled < 0 {
				filled = 0
			}
			if filled > progressBarWidth {
				filled = progressBarWidth
			}
			var bar strings.Builder
			denominator := progressBarWidth - 1
			if denominator <= 0 {
				denominator = 1
			}
			for i := 0; i < progressBarWidth; i++ {
				if i < filled {
					ratio := float64(i) / float64(denominator)
					gradColor := startColor.BlendLuv(endColor, ratio).Hex()
					bar.WriteString(termenv.String("█").Foreground(profile.Color(gradColor)).String())
				} else {
					bar.WriteString(termenv.String("░").Foreground(profile.Color("#3a3a3a")).String())
				}
			}
			eta := "--"
			if displayProgress > 0.5 {
				total := time.Duration(float64(elapsed) * (100.0 / displayProgress))
				remaining := total - elapsed
				if remaining < 0 {
					remaining = 0
				}
				eta = remaining.Round(time.Second).String()
			}

			fmt.Printf("\r\033[2K%sProgress (estimated)%s [%s] %5.1f%% | ETA ~%s | Elapsed %s",
				ColorBlue, ColorReset, bar.String(), displayProgress, eta, elapsed.Round(time.Second))
		}

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				render()
			}
		}
	}()

	return func(doneText string) {
		close(stop)
		<-done
		if ansiEnabled {
			var finalBar strings.Builder
			denominator := progressBarWidth - 1
			if denominator <= 0 {
				denominator = 1
			}
			for i := 0; i < progressBarWidth; i++ {
				ratio := float64(i) / float64(denominator)
				gradColor := startColor.BlendLuv(endColor, ratio).Hex()
				finalBar.WriteString(termenv.String("█").Foreground(profile.Color(gradColor)).String())
			}
			fmt.Printf("\r\033[2K%sProgress%s [%s] 100.0%% | %s%s%s\n", ColorBlue, ColorReset, finalBar.String(), ColorGreen, doneText, ColorReset)
		} else {
			fmt.Printf("\rProgress: 100.0%% | %s%s%s\n", ColorGreen, doneText, ColorReset)
		}
	}
}

// --- Banner ---
func renderBanner(v string, profile termenv.Profile, startColor, endColor colorful.Color) {
	banner := `
   █████████  █████       █████ ██████   ██████ ███████████   █████████  ██████████
  ███░░░░░███░░███       ░░███ ░░██████ ██████ ░░███░░░░░███ ███░░░░░███░░███░░░░░█
 ███     ░░░  ░███        ░███  ░███░█████░███  ░███    ░███░███    ░░░  ░███  █ ░
░███          ░███        ░███  ░███░░███ ░███  ░██████████ ░░█████████  ░██████
░███    █████ ░███        ░███  ░███ ░░░  ░███  ░███░░░░░░   ░░░░░░░░███ ░███░░█
░░███  ░░███  ░███      █ ░███  ░███      ░███  ░███         ███    ░███ ░███ ░   █
 ░░█████████  ███████████ █████ █████     █████ █████       ░░█████████  ██████████
  ░░░░░░░░░  ░░░░░░░░░░░ ░░░░░ ░░░░░     ░░░░░ ░░░░░         ░░░░░░░░░  ░░░░░░░░░░

`
	lines := strings.Split(strings.Trim(banner, "\n"), "\n")

	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	fmt.Println()
	if ansiEnabled {
		for _, line := range lines {
			for x, char := range line {
				ratio := float64(x) / float64(maxWidth)
				gradColor := startColor.BlendLuv(endColor, ratio).Hex()
				out := termenv.String(string(char)).Foreground(profile.Color(gradColor))
				fmt.Print(out)
			}
			fmt.Println()
		}
	} else {
		fmt.Println("GLIMPSE ARCHITECT")
	}
	fmt.Println()

	fmt.Println(ColorBlue + ColorBold + "\n        ✨ GLIMPSE ARCHITECT ✨" + ColorReset)
	//fmt.Println(ColorYellow + "                 " + shortBannerVersion(v) + ColorReset)
	fmt.Println(ColorCyan + "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" + ColorReset)
	fmt.Println(ColorCyan + "  Code Analysis & Slidev Generation" + ColorReset)
	fmt.Println(ColorYellow + "  Version: " + v + ColorReset)
	fmt.Println(ColorGray + ColorDim + "  created by Zafer Kılıçaslan" + ColorReset)
	fmt.Println("  (Abort anytime with " + ColorRed + "Ctrl+C" + ColorReset + ")")
	fmt.Println("")
}

// --- User config prompting ---
func promptUserConfig(line *liner.State, reader *bufio.Reader, workDir string) *config.Config {
	if line != nil {
		line.SetCompleter(func(input string) (c []string) {
			path := strings.TrimSpace(input)
			if path == "" {
				path = "./"
			}
			matches, _ := filepath.Glob(path + "*")
			for _, m := range matches {
				if info, err := os.Stat(m); err == nil && info.IsDir() {
					c = append(c, m+"/")
				} else {
					c = append(c, m)
				}
			}
			return c
		})
	}
	projectPath := ask(line, reader, "Project Path", "../")
	if line != nil {
		line.SetCompleter(nil)
	}

	theme := ask(line, reader, "Slidev Theme", "seriph")
	if err := ensureSlidevTheme(theme, workDir); err != nil {
		fmt.Printf("%s❌ Theme error: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	model := ask(line, reader, "AI Model (gpt-4o, gemini-2.0-flash, claude-3-5-sonnet-latest, local)", "gemini-2.0-flash")
	lang := ask(line, reader, "Language", "de")
	out := ask(line, reader, "File Name", "slides.md")

	localBaseURL := ""
	needsAPIKey := ai.RequiresAPIKey(model)
	if !needsAPIKey {
		localBaseURL = ask(line, reader, "Local LLM URL", defaultLocalLLMURL)
	}

	apiPrompt := "API Key (OpenAI/Gemini/Anthropic)"
	if !needsAPIKey {
		apiPrompt = "API Key (optional for local)"
	}
	apiKey := ask(line, reader, apiPrompt, "")
	if needsAPIKey && apiKey == "" {
		fmt.Println(ColorRed + "\n❌ Error: Cannot continue without an API key." + ColorReset)
		os.Exit(1)
	}

	unsplashURL := os.Getenv("UNSPLASH_IMAGE_URL")
	if unsplashURL == "" {
		unsplashURL = config.DefaultUnsplashBaseURL
	}

	return &config.Config{
		APIKey:          apiKey,
		LocalBaseURL:    localBaseURL,
		Theme:           theme,
		Model:           model,
		Language:        lang,
		Output:          out,
		ProjectPath:     projectPath,
		UnsplashBaseURL: unsplashURL,
	}
}

// --- Code processing ---
func scanAndGenerate(cfg *config.Config, profile termenv.Profile, startColor, endColor colorful.Color) {
	// Resolve the path early so the user sees exactly what will be scanned.
	// Relative paths are resolved from the process CWD (where glimpse was
	// started), not from the binary location — making this visible avoids
	// confusion when the user enters paths like ../../some-project.
	resolved := cfg.ProjectPath
	if strings.HasPrefix(resolved, "~/") || resolved == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			if resolved == "~" {
				resolved = home
			} else {
				resolved = filepath.Join(home, resolved[2:])
			}
		}
	}
	if abs, err := filepath.Abs(resolved); err == nil {
		resolved = abs
	}
	fmt.Printf("\n%s🔍 Scanning files in: %s%s%s\n", ColorBlue, ColorBold, resolved, ColorReset)
	content, err := crawler.CollectProject(cfg.ProjectPath)
	if err != nil {
		fmt.Printf("%s❌ Error scanning project: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	docs, biz, sup := content.Stats()
	total := docs + biz + sup
	if total == 0 {
		fmt.Printf("%s⚠️ No supported files found in: %s%s\n", ColorYellow, resolved, ColorReset)
		return
	}
	fmt.Printf("%s  📄 %d docs, 💼 %d business logic, 🔧 %d support files%s\n",
		ColorGray, docs, biz, sup, ColorReset)

	// Hint: large docs projects benefit from cloud models with stronger summarization.
	if ai.IsLocalModel(cfg.Model) && docs > 20 {
		totalDocBytes := 0
		for _, d := range content.Docs {
			totalDocBytes += len(d.Content)
		}
		if totalDocBytes > 50000 {
			fmt.Printf("%s  💡 Tip: This project has %dKB of docs — a cloud model produces richer slides.%s\n",
				ColorYellow, totalDocBytes/1000, ColorReset)
			fmt.Printf("%s       glimpse -model gemini-2.0-flash    (requires GOOGLE_API_KEY)%s\n", ColorYellow, ColorReset)
			fmt.Printf("%s       glimpse -model claude-3-5-sonnet   (requires ANTHROPIC_API_KEY)%s\n", ColorYellow, ColorReset)
		}
	}

	stopLoader := startFancyLoader("🧠 AI is analyzing code...", profile, startColor, endColor)
	slides, err := ai.GenerateSlides(cfg, content)
	if err != nil {
		stopLoader("🧠 AI analysis stopped.")
		fmt.Printf("%s❌ AI error: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	stopLoader("🧠 AI analysis complete.")

	if err := os.WriteFile(cfg.Output, []byte(slides), 0644); err != nil {
		fmt.Printf("%s❌ Error while saving: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	fmt.Printf("\n%s✅ DONE!%s Your presentation is ready: %s\n\n", ColorGreen+ColorBold, ColorReset, cfg.Output)
}

// launchSlidev starts Slidev directly without prompting (used in non-interactive mode).
func launchSlidev(output, workDir string) {
	absOutput, err := filepath.Abs(output)
	if err != nil {
		fmt.Printf("%s❌ Cannot resolve output path: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	fmt.Printf("%s🚀 Starting Slidev: npx --yes @slidev/cli %s%s\n", ColorCyan, absOutput, ColorReset)
	cmd := exec.Command("npx", "--yes", "@slidev/cli", absOutput)
	cmd.Dir = workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s❌ Failed to start Slidev: %v%s\n", ColorRed, err, ColorReset)
	}
}

// --- Slidev launch ---
func promptAndLaunchSlidev(line *liner.State, reader *bufio.Reader, output, workDir string) {
	runSlidev := strings.ToLower(strings.TrimSpace(ask(line, reader, "Start Slidev now? (Y/n)", "Y")))
	if runSlidev != "y" && runSlidev != "yes" && runSlidev != "" {
		return
	}

	// Resolve output to absolute path so Slidev finds it regardless of cmd.Dir.
	absOutput, err := filepath.Abs(output)
	if err != nil {
		fmt.Printf("%s❌ Cannot resolve output path: %v%s\n", ColorRed, err, ColorReset)
		return
	}

	fmt.Printf("%s🚀 Starting Slidev: npx --yes @slidev/cli %s%s\n", ColorCyan, absOutput, ColorReset)
	cmd := exec.Command("npx", "--yes", "@slidev/cli", absOutput)
	cmd.Dir = workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("%s❌ Failed to start Slidev: %v%s\n", ColorRed, err, ColorReset)
	}
}

// --- CLI Flags ---

var (
	flagVersion  = flag.Bool("version", false, "Print version and exit")
	flagPath     = flag.String("path", "", "Project path to scan")
	flagModel    = flag.String("model", "", "AI model (e.g. gpt-4o, gemini-2.0-flash, local/qwen2.5-coder:7b)")
	flagOutput   = flag.String("output", "", "Output file name (default: slides.md)")
	flagTheme    = flag.String("theme", "", "Slidev theme (default: seriph)")
	flagLang     = flag.String("lang", "", "Presentation language (default: de)")
	flagAPIKey   = flag.String("api-key", "", "API key (or set GLIMPSE_API_KEY env)")
	flagLocalURL = flag.String("local-url", "", "Local LLM base URL (default: http://localhost:8080)")
	flagSlidev   = flag.Bool("slidev", false, "Auto-start Slidev after generation (non-interactive)")
)

// configFromFlags builds a Config from CLI flags. Returns nil if --path is not
// set, signalling that the interactive mode should be used instead.
func configFromFlags() *config.Config {
	if *flagPath == "" {
		return nil
	}

	model := stringDefault(*flagModel, "gemini-2.0-flash")
	theme := stringDefault(*flagTheme, "seriph")
	lang := stringDefault(*flagLang, "de")
	output := stringDefault(*flagOutput, "slides.md")
	localURL := stringDefault(*flagLocalURL, defaultLocalLLMURL)

	apiKey := *flagAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("GLIMPSE_API_KEY")
	}
	if ai.RequiresAPIKey(model) && apiKey == "" {
		fmt.Fprintf(os.Stderr, "%s❌ Error: --api-key required for %s (or set GLIMPSE_API_KEY env)%s\n", ColorRed, model, ColorReset)
		os.Exit(1)
	}

	unsplashURL := os.Getenv("UNSPLASH_IMAGE_URL")
	if unsplashURL == "" {
		unsplashURL = config.DefaultUnsplashBaseURL
	}

	return &config.Config{
		APIKey:          apiKey,
		LocalBaseURL:    localURL,
		Theme:           theme,
		Model:           model,
		Language:        lang,
		Output:          output,
		ProjectPath:     *flagPath,
		UnsplashBaseURL: unsplashURL,
	}
}

func stringDefault(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

// --- Main ---
func main() {
	initTerminalAppearance()
	v := resolvedVersion()

	// Register -v as alias for --version.
	flag.BoolVar(flagVersion, "v", false, "Print version and exit")
	flag.Parse()

	if *flagVersion {
		fmt.Printf("glimpse %s\n", v)
		return
	}

	setupInterruptHandler()

	startColor, _ := colorful.Hex(bannerStartHex)
	endColor, _ := colorful.Hex(bannerEndHex)
	profile := termenv.ColorProfile()

	workDir, err := glimpseWorkDir()
	if err != nil {
		fmt.Printf("%s❌ Error: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}
	if err := initWorkDir(workDir); err != nil {
		fmt.Printf("%s❌ Error: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	// Non-interactive mode: all config from CLI flags.
	if cfg := configFromFlags(); cfg != nil {
		if err := ensureSlidevTheme(cfg.Theme, workDir); err != nil {
			fmt.Printf("%s❌ Theme error: %v%s\n", ColorRed, err, ColorReset)
			os.Exit(1)
		}
		scanAndGenerate(cfg, profile, startColor, endColor)
		if *flagSlidev {
			launchSlidev(cfg.Output, workDir)
		}
		return
	}

	// Interactive mode.
	renderBanner(v, profile, startColor, endColor)

	reader, readerCloser := promptInputReader()
	if readerCloser != nil {
		defer func() { _ = readerCloser.Close() }()
	}
	var line *liner.State
	if supportsLineEditing() {
		line = liner.NewLiner()
		defer func() { _ = line.Close() }()
		line.SetCtrlCAborts(true)
	}

	cfg := promptUserConfig(line, reader, workDir)

	scanAndGenerate(cfg, profile, startColor, endColor)

	promptAndLaunchSlidev(line, reader, cfg.Output, workDir)
}
