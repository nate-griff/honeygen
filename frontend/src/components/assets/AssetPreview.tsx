import { useMemo } from "react";
import DOMPurify from "dompurify";
import { marked } from "marked";
import type { Asset, AssetContentResponse } from "../../types/assets";
import { EmptyState } from "../layout/EmptyState";
import { normalizeDownloadPath } from "../../app/format";

interface AssetPreviewProps {
  asset: Asset | null;
  content: AssetContentResponse | null;
}

function isMarkdownAsset(asset: Asset): boolean {
  return asset.rendered_type === "markdown" || asset.mime_type.includes("markdown");
}

function isHTMLAsset(asset: Asset): boolean {
  return asset.rendered_type === "html" || asset.mime_type.includes("html");
}

function isTextAsset(asset: Asset): boolean {
  return asset.mime_type.startsWith("text/") || asset.rendered_type === "text" || asset.rendered_type === "csv";
}

export function AssetPreview({ asset, content }: AssetPreviewProps) {
  const previewDocument = useMemo(() => {
    if (!asset || !content?.previewable || !content.content) {
      return "";
    }

    if (isMarkdownAsset(asset)) {
      return buildPreviewDocument(DOMPurify.sanitize(marked.parse(content.content) as string));
    }

    if (isHTMLAsset(asset)) {
      return buildPreviewDocument(DOMPurify.sanitize(content.content));
    }

    return "";
  }, [asset, content]);

  if (!asset) {
    return <EmptyState title="No asset selected" message="Choose a file from the asset tree to inspect metadata and preview content." />;
  }

  if (!content) {
    return <EmptyState title="Preview unavailable" message="Select a file to request preview content from the API." />;
  }

  if (!content.previewable) {
    return (
      <div className="empty-state empty-state--left">
        <strong>Binary asset</strong>
        <p>{content.message || "This asset is download-only."}</p>
        <a className="button button--ghost" download href={normalizeDownloadPath(asset.path)}>
          Download asset
        </a>
      </div>
    );
  }

  if (previewDocument) {
    return (
      <div className="preview-surface preview-surface--frame">
        <iframe className="preview-frame" sandbox="" srcDoc={previewDocument} title={`${asset.path} preview`} />
      </div>
    );
  }

  if (content.content && isTextAsset(asset)) {
    return <pre className="preview-text">{content.content}</pre>;
  }

  return (
    <div className="empty-state empty-state--left">
      <strong>Preview not supported</strong>
      <p>This content type is not rendered inline.</p>
    </div>
  );
}

function buildPreviewDocument(body: string): string {
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <style>
      :root { color-scheme: light; }
      body {
        margin: 0;
        padding: 1rem;
        color: #0f172a;
        background: #ffffff;
        font: 14px/1.6 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      }
      img, iframe, table { max-width: 100%; }
      pre {
        white-space: pre-wrap;
        overflow-wrap: anywhere;
        background: #f8fafc;
        border-radius: 0.75rem;
        padding: 0.75rem;
      }
      table {
        border-collapse: collapse;
      }
      th, td {
        border: 1px solid #cbd5e1;
        padding: 0.35rem 0.5rem;
        text-align: left;
      }
      a { color: #2563eb; }
    </style>
  </head>
  <body>${body}</body>
</html>`;
}
