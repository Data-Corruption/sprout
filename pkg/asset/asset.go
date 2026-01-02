// Package asset provides cache-busted static asset serving.
package asset

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
)

// Asset represents an embedded static asset with cache busting.
type Asset struct {
	path        string // cache-busted path, e.g. "/output.abcd1234.css"
	data        []byte
	contentType string
}

// New creates an Asset from an embedded filesystem.
// The filename is the name of the file in the embed.FS (e.g. "output.css").
// The extension is what appears in the URL (e.g. ".css").
// The contentType is the MIME type for the Content-Type header.
func New(fs embed.FS, filename, extension, contentType string) (*Asset, error) {
	data, err := fs.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:8])

	// Build path like "/output.abcd1234.css"
	// Strip extension from filename to get base name
	baseName := filename
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			baseName = filename[:i]
			break
		}
	}

	return &Asset{
		path:        "/" + baseName + "." + hash + extension,
		data:        data,
		contentType: contentType,
	}, nil
}

// Path returns the cache-busted URL path for this asset.
func (a *Asset) Path() string {
	return a.path
}

// Data returns the raw bytes of this asset.
func (a *Asset) Data() []byte {
	return a.data
}

// Handler returns an http.HandlerFunc that serves this asset with
// appropriate caching headers (1 year, immutable).
func (a *Asset) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", a.contentType)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Write(a.data)
	}
}
