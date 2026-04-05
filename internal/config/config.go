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

package config

// DefaultUnsplashBaseURL is the fallback Unsplash URL template used when
// UNSPLASH_IMAGE_URL is not set in the environment.
// {keywords} is replaced with comma-separated keywords chosen by the AI.
const DefaultUnsplashBaseURL = "https://source.unsplash.com/1920x1080/?{keywords}"

// Config holds runtime options collected from the interactive CLI.
type Config struct {
	APIKey          string
	LocalBaseURL    string
	Theme           string
	Model           string
	Language        string
	Output          string
	ProjectPath     string
	UnsplashBaseURL string
}
