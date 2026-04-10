package rendering

import (
	"context"
	"strings"
)

type CSVRenderer struct{}

func (CSVRenderer) Render(_ context.Context, document Document) (Output, error) {
	content := strings.ReplaceAll(document.Body, "\r\n", "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return Output{
		Bytes:       []byte(content),
		MIMEType:    "text/csv; charset=utf-8",
		Previewable: true,
	}, nil
}
