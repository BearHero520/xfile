import type { FavoriteEntry } from "../types";
import {
  ArrowSync20Regular,
  Delete20Regular,
  FolderOpen20Regular,
  Search20Regular,
  Star20Regular,
} from "@fluentui/react-icons";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { api, formatTime } from "../api";
import {
  Button,
  Empty,
  ErrorBanner,
  Loading,
  PageHeader,
} from "../components/ui";
import { useToast } from "../state";

function parentPath(path: string) {
  const parts = path.split("/").filter(Boolean);
  parts.pop();
  return parts.join("/");
}

function exitDelay() {
  const reduced =
    document.documentElement.dataset.motion === "reduced" ||
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  return new Promise((resolve) =>
    window.setTimeout(resolve, reduced ? 0 : 180),
  );
}

export default function FavoritesPage({
  embedded = false,
  onLocate,
}: {
  embedded?: boolean;
  onLocate?: (item: FavoriteEntry) => void;
}) {
  const { show } = useToast();
  const [items, setItems] = useState<FavoriteEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [removingIds, setRemovingIds] = useState<number[]>([]);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await api.favorites());
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "收藏加载失败");
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
      `${item.path} ${item.storageKey}`.toLowerCase().includes(keyword),
    );
  }, [items, query]);

  async function remove(item: FavoriteEntry) {
    if (!confirm(`确定取消收藏“${item.path}”吗？`)) return;
    setRemovingIds((current) => [...current, item.id]);
    const request = api.deleteFavorite(item.id);
    await exitDelay();
    setItems((current) => current.filter((entry) => entry.id !== item.id));
    try {
      await request;
      show("已取消收藏", "success");
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

  const headerActions = (
    <>
      <label className="resource-search">
        <Search20Regular aria-hidden="true" />
        <input
          aria-label="查询收藏"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="查询路径或存储源"
        />
      </label>
      <Button icon={<ArrowSync20Regular />} onClick={load}>
        刷新
      </Button>
    </>
  );

  return (
    <div className={`page-stack ${embedded ? "quick-manage-page" : ""}`}>
      {embedded ? (
        <div className="quick-manage-toolbar">
          <p>查找常用文件和文件夹，定位后会回到当前文件列表。</p>
          <div className="page-actions">{headerActions}</div>
        </div>
      ) : (
        <PageHeader
          eyebrow="个人资源"
          title="我的收藏"
          description="查询常用文件和文件夹，或随时取消收藏。"
          actions={headerActions}
        />
      )}
      {error && <ErrorBanner error={error} onRetry={load} />}
      {loading ? (
        <Loading />
      ) : filtered.length === 0 ? (
        <Empty
          title={query ? "没有匹配的收藏" : "还没有收藏"}
          description="在文件管理页右键文件或文件夹，选择“加入收藏”。"
        />
      ) : (
        <section className="table-panel glass-panel">
          <table className="data-table">
            <thead>
              <tr>
                <th>收藏项目</th>
                <th>存储源</th>
                <th>收藏时间</th>
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
                        <Star20Regular />
                      </span>
                      <div>
                        <strong>
                          {item.path.split("/").pop() || item.path}
                        </strong>
                        <small>{item.path}</small>
                      </div>
                    </div>
                  </td>
                  <td>{item.storageKey}</td>
                  <td>{formatTime(item.createdAt)}</td>
                  <td>
                    <div className="row-actions">
                      {embedded && onLocate ? (
                        <Button
                          variant="ghost"
                          icon={<FolderOpen20Regular />}
                          onClick={() => onLocate(item)}
                        >
                          定位
                        </Button>
                      ) : (
                        <Link
                          className="button button-ghost button-medium"
                          to={`/?source=${encodeURIComponent(item.storageKey)}&path=${encodeURIComponent(parentPath(item.path))}`}
                        >
                          <FolderOpen20Regular />
                          定位
                        </Link>
                      )}
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
