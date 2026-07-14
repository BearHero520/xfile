import {
  ArrowSync20Regular,
  BookOpen20Regular,
  BranchFork20Regular,
  Calendar20Regular,
  Code20Regular,
  History20Regular,
  Open20Regular,
  Person20Regular,
  Star20Regular,
} from "@fluentui/react-icons";
import { useCallback, useEffect, useMemo, useState } from "react";
import ReactMarkdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";
import { api } from "../api";
import BrandMark from "../components/BrandMark";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Loading,
  PageHeader,
} from "../components/ui";
import type { AboutData, AboutDocument } from "../types";

const preferredDocumentPaths = [
  "docs/更新日志.md",
  "docs/changelog.md",
  "changelog.md",
];

function preferredDocumentPath(documents: AboutDocument[]) {
  for (const preferredPath of preferredDocumentPaths) {
    const document = documents.find(
      (item) =>
        item.path.replace(/\\/g, "/").toLocaleLowerCase() ===
        preferredPath.toLocaleLowerCase(),
    );
    if (document) return document.path;
  }
  return documents[0]?.path || "";
}

function formatDate(value: string, includeTime = false) {
  if (!value) return "暂无记录";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "short",
    day: "numeric",
    ...(includeTime ? { hour: "2-digit", minute: "2-digit" } : {}),
  }).format(date);
}

function resolveDocumentPath(currentPath: string, target: string) {
  const cleanTarget = target.split(/[?#]/, 1)[0];
  const parts = cleanTarget.startsWith("/")
    ? []
    : currentPath.split("/").slice(0, -1);
  for (const part of cleanTarget.replace(/^\/+/, "").split("/")) {
    if (!part || part === ".") continue;
    if (part === "..") parts.pop();
    else parts.push(part);
  }
  return parts.map(encodeURIComponent).join("/");
}

function markdownURL(
  value: string,
  key: string,
  document: AboutDocument,
  data: AboutData,
) {
  if (/^(https?:|mailto:|tel:|#)/i.test(value)) {
    return defaultUrlTransform(value);
  }
  const resolved = resolveDocumentPath(document.path, value);
  if (!resolved) return data.repository.htmlUrl;
  if (key === "src") {
    return `https://raw.githubusercontent.com/${data.repository.fullName}/${encodeURIComponent(data.repository.defaultBranch)}/${resolved}`;
  }
  return `${data.repository.htmlUrl}/blob/${encodeURIComponent(data.repository.defaultBranch)}/${resolved}`;
}

export default function AboutPage() {
  const [data, setData] = useState<AboutData | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [selectedPath, setSelectedPath] = useState("");

  const load = useCallback(async (refresh = false) => {
    if (refresh) setRefreshing(true);
    else setLoading(true);
    setError("");
    try {
      const next = await api.about(refresh);
      setData(next);
      setSelectedPath((current) =>
        next.documents.some((document) => document.path === current)
          ? current
          : preferredDocumentPath(next.documents),
      );
    } catch (loadError) {
      setError(
        loadError instanceof Error ? loadError.message : "关于页面加载失败",
      );
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const selectedDocument = useMemo(
    () => data?.documents.find((document) => document.path === selectedPath),
    [data, selectedPath],
  );

  if (loading && !data) {
    return (
      <div className="page-stack about-page">
        <PageHeader
          eyebrow="PROJECT"
          title="关于 XFile"
          description="项目作者、更新日志与 GitHub 文档"
        />
        <section className="glass-panel about-loading-panel">
          <Loading label="正在从 GitHub 读取项目信息" />
        </section>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="page-stack about-page">
        <PageHeader
          eyebrow="PROJECT"
          title="关于 XFile"
          description="项目作者、更新日志与 GitHub 文档"
        />
        <ErrorBanner
          error={error || "GitHub 项目信息暂时无法读取"}
          onRetry={() => void load(true)}
        />
      </div>
    );
  }

  const { repository } = data;
  return (
    <div className="page-stack about-page">
      <PageHeader
        eyebrow="PROJECT"
        title="关于 XFile"
        description="直接连接 GitHub，查看作者信息、最近更新与项目文档。"
        actions={
          <>
            <a
              className="button button-secondary button-medium"
              href={repository.htmlUrl}
              target="_blank"
              rel="noreferrer"
            >
              <Code20Regular />
              查看 GitHub
            </a>
            <Button
              icon={<ArrowSync20Regular />}
              loading={refreshing}
              onClick={() => void load(true)}
            >
              重新读取
            </Button>
          </>
        }
      />

      {(error || data.stale || data.warnings?.length) && (
        <div className="about-notices">
          {error && (
            <ErrorBanner error={error} onRetry={() => void load(true)} />
          )}
          {data.stale && (
            <div className="about-warning">
              当前展示的是上次成功读取的 GitHub 内容。
            </div>
          )}
          {data.warnings?.map((warning) => (
            <div className="about-warning" key={warning}>
              {warning}
            </div>
          ))}
        </div>
      )}

      <section className="glass-panel about-hero">
        <div className="about-project-main">
          <div className="about-project-logo">
            <BrandMark />
          </div>
          <div>
            <span className="about-repository-name">{repository.fullName}</span>
            <h2>{repository.name.toUpperCase()}</h2>
            <p>
              {repository.description || "轻量、私有的文件管理与分享服务。"}
            </p>
            <div className="about-meta-row">
              <span>
                <BranchFork20Regular /> {repository.defaultBranch}
              </span>
              <span>
                <Calendar20Regular /> 最近更新 {formatDate(repository.pushedAt)}
              </span>
            </div>
          </div>
        </div>
        <a
          className="about-author-card"
          href={repository.author.htmlUrl}
          target="_blank"
          rel="noreferrer"
        >
          <img
            src={repository.author.avatarUrl}
            alt={`${repository.author.login} 的 GitHub 头像`}
          />
          <span>
            <small>项目作者</small>
            <strong>{repository.author.login}</strong>
            <em>
              查看 GitHub 主页 <Open20Regular />
            </em>
          </span>
        </a>
      </section>

      <section className="about-facts" aria-label="GitHub 项目概览">
        <article>
          <Star20Regular />
          <span>
            <small>Stars</small>
            <strong>{repository.stars}</strong>
          </span>
        </article>
        <article>
          <BranchFork20Regular />
          <span>
            <small>Forks</small>
            <strong>{repository.forks}</strong>
          </span>
        </article>
        <article>
          <History20Regular />
          <span>
            <small>更新记录</small>
            <strong>{data.changes.length}</strong>
          </span>
        </article>
        <article>
          <BookOpen20Regular />
          <span>
            <small>GitHub 文档</small>
            <strong>{data.documents.length}</strong>
          </span>
        </article>
      </section>

      <div className="about-content-grid">
        <section className="glass-panel about-section about-changelog">
          <header className="about-section-header">
            <div>
              <span className="about-section-icon">
                <History20Regular />
              </span>
              <span>
                <h2>更新日志</h2>
                <p>优先读取 Releases，没有发布版本时显示最近提交。</p>
              </span>
            </div>
            <small>GitHub · {formatDate(data.fetchedAt, true)}</small>
          </header>

          {data.changes.length ? (
            <div className="about-change-list">
              {data.changes.map((change) => (
                <article className="about-change" key={change.id}>
                  <div className="about-change-marker" aria-hidden="true" />
                  <div>
                    <div className="about-change-topline">
                      <Badge
                        tone={change.type === "release" ? "success" : "info"}
                      >
                        {change.type === "release"
                          ? "Release"
                          : change.tag || "Commit"}
                      </Badge>
                      <time dateTime={change.publishedAt}>
                        {formatDate(change.publishedAt)}
                      </time>
                    </div>
                    <a href={change.htmlUrl} target="_blank" rel="noreferrer">
                      {change.title} <Open20Regular />
                    </a>
                    {change.body && <p>{change.body.slice(0, 280)}</p>}
                    {change.author && (
                      <small className="about-change-author">
                        <Person20Regular /> {change.author}
                      </small>
                    )}
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <Empty
              title="暂无更新记录"
              description="GitHub 还没有 Release 或可展示的提交。"
            />
          )}
        </section>

        <section className="glass-panel about-section about-documents">
          <header className="about-section-header">
            <div>
              <span className="about-section-icon">
                <BookOpen20Regular />
              </span>
              <span>
                <h2>项目文档</h2>
                <p>优先展示《更新日志》，其余内容直接读取自 GitHub。</p>
              </span>
            </div>
            {selectedDocument && (
              <a
                className="about-source-link"
                href={selectedDocument.htmlUrl}
                target="_blank"
                rel="noreferrer"
              >
                查看源文件 <Open20Regular />
              </a>
            )}
          </header>

          {data.documents.length && selectedDocument ? (
            <>
              <div
                className="about-document-tabs"
                role="tablist"
                aria-label="GitHub 文档"
              >
                {data.documents.map((document) => (
                  <button
                    type="button"
                    role="tab"
                    aria-selected={document.path === selectedDocument.path}
                    className={
                      document.path === selectedDocument.path ? "is-active" : ""
                    }
                    key={document.path}
                    onClick={() => setSelectedPath(document.path)}
                  >
                    {document.title}
                    <small>{document.path}</small>
                  </button>
                ))}
              </div>
              <article className="about-markdown">
                <ReactMarkdown
                  remarkPlugins={[remarkGfm]}
                  urlTransform={(value, key) =>
                    markdownURL(value, key, selectedDocument, data)
                  }
                  components={{
                    a: ({ children, ...props }) => (
                      <a {...props} target="_blank" rel="noreferrer">
                        {children}
                      </a>
                    ),
                    img: ({ alt, ...props }) => (
                      <img
                        {...props}
                        alt={alt || "GitHub 文档图片"}
                        loading="lazy"
                      />
                    ),
                  }}
                >
                  {selectedDocument.content}
                </ReactMarkdown>
              </article>
            </>
          ) : (
            <Empty
              title="没有找到项目文档"
              description="在 GitHub 的 docs 目录添加 Markdown 文件后，这里会自动显示。"
            />
          )}
        </section>
      </div>
    </div>
  );
}
