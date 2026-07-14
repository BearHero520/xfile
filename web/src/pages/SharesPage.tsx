import type { ShareEntry } from "../types";
import {
  ArrowSync20Regular,
  ClipboardLink20Regular,
  Delete20Regular,
  LockClosed20Regular,
  Search20Regular,
  Share20Regular,
} from "@fluentui/react-icons";
import { useEffect, useMemo, useState } from "react";
import { api, formatTime } from "../api";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Loading,
  PageHeader,
} from "../components/ui";
import { useToast } from "../state";

export default function SharesPage() {
  const { show } = useToast();
  const [items, setItems] = useState<ShareEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [removingIds, setRemovingIds] = useState<number[]>([]);
  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await api.shares());
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "分享加载失败");
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);

  const filtered = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return items;
    return items.filter((item) =>
      `${item.path} ${item.paths?.join(" ") || ""} ${item.storageKey} ${item.token}`
        .toLowerCase()
        .includes(keyword),
    );
  }, [items, query]);

  function exitDelay() {
    const reduced =
      document.documentElement.dataset.motion === "reduced" ||
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    return new Promise((resolve) =>
      window.setTimeout(resolve, reduced ? 0 : 180),
    );
  }

  async function remove(item: ShareEntry) {
    if (!confirm(`确定删除分享“${item.path}”吗？`)) return;
    setRemovingIds((current) => [...current, item.id]);
    const request = api.deleteShare(item.id);
    await exitDelay();
    setItems((current) => current.filter((entry) => entry.id !== item.id));
    try {
      await request;
      show("分享已删除", "success");
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "删除失败",
        "error",
      );
      await load();
    } finally {
      setRemovingIds((current) => current.filter((id) => id !== item.id));
    }
  }
  async function clearExpired() {
    const result = await api.deleteExpiredShares();
    show(`已清理 ${result.deleted} 个过期分享`, "success");
    const expiredIds = items
      .filter(
        (item) =>
          item.expiresAt && new Date(item.expiresAt).getTime() <= Date.now(),
      )
      .map((item) => item.id);
    setRemovingIds(expiredIds);
    await exitDelay();
    setItems((current) =>
      current.filter((item) => !expiredIds.includes(item.id)),
    );
    setRemovingIds([]);
  }
  function copy(item: ShareEntry) {
    navigator.clipboard
      .writeText(`${location.origin}${item.url}`)
      .then(() => show("分享链接已复制", "success"));
  }
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="资源管理"
        title="分享管理"
        description="集中查看带密码、有效期和访问统计的公开分享。"
        actions={
          <>
            <label className="resource-search">
              <Search20Regular aria-hidden="true" />
              <input
                aria-label="查询分享"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="查询路径、存储源或令牌"
              />
            </label>
            <Button onClick={clearExpired}>清理过期</Button>
            <Button icon={<ArrowSync20Regular />} onClick={load}>
              刷新
            </Button>
          </>
        }
      />
      {error && <ErrorBanner error={error} onRetry={load} />}
      {loading ? (
        <Loading />
      ) : filtered.length === 0 ? (
        <Empty
          title={query ? "没有匹配的分享" : "还没有分享"}
          description={
            query
              ? "换一个关键词查询路径、存储源或令牌。"
              : "在文件管理页选择文件并点击“分享”即可创建。"
          }
        />
      ) : (
        <section className="table-panel glass-panel">
          <table className="data-table">
            <thead>
              <tr>
                <th>分享文件</th>
                <th>安全</th>
                <th>有效期</th>
                <th>访问</th>
                <th>最近访问</th>
                <th>创建时间</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {filtered.map((item) => (
                <tr
                  key={item.id}
                  className={removingIds.includes(item.id) ? "is-removing" : ""}
                >
                  <td>
                    <div className="identity-cell">
                      <span>
                        <Share20Regular />
                      </span>
                      <div>
                        <strong>
                          {item.itemCount > 1
                            ? `聚合分享 · ${item.itemCount} 项`
                            : item.path}
                        </strong>
                        <small>
                          {item.itemCount > 1
                            ? item.paths?.slice(0, 2).join("、")
                            : item.storageKey}{" "}
                          · {item.token}
                        </small>
                      </div>
                    </div>
                  </td>
                  <td>
                    {item.protected ? (
                      <Badge tone="warning">
                        <LockClosed20Regular />
                        密码保护
                      </Badge>
                    ) : (
                      <Badge tone="success">公开</Badge>
                    )}
                  </td>
                  <td>
                    {item.expiresAt ? formatTime(item.expiresAt) : "永久"}
                  </td>
                  <td>
                    {item.viewCount || 0} 次浏览 · {item.downloadCount || 0}{" "}
                    次下载
                  </td>
                  <td>{formatTime(item.lastAccessAt)}</td>
                  <td>{formatTime(item.createdAt)}</td>
                  <td>
                    <div className="row-actions">
                      <Button
                        variant="ghost"
                        icon={<ClipboardLink20Regular />}
                        onClick={() => copy(item)}
                      >
                        复制
                      </Button>
                      <Button
                        variant="ghost"
                        icon={<Delete20Regular />}
                        onClick={() => void remove(item)}
                      >
                        删除
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
    </div>
  );
}
