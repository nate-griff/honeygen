import { useState } from "react";
import { useLoaderData } from "react-router-dom";
import {
  getProviderSettings,
  testProviderConnection,
  updateProviderSettings,
  type ProviderSettings,
} from "../api/settings";
import { PageHeader } from "../components/layout/PageHeader";
import { Panel } from "../components/layout/Panel";
import { ErrorAlert } from "../components/layout/ErrorAlert";
import { StatusBadge } from "../components/layout/StatusBadge";

interface SettingsLoaderData {
  provider: ProviderSettings;
}

export async function settingsLoader(): Promise<SettingsLoaderData> {
  const provider = await getProviderSettings();
  return { provider };
}

export default function SettingsPage() {
  const loaderData = useLoaderData() as SettingsLoaderData;
  const [baseUrl, setBaseUrl] = useState(loaderData.provider.base_url);
  const [apiKey, setApiKey] = useState(loaderData.provider.api_key);
  const [model, setModel] = useState(loaderData.provider.model);
  const [ready, setReady] = useState(loaderData.provider.ready);
  const [mode, setMode] = useState(loaderData.provider.mode);
  const [isSaving, setIsSaving] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  const [saveError, setSaveError] = useState<string>();
  const [saveSuccess, setSaveSuccess] = useState<string>();
  const [testError, setTestError] = useState<string>();
  const [testSuccess, setTestSuccess] = useState<string>();

  async function handleSave() {
    setIsSaving(true);
    setSaveError(undefined);
    setSaveSuccess(undefined);
    try {
      const result = await updateProviderSettings({
        base_url: baseUrl,
        api_key: apiKey,
        model,
      });
      setReady(result.ready);
      setMode(result.mode);
      setApiKey(result.api_key);
      setSaveSuccess("Provider settings saved.");
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Unable to save settings");
    } finally {
      setIsSaving(false);
    }
  }

  async function handleTest() {
    setIsTesting(true);
    setTestError(undefined);
    setTestSuccess(undefined);
    try {
      const result = await testProviderConnection();
      setTestSuccess(
        `Connection successful — provider is ${result.ready ? "ready" : "not ready"} (${result.mode}).`,
      );
    } catch (error) {
      setTestError(error instanceof Error ? error.message : "Connection test failed");
    } finally {
      setIsTesting(false);
    }
  }

  return (
    <div className="stack">
      <PageHeader
        title="Settings"
        description="Configure the LLM provider used for content generation and world model synthesis."
      />
      <div className="two-column">
        <Panel title="LLM Provider" subtitle="OpenAI-compatible endpoint configuration">
          <div className="stack stack--compact">
            <div className="list-card__title-row">
              <strong>Status</strong>
              <StatusBadge value={ready} />
              <span className="muted">{mode}</span>
            </div>
            <label className="toolbar-field">
              Base URL
              <input
                type="text"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder="https://api.openai.com/v1"
              />
            </label>
            <label className="toolbar-field">
              API Key
              <input
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="sk-..."
              />
            </label>
            <label className="toolbar-field">
              Model
              <input
                type="text"
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder="gpt-4o-mini"
              />
            </label>
            {saveError ? <ErrorAlert message={saveError} /> : null}
            {saveSuccess ? <p className="success-text">{saveSuccess}</p> : null}
            <div className="button-row">
              <button className="button" onClick={handleSave} disabled={isSaving}>
                {isSaving ? "Saving…" : "Save settings"}
              </button>
            </div>
          </div>
        </Panel>
        <Panel title="Connection test" subtitle="Verify the provider is reachable and returns valid data">
          <div className="stack stack--compact">
            <p>
              Send a test request to the configured LLM endpoint to verify connectivity and
              authentication.
            </p>
            {testError ? <ErrorAlert message={testError} /> : null}
            {testSuccess ? <p className="success-text">{testSuccess}</p> : null}
            <div className="button-row">
              <button className="button button--ghost" onClick={handleTest} disabled={isTesting}>
                {isTesting ? "Testing…" : "Test connection"}
              </button>
            </div>
          </div>
        </Panel>
      </div>
    </div>
  );
}
