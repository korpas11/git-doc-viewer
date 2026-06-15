// mdview serves all markdown files in a directory tree as HTML, styled with a
// configurable Bootswatch (https://bootswatch.com/) theme.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

const (
	bootstrapJS     = "https://cdn.jsdelivr.net/npm/bootstrap@5.3.8/dist/js/bootstrap.bundle.min.js"
	bootswatchVer   = "5.3.8" // used only for the offline fallback theme list
	bootswatchAPI   = "https://bootswatch.com/api/5.json"
	themeCookieName = "mdview-theme"
)

// fallbackThemes is used when the Bootswatch API can't be reached at startup.
var fallbackThemes = []string{
	"cerulean", "cosmo", "cyborg", "darkly", "flatly", "journal", "litera",
	"lumen", "lux", "materia", "minty", "morph", "pulse", "quartz", "sandstone",
	"simplex", "sketchy", "slate", "solar", "spacelab", "superhero", "united",
	"vapor", "yeti", "zephyr", "brite",
}

// --- globals configured in main ---
var (
	baseDir      string
	defaultTheme string
	themeNames   []string
	themeURLs    map[string]string
	md           goldmark.Markdown
	tmpl         = template.Must(template.New("layout").Parse(layoutHTML))
)

type bswTheme struct {
	Name   string `json:"name"`
	CSSCdn string `json:"cssCdn"`
}

type bswAPI struct {
	Version string     `json:"version"`
	Themes  []bswTheme `json:"themes"`
}

type pageData struct {
	Title           string
	Current         string
	Content         template.HTML
	BootstrapJS     string
	ThemeNames      []string
	ThemesJS        template.JS
	DefaultTheme    string
	DefaultThemeURL string
}

func main() {
	dir := flag.String("dir", ".", "directory to scan for markdown files")
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	theme := flag.String("theme", "flatly", "default Bootswatch theme (e.g. flatly, darkly, cosmo)")
	open := flag.Bool("open", true, "open the default web browser on start")
	flag.Parse()

	abs, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("resolving dir: %v", err)
	}
	baseDir = abs

	md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.Typographer,
			extension.DefinitionList,
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
	)

	themeNames, themeURLs = loadThemes()
	defaultTheme = strings.ToLower(*theme)
	if _, ok := themeURLs[defaultTheme]; !ok {
		log.Printf("theme %q not found; using %q", *theme, themeNames[0])
		defaultTheme = themeNames[0]
	}

	http.HandleFunc("/", handler)

	display := *addr
	if strings.HasPrefix(display, ":") {
		display = "localhost" + display
	}
	urlStr := "http://" + display + "/"
	log.Printf("serving markdown from %s", baseDir)
	log.Printf("listening on %s", urlStr)

	if *open {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(urlStr)
		}()
	}

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}

// loadThemes fetches the live Bootswatch theme list, falling back to a built-in
// list if the API is unreachable.
func loadThemes() ([]string, map[string]string) {
	names := []string{}
	urls := map[string]string{}

	client := &http.Client{Timeout: 6 * time.Second}
	if resp, err := client.Get(bootswatchAPI); err == nil {
		defer resp.Body.Close()
		var api bswAPI
		if json.NewDecoder(resp.Body).Decode(&api) == nil && len(api.Themes) > 0 {
			ver := api.Version
			if ver == "" {
				ver = bootswatchVer
			}
			for _, t := range api.Themes {
				n := strings.ToLower(t.Name)
				u := t.CSSCdn
				if u == "" {
					u = cssURL(ver, n)
				}
				names = append(names, n)
				urls[n] = u
			}
			sort.Strings(names)
			return names, urls
		}
	}

	log.Printf("could not reach Bootswatch API; using built-in theme list")
	for _, n := range fallbackThemes {
		names = append(names, n)
		urls[n] = cssURL(bootswatchVer, n)
	}
	sort.Strings(names)
	return names, urls
}

func cssURL(version, theme string) string {
	return fmt.Sprintf("https://cdn.jsdelivr.net/npm/bootswatch@%s/dist/%s/bootstrap.min.css", version, theme)
}

func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		renderIndex(w)
		return
	}

	// Clean the request path and map it onto the base directory, guarding
	// against path traversal.
	rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	full := filepath.Join(baseDir, filepath.FromSlash(rel))
	if full != baseDir && !strings.HasPrefix(full, baseDir+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	switch strings.ToLower(filepath.Ext(full)) {
	case ".md", ".markdown":
		renderMarkdown(w, full, rel)
	default:
		http.ServeFile(w, r, full)
	}
}

func renderMarkdown(w http.ResponseWriter, full, rel string) {
	src, err := os.ReadFile(full)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rel = filepath.ToSlash(rel)
	renderPage(w, rel, rel, template.HTML(buf.String()))
}

func renderIndex(w http.ResponseWriter) {
	files, err := findMarkdown(baseDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var b strings.Builder
	b.WriteString(`<h1 class="mt-4 mb-3">Markdown files</h1>`)
	if len(files) == 0 {
		b.WriteString(`<p class="text-secondary">No markdown files found under this directory.</p>`)
	} else {
		fmt.Fprintf(&b, `<p class="text-secondary">%d file(s) found.</p>`, len(files))
		b.WriteString(`<div class="list-group shadow-sm">`)
		for _, f := range files {
			href := (&url.URL{Path: "/" + f}).String()
			fmt.Fprintf(&b,
				`<a class="list-group-item list-group-item-action" href="%s">%s</a>`,
				href, template.HTMLEscapeString(f))
		}
		b.WriteString(`</div>`)
	}
	renderPage(w, "Index", "", template.HTML(b.String()))
}

func renderPage(w http.ResponseWriter, title, current string, content template.HTML) {
	themesJSON, _ := json.Marshal(themeURLs)
	data := pageData{
		Title:           title,
		Current:         current,
		Content:         content,
		BootstrapJS:     bootstrapJS,
		ThemeNames:      themeNames,
		ThemesJS:        template.JS(themesJSON),
		DefaultTheme:    defaultTheme,
		DefaultThemeURL: themeURLs[defaultTheme],
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template: %v", err)
	}
}

// findMarkdown walks baseDir and returns slash-separated relative paths to all
// markdown files, skipping hidden directories.
func findMarkdown(base string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name != "." && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(p)) {
		case ".md", ".markdown":
			if rel, err := filepath.Rel(base, p); err == nil {
				files = append(files, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func openBrowser(target string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", ""}
	default:
		cmd = "xdg-open"
	}
	args = append(args, target)
	_ = exec.Command(cmd, args...).Start()
}

const layoutHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link id="theme-css" rel="stylesheet" href="{{.DefaultThemeURL}}">
<script>
  window.THEMES = {{.ThemesJS}};
  window.DEFAULT_THEME = "{{.DefaultTheme}}";
  (function () {
    try {
      var t = localStorage.getItem("mdview-theme") || window.DEFAULT_THEME;
      if (window.THEMES[t]) document.getElementById("theme-css").href = window.THEMES[t];
    } catch (e) {}
  })();
</script>
<style>
  body { display: flex; min-height: 100vh; margin: 0; }
  .md-sidebar {
    width: 260px; flex: 0 0 260px;
    background: var(--bs-tertiary-bg, #f6f8fa);
    border-right: 1px solid var(--bs-border-color);
    padding: 1rem; display: flex; flex-direction: column; gap: 1rem;
    position: sticky; top: 0; height: 100vh; overflow-y: auto;
  }
  .md-sidebar .brand {
    font-weight: 600; font-size: 1.1rem; text-decoration: none;
    color: var(--bs-body-color);
  }
  .md-main { flex: 1 1 auto; min-width: 0; }
  .md-body { padding: 1.5rem 2rem 4rem; }
  .md-current {
    font-size: .85rem; color: var(--bs-secondary-color);
    word-break: break-all;
  }
  .md-body img { max-width: 100%; height: auto; }
  .md-body table { border-collapse: collapse; margin-bottom: 1rem; }
  .md-body table th, .md-body table td {
    border: 1px solid var(--bs-border-color); padding: .4rem .65rem;
  }
  .md-body pre {
    background: var(--bs-tertiary-bg, #f6f8fa);
    padding: 1rem; border-radius: .375rem; overflow: auto;
  }
  .md-body :not(pre) > code {
    background: var(--bs-tertiary-bg, #f6f8fa);
    padding: .1rem .35rem; border-radius: .25rem;
  }
  .md-body blockquote {
    border-left: .25rem solid var(--bs-border-color);
    padding-left: 1rem; color: var(--bs-secondary-color);
  }
  .theme-menu { max-height: 70vh; overflow-y: auto; }
  @media (max-width: 768px) {
    body { flex-direction: column; }
    .md-sidebar {
      width: 100%; flex: 0 0 auto; height: auto; position: static;
      border-right: 0; border-bottom: 1px solid var(--bs-border-color);
    }
  }
  @media print {
    body { display: block; }
    .md-sidebar { display: none; }
    .md-main, .md-body { padding: 0; }
  }
</style>
</head>
<body>
<aside class="md-sidebar">
  <a class="brand" href="/">📄 Markdown Browser</a>
  <div class="dropdown">
    <button class="btn btn-sm btn-outline-secondary dropdown-toggle w-100" type="button"
            data-bs-toggle="dropdown" aria-expanded="false">
      Theme: <span id="theme-label"></span>
    </button>
    <ul class="dropdown-menu theme-menu">
      {{range .ThemeNames}}
      <li><a class="dropdown-item theme-opt" href="#" data-theme="{{.}}">{{.}}</a></li>
      {{end}}
    </ul>
  </div>
  {{if .Current}}<div class="md-current">{{.Current}}</div>{{end}}
</aside>

<main class="md-main md-body">
{{.Content}}
</main>

<script src="{{.BootstrapJS}}"></script>
<script>
  function setTheme(t) {
    if (!window.THEMES[t]) return;
    document.getElementById("theme-css").href = window.THEMES[t];
    document.getElementById("theme-label").textContent = t;
    try { localStorage.setItem("mdview-theme", t); } catch (e) {}
  }
  document.querySelectorAll(".theme-opt").forEach(function (el) {
    el.addEventListener("click", function (e) {
      e.preventDefault();
      setTheme(el.dataset.theme);
    });
  });
  (function () {
    var saved;
    try { saved = localStorage.getItem("mdview-theme"); } catch (e) {}
    setTheme(saved || window.DEFAULT_THEME);
  })();
</script>
</body>
</html>`
