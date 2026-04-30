package rendering

import (
	"context"
	"testing"
)

func TestHTMLRendererLeavesStandaloneDocumentUnchanged(t *testing.T) {
	t.Parallel()

	body := "<!doctype html><html><head><meta charset=\"utf-8\"><title>Standalone</title></head><body><h1>Already wrapped</h1></body></html>"

	output, err := (HTMLRenderer{}).Render(context.Background(), Document{
		Title: "Ignored",
		Body:  body,
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := string(output.Bytes); got != body {
		t.Fatalf("Render() bytes = %q, want %q", got, body)
	}
}

func TestHTMLRendererWrapsFragmentsIntoStandaloneDocument(t *testing.T) {
	t.Parallel()

	output, err := (HTMLRenderer{}).Render(context.Background(), Document{
		Title: "Status Update",
		Body:  "Quarterly update\n\nAll systems nominal.",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := "<!doctype html><html><head><meta charset=\"utf-8\"><title>Status Update</title></head><body><p>Quarterly update</p><p>All systems nominal.</p></body></html>"
	if got := string(output.Bytes); got != want {
		t.Fatalf("Render() bytes = %q, want %q", got, want)
	}
}
