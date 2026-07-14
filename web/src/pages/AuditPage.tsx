import type { AccessLog, AccessLogPage } from "../types";
import {
  ArrowSync20Regular,
  Delete20Regular,
  Search20Regular,
} from "@fluentui/react-icons";
import { FormEvent, useEffect, useState } from "react";
import { api, formatTime } from "../api";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Loading,
  PageHeader,
} from "../components/ui";
import { useApp, useToast } from "../state";

export default function AuditPage() {
  const { session } = useApp();
  const { show } = useToast();
  const [data, setData] = useState<AccessLogPage>({
    items: [],
    total: 0,
    page: 1,
    pageSize: 50,
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [action, setAction] = useState("");
  const [path, setPath] = useState("");
  const load = async (page = data.page) => {
    setLoading(true);
    setError("");
    try {
      setData(await api.auditEvents({ page, pageSize: 50, action, path }));
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "日志加载失败");
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void load(1);
  }, []);
  async function cleanup(all = false) {
    if (!confirm(all ? "确定清空全部审计日志吗？" : "确定清理 30 天前日志吗？"))
      return;
    const result = await api.deleteAuditEvents(all ? undefined : 30, all);
    show(`已清理 ${result.deleted} 条日志`, "success");
    await load(1);
  }
  function submit(event: FormEvent) {
    event.preventDefault();
    void load(1);
  }
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="安全与系统"
        title="审计日志"
        description="跟踪登录、下载、分享、文件操作与管理动作。"
        actions={
          <>
            <Button icon={<ArrowSync20Regular />} onClick={() => load()}>
              刷新
            </Button>
            {session?.user?.role === "super_admin" && (
              <Button
                variant="danger"
                icon={<Delete20Regular />}
                onClick={() => cleanup(false)}
              >
                清理旧日志
              </Button>
            )}
          </>
        }
      />
      <form className="filter-bar glass-panel" onSubmit={submit}>
        <div>
          <Search20Regular />
          <input
            placeholder="操作类型，例如 download"
            value={action}
            onChange={(event) => setAction(event.target.value)}
          />
        </div>
        <input
          placeholder="文件路径"
          value={path}
          onChange={(event) => setPath(event.target.value)}
        />
        <Button type="submit" variant="primary">
          筛选
        </Button>
      </form>
      {error && <ErrorBanner error={error} onRetry={() => load()} />}
      {loading ? (
        <Loading />
      ) : data.items.length === 0 ? (
        <Empty title="没有匹配的日志" />
      ) : (
        <section className="table-panel glass-panel">
          <table className="data-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>操作</th>
                <th>目标</th>
                <th>IP</th>
                <th>客户端</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((item: AccessLog) => (
                <tr key={item.id}>
                  <td>{formatTime(item.createdAt)}</td>
                  <td>
                    <Badge tone={tone(item.action)}>
                      {actionLabel(item.action)}
                    </Badge>
                  </td>
                  <td>
                    <code>{item.path || "—"}</code>
                  </td>
                  <td>{item.ip || "—"}</td>
                  <td className="truncate">{item.userAgent || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="pagination">
            <Button
              disabled={data.page <= 1}
              onClick={() => load(data.page - 1)}
            >
              上一页
            </Button>
            <span>
              第 {data.page} 页 · 共 {data.total} 条
            </span>
            <Button
              disabled={data.page * data.pageSize >= data.total}
              onClick={() => load(data.page + 1)}
            >
              下一页
            </Button>
          </div>
        </section>
      )}
    </div>
  );
}
function tone(
  action: string,
): "success" | "warning" | "danger" | "info" | "neutral" {
  if (action.includes("delete") || action.includes("failed")) return "danger";
  if (action.includes("upload") || action.includes("create")) return "success";
  if (action.includes("login") || action.includes("session")) return "warning";
  if (action.includes("download") || action.includes("share")) return "info";
  return "neutral";
}
function actionLabel(action: string) {
  return (
    (
      {
        download: "下载",
        upload: "上传",
        delete: "删除",
        login: "登录",
        direct: "直链访问",
        "share-view": "查看分享",
        "share-download": "分享下载",
      } as Record<string, string>
    )[action] || action
  );
}
