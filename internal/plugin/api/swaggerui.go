package api

import (
	"bytes"
	"html/template"
	"net/http"
	"path"
)

// SwaggerUIOpts configures the Swaggerui middlewares
type SwaggerUIOpts struct {
	// BasePath for the UI path, defaults to: /
	BasePath string
	// Path combines with BasePath for the full UI path, defaults to: docs
	Path string
	// SpecURL the url to find the spec for
	SpecURL string

	// The three components needed to embed swagger-ui
	SwaggerURL       string
	SwaggerPresetURL string
	SwaggerStylesURL string

	Favicon32 string
	Favicon16 string

	// Title for the documentation site, default to: API documentation
	Title string
}

// EnsureDefaults in case some options are missing
func (r *SwaggerUIOpts) EnsureDefaults() {
	if r.BasePath == "" {
		r.BasePath = "/"
	}
	if r.Path == "" {
		r.Path = "docs"
	}
	if r.SpecURL == "" {
		r.SpecURL = "/openapi.yaml"
	}
	if r.SwaggerURL == "" {
		r.SwaggerURL = swaggerLatest
	}
	if r.SwaggerPresetURL == "" {
		r.SwaggerPresetURL = swaggerPresetLatest
	}
	if r.SwaggerStylesURL == "" {
		r.SwaggerStylesURL = swaggerStylesLatest
	}
	if r.Favicon16 == "" {
		r.Favicon16 = swaggerFavicon16Latest
	}
	if r.Favicon32 == "" {
		r.Favicon32 = swaggerFavicon32Latest
	}
	if r.Title == "" {
		r.Title = "API documentation"
	}
}

func swaggerUIHandler(opts SwaggerUIOpts) http.Handler {
	opts.EnsureDefaults()

	pth := path.Join(opts.BasePath, opts.Path)
	tmpl := template.Must(template.New("swaggerui").Parse(swaggeruiTemplate))

	buf := bytes.NewBuffer(nil)
	_ = tmpl.Execute(buf, &opts)
	b := buf.Bytes()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path.Join(r.URL.Path) == pth {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)

			_, _ = w.Write(b)
			return
		}
	})
}

const (
	swaggerLatest          = "https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"
	swaggerPresetLatest    = "https://unpkg.com/swagger-ui-dist/swagger-ui-standalone-preset.js"
	swaggerStylesLatest    = "https://unpkg.com/swagger-ui-dist/swagger-ui.css"
	swaggerFavicon32Latest = "https://unpkg.com/swagger-ui-dist/favicon-32x32.png"
	swaggerFavicon16Latest = "https://unpkg.com/swagger-ui-dist/favicon-16x16.png"
	swaggeruiTemplate      = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8">
		<title>{{ .Title }}</title>
    <link rel="stylesheet" type="text/css" href="{{ .SwaggerStylesURL }}" >
    <link rel="icon" type="image/png" href="{{ .Favicon32 }}" sizes="32x32" />
    <link rel="icon" type="image/png" href="{{ .Favicon16 }}" sizes="16x16" />
    <style>
      html
      {
        box-sizing: border-box;
        overflow: -moz-scrollbars-vertical;
        overflow-y: scroll;
      }
      *,
      *:before,
      *:after
      {
        box-sizing: inherit;
      }
      body
      {
        margin:0;
        background: #fafafa;
      }
    </style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="{{ .SwaggerURL }}"> </script>
    <script src="{{ .SwaggerPresetURL }}"> </script>
    <script>
    window.onload = function() {
      // Begin Swagger UI call region
      const ui = SwaggerUIBundle({
        url: '{{ .SpecURL }}',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout",
      })
      // End Swagger UI call region
      window.ui = ui
    }
  </script>
  </body>
</html>
`
)
