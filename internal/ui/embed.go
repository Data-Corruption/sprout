package ui

import (
	"embed"
	"sprout/pkg/asset"
)

//go:embed assets/css/output.css
var cssFS embed.FS

//go:embed assets/js/output.js
var jsFS embed.FS

func LoadEmbedAssets() (*asset.Asset, *asset.Asset, error) {
	var err error
	CSS, err := asset.New(cssFS, "assets/css/output.css", ".css", "text/css; charset=utf-8")
	if err != nil {
		return nil, nil, err
	}
	JS, err := asset.New(jsFS, "assets/js/output.js", ".js", "application/javascript; charset=utf-8")
	if err != nil {
		return nil, nil, err
	}
	return CSS, JS, nil
}
