// Package docs embeds the Swagger 2.0 spec files (swagger.json, swagger.yaml)
// and the swagger-ui dist for in-binary static serving.
package docs

import (
	"bytes"
	_ "embed" // required for //go:embed directives
	"html/template"
	"io"
	"net/http"
	"path"
)

// Spec files (always present).

//go:embed swagger.json
var swaggerJSON []byte

//go:embed swagger.yaml
var swaggerYAML []byte

// swagger-ui dist (populated by `make download-swagger-ui`).

//go:embed swagger-ui/index.html
var swaggerUIHTML string

//go:embed swagger-ui/swagger-initializer.js
var swaggerUIJS string

//go:embed swagger-ui/index.css
var swaggerUICSS []byte

//go:embed swagger-ui/swagger-ui.css
var swaggerUICSS2 []byte

//go:embed swagger-ui/swagger-ui-bundle.js
var swaggerUIBundleJS []byte

//go:embed swagger-ui/swagger-ui-standalone-preset.js
var swaggerUIStandaloneJS []byte

//go:embed swagger-ui/swagger-ui.js
var swaggerUIJSMain []byte

//go:embed swagger-ui/swagger-ui-es-bundle-core.js
var swaggerUICoreJS []byte

//go:embed swagger-ui/swagger-ui-es-bundle.js
var swaggerUIBundleESJS []byte

//go:embed swagger-ui/oauth2-redirect.html
var oauth2Redirect []byte

//go:embed swagger-ui/favicon-16x16.png
var favicon16 []byte

//go:embed swagger-ui/favicon-32x32.png
var favicon32 []byte

// GetSwaggerJSON returns the raw swagger.json bytes.
func GetSwaggerJSON() []byte { return swaggerJSON }

// GetSwaggerYAML returns the raw swagger.yaml bytes.
func GetSwaggerYAML() []byte { return swaggerYAML }

var indexTmpl = template.Must(template.New("index.html").Parse(swaggerUIHTML))

// EmbeddedSwaggerUI returns an http.Handler that serves the swagger-ui
// with the spec URL injected at runtime from the incoming request.
func EmbeddedSwaggerUI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		if p == "/" || p == "/index.html" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			base := scheme + "://" + r.Host
			data := struct {
				SwiftURL   string
				SwaggerURL string
				BaseURL    string
			}{
				BaseURL:    base + "/swagger",
				SwiftURL:   base + "/swagger/doc.yaml",
				SwaggerURL: base + "/swagger/doc.json",
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			indexTmpl.Execute(w, data)
			return
		}

		if err := serveAsset(w, p); err != nil {
			http.NotFound(w, r)
		}
	})
}

// serveAsset writes the embedded file for the given URL path.
// It detects content-type from the extension.
func serveAsset(w http.ResponseWriter, p string) error {
	var (
		data        []byte
		contentType string
	)
	switch p {
	case "/doc.json":
		data, contentType = swaggerJSON, "application/json; charset=utf-8"
	case "/doc.yaml":
		data, contentType = swaggerYAML, "application/yaml; charset=utf-8"
	case "/oauth2-redirect.html":
		data, contentType = oauth2Redirect, "text/html; charset=utf-8"
	case "/swagger-initializer.js":
		// Inject spec URL into the JS at runtime.
		scheme := "http"
		data, contentType = []byte("window.SWAGGER_BASE_URL = '/swagger';\n"+swaggerUIJS),
			"application/javascript; charset=utf-8"
		_ = scheme // kept for future use if we need absolute URLs
	case "/index.css":
		data, contentType = swaggerUICSS, "text/css; charset=utf-8"
	case "/swagger-ui.css":
		data, contentType = swaggerUICSS2, "text/css; charset=utf-8"
	case "/swagger-ui-bundle.js":
		data, contentType = swaggerUIBundleJS, "application/javascript; charset=utf-8"
	case "/swagger-ui-standalone-preset.js":
		data, contentType = swaggerUIStandaloneJS, "application/javascript; charset=utf-8"
	case "/swagger-ui.js":
		data, contentType = swaggerUIJSMain, "application/javascript; charset=utf-8"
	case "/swagger-ui-es-bundle-core.js":
		data, contentType = swaggerUICoreJS, "application/javascript; charset=utf-8"
	case "/swagger-ui-es-bundle.js":
		data, contentType = swaggerUIBundleESJS, "application/javascript; charset=utf-8"
	case "/favicon-16x16.png":
		data, contentType = favicon16, "image/png"
	case "/favicon-32x32.png":
		data, contentType = favicon32, "image/png"
	default:
		return http.ErrMissingFile
	}
	w.Header().Set("Content-Type", contentType)
	io.Copy(w, bytes.NewReader(data))
	return nil
}
