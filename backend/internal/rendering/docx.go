package rendering

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"strings"
)

// DOCXRenderer creates a minimal .docx file from the document body.
// DOCX is an Open XML format: a ZIP archive containing XML parts.
type DOCXRenderer struct{}

func (DOCXRenderer) Render(_ context.Context, document Document) (Output, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`
	if err := writeZipEntry(w, "[Content_Types].xml", contentTypes); err != nil {
		return Output{}, err
	}

	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`
	if err := writeZipEntry(w, "_rels/.rels", rels); err != nil {
		return Output{}, err
	}

	docXML := buildDocumentXML(document.Title, document.Body)
	if err := writeZipEntry(w, "word/document.xml", docXML); err != nil {
		return Output{}, err
	}

	if err := w.Close(); err != nil {
		return Output{}, err
	}

	return Output{
		Bytes:       buf.Bytes(),
		MIMEType:    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Previewable: false,
	}, nil
}

func writeZipEntry(w *zip.Writer, name, content string) error {
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(content))
	return err
}

func buildDocumentXML(title, body string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	b.WriteString(`<w:body>`)

	// Title paragraph (bold, larger).
	if title != "" {
		b.WriteString(`<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr>`)
		b.WriteString(`<w:r><w:rPr><w:b/><w:sz w:val="32"/></w:rPr>`)
		b.WriteString(`<w:t>`)
		b.WriteString(xmlEscape(title))
		b.WriteString(`</w:t></w:r></w:p>`)
	}

	// Body paragraphs split on double newlines.
	paragraphs := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n\n")
	for _, para := range paragraphs {
		text := strings.TrimSpace(para)
		if text == "" {
			continue
		}
		// Handle lines within a paragraph as separate runs with line breaks.
		lines := strings.Split(text, "\n")
		b.WriteString(`<w:p>`)
		for i, line := range lines {
			b.WriteString(`<w:r><w:t xml:space="preserve">`)
			b.WriteString(xmlEscape(line))
			b.WriteString(`</w:t></w:r>`)
			if i < len(lines)-1 {
				b.WriteString(`<w:r><w:br/></w:r>`)
			}
		}
		b.WriteString(`</w:p>`)
	}

	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}
