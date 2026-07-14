import type { DirectLinkEntry } from "../types";
import {
  ArrowSync20Regular,
  ClipboardLink20Regular,
  Delete20Regular,
  Link20Regular,
  Search20Regular,
} from "@fluentui/react-icons";
import { useCallback, useEffect, useMemo, useState } from "react";
import { api, formatTime } from "../api";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Loading,
  PageHeader,
  Switch,
} from "../components/ui";
import { useToast } from "../state";

let deliveryLinksCache: DirectLinkEntry[] | null = null;
let deliveryLinksRequest: Promise<DirectLinkEntry[]> | null = null;

function getDeliveryLinks() {
  if (deliveryLinksRequest) return deliveryLinksRequest;

  const request = api
    .deliveryLinks()
    .then((items) => {
      deliveryLinksCache = items;
      return items;
    })
    .finally(() => {
      if (deliveryLinksRequest === request) deliveryLinksRequest = null;
    });
  deliveryLinksRequest = request;
  return request;
}

export default function DeliveryPage({
  embedded = false,
}: {
  embedded?: boolean;
}) {
  const { show } = useToast();
  const [items, setItems] = useState<DirectLinkEntry[]>(
    () => deliveryLinksCache || [],
  );
  const [loading, setLoading] = useState(() => deliveryLinksCache === null);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [removingIds, setRemovingIds] = useState<number[]>([]);
  const load = useCallback(async (force = false) => {
    if (force || !deliveryLinksCache) setLoading(true);
    setError("");
    try {
      setItems(await getDeliveryLinks());
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "直链加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  const updateItems = useCallback(
    (update: (current: DirectLinkEntry[]) => DirectLinkEntry[]) => {
      setItems((current) => {
        const next = update(current);
        deliveryLinksCache = next;
        return next;
      });
    },
    [],
  );

  useEffect(() => {
    void load();
  }, [load]);
  const filtered = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return items;
    return items.filter((item) =>
      `${item.path} ${item.storageKey} ${item.token}`
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

  async function toggle(item: DirectLinkEntry, enabled: boolean) {
    updateItems((current) =>
      current.map((entry) =>
        entry.id === item.id ? { ...entry, enabled } : entry,
      ),
    );
    try {
      await api.updateDeliveryLink(item.id, enabled);
    } catch (nextError) {
      updateItems((current) =>
        current.map((entry) =>
          entry.id === item.id ? { ...entry, enabled: item.enabled } : entry,
        ),
      );
      show(
        nextError instanceof Error ? nextError.message : "状态更新失败",
        "error",
      );
    }
  }
  async function remove(item: DirectLinkEntry) {
    if (!confirm(`确定删除“${item.path}”的直链吗？`)) return;
    setRemovingIds((current) => [...current, item.id]);
    const request = api.deleteDeliveryLink(item.id);
    await exitDelay();
    updateItems((current) => current.filter((entry) => entry.id !== item.id));
    try {
      await request;
      show("直链已删除", "success");
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "删除失败",
        "error",
      );
      await load(true);
    } finally {
      setRemovingIds((current) => current.filter((id) => id !== item.id));
    }
  }
  function copy(item: DirectLinkEntry) {
    navigator.clipboard
      .writeText(`${location.origin}${item.url}`)
      .then(() => show("直链已复制", "success"));
  }
  const headerActions = (
    <>
      <label className="resource-search">
        <Search20Regular aria-hidden="true" />
        <input
          aria-label="查询短链"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="查询路径、存储源或令牌"
        />
      </label>
      <Button icon={<ArrowSync20Regular />} onClick={() => void load(true)}>
        刷新
      </Button>
    </>
  );
  return (
    <div className={`page-stack ${embedded ? "quick-manage-page" : ""}`}>
      {embedded ? (
        <div className="quick-manage-toolbar">
          <p>快速启用、停用、复制或删除稳定下载短链。</p>
          <div className="page-actions">{headerActions}</div>
        </div>
      ) : (
        <PageHeader
          eyebrow="资源管理"
          title="短链管理"
          description="同一文件复用同一个稳定下载短链，并可随时停用或删除。"
          actions={headerActions}
        />
      )}
      {error && <ErrorBanner error={error} onRetry={() => void load(true)} />}
      {loading ? (
        <Loading />
      ) : filtered.length === 0 ? (
        <Empty
          title={query ? "没有匹配的短链" : "还没有短链"}
          description={
            query
              ? "换一个关键词查询路径、存储源或令牌。"
              : "在文件管理页选择文件并点击“直链”即可创建。"
          }
        />
      ) : (
        <section className="table-panel glass-panel">
          <table className="data-table">
            <thead>
              <tr>
                <th>文件</th>
                <th>状态</th>
                <th>访问次数</th>
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
                        <Link20Regular />
                      </span>
                      <div>
                        <strong>{item.path}</strong>
                        <small>
                          {item.storageKey} · {item.token}
                        </small>
                      </div>
                    </div>
                  </td>
                  <td>
                    <div className="inline-status">
                      <Switch
                        label={`启用 ${item.path}`}
                        checked={item.enabled}
                        onChange={(value) => void toggle(item, value)}
                      />
                      <Badge tone={item.enabled ? "success" : "neutral"}>
                        {item.enabled ? "可用" : "停用"}
                      </Badge>
                    </div>
                  </td>
                  <td>{item.accessCount || 0}</td>
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
