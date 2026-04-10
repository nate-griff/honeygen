package decoy

import (
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/natet/honeygen/backend/internal/config"
)

func NewHandler(cfg config.Config, logger *slog.Logger) (http.Handler, error) {
	if strings.TrimSpace(cfg.GeneratedAssetsDir) == "" {
		return nil, fmt.Errorf("generated assets directory is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/generated/", http.StripPrefix("/generated/", http.FileServer(http.Dir(cfg.GeneratedAssetsDir))))
	mux.HandleFunc("/", landingHandler(cfg.GeneratedAssetsDir))

	return LoggingMiddleware(mux, newHTTPEventRecorder(cfg.InternalAPIBaseURL, nil), logger), nil
}

func landingHandler(generatedDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		links := discoverLandingLinks(generatedDir, 8)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		var builder strings.Builder
		builder.WriteString("<!doctype html><html><head><title>Decoy Web</title></head><body>")
		builder.WriteString("<h1>Decoy web service</h1>")
		builder.WriteString("<p>Generated assets are served from <code>/generated/</code>.</p>")
		if len(links) == 0 {
			builder.WriteString("<p>No generated files are available yet.</p>")
		} else {
			builder.WriteString("<p>Sample generated files:</p><ul>")
			for _, link := range links {
				builder.WriteString(`<li><a href="`)
				builder.WriteString(html.EscapeString(link))
				builder.WriteString(`">`)
				builder.WriteString(html.EscapeString(link))
				builder.WriteString("</a></li>")
			}
			builder.WriteString("</ul>")
		}
		builder.WriteString("</body></html>")

		_, _ = w.Write([]byte(builder.String()))
	}
}

func discoverLandingLinks(root string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	links := make([]string, 0, limit)
	_ = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		relative, relErr := filepath.Rel(root, current)
		if relErr != nil {
			return nil
		}
		links = append(links, "/generated/"+path.Clean(filepath.ToSlash(relative)))
		if len(links) >= limit {
			return fs.SkipAll
		}
		return nil
	})

	sort.Strings(links)
	return links
}
