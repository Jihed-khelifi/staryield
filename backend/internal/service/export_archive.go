package service

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Jihed-khelifi/staryield/backend/internal/i18n"
)

// ExportFormat versions the archive layout.
const ExportFormat = "staryield-export/1"

// ExportDocument is one JSON file of a personal-data export.
type ExportDocument struct {
	Path string
	Data any
}

// ExportBlob is one binary file (an image) of a personal-data export.
type ExportBlob struct {
	Path string
	Data []byte
}

// ExportBundle is everything an export archive contains.
type ExportBundle struct {
	UserID      uuid.UUID
	Locale      string
	GeneratedAt time.Time
	Documents   []ExportDocument
	Blobs       []ExportBlob
	// Omitted explains anything left out because of size limits.
	Omitted []string
}

// exportManifestFile describes one file of the archive.
type exportManifestFile struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type exportManifest struct {
	Format      string               `json:"format"`
	UserID      uuid.UUID            `json:"user_id"`
	GeneratedAt time.Time            `json:"generated_at"`
	Files       []exportManifestFile `json:"files"`
	Omitted     []string             `json:"omitted"`
	Excluded    []string             `json:"excluded"`
}

// exportExcluded documents what an export deliberately leaves out.
var exportExcluded = []string{
	"Notes that advisors wrote about you: they are the advisors' working notes, not data you provided.",
	"Security secrets such as your password hash, sign-in tokens and encrypted payout details.",
}

func validArchivePath(p string) bool {
	if p == "" || len(p) > 200 || strings.HasPrefix(p, "/") || strings.Contains(p, "..") || strings.Contains(p, "\\") {
		return false
	}
	for _, r := range p {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/-_.", r)) {
			return false
		}
	}
	return true
}

// exportReadme is the human-readable guide at the root of the archive.
func exportReadme(b *ExportBundle) string {
	t := func(s string) string { return i18n.T(b.Locale, s) }
	lines := []string{
		t("This archive contains a copy of the personal data of your Staryield account."),
		t(fmt.Sprintf("It was generated on %s (UTC).", b.GeneratedAt.UTC().Format("2006-01-02 15:04"))),
		"",
		t("Each JSON file covers one part of your account; manifest.json lists every file with its checksum."),
		t("Images you shared in chats are in the chat/attachments folder."),
		"",
		t("Not included:"),
	}
	for _, e := range exportExcluded {
		lines = append(lines, "- "+t(e))
	}
	for _, o := range b.Omitted {
		lines = append(lines, "- "+t(o))
	}
	return strings.Join(lines, "\n") + "\n"
}

// WriteExportArchive writes the bundle as a ZIP archive: README.txt, the
// JSON documents, the binary files and manifest.json (path, size and
// SHA-256 of every other file). Paths are fixed by the collector and
// validated here; duplicates are rejected.
func WriteExportArchive(w io.Writer, b *ExportBundle) error {
	zw := zip.NewWriter(w)
	seen := map[string]bool{}
	var files []exportManifestFile
	add := func(path string, data []byte, compress bool) error {
		if !validArchivePath(path) || seen[path] {
			return fmt.Errorf("export archive: invalid or duplicate path %q", path)
		}
		seen[path] = true
		method := zip.Deflate
		if !compress {
			method = zip.Store
		}
		f, err := zw.CreateHeader(&zip.FileHeader{Name: path, Method: method, Modified: b.GeneratedAt.UTC()})
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if path != "manifest.json" {
			files = append(files, exportManifestFile{Path: path, Bytes: len(data), SHA256: hex.EncodeToString(sum[:])})
		}
		return nil
	}
	if err := add("README.txt", []byte(exportReadme(b)), true); err != nil {
		return err
	}
	for _, d := range b.Documents {
		if !strings.HasSuffix(d.Path, ".json") {
			return fmt.Errorf("export archive: %q is not a JSON path", d.Path)
		}
		data, err := json.MarshalIndent(d.Data, "", "  ")
		if err != nil {
			return fmt.Errorf("export archive: encode %s: %w", d.Path, err)
		}
		if err := add(d.Path, append(data, '\n'), true); err != nil {
			return err
		}
	}
	for _, blob := range b.Blobs {
		// Images are already compressed.
		if err := add(blob.Path, blob.Data, false); err != nil {
			return err
		}
	}
	excluded := make([]string, len(exportExcluded))
	for i, e := range exportExcluded {
		excluded[i] = i18n.T(b.Locale, e)
	}
	omitted := b.Omitted
	if omitted == nil {
		omitted = []string{}
	}
	manifest, err := json.MarshalIndent(exportManifest{Format: ExportFormat, UserID: b.UserID, GeneratedAt: b.GeneratedAt.UTC(),
		Files: files, Omitted: omitted, Excluded: excluded}, "", "  ")
	if err != nil {
		return err
	}
	if err := add("manifest.json", append(manifest, '\n'), true); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return errors.Join(errors.New("export archive: close"), err)
	}
	return nil
}
