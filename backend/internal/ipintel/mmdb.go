package ipintel

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	mmdbDownloadURL      = "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz"
	mmdbRefreshInterval  = 7 * 24 * time.Hour
)

// Updater downloads and refreshes the GeoLite2-City MMDB file under app storage.
// It degrades gracefully when credentials are missing or downloads fail.
type Updater struct {
	accountID  string
	licenseKey string
	dbPath     string
	logger     *slog.Logger
	httpClient *http.Client
}

// NewUpdater creates an Updater. If accountID or licenseKey is empty,
// EnsureFresh will log a warning and skip the download.
func NewUpdater(accountID, licenseKey, dbPath string, logger *slog.Logger) *Updater {
	return &Updater{
		accountID:  accountID,
		licenseKey: licenseKey,
		dbPath:     dbPath,
		logger:     logger,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// EnsureFresh downloads the MMDB if it is absent or older than the refresh
// interval. Returns nil when the file is already fresh. Errors are logged and
// returned so the caller can decide whether to proceed without GeoIP support.
func (u *Updater) EnsureFresh(ctx context.Context) error {
	if u.accountID == "" || u.licenseKey == "" {
		if u.logger != nil {
			u.logger.Warn("MaxMind credentials not configured; GeoIP enrichment unavailable",
				"hint", "set MM_ACCOUNT_ID and MM_LICENSE_KEY to enable")
		}
		return fmt.Errorf("maxmind credentials not configured")
	}

	if u.dbPath == "" {
		return fmt.Errorf("maxmind db path not configured")
	}

	// Check if existing file is fresh enough.
	if info, err := os.Stat(u.dbPath); err == nil {
		if time.Since(info.ModTime()) < mmdbRefreshInterval {
			return nil
		}
	}

	if u.logger != nil {
		u.logger.Info("downloading GeoLite2-City MMDB", "path", u.dbPath)
	}

	if err := os.MkdirAll(filepath.Dir(u.dbPath), 0o755); err != nil {
		return fmt.Errorf("create geoip directory: %w", err)
	}

	if err := u.download(ctx); err != nil {
		if u.logger != nil {
			u.logger.Warn("failed to download GeoLite2-City MMDB; GeoIP enrichment unavailable",
				"error", err)
		}
		return fmt.Errorf("download GeoLite2-City MMDB: %w", err)
	}

	if u.logger != nil {
		u.logger.Info("GeoLite2-City MMDB updated", "path", u.dbPath)
	}
	return nil
}

func (u *Updater) download(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mmdbDownloadURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(u.accountID, u.licenseKey)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from MaxMind download", resp.StatusCode)
	}

	return u.extractMMDB(resp.Body)
}

func (u *Updater) extractMMDB(r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasSuffix(hdr.Name, ".mmdb") {
			continue
		}

		tmp := u.dbPath + ".tmp"
		f, err := os.Create(tmp)
		if err != nil {
			return fmt.Errorf("create temp mmdb file: %w", err)
		}

		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("write mmdb file: %w", err)
		}
		f.Close()

		if err := os.Rename(tmp, u.dbPath); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("install mmdb file: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no .mmdb file found in MaxMind download archive")
}
