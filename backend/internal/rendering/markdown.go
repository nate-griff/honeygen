package rendering

import (
	"context"
)

type Document struct {
	Title    string
	Body     string
	Metadata map[string]string
}

type Output struct {
	Bytes       []byte
	MIMEType    string
	Previewable bool
}

type Renderer interface {
	Render(context.Context, Document) (Output, error)
}

type MarkdownRenderer struct{}

func (MarkdownRenderer) Render(_ context.Context, document Document) (Output, error) {
	return Output{
		Bytes:       []byte(document.Body),
		MIMEType:    "text/markdown; charset=utf-8",
		Previewable: true,
	}, nil
}
