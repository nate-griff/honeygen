package worldmodels

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var demoSeedRelativePath = filepath.Join("sample-data", "world-models", DemoWorldModelID+".json")

func loadDemoSeed() ([]byte, error) {
	for _, path := range demoSeedCandidatePaths() {
		contents, err := os.ReadFile(path)
		if err == nil {
			return contents, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read demo seed %q: %w", path, err)
		}
	}

	return nil, fmt.Errorf("demo seed %q not found", demoSeedRelativePath)
}

func demoSeedCandidatePaths() []string {
	paths := []string{
		filepath.Join(".", demoSeedRelativePath),
		filepath.Join("..", demoSeedRelativePath),
	}

	if executablePath, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(executablePath), demoSeedRelativePath))
	}

	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		paths = append(paths, filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", demoSeedRelativePath))
	}

	return uniquePaths(paths)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		unique = append(unique, cleaned)
	}
	return unique
}
