package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// ---------- text extraction ----------

var errBinary = errors.New("binary file (not text)")

var docExts = map[string]string{
	".docx": "docx", ".docm": "docx", ".dotx": "docx", ".pptx": "pptx", ".xlsx": "xlsx", ".xlsm": "xlsx",
	".odt": "odf", ".ods": "odf", ".odp": "odf", ".pdf": "pdf",
}

var textExts = map[string]bool{}

func init() {
	for _, e := range strings.Fields("txt md markdown csv tsv json jsonl xml html htm log ini cfg conf toml yaml yml go py js ts tsx jsx java kt c cpp h hpp cs php rb rs sh bat cmd ps1 sql srt vtt rtf tex css scss lua swift r m eml") {
		textExts["."+e] = true
	}
}

func isDocExt(ext string) bool { _, ok := docExts[strings.ToLower(ext)]; return ok }

func decodeText(b []byte) (string, error) {
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return utf16Decode(b[2:], binary.LittleEndian), nil
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return utf16Decode(b[2:], binary.BigEndian), nil
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		b = b[3:]
	}
	chk := b
	if len(chk) > 8000 {
		chk = chk[:8000]
	}
	if bytes.IndexByte(chk, 0) >= 0 {
		return "", errBinary
	}
	if utf8.Valid(b) || utf8.Valid(b[:max(0, len(b)-3)]) {
		return strings.ToValidUTF8(string(b), ""), nil
	}
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r), nil
}

func utf16Decode(b []byte, bo binary.ByteOrder) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = bo.Uint16(b[2*i:])
	}
	return string(utf16.Decode(u))
}

func readTextFile(p string, limit int) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)))
	if err != nil {
		return "", err
	}
	return decodeText(b)
}

// textOpts: how long OCR may run for scanned PDF pages and images (0 = only what's
// cached), which PDF pages to read (nil = all), and a context to stop waiting.
type textOpts struct {
	ocrWait time.Duration
	pages   map[int]bool
	ctx     context.Context
	ocrUpTo int  // only OCR scanned pages up to this page (0 = all)
	legacy  bool // open .doc/.xls/.ppt through the installed Office
}

func extractText(p string, limit int) (string, error) { return extractTextOpt(p, limit, textOpts{}) }

func extractTextOpt(p string, limit int, o textOpts) (string, error) {
	ext := strings.ToLower(filepath.Ext(p))
	if isImageExt(ext) {
		return imageText(p, o)
	}
	switch docExts[ext] {
	case "pdf":
		return pdfFileText(p, limit, o)
	case "docx":
		return zipText(p, func(n string) bool { return n == "word/document.xml" }, limit)
	case "pptx":
		return zipText(p, func(n string) bool { return strings.HasPrefix(n, "ppt/slides/slide") && strings.HasSuffix(n, ".xml") }, limit)
	case "xlsx":
		return xlsxText(p, "", limit)
	case "odf":
		return zipText(p, func(n string) bool { return n == "content.xml" }, limit)
	}
	if _, ok := legacyOffice[ext]; ok {
		if !o.legacy {
			return "", errBinary
		}
		np, err := legacyCopy(p)
		if err != nil {
			return "", err
		}
		return extractTextOpt(np, limit, o)
	}
	t, err := readTextFile(p, limit)
	if err != nil {
		return "", err
	}
	if ext == ".html" || ext == ".htm" {
		t = stripHTML(t)
	}
	return t, nil
}

func zipText(p string, want func(string) bool, limit int) (string, error) {
	zr, err := zip.OpenReader(p)
	if err != nil {
		return "", fmt.Errorf("couldn't open document: %v", err)
	}
	defer zr.Close()
	var files []*zip.File
	for _, f := range zr.File {
		if want(f.Name) {
			files = append(files, f)
		}
	}
	sort.Slice(files, func(a, b int) bool { return natLess(files[a].Name, files[b].Name) })
	var b strings.Builder
	for _, f := range files {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		dec := xml.NewDecoder(rc)
		for b.Len() < limit {
			tok, err := dec.Token()
			if err != nil {
				break
			}
			switch t := tok.(type) {
			case xml.CharData:
				b.Write(t)
			case xml.EndElement:
				switch t.Name.Local {
				case "p", "si", "tr", "br", "h":
					b.WriteByte('\n')
				case "tc", "tab":
					b.WriteByte('\t')
				}
			}
		}
		rc.Close()
		b.WriteByte('\n')
	}
	return b.String(), nil
}

var (
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	blockTagRe    = regexp.MustCompile(`(?i)</?(p|div|br|li|tr|h[1-6])[^>]*>`)
	tagRe         = regexp.MustCompile(`<[^>]+>`)
	blankRe       = regexp.MustCompile(`\n\s*\n+`)
)

func stripHTML(s string) string {
	s = scriptStyleRe.ReplaceAllString(s, " ")
	s = blockTagRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(blankRe.ReplaceAllString(html.UnescapeString(s), "\n\n"))
}

func containsFold(xs []string, s string) bool {
	for _, x := range xs {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
