package rendering

import (
	"context"
	"html"
	"strings"
)

type HTMLRenderer struct{}

func (HTMLRenderer) Render(_ context.Context, document Document) (Output, error) {
	body := document.Body
	if !isStandaloneHTMLDocument(body) {
		body = renderHTMLDocument(document.Title, body)
	}

	return Output{
		Bytes:       []byte(body),
		MIMEType:    "text/html; charset=utf-8",
		Previewable: true,
	}, nil
}

func isStandaloneHTMLDocument(body string) bool {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return false
	}

	lowered := strings.ToLower(trimmed)
	if strings.HasPrefix(lowered, "<!doctype html") || strings.HasPrefix(lowered, "<html") {
		return strings.Contains(lowered, "</html>")
	}

	return false
}

func renderHTMLDocument(title, body string) string {
	var builder strings.Builder
	builder.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>")
	builder.WriteString(html.EscapeString(title))
	builder.WriteString("</title></head><body>")

	for _, block := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n\n") {
		trimmed := strings.TrimSpace(block)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "<") {
			builder.WriteString(trimmed)
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			builder.WriteString("<h1>")
			builder.WriteString(html.EscapeString(strings.TrimPrefix(trimmed, "# ")))
			builder.WriteString("</h1>")
			continue
		}
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(strings.ReplaceAll(trimmed, "\n", " ")))
		builder.WriteString("</p>")
	}

	builder.WriteString("</body></html>")
	return builder.String()
}

type RegistryConfig struct {
	Markdown Renderer
	HTML     Renderer
	CSV      Renderer
	Text     Renderer
	PDF      Renderer
	DOCX     Renderer
	XLSX     Renderer
}

type Registry struct {
	markdown Renderer
	html     Renderer
	csv      Renderer
	text     Renderer
	pdf      Renderer
	docx     Renderer
	xlsx     Renderer
}

func NewRegistry(config RegistryConfig) Registry {
	registry := Registry{
		markdown: config.Markdown,
		html:     config.HTML,
		csv:      config.CSV,
		text:     config.Text,
		pdf:      config.PDF,
		docx:     config.DOCX,
		xlsx:     config.XLSX,
	}
	if registry.markdown == nil {
		registry.markdown = MarkdownRenderer{}
	}
	if registry.html == nil {
		registry.html = HTMLRenderer{}
	}
	if registry.csv == nil {
		registry.csv = CSVRenderer{}
	}
	if registry.text == nil {
		registry.text = TextRenderer{}
	}
	if registry.pdf == nil {
		registry.pdf = WKHTMLToPDFRenderer{HTML: registry.html, Command: "wkhtmltopdf"}
	}
	if registry.docx == nil {
		registry.docx = DOCXRenderer{}
	}
	if registry.xlsx == nil {
		registry.xlsx = XLSXRenderer{}
	}
	return registry
}

func (r Registry) Render(ctx context.Context, renderedType string, document Document) (Output, error) {
	switch renderedType {
	case "markdown":
		return r.markdown.Render(ctx, document)
	case "html":
		return r.html.Render(ctx, document)
	case "csv":
		return r.csv.Render(ctx, document)
	case "text":
		return r.text.Render(ctx, document)
	case "pdf":
		return r.pdf.Render(ctx, document)
	case "docx":
		return r.docx.Render(ctx, document)
	case "xlsx":
		return r.xlsx.Render(ctx, document)
	default:
		return Output{}, ErrUnknownRenderedType(renderedType)
	}
}
