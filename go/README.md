# mdview

Serves every markdown file in a directory tree as HTML in your browser, styled
with a configurable [Bootswatch](https://bootswatch.com/) theme.

## Build

Requires Go 1.22+.

```sh
go mod tidy          # downloads the goldmark dependency (needs network once)
go build -o mdview   # produces a standalone executable
```

This yields a single self-contained binary (`mdview`, or `mdview.exe` on
Windows). The markdown dependency is compiled in; only the Bootswatch/Bootstrap
CSS is loaded from a CDN at view time, so the machine running it needs internet
access for the styling to appear.

### Cross-compiling

```sh
GOOS=windows GOARCH=amd64 go build -o mdview.exe
GOOS=darwin  GOARCH=arm64 go build -o mdview-mac
GOOS=linux   GOARCH=amd64 go build -o mdview-linux
```

## Run

From the directory you want to browse:

```sh
./mdview
```

It scans the current directory and all subdirectories, starts a local server,
and opens your browser at the index page.

### Flags

| Flag      | Default          | Description                                            |
|-----------|------------------|--------------------------------------------------------|
| `-dir`    | `.`              | Directory to scan for `.md` / `.markdown` files        |
| `-addr`   | `localhost:8080` | Listen address                                         |
| `-theme`  | `flatly`         | Default Bootswatch theme                               |
| `-open`   | `true`           | Open the default browser on start (`-open=false` to skip) |

Example:

```sh
./mdview -dir ./docs -theme darkly -addr :9000
```

## Theme selection

The theme list is fetched live from the Bootswatch API at startup (with a
built-in fallback if offline), so it stays current. Pick the startup theme with
`-theme`, or switch any time from the **Theme** dropdown in the navbar — your
choice is remembered in the browser via `localStorage`.

## Notes

- URL paths mirror the filesystem, so relative image links inside your markdown
  (e.g. `![](images/diagram.png)`) resolve correctly.
- Hidden directories (those starting with `.`) are skipped.
- GitHub-flavored markdown is supported: tables, strikethrough, task lists,
  autolinks, footnotes, and auto heading IDs.
