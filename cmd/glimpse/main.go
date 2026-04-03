package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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

const (
	ColorBlue    = "\033[34m"
	ColorCyan    = "\033[36m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorRed     = "\033[31m"
	ColorMagenta = "\033[35m"
	ColorReset   = "\033[0m"
	ColorBold    = "\033[1m"
)

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

func startFancyLoader(message string) func(doneText string) {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		frames := []string{"▰▱▱▱▱▱▱▱▱▱", "▰▰▱▱▱▱▱▱▱▱", "▰▰▰▱▱▱▱▱▱▱", "▰▰▰▰▱▱▱▱▱▱", "▰▰▰▰▰▱▱▱▱▱", "▰▰▰▰▰▰▱▱▱▱", "▰▰▰▰▰▰▰▱▱▱", "▰▰▰▰▰▰▰▰▱▱", "▰▰▰▰▰▰▰▰▰▱", "▰▰▰▰▰▰▰▰▰▰"}
		colors := []string{ColorCyan, ColorBlue, ColorMagenta}
		i := 0
		start := time.Now()
		ticker := time.NewTicker(110 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				frame := frames[i%len(frames)]
				color := colors[i%len(colors)]
				elapsed := time.Since(start).Round(100 * time.Millisecond)
				fmt.Printf("\r%s%s %s%s %s[%s]%s %s(%s)%s", ColorBold, message, color, frame, ColorYellow, "working", ColorReset, ColorCyan, elapsed, ColorReset)
				i++
			}
		}
	}()

	return func(doneText string) {
		close(stop)
		<-done
		fmt.Printf("\r%s%s%s %s\n", ColorGreen, doneText, ColorReset, strings.Repeat(" ", 24))
	}
}

func main() {
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
	//startColor, _ := colorful.Hex("#4facfe")
	//endColor, _ := colorful.Hex("#ee0979")

	p := termenv.ColorProfile()

	lines := strings.Split(strings.Trim(banner, "\n"), "\n")

	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	fmt.Println()
	for _, line := range lines {
		for x, char := range line {
			ratio := float64(x) / float64(maxWidth)
			gradColor := startColor.BlendLuv(endColor, ratio).Hex()
			out := termenv.String(string(char)).Foreground(p.Color(gradColor))
			fmt.Print(out)
		}
		fmt.Println()
	}
	fmt.Println()

	reader, readerCloser := promptInputReader()
	if readerCloser != nil {
		defer func(readerCloser io.Closer) {
			err := readerCloser.Close()
			if err != nil {

			}
		}(readerCloser)
	}
	var line *liner.State
	if supportsLineEditing() {
		line = liner.NewLiner()
		defer func(line *liner.State) {
			err := line.Close()
			if err != nil {

			}
		}(line)
		line.SetCtrlCAborts(true)
	}

	fmt.Println(ColorBlue + ColorBold + "\n        ✨ GLIMPSE ARCHITECT ✨" + ColorReset)
	fmt.Println(ColorCyan + "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" + ColorReset)
	fmt.Println(ColorCyan + "  Code Analysis & Slidev Generation" + ColorReset)
	fmt.Println("  (Abort anytime with " + ColorRed + "Ctrl+C" + ColorReset + ")")
	fmt.Println("")

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
	model := ask(line, reader, "AI Model (gpt-4o or gemini-1.5-pro)", "gemini-1.5-pro")
	lang := ask(line, reader, "Language", "de")
	out := ask(line, reader, "File Name", "slides.md")

	apiKey := ask(line, reader, "API Key (OpenAI/Gemini)", "")
	if apiKey == "" {
		fmt.Println(ColorRed + "\n❌ Error: Cannot continue without an API key." + ColorReset)
		os.Exit(1)
	}

	cfg := &config.Config{
		APIKey: apiKey, Theme: theme, Model: model, Language: lang, Output: out,
	}

	fmt.Printf("\n%s🔍 Scanning files in: %s%s%s\n", ColorBlue, ColorBold, projectPath, ColorReset)
	code, err := crawler.CollectCode(projectPath)
	if err != nil || len(code) == 0 {
		fmt.Printf("%s⚠️ No files found.%s\n", ColorYellow, ColorReset)
		return
	}

	stopLoader := startFancyLoader("🧠 AI is analyzing code...")
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
}
