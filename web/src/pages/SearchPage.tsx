import type { FileEntry, StorageSource } from "../types";
import {
  ArrowDownload20Regular,
  Folder20Filled,
  Search20Regular,
  Share20Regular,
} from "@fluentui/react-icons";
import { FormEvent, useEffect, useState } from "react";
import { api, contentUrl, formatBytes, formatTime } from "../api";
import {
  Badge,
  Button,
  Empty,
  Field,
  Loading,
  PageHeader,
  Select,
} from "../components/ui";
import { useToast } from "../state";

export default function SearchPage() {
  const { show } = useToast();
  const [sources, setSources] = useState<StorageSource[]>([]);
  const [sourceKey, setSourceKey] = useState("");
  const [q, setQ] = useState(
    new URLSearchParams(location.search).get("q") || "",
  );
  const [items, setItems] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    api.storageNodes().then((value) => {
      setSources(value.filter((source) => source.enabled));
      setSourceKey(value.find((source) => source.enabled)?.key || "");
    });
  }, []);
  async function search(event?: FormEvent) {
    event?.preventDefault();
    if (!sourceKey || !q.trim()) return;
    setLoading(true);
    try {
      setItems(await api.search(sourceKey, q.trim()));
    } catch (error) {
      show(error instanceof Error ? error.message : "搜索失败", "error");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    if (sourceKey && q.trim()) void search();
  }, [sourceKey]);
  async function share(file: FileEntry) {
    const value = await api.createShare({
      storageKey: sourceKey,
      path: file.path,
    });
    await navigator.clipboard.writeText(`${location.origin}${value.url}`);
    show("分享链接已创建并复制", "success");
  }
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="文件服务"
        title="全局搜索"
        description="跨目录搜索名称与文件说明，结果可直接下载或分享。"
      />
      <form className="search-hero glass-panel" onSubmit={search}>
        <Field label="存储源">
          <Select
            value={sourceKey}
            onChange={(event) => setSourceKey(event.target.value)}
          >
            {sources.map((source) => (
              <option key={source.key} value={source.key}>
                {source.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="关键词">
          <div className="input-with-icon">
            <Search20Regular />
            <input
              autoFocus
              value={q}
              onChange={(event) => setQ(event.target.value)}
              placeholder="输入文件名、路径或说明"
            />
          </div>
        </Field>
        <Button variant="primary" type="submit">
          搜索
        </Button>
      </form>
      {loading ? (
        <Loading label="正在搜索" />
      ) : items.length === 0 ? (
        <Empty title={q ? "没有匹配结果" : "输入关键词开始搜索"} />
      ) : (
        <section className="result-list">
          {items.map((file) => (
            <article className="result-item glass-panel" key={file.path}>
              <span className="result-icon">
                {file.type === "folder" ? (
                  <Folder20Filled />
                ) : (
                  <Search20Regular />
                )}
              </span>
              <div>
                <strong>{file.name}</strong>
                <code>{file.path}</code>
                {file.description && <p>{file.description}</p>}
              </div>
              <div className="result-meta">
                <Badge tone={file.type === "folder" ? "warning" : "info"}>
                  {file.type === "folder" ? "文件夹" : formatBytes(file.size)}
                </Badge>
                <span>{formatTime(file.modifiedAt)}</span>
              </div>
              <div className="row-actions">
                {file.type === "file" && (
                  <a
                    className="button button-ghost"
                    href={contentUrl(sourceKey, file.path)}
                  >
                    <ArrowDownload20Regular />
                    下载
                  </a>
                )}
                <Button
                  variant="ghost"
                  icon={<Share20Regular />}
                  onClick={() => void share(file)}
                >
                  分享
                </Button>
              </div>
            </article>
          ))}
        </section>
      )}
    </div>
  );
}
