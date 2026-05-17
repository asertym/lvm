package manager

import (
	"os"
	"path/filepath"
)

// legacyNames maps canonical modern binary names to their older equivalents.
// llama.cpp renamed its binaries around build b2900 (late 2024):
//
//	main       → llama-cli
//	server     → llama-server
//	quantize   → llama-quantize
//	embedding  → llama-embedding
//	perplexity → llama-perplexity
//	imatrix    → llama-imatrix
//	simple     → llama-simple
var legacyNames = map[string][]string{
	"llama-cli":        {"main"},
	"llama-server":     {"server"},
	"llama-quantize":   {"quantize"},
	"llama-embedding":  {"embedding"},
	"llama-perplexity": {"perplexity"},
	"llama-tokenize":   {"tokenize"},
	"llama-bench":      {"llama-bench"},
	"llama-run":        {"llama-run"},
	"llama-imatrix":    {"imatrix"},
	"llama-simple":     {"simple"},
}

// ResolveAliases returns a canonical-name → real-filename map for the given
// version directory. It probes the filesystem to find which names actually
// exist, preferring the modern name and falling back to legacy names.
// binaryExt should be "" on Unix or ".exe" on Windows.
func ResolveAliases(versionDir string, binaryExt string) map[string]string {
	aliases := make(map[string]string)

	for canonical, fallbacks := range legacyNames {
		// Try the modern canonical name first.
		if fileExistsInDir(versionDir, canonical+binaryExt) {
			aliases[canonical] = canonical + binaryExt
			continue
		}
		// Try each legacy fallback name.
		found := false
		for _, legacy := range fallbacks {
			if fileExistsInDir(versionDir, legacy+binaryExt) {
				aliases[canonical] = legacy + binaryExt
				found = true
				break
			}
		}
		// If nothing matched, default to the canonical name — the shim will
		// surface a clear "binary not found" error at runtime.
		if !found {
			aliases[canonical] = canonical + binaryExt
		}
	}

	return aliases
}

func fileExistsInDir(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
