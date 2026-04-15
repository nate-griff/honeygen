package deployments

import (
	"path/filepath"
	"testing"
)

func TestResolveFilePathStripsStoredGeneratedPrefix(t *testing.T) {
	manager := &Manager{
		storageRoot: filepath.Join("app", "storage"),
	}

	testCases := []struct {
		name     string
		rootPath string
	}{
		{name: "stored relative path", rootPath: "generated/wm_123/job_456/public"},
		{name: "leading slash path", rootPath: "/generated/wm_123/job_456/public"},
	}

	want := filepath.ToSlash(filepath.Join("app", "storage", "generated", "wm_123", "job_456", "public"))
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := filepath.ToSlash(manager.resolveFilePath(Deployment{
				WorldModelID:    "wm_123",
				GenerationJobID: "job_456",
				RootPath:        tc.rootPath,
			}))

			if got != want {
				t.Fatalf("resolveFilePath() = %q, want %q", got, want)
			}
		})
	}
}
