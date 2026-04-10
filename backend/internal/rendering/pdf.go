package rendering

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type pdfTypeError string

func (e pdfTypeError) Error() string {
	return string(e)
}

func ErrUnknownRenderedType(renderedType string) error {
	return pdfTypeError("unknown rendered type: " + renderedType)
}

type WKHTMLToPDFRenderer struct {
	HTML    Renderer
	Command string
}

func (r WKHTMLToPDFRenderer) Render(ctx context.Context, document Document) (Output, error) {
	htmlRenderer := r.HTML
	if htmlRenderer == nil {
		htmlRenderer = HTMLRenderer{}
	}
	htmlOutput, err := htmlRenderer.Render(ctx, document)
	if err != nil {
		return Output{}, err
	}

	command := r.Command
	if command == "" {
		command = "wkhtmltopdf"
	}

	cmd := exec.CommandContext(ctx, command, "--quiet", "-", "-")
	cmd.Stdin = bytes.NewReader(htmlOutput.Bytes)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return Output{}, fmt.Errorf("render pdf with %s: %s", command, strings.TrimSpace(stderr.String()))
		}
		return Output{}, fmt.Errorf("render pdf with %s: %w", command, err)
	}

	return Output{
		Bytes:       stdout.Bytes(),
		MIMEType:    "application/pdf",
		Previewable: false,
	}, nil
}

type StaticPDFRenderer []byte

func (r StaticPDFRenderer) Render(_ context.Context, _ Document) (Output, error) {
	return Output{
		Bytes:       append([]byte(nil), r...),
		MIMEType:    "application/pdf",
		Previewable: false,
	}, nil
}
