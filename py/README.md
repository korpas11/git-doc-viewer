# serve_docs.py

A single-file Python script that lets you browse and read all Markdown files in a repository via your web browser, rendered with [Markdeep](https://casual-effects.com/markdeep/).

No dependencies beyond the Python standard library.

## Features

- Recursively finds all `.md` files from the script's directory (or a specified root)
- Index page groups files by subdirectory with a live filter box
- Renders each file using Markdeep (loaded from CDN, with optional local fallback)
- Fixed back-to-index bar on every rendered page
- Opens your browser automatically on launch
- Blocks path traversal — only serves files within the scanned root

## Usage

Place `serve_docs.py` in your repository root (or any subdirectory) and run it:

```bash
python serve_docs.py
```

The script scans from its own directory by default and opens `http://localhost:8765`.

**Optional arguments:**

```bash
python serve_docs.py [root_dir] [port]
```

| Argument   | Default                        | Description                        |
|------------|--------------------------------|------------------------------------|
| `root_dir` | Directory containing the script | Root directory to scan for `.md` files |
| `port`     | `8765`                         | Port to listen on                  |

**Examples:**

```bash
# Serve from the current directory on default port
python serve_docs.py

# Serve a specific directory
python serve_docs.py /path/to/repo

# Serve a specific directory on a custom port
python serve_docs.py /path/to/repo 9000
```

Press `Ctrl+C` to stop the server.

## Markdeep

[Markdeep](https://casual-effects.com/markdeep/) extends Markdown with diagrams, math, tables, and more. It is loaded from the CDN at runtime, so an internet connection is required by default.

**Optional local fallback:** Download [`markdeep.min.js`](https://morgan3d.github.io/markdeep/latest/markdeep.min.js) and place it alongside `serve_docs.py`. The rendered pages will try the local copy first, then fall back to the CDN.

## Excluded directories

The following directories are skipped during scanning:

`.git`, `node_modules`, `__pycache__`, `.venv`, `venv`, `.tox`

To change this, edit the `EXCLUDE_DIRS` set near the top of the script.

## Requirements

- Python 3.9+
- No third-party packages

## License

MIT
