package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Zafer83/glimpse/internal/ai"
	"github.com/Zafer83/glimpse/internal/config"
	"github.com/Zafer83/glimpse/internal/crawler"
)

func main() {
	// 1. Load configuration from .env and Environment
	cfg := config.LoadConfig()

	// 2. CLI Flags (can override .env settings)
	pathPtr := flag.String("path", "./src", "Path to the source code directory")
	outPtr := flag.String("out", cfg.Output, "Output filename for the slides")
	flag.Parse()

	if cfg.APIKey == "" {
		fmt.Println("❌ Error: OPENAI_API_KEY not found. Please check your .env file.")
		os.Exit(1)
	}

	fmt.Printf("🔍 Glimpse is inspecting: %s\n", *pathPtr)
	fmt.Printf("🎨 Theme: %s | 🤖 Model: %s | 🌍 Lang: %s\n", cfg.Theme, cfg.Model, cfg.Language)

	// 3. Collect code
	code, err := crawler.CollectCode(*pathPtr)
	if err != nil {
		fmt.Printf("❌ Error crawling files: %v\n", err)
		return
	}

	if len(code) == 0 {
		fmt.Println("⚠️ No relevant code files found in the specified directory.")
		return
	}

	// 4. Generate Slides via AI
	fmt.Println("🧠 AI is analyzing your logic and crafting slides...")
	slides, err := ai.GenerateSlides(cfg, code)
	if err != nil {
		fmt.Printf("❌ AI Generation Error: %v\n", err)
		return
	}

	// 5. Save result
	err = os.WriteFile(*outPtr, []byte(slides), 0644)
	if err != nil {
		fmt.Printf("❌ Error saving file: %v\n", err)
		return
	}

	fmt.Printf("✅ Success! Your Slidev presentation is ready: %s\n", *outPtr)
}
