package router

import (
	"embed"
	"sprout/pkg/asset"
)

//go:embed css/output.css
var cssFS embed.FS

//go:embed js/output.js
var jsFS embed.FS

var (
	CSS *asset.Asset
	JS  *asset.Asset
)

func LoadEmbedAssets() error {
	var err error
	CSS, err = asset.New(cssFS, "css/output.css", ".css", "text/css; charset=utf-8")
	if err != nil {
		return err
	}
	JS, err = asset.New(jsFS, "js/output.js", ".js", "application/javascript; charset=utf-8")
	if err != nil {
		return err
	}
	return nil
}
