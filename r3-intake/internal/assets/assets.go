// Package assets embeds the static web assets and HTML templates into the binary.
// This lets the app deploy as a single static binary without needing a public/
// directory on the target host.
package assets

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"time"
)

// FS contains all files from the public/ directory at build time.
//
//go:embed all:public
var FS embed.FS

// HTTPFileSystem returns a net/http.FileSystem backed by the embedded FS,
// rooted at public/ so that /static/htmx.min.js maps to public/htmx.min.js.
func HTTPFileSystem() http.FileSystem {
	root, err := fs.Sub(FS, "public")
	if err != nil {
		// The embed tag guarantees public/ exists; failure is a build-time bug.
		panic(err)
	}
	return http.FS(root)
}

// TemplateBytes returns the raw bytes of public/index.html.
func TemplateBytes() ([]byte, error) {
	return FS.ReadFile("public/index.html")
}

// TemplateString returns public/index.html as a string.
func TemplateString() (string, error) {
	b, err := TemplateBytes()
	return string(b), err
}

// embeddedFileInfo wraps fs.FileInfo so that http.ServeContent can read via
// an io.ReadSeeker we build from the embedded file bytes.
type embeddedFileInfo struct {
	fs.FileInfo
}

// embeddedFile adapts an embed file for http.ServeContent.
type embeddedFile struct {
	content []byte
	pos     int
	info    embeddedFileInfo
}

func (f *embeddedFile) Read(p []byte) (int, error) {
	if f.pos >= len(f.content) {
		return 0, io.EOF
	}
	n := copy(p, f.content[f.pos:])
	f.pos += n
	return n, nil
}

func (f *embeddedFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.pos = int(offset)
	case io.SeekCurrent:
		f.pos += int(offset)
	case io.SeekEnd:
		f.pos = len(f.content) + int(offset)
	}
	if f.pos < 0 {
		f.pos = 0
	}
	if f.pos > len(f.content) {
		f.pos = len(f.content)
	}
	return int64(f.pos), nil
}

func (f *embeddedFile) Close() error { return nil }
func (f *embeddedFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

// OpenReadSeeker opens a file from the embedded FS as an io.ReadSeeker, which
// is the minimal interface http.ServeContent needs.
func OpenReadSeeker(name string) (io.ReadSeeker, fs.FileInfo, error) {
	b, err := FS.ReadFile("public/" + name)
	if err != nil {
		return nil, nil, err
	}
	fi, err := FS.Open("public/" + name)
	if err != nil {
		return nil, nil, err
	}
	info, err := fi.Stat()
	fi.Close()
	if err != nil {
		return nil, nil, err
	}
	return &embeddedFile{content: b, info: embeddedFileInfo{FileInfo: info}, pos: 0}, info, nil
}

// StaticModTime returns a stable modification time for embedded assets. Since
// embedded files have no real mtime, we return the build time. This keeps
// http.ServeContent from serving zero-value modtimes, which break caching.
func StaticModTime() time.Time {
	return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
}
