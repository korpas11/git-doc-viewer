#!/usr/bin/env python3
"""
serve_docs.py - Browse and render local Markdown files via Markdeep.

Place this script anywhere in your repository (or a parent directory).
Run with: python serve_docs.py [root_dir] [port]

  root_dir  Directory to scan for .md files (default: script's own directory)
  port      Port to listen on (default: 8765)
"""

import sys
import os
import http.server
import urllib.parse
import pathlib
import socketserver
import webbrowser
import threading

# ── Configuration ────────────────────────────────────────────────────────────

DEFAULT_PORT = 8765
EXCLUDE_DIRS = {".git", "node_modules", "__pycache__", ".venv", "venv", ".tox"}

MARKDEEP_SNIPPET = (
    "<!-- Markdeep: -->"
    '<style class="fallback">body{visibility:hidden;white-space:pre;font-family:monospace}</style>'
    "<script>window.markdeepOptions = {tocStyle:'long',tocDepth:2, detectMath:true};</script>"
    '<script src="markdeep.min.js" charset="utf-8"></script>'
    '<script src="https://morgan3d.github.io/markdeep/latest/markdeep.min.js" charset="utf-8"></script>'
    '<script>window.alreadyProcessedMarkdeep||(document.body.style.visibility="visible")</script>'
)

INDEX_STYLE = """
<style>
  * { box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
         background: #f5f5f5; margin: 0; padding: 0; }
  header { background: #24292e; color: #fff; padding: 16px 32px;
           display: flex; align-items: center; gap: 12px; }
  header svg { flex-shrink: 0; }
  h1 { margin: 0; font-size: 1.2rem; font-weight: 600; }
  .root-label { font-size: 0.8rem; color: #aaa; margin-top: 2px; }
  main { max-width: 860px; margin: 32px auto; padding: 0 16px; }
  .search-wrap { margin-bottom: 20px; }
  #search { width: 100%; padding: 10px 14px; font-size: 0.95rem;
            border: 1px solid #ccc; border-radius: 6px; outline: none; }
  #search:focus { border-color: #0366d6; box-shadow: 0 0 0 3px rgba(3,102,214,.15); }
  .group { margin-bottom: 28px; }
  .group-title { font-size: 0.72rem; font-weight: 700; text-transform: uppercase;
                 letter-spacing: .08em; color: #888; margin-bottom: 8px;
                 padding-bottom: 4px; border-bottom: 1px solid #e1e4e8; }
  ul { list-style: none; margin: 0; padding: 0; }
  li { margin: 3px 0; }
  li.hidden { display: none; }
  a { display: block; padding: 7px 12px; border-radius: 5px;
      color: #0366d6; text-decoration: none; font-size: 0.93rem;
      background: #fff; border: 1px solid #e1e4e8; }
  a:hover { background: #f0f7ff; border-color: #0366d6; }
  .fname { font-weight: 500; }
  .fdir { font-size: 0.78rem; color: #888; margin-left: 8px; }
  .empty { color: #888; font-style: italic; padding: 12px; }
</style>
"""

INDEX_SCRIPT = """
<script>
  const search = document.getElementById('search');
  search.addEventListener('input', () => {
    const q = search.value.toLowerCase();
    document.querySelectorAll('li[data-name]').forEach(li => {
      li.classList.toggle('hidden', !li.dataset.name.includes(q));
    });
    document.querySelectorAll('.group').forEach(g => {
      const any = [...g.querySelectorAll('li')].some(li => !li.classList.contains('hidden'));
      g.style.display = any ? '' : 'none';
    });
  });
  search.focus();
</script>
"""

# ── Helpers ───────────────────────────────────────────────────────────────────

def find_md_files(root: pathlib.Path) -> list[pathlib.Path]:
    results = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]
        for f in sorted(filenames):
            if f.lower().endswith(".md"):
                results.append(pathlib.Path(dirpath) / f)
    return sorted(results)


def build_index(root: pathlib.Path) -> str:
    files = find_md_files(root)

    # Group by directory
    groups: dict[str, list[pathlib.Path]] = {}
    for f in files:
        rel = f.relative_to(root)
        folder = str(rel.parent) if str(rel.parent) != "." else "(root)"
        groups.setdefault(folder, []).append(f)

    body_parts = []
    if not files:
        body_parts.append('<p class="empty">No .md files found.</p>')
    else:
        for folder in sorted(groups):
            items = "".join(
                f'<li data-name="{f.name.lower()} {folder.lower()}">'
                f'<a href="/view/{urllib.parse.quote(str(f.relative_to(root)))}">'
                f'<span class="fname">{f.stem}</span>'
                f'<span class="fdir">{f.name}</span>'
                f'</a></li>'
                for f in groups[folder]
            )
            label = folder.replace("\\", "/")
            body_parts.append(
                f'<div class="group">'
                f'<div class="group-title">{label}</div>'
                f'<ul>{items}</ul>'
                f'</div>'
            )

    count = len(files)
    noun = "file" if count == 1 else "files"

    return f"""<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Markdown Browser</title>{INDEX_STYLE}</head>
<body>
<header>
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor"
       stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
    <polyline points="14 2 14 8 20 8"/>
    <line x1="16" y1="13" x2="8" y2="13"/>
    <line x1="16" y1="17" x2="8" y2="17"/>
    <polyline points="10 9 9 9 8 9"/>
  </svg>
  <div>
    <h1>Markdown Browser &mdash; {count} {noun}</h1>
    <div class="root-label">{root}</div>
  </div>
</header>
<main>
  <div class="search-wrap">
    <input id="search" type="search" placeholder="Filter files…" autocomplete="off">
  </div>
  {''.join(body_parts)}
</main>
{INDEX_SCRIPT}
</body></html>"""


def build_viewer(root: pathlib.Path, rel_path: str) -> tuple[int, str]:
    """Return (status_code, html_or_error)."""
    try:
        decoded = urllib.parse.unquote(rel_path)
        target = (root / decoded).resolve()
        # Safety: must stay within root
        target.relative_to(root.resolve())
    except (ValueError, Exception):
        return 404, "<h1>404 Not Found</h1>"

    if not target.exists() or not target.is_file():
        return 404, f"<h1>404 — {decoded} not found</h1>"
    if target.suffix.lower() != ".md":
        return 403, "<h1>403 — Only .md files are served</h1>"

    content = target.read_text(encoding="utf-8", errors="replace")
    # Markdeep expects the raw markdown as page body text
    # We escape only what would break HTML structure
    escaped = content.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")

    back_url = "/"
    title = target.stem

    html = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{title}</title>
<style>
  .back-bar {{
    position: fixed; top: 0; left: 0; right: 0; z-index: 9999;
    background: #24292e; padding: 6px 16px;
    display: flex; align-items: center; gap: 10px;
  }}
  .back-bar a {{
    color: #ccc; text-decoration: none; font-family: sans-serif;
    font-size: 0.85rem;
  }}
  .back-bar a:hover {{ color: #fff; }}
  .back-bar .sep {{ color: #666; }}
  .back-bar .current {{ color: #fff; font-weight: 500; font-size: 0.85rem;
                        font-family: sans-serif; }}
  body {{ padding-top: 36px; }}
</style>
</head>
<body>
<div class="back-bar">
  <a href="{back_url}">&#8592; Index</a>
  <span class="sep">/</span>
  <span class="current">{urllib.parse.unquote(rel_path)}</span>
</div>

{escaped}

{MARKDEEP_SNIPPET}
</body>
</html>"""
    return 200, html


# ── HTTP Handler ──────────────────────────────────────────────────────────────

def make_handler(root: pathlib.Path):
    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, format, *args):
            pass  # suppress default access log

        def send_html(self, code: int, html: str):
            encoded = html.encode("utf-8")
            self.send_response(code)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def do_GET(self):
            parsed = urllib.parse.urlparse(self.path)
            path = parsed.path

            if path == "/" or path == "":
                self.send_html(200, build_index(root))
            elif path.startswith("/view/"):
                rel = path[len("/view/"):]
                code, html = build_viewer(root, rel)
                self.send_html(code, html)
            else:
                self.send_html(404, "<h1>404</h1>")

    return Handler


# ── Entry Point ───────────────────────────────────────────────────────────────

def main():
    args = sys.argv[1:]
    root = pathlib.Path(args[0]).resolve() if args else pathlib.Path(__file__).parent.resolve()
    port = int(args[1]) if len(args) > 1 else DEFAULT_PORT

    if not root.is_dir():
        print(f"Error: '{root}' is not a directory.", file=sys.stderr)
        sys.exit(1)

    url = f"http://localhost:{port}"
    print(f"Serving docs from: {root}")
    print(f"Open:              {url}")
    print("Press Ctrl+C to stop.\n")

    with socketserver.TCPServer(("", port), make_handler(root)) as httpd:
        httpd.allow_reuse_address = True
        threading.Timer(0.5, webbrowser.open, args=[url]).start()
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nStopped.")


if __name__ == "__main__":
    main()
