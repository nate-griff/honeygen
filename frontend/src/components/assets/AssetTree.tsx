import type { AssetTreeNode } from "../../types/assets";

interface AssetTreeProps {
  nodes: AssetTreeNode[];
  selectedAssetID?: string;
  onSelectAsset: (assetID: string) => void;
}

function AssetTreeBranch({
  node,
  selectedAssetID,
  onSelectAsset,
}: {
  node: AssetTreeNode;
  selectedAssetID?: string;
  onSelectAsset: (assetID: string) => void;
}) {
  if (node.kind === "file" && node.asset_id) {
    return (
      <li>
        <button
          className={`tree-node tree-node--file${selectedAssetID === node.asset_id ? " is-selected" : ""}`}
          onClick={() => onSelectAsset(node.asset_id!)}
          type="button"
        >
          {node.name}
        </button>
      </li>
    );
  }

  return (
    <li>
      <details className="tree-group" open>
        <summary>{node.name}</summary>
        <ul className="tree-list">
          {(node.children ?? []).map((child) => (
            <AssetTreeBranch key={`${node.path}-${child.path}`} node={child} onSelectAsset={onSelectAsset} selectedAssetID={selectedAssetID} />
          ))}
        </ul>
      </details>
    </li>
  );
}

export function AssetTree({ nodes, selectedAssetID, onSelectAsset }: AssetTreeProps) {
  return (
    <ul className="tree-list">
      {nodes.map((node) => (
        <AssetTreeBranch key={node.path} node={node} onSelectAsset={onSelectAsset} selectedAssetID={selectedAssetID} />
      ))}
    </ul>
  );
}
