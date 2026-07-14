import type { FileEntry, ShareDetail } from "../types";
import {
  ArrowDownload20Regular,
  ArrowLeft20Regular,
  ChevronRight20Regular,
  Folder20Filled,
  LockClosed20Regular,
  Share20Regular,
  Video20Regular,
} from "@fluentui/react-icons";
import { FormEvent, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, formatBytes, formatTime, shareContentUrl } from "../api";
import AppearanceControl from "../components/AppearanceControl";
import BrandMark from "../components/BrandMark";
import FilePreview, { filePreviewKind } from "../components/FilePreview";
import { Badge, Button, Empty, Field, Loading, Modal } from "../components/ui";
import { useApp } from "../state";

type ShareBreadcrumb = { label: string; path: string };
type FileSelection = { file: FileEntry; url: string };

export default function SharePage() {
  const { site } = useApp();
  const { token = "" } = useParams();
  const [password, setPassword] = useState("");
  const [path, setPath] = useState("");
  const [detail, setDetail] = useState<ShareDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [rootLabel, setRootLabel] = useState("分享内容");
  const [breadcrumbs, setBreadcrumbs] = useState<ShareBreadcrumb[]>([]);
  const [previewFile, setPreviewFile] = useState<FileSelection | null>(null);
  const loadRequestId = useRef(0);

  const load = async () => {
    const requestId = ++loadRequestId.current;
    setLoading(true);
    setError("");
    try {
      const nextDetail = await api.share(token, password, path);
      if (requestId !== loadRequestId.current) return;
      setDetail(nextDetail);
      if (!path) {
        setRootLabel(nextDetail.itemCount > 1 ? "聚合分享" : nextDetail.name);
        setBreadcrumbs([]);
      }
    } catch (nextError) {
      if (requestId !== loadRequestId.current) return;
      setError(nextError instanceof Error ? nextError.message : "分享加载失败");
    } finally {
      if (requestId === loadRequestId.current) setLoading(false);
    }
  };

  useEffect(() => {
    setPath("");
    setDetail(null);
    setError("");
    setBreadcrumbs([]);
    setPreviewFile(null);
  }, [token]);

  useEffect(() => {
    void load();
  }, [token, path]);

  function unlock(event: FormEvent) {
    event.preventDefault();
    void load();
  }

  function open(file: FileEntry) {
    if (loading) return;
    const childPath = relativeSharePath(detail?.path || "", file.path);
    if (file.type === "folder") {
      setLoading(true);
      setBreadcrumbs((current) => {
        const existingIndex = current.findIndex(
          (crumb) => crumb.path === childPath,
        );
        if (existingIndex >= 0) return current.slice(0, existingIndex + 1);
        return [...current, { label: file.name, path: childPath }];
      });
      setPath(childPath);
      return;
    }
    const url = shareContentUrl(token, password, childPath);
    setPreviewFile({ file, url });
  }

  function goBack() {
    if (loading) return;
    const next = breadcrumbs.slice(0, -1);
    setLoading(true);
    setBreadcrumbs(next);
    setPath(next.at(-1)?.path || "");
  }

  function goToBreadcrumb(index: number) {
    if (loading) return;
    const targetPath = index < 0 ? "" : breadcrumbs[index]?.path || "";
    if (targetPath === path) return;
    setLoading(true);
    if (index < 0) {
      setBreadcrumbs([]);
      setPath("");
      return;
    }
    const next = breadcrumbs.slice(0, index + 1);
    setBreadcrumbs(next);
    setPath(next.at(-1)?.path || "");
  }

  const needsPassword = !detail && error.toLowerCase().includes("password");
  const initialLoading = loading && !detail;

  return (
    <div className="share-page">
      <header className="public-topbar glass">
        <Link to="/" className="brand">
          <BrandMark />
          <strong>XFile</strong>
        </Link>
        <div className="public-topbar-actions">
          <Badge tone="info">
            <Share20Regular />
            安全分享
          </Badge>
          <AppearanceControl />
        </div>
      </header>
      <main className="share-main">
        {initialLoading ? (
          <Loading label="正在打开分享" />
        ) : needsPassword ? (
          <form className="share-unlock glass-panel" onSubmit={unlock}>
            <span className="share-lock">
              <LockClosed20Regular />
            </span>
            <h1>此分享需要密码</h1>
            <p>输入分享密码后即可浏览或下载文件。</p>
            <Field label="访问密码">
              <input
                autoFocus
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </Field>
            <Button variant="primary" type="submit">
              验证并打开
            </Button>
          </form>
        ) : error && !detail ? (
          <div className="share-unlock glass-panel">
            <h1>无法打开分享</h1>
            <p>{error}</p>
            <Button onClick={load}>重试</Button>
          </div>
        ) : (
          detail && (
            <section
              className={`share-card glass-panel ${
                detail.type === "file" &&
                filePreviewKind(detail.name) === "video"
                  ? "share-card-media"
                  : ""
              }`}
            >
              <header>
                <div>
                  <span className="share-icon">
                    {detail.type === "folder" ? (
                      <Folder20Filled />
                    ) : (
                      <Share20Regular />
                    )}
                  </span>
                  <div>
                    <h1>{detail.name}</h1>
                    <p>{detail.description || "来自 XFile 的安全分享"}</p>
                  </div>
                </div>
                <div className="share-meta">
                  {detail.itemCount > 1 && (
                    <Badge tone="info">聚合分享 · {detail.itemCount} 项</Badge>
                  )}
                  <Badge tone={detail.protected ? "warning" : "success"}>
                    {detail.protected ? "密码保护" : "公开分享"}
                  </Badge>
                  <span>
                    {detail.expiresAt
                      ? `有效期至 ${formatTime(detail.expiresAt)}`
                      : "永久有效"}
                  </span>
                  {detail.maxAccessCount > 0 && (
                    <span>限 {detail.maxAccessCount} 次访问</span>
                  )}
                </div>
              </header>
              {detail.type === "file" ? (
                <div
                  className={`shared-preview ${
                    filePreviewKind(detail.name) === "video" ? "is-video" : ""
                  }`}
                >
                  <FilePreview
                    file={{
                      name: detail.name,
                      path: detail.path,
                      type: "file",
                      size: detail.size,
                      modifiedAt: detail.createdAt,
                      description: detail.description,
                    }}
                    url={shareContentUrl(token, password)}
                    settings={site?.preferences}
                    immersiveVideo
                  />
                  <div>
                    <span>
                      {formatBytes(detail.size)} · 创建于{" "}
                      {formatTime(detail.createdAt)}
                    </span>
                    <a
                      className="button button-secondary"
                      href={shareContentUrl(token, password)}
                      download={detail.name}
                    >
                      <ArrowDownload20Regular />
                      下载原文件
                    </a>
                  </div>
                </div>
              ) : (
                <>
                  <div className="share-toolbar">
                    <Button
                      icon={<ArrowLeft20Regular />}
                      disabled={loading || !path}
                      onClick={goBack}
                    >
                      返回上级
                    </Button>
                    <nav className="share-breadcrumbs" aria-label="分享路径">
                      <button
                        className="share-breadcrumb-root"
                        aria-current={
                          breadcrumbs.length === 0 ? "page" : undefined
                        }
                        disabled={loading || breadcrumbs.length === 0}
                        onClick={() => goToBreadcrumb(-1)}
                      >
                        {rootLabel}
                      </button>
                      {breadcrumbs.map((crumb, index) => (
                        <span key={crumb.path}>
                          <ChevronRight20Regular aria-hidden="true" />
                          <button
                            aria-current={
                              index === breadcrumbs.length - 1
                                ? "page"
                                : undefined
                            }
                            disabled={
                              loading || index === breadcrumbs.length - 1
                            }
                            onClick={() => goToBreadcrumb(index)}
                          >
                            {crumb.label}
                          </button>
                        </span>
                      ))}
                    </nav>
                  </div>
                  <div
                    className={`share-content-region${loading ? " is-loading" : ""}`}
                    aria-busy={loading}
                  >
                    {detail.files?.length ? (
                      <div className="share-list">
                        {detail.files.map((file) => (
                          <button
                            key={file.path}
                            disabled={loading}
                            onClick={() => open(file)}
                          >
                            <span>
                              {file.type === "folder" ? (
                                <Folder20Filled />
                              ) : filePreviewKind(file.name) === "video" ? (
                                <Video20Regular />
                              ) : (
                                <Share20Regular />
                              )}
                            </span>
                            <div>
                              <strong>{file.name}</strong>
                              <small>
                                {file.type === "folder"
                                  ? "文件夹"
                                  : formatBytes(file.size)}
                              </small>
                            </div>
                            <time>{formatTime(file.modifiedAt)}</time>
                          </button>
                        ))}
                      </div>
                    ) : (
                      <Empty title="文件夹为空" />
                    )}
                    {loading && (
                      <div className="share-list-loading" role="status">
                        <Loading label="正在加载目录" />
                      </div>
                    )}
                    {!loading && error && (
                      <div className="share-list-error" role="alert">
                        <span>{error}</span>
                        <Button onClick={load}>重试</Button>
                      </div>
                    )}
                  </div>
                </>
              )}
            </section>
          )
        )}
      </main>
      <footer className="public-footer">
        <span>由 XFile 安全分享</span>
        <Link to="/">返回首页</Link>
      </footer>
      <Modal
        open={!!previewFile}
        title={previewFile?.file.name || "文件预览"}
        description={
          previewFile
            ? `${formatBytes(previewFile.file.size)} · ${formatTime(previewFile.file.modifiedAt)}`
            : undefined
        }
        size="large"
        className={`modal-preview ${
          previewFile && filePreviewKind(previewFile.file.name) === "video"
            ? "modal-preview-video"
            : ""
        }`}
        bodyClassName="modal-preview-body"
        onClose={() => setPreviewFile(null)}
        footer={
          previewFile && (
            <>
              <a
                className="button button-secondary"
                href={previewFile.url}
                download={previewFile.file.name}
              >
                <ArrowDownload20Regular />
                下载原文件
              </a>
              <Button variant="primary" onClick={() => setPreviewFile(null)}>
                关闭
              </Button>
            </>
          )
        }
      >
        {previewFile && (
          <div className="xfile-preview">
            <FilePreview
              file={previewFile.file}
              url={previewFile.url}
              settings={site?.preferences}
              immersiveVideo
            />
          </div>
        )}
      </Modal>
    </div>
  );
}

function relativeSharePath(basePath: string, filePath: string) {
  const base = basePath.replace(/^\/+|\/+$/g, "");
  const target = filePath.replace(/^\/+|\/+$/g, "");
  if (!base || target === base) return base ? "" : target;
  return target.startsWith(`${base}/`) ? target.slice(base.length + 1) : target;
}
