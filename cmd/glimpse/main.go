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
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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

func normalizeTripletVersion(raw string) (string, bool) {
	m := semverTripletRe.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) != 4 {
		return "", false
	}
	return fmt.Sprintf("%s.%s.%s", m[1], m[2], m[3]), true
}

func resolvedVersion() string {
	if v, ok := normalizeTripletVersion(version); ok {
		if v != "0.0.0" {
			return v
		}
	}
	if v, ok := autoVersionFromGit(); ok {
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

func initTerminalAppearance() {
	if forcePlainMode == "1" {
		ansiEnabled = false
	}
	if os.Getenv("NO_COLOR") != "" {
		ansiEnabled = false
	}
	// Whisky/Wine and some Windows shells do not interpret ANSI reliably.
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

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func autoVersionFromGit() (string, bool) {
	// Versioning strategy:
	// - major: fixed to 0 for now
	// - minor: number of feat commits
	// - build: total commit count
	buildRaw, err := gitOutput("rev-list", "--count", "HEAD")
	if err != nil {
		return "", false
	}
	featRaw, err := gitOutput("log", "--pretty=%s")
	if err != nil {
		return "", false
	}

	build, err := strconv.Atoi(buildRaw)
	if err != nil {
		return "", false
	}

	featCount := 0
	for _, line := range strings.Split(featRaw, "\n") {
		msg := strings.TrimSpace(strings.ToLower(line))
		if strings.HasPrefix(msg, "feat:") || strings.HasPrefix(msg, "feat(") {
			featCount++
		}
	}

	return fmt.Sprintf("0.%d.%d", featCount, build), true
}

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
	// Some embedded IDE terminals report as TTY but can still break readline UIs.
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

	// Auto-correct mixed form like "@slidev/theme-slidev-theme-dracula".
	if strings.HasPrefix(t, "@slidev/theme-slidev-theme-") {
		trimmed := strings.TrimPrefix(t, "@slidev/theme-")
		return uniqueStrings([]string{trimmed, "@slidev/theme-" + strings.TrimPrefix(trimmed, "slidev-theme-")}), false
	}

	// Full package input (official/community/custom).
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

	// Short name input: try official first, then common community naming.
	return uniqueStrings([]string{
		"@slidev/theme-" + t,
		"slidev-theme-" + t,
	}), false
}

func npmPackageExists(pkg string) bool {
	cmd := exec.Command("npm", "view", pkg, "name", "--silent")
	return cmd.Run() == nil
}

func npmPackageInstalled(pkg string) bool {
	parts := strings.Split(pkg, "/")
	path := filepath.Join(append([]string{"node_modules"}, parts...)...)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return true
	}
	return false
}

func ensureSlidevTheme(theme string) error {
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
	if npmPackageInstalled(pkg) {
		return nil
	}

	fmt.Printf("%s📦 Installing Slidev theme package: %s%s\n", ColorCyan, pkg, ColorReset)
	cmd := exec.Command("npm", "install", "-D", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install theme package %s: %w", pkg, err)
	}
	return nil
}

func startFancyLoader(message string, profile termenv.Profile, startColor, endColor colorful.Color) func(doneText string) {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		if !isTTY || !ansiEnabled {
			fmt.Printf("%s%s%s\n", ColorMagenta, message, ColorReset)
			start := time.Now()
			ticker := time.NewTicker(300 * time.Millisecond)
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
		ticker := time.NewTicker(110 * time.Millisecond)
		defer ticker.Stop()

		render := func() {
			elapsed := time.Since(start)
			sec := elapsed.Seconds()

			// Monotonic estimated curve: fast start, then smooth slowdown.
			displayProgress := 8.0 + 89.0*(1.0-1.0/(1.0+sec/10.0))
			if displayProgress > 97.0 {
				displayProgress = 97.0
			}

			barWidth := 26
			filled := int(displayProgress / 100.0 * float64(barWidth))
			if filled < 0 {
				filled = 0
			}
			if filled > barWidth {
				filled = barWidth
			}
			var bar strings.Builder
			denominator := barWidth - 1
			if denominator <= 0 {
				denominator = 1
			}
			for i := 0; i < barWidth; i++ {
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
		finalBar := strings.Repeat("█", 26)
		if ansiEnabled {
			fmt.Printf("\r\033[2K%sProgress%s [%s] 100.0%% | ETA 0s | %s\n", ColorBlue, ColorReset, finalBar, doneText)
		} else {
			fmt.Printf("\rProgress: 100.0%% | ETA 0s | %s\n", doneText)
		}
		fmt.Printf("%s%s%s\n", ColorGreen, doneText, ColorReset)
	}
}

func main() {
	initTerminalAppearance()
	v := resolvedVersion()

	if len(os.Args) > 1 {
		arg := strings.ToLower(strings.TrimSpace(os.Args[1]))
		if arg == "--version" || arg == "-v" || arg == "version" {
			fmt.Printf("glimpse %s\n", v)
			return
		}
	}

	setupInterruptHandler()

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
	// "Cyberpunk Night"
	startColor, _ := colorful.Hex("#00f2fe")
	endColor, _ := colorful.Hex("#7117ea")

	p := termenv.ColorProfile()

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
				out := termenv.String(string(char)).Foreground(p.Color(gradColor))
				fmt.Print(out)
			}
			fmt.Println()
		}
	} else {
		fmt.Println("GLIMPSE ARCHITECT")
	}
	fmt.Println()

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

	fmt.Println(ColorBlue + ColorBold + "\n        ✨ GLIMPSE ARCHITECT ✨" + ColorReset)
	fmt.Println(ColorYellow + "                 " + shortBannerVersion(v) + ColorReset)
	fmt.Println(ColorCyan + "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" + ColorReset)
	fmt.Println(ColorCyan + "  Code Analysis & Slidev Generation" + ColorReset)
	fmt.Println(ColorYellow + "  Version: " + v + ColorReset)
	fmt.Println(ColorGray + ColorDim + "  created by Zafer Kılıçaslan" + ColorReset)
	fmt.Println("  (Abort anytime with " + ColorRed + "Ctrl+C" + ColorReset + ")")
	fmt.Println("")

	if line != nil {
		// Enable path completion only for the project path input.
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
	if err := ensureSlidevTheme(theme); err != nil {
		fmt.Printf("%s❌ Theme error: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}
	model := ask(line, reader, "AI Model (gpt-4o, gemini-2.0-flash, claude-3-5-sonnet-latest, local)", "gemini-2.0-flash")
	lang := ask(line, reader, "Language", "de")
	out := ask(line, reader, "File Name", "slides.md")

	localBaseURL := ""
	needsAPIKey := ai.RequiresAPIKey(model)
	if !needsAPIKey {
		localBaseURL = ask(line, reader, "Local LLM URL", "http://localhost:8080")
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

	cfg := &config.Config{
		APIKey: apiKey, LocalBaseURL: localBaseURL, Theme: theme, Model: model, Language: lang, Output: out,
	}

	// Scan code first, then hand one consolidated prompt to the selected model provider.
	fmt.Printf("\n%s🔍 Scanning files in: %s%s%s\n", ColorBlue, ColorBold, projectPath, ColorReset)
	code, err := crawler.CollectCode(projectPath)
	if err != nil || len(code) == 0 {
		fmt.Printf("%s⚠️ No files found.%s\n", ColorYellow, ColorReset)
		return
	}

	fmt.Printf("%s🧠 AI is analyzing code...%s\n", ColorMagenta, ColorReset)
	stopLoader := startFancyLoader("🧠 AI is analyzing code...", p, startColor, endColor)
	slides, err := ai.GenerateSlides(cfg, code)
	if err != nil {
		stopLoader("🧠 AI analysis stopped.")
		fmt.Printf("%s❌ AI error: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	stopLoader("🧠 AI analysis complete.")

	err = os.WriteFile(cfg.Output, []byte(slides), 0644)
	if err != nil {
		fmt.Printf("%s❌ Error while saving: %v%s\n", ColorRed, err, ColorReset)
		return
	}
	fmt.Printf("\n%s✅ DONE!%s Your presentation is ready: %s\n\n", ColorGreen+ColorBold, ColorReset, cfg.Output)

	runSlidev := strings.ToLower(strings.TrimSpace(ask(line, reader, "Start Slidev now? (Y/n)", "Y")))
	if runSlidev == "y" || runSlidev == "yes" || runSlidev == "" {
		fmt.Printf("%s🚀 Starting Slidev: npx --yes @slidev/cli %s%s\n", ColorCyan, cfg.Output, ColorReset)
		cmd := exec.Command("npx", "--yes", "@slidev/cli", cfg.Output)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%s⚠️ @slidev/cli failed, trying legacy command...%s\n", ColorYellow, ColorReset)
			legacy := exec.Command("npx", "slidev", cfg.Output)
			legacy.Stdin = os.Stdin
			legacy.Stdout = os.Stdout
			legacy.Stderr = os.Stderr
			if legacyErr := legacy.Run(); legacyErr != nil {
				fmt.Printf("%s❌ Failed to start Slidev: %v%s\n", ColorRed, legacyErr, ColorReset)
			}
		}
	}
}
