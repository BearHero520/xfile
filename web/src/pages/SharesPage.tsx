import type { ShareEntry } from "../types";
import {
  ArrowSync20Regular,
  ClipboardLink20Regular,
  Delete20Regular,
  Edit20Regular,
  LockClosed20Regular,
  Search20Regular,
  Share20Regular,
} from "@fluentui/react-icons";
import { useCallback, useEffect, useMemo, useState } from "react";
import { api, formatTime, timeValue } from "../api";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Field,
  Loading,
  Modal,
  PageHeader,
} from "../components/ui";
import { useToast } from "../state";

let sharesCache: ShareEntry[] | null = null;
let sharesRequest: Promise<ShareEntry[]> | null = null;

type ShareLimitDraft = {
  item: ShareEntry;
  expiresAt: string;
  maxAccessCount: string;
};

function getShares() {
  if (sharesRequest) return sharesRequest;

  const request = api
    .shares()
    .then((items) => {
      sharesCache = items;
      return items;
    })
    .finally(() => {
      if (sharesRequest === request) sharesRequest = null;
    });
  sharesRequest = request;
  return request;
}

function ShareExpiration({ item }: { item: ShareEntry }) {
  if (!item.expiresAt) return <Badge tone="neutral">永久有效</Badge>;
  const expired = timeValue(item.expiresAt) <= Date.now();
  return (
    <div className="share-expiration">
      <Badge tone={expired ? "danger" : "info"}>
        {expired ? "已过期" : "有效至"}
      </Badge>
      <span>{formatTime(item.expiresAt)}</span>
    </div>
  );
}

function localDateTimeValue(value?: string) {
  const timestamp = timeValue(value);
  if (Number.isNaN(timestamp)) return "";
  const date = new Date(timestamp);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function ShareAccessSummary({ item }: { item: ShareEntry }) {
  return (
    <div className="share-access-summary">
      <span>
        {item.maxAccessCount > 0
          ? `${item.viewCount || 0} / ${item.maxAccessCount} 次访问`
          : `${item.viewCount || 0} 次访问（不限）`}{" "}
        · {item.downloadCount || 0} 次下载
      </span>
      {item.maxAccessCount > 0 && item.viewCount >= item.maxAccessCount && (
        <Badge tone="warning">已用完</Badge>
      )}
    </div>
  );
}

export default function SharesPage({
  embedded = false,
}: {
  embedded?: boolean;
}) {
  const { show } = useToast();
  const [items, setItems] = useState<ShareEntry[]>(() => sharesCache || []);
  const [loading, setLoading] = useState(() => sharesCache === null);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [removingIds, setRemovingIds] = useState<number[]>([]);
  const [clearingExpired, setClearingExpired] = useState(false);
  const [limitDraft, setLimitDraft] = useState<ShareLimitDraft | null>(null);
  const [savingLimits, setSavingLimits] = useState(false);
  const load = useCallback(async (force = false) => {
    if (force || !sharesCache) setLoading(true);
    setError("");
    try {
      setItems(await getShares());
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "分享加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  const updateItems = useCallback(
    (update: (current: ShareEntry[]) => ShareEntry[]) => {
      setItems((current) => {
        const next = update(current);
        sharesCache = next;
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
      `${item.path} ${item.paths?.join(" ") || ""} ${item.storageKey} ${item.token}`
        .toLowerCase()
        .includes(keyword),
    );
  }, [items, query]);

  const expiredCount = useMemo(
    () =>
      items.filter(
        (item) => item.expiresAt && timeValue(item.expiresAt) <= Date.now(),
      ).length,
    [items],
  );

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
    updateItems((current) => current.filter((entry) => entry.id !== item.id));
    try {
      await request;
      show("分享已删除", "success");
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
  function editLimits(item: ShareEntry) {
    setLimitDraft({
      item,
      expiresAt: localDateTimeValue(item.expiresAt),
      maxAccessCount:
        item.maxAccessCount > 0 ? String(item.maxAccessCount) : "",
    });
  }
  async function saveLimits() {
    if (!limitDraft || savingLimits) return;
    const accessValue = limitDraft.maxAccessCount.trim();
    const maxAccessCount = accessValue ? Number(accessValue) : 0;
    if (
      accessValue &&
      (!Number.isSafeInteger(maxAccessCount) || maxAccessCount < 1)
    ) {
      show("可访问次数必须是大于 0 的整数", "error");
      return;
    }
    let expiresAt = "";
    if (limitDraft.expiresAt) {
      const expires = new Date(limitDraft.expiresAt);
      if (Number.isNaN(expires.getTime())) {
        show("到期时间格式无效", "error");
        return;
      }
      expiresAt = expires.toISOString();
    }
    setSavingLimits(true);
    try {
      const updated = await api.updateShareLimits(limitDraft.item.id, {
        expiresAt,
        maxAccessCount,
      });
      updateItems((current) =>
        current.map((item) =>
          item.id === limitDraft.item.id ? { ...item, ...updated } : item,
        ),
      );
      setLimitDraft(null);
      show("分享限制已更新", "success");
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "更新分享限制失败",
        "error",
      );
    } finally {
      setSavingLimits(false);
    }
  }
  async function clearExpired() {
    setClearingExpired(true);
    try {
      const result = await api.deleteExpiredShares();
      if (result.deleted === 0) {
        show("没有需要清理的过期链接", "success");
        return;
      }
      show(`已清理 ${result.deleted} 个过期链接`, "success");
      const expiredIds = items
        .filter(
          (item) => item.expiresAt && timeValue(item.expiresAt) <= Date.now(),
        )
        .map((item) => item.id);
      setRemovingIds(expiredIds);
      await exitDelay();
      updateItems((current) =>
        current.filter((item) => !expiredIds.includes(item.id)),
      );
      setRemovingIds([]);
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "清理过期链接失败",
        "error",
      );
    } finally {
      setClearingExpired(false);
    }
  }
  function copy(item: ShareEntry) {
    navigator.clipboard
      .writeText(`${location.origin}${item.url}`)
      .then(() => show("分享链接已复制", "success"));
  }
  const headerActions = (
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
      <Button
        variant={expiredCount > 0 ? "danger" : "secondary"}
        icon={<Delete20Regular />}
        loading={clearingExpired}
        onClick={() => void clearExpired()}
      >
        清理过期链接{expiredCount > 0 ? `（${expiredCount}）` : ""}
      </Button>
      <Button icon={<ArrowSync20Regular />} onClick={() => void load(true)}>
        刷新
      </Button>
    </>
  );
  return (
    <div className={`page-stack ${embedded ? "quick-manage-page" : ""}`}>
      {embedded ? (
        <div className="quick-manage-toolbar">
          <p>查看访问状态、调整分享限制，或复制和清理分享链接。</p>
          <div className="page-actions">{headerActions}</div>
        </div>
      ) : (
        <PageHeader
          eyebrow="资源管理"
          title="分享管理"
          description="集中查看带密码、有效期、访问次数限制和访问统计的公开分享。"
          actions={headerActions}
        />
      )}
      {error && <ErrorBanner error={error} onRetry={() => void load(true)} />}
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
        <>
          <section className="table-panel glass-panel shares-table-desktop">
            <table className="data-table">
              <thead>
                <tr>
                  <th>分享文件</th>
                  <th>安全</th>
                  <th>到期时间</th>
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
                    className={
                      removingIds.includes(item.id) ? "is-removing" : ""
                    }
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
                      <ShareExpiration item={item} />
                    </td>
                    <td>
                      <ShareAccessSummary item={item} />
                    </td>
                    <td>{formatTime(item.lastAccessAt)}</td>
                    <td>{formatTime(item.createdAt)}</td>
                    <td>
                      <div className="row-actions">
                        <Button
                          variant="ghost"
                          icon={<Edit20Regular />}
                          onClick={() => editLimits(item)}
                        >
                          编辑限制
                        </Button>
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
          <section className="share-mobile-list" aria-label="分享列表">
            {filtered.map((item) => (
              <article
                className={`share-mobile-card glass-panel ${
                  removingIds.includes(item.id) ? "is-removing" : ""
                }`}
                key={item.id}
              >
                <header>
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
                        {item.storageKey} · {item.token}
                      </small>
                    </div>
                  </div>
                  {item.protected ? (
                    <Badge tone="warning">
                      <LockClosed20Regular />
                      密码保护
                    </Badge>
                  ) : (
                    <Badge tone="success">公开</Badge>
                  )}
                </header>
                <div className="share-mobile-meta">
                  <div>
                    <span>到期时间</span>
                    <ShareExpiration item={item} />
                  </div>
                  <div>
                    <span>访问情况</span>
                    <ShareAccessSummary item={item} />
                  </div>
                  <div>
                    <span>最近访问</span>
                    <strong>{formatTime(item.lastAccessAt)}</strong>
                  </div>
                  <div>
                    <span>创建时间</span>
                    <strong>{formatTime(item.createdAt)}</strong>
                  </div>
                </div>
                <footer className="row-actions">
                  <Button
                    variant="ghost"
                    icon={<Edit20Regular />}
                    onClick={() => editLimits(item)}
                  >
                    编辑限制
                  </Button>
                  <Button
                    variant="ghost"
                    icon={<ClipboardLink20Regular />}
                    onClick={() => copy(item)}
                  >
                    复制链接
                  </Button>
                  <Button
                    variant="ghost"
                    icon={<Delete20Regular />}
                    onClick={() => void remove(item)}
                  >
                    删除
                  </Button>
                </footer>
              </article>
            ))}
          </section>
        </>
      )}
      <Modal
        open={!!limitDraft}
        title="编辑分享限制"
        description={limitDraft ? limitDraft.item.path : undefined}
        onClose={() => !savingLimits && setLimitDraft(null)}
        footer={
          <>
            <Button disabled={savingLimits} onClick={() => setLimitDraft(null)}>
              取消
            </Button>
            <Button
              variant="primary"
              loading={savingLimits}
              onClick={() => void saveLimits()}
            >
              保存限制
            </Button>
          </>
        }
      >
        {limitDraft && (
          <div className="form-grid">
            <Field label="到期时间（选填）" hint="留空表示永久有效。">
              <input
                type="datetime-local"
                value={limitDraft.expiresAt}
                onChange={(event) =>
                  setLimitDraft((current) =>
                    current
                      ? { ...current, expiresAt: event.target.value }
                      : current,
                  )
                }
              />
            </Field>
            <Field
              label="可访问次数（选填）"
              hint="留空表示不限；若小于已访问次数，保存后会立即失效。"
            >
              <input
                type="number"
                min="1"
                step="1"
                inputMode="numeric"
                placeholder="不限次数"
                value={limitDraft.maxAccessCount}
                onChange={(event) =>
                  setLimitDraft((current) =>
                    current
                      ? { ...current, maxAccessCount: event.target.value }
                      : current,
                  )
                }
              />
            </Field>
          </div>
        )}
      </Modal>
    </div>
  );
}
