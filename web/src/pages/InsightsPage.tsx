import type { LinkAnalytics } from "../types";
import { ArrowSync20Regular } from "@fluentui/react-icons";
import { useEffect, useState } from "react";
import { api, formatTime } from "../api";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Loading,
  PageHeader,
} from "../components/ui";

const emptyAnalytics: LinkAnalytics = {
  shareVisits: [],
  downloadRanking: [],
  directLinkAccesses: [],
};

export default function InsightsPage() {
  const [data, setData] = useState<LinkAnalytics>(emptyAnalytics);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setData(await api.insights());
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "统计加载失败");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="链接与统计"
        title="下载排行统计"
        description="查看热门下载路径、分享访问和直链访问记录。"
        actions={
          <Button icon={<ArrowSync20Regular />} onClick={load}>
            刷新
          </Button>
        }
      />
      {error && <ErrorBanner error={error} onRetry={load} />}
      {loading ? (
        <Loading />
      ) : data.downloadRanking.length === 0 ? (
        <Empty
          title="暂无下载统计"
          description="产生下载记录后会在这里形成排行。"
        />
      ) : (
        <>
          <section className="table-panel glass-panel">
            <table className="data-table">
              <thead>
                <tr>
                  <th>排行</th>
                  <th>文件或路径</th>
                  <th>下载次数</th>
                  <th>最近访问</th>
                </tr>
              </thead>
              <tbody>
                {data.downloadRanking.map((item, index) => (
                  <tr key={`${item.path}-${index}`}>
                    <td>
                      <Badge tone={index < 3 ? "warning" : "neutral"}>
                        #{index + 1}
                      </Badge>
                    </td>
                    <td>
                      <code>{item.path}</code>
                    </td>
                    <td>{item.count}</td>
                    <td>{formatTime(item.lastAccessAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </section>
          <section className="metric-grid">
            <article className="metric glass-panel">
              <span>近期分享访问</span>
              <strong>{data.shareVisits.length}</strong>
            </article>
            <article className="metric glass-panel">
              <span>近期直链访问</span>
              <strong>{data.directLinkAccesses.length}</strong>
            </article>
          </section>
        </>
      )}
    </div>
  );
}
