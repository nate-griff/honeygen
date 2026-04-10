package rendering

import "context"

type TextRenderer struct{}

func (TextRenderer) Render(_ context.Context, document Document) (Output, error) {
	return Output{
		Bytes:       []byte(document.Body),
		MIMEType:    "text/plain; charset=utf-8",
		Previewable: true,
	}, nil
}
