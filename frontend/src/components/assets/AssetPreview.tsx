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
  const sanitizedHTML = useMemo(() => {
    if (!asset || !content?.previewable || !content.content) {
      return "";
    }

    if (isMarkdownAsset(asset)) {
      return DOMPurify.sanitize(marked.parse(content.content) as string);
    }

    if (isHTMLAsset(asset)) {
      return DOMPurify.sanitize(content.content);
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
        <a className="button button--ghost" href={normalizeDownloadPath(asset.path)} rel="noreferrer" target="_blank">
          Download asset
        </a>
      </div>
    );
  }

  if (sanitizedHTML) {
    return <div className="preview-surface" dangerouslySetInnerHTML={{ __html: sanitizedHTML }} />;
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
