import type { FileEntry } from "../types";
import {
  ArrowDownload20Regular,
  Document20Regular,
  Edit20Regular,
  MusicNote220Regular,
  Warning20Regular,
} from "@fluentui/react-icons";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import {
  buildExternalPreviewUrl,
  normalizeExternalPreviewBaseUrl,
  normalizeExternalPreviewProvider,
  onlyOfficeDocumentType,
  stablePreviewKey,
} from "../externalPreview";
import VideoPreview, { isVideoFileName } from "./VideoPreview";
import { Button, Loading } from "./ui";

export type FilePreviewKind =
  | "image"
  | "video"
  | "audio"
  | "pdf"
  | "text"
  | "office"
  | "unsupported";

const imageExtensions = new Set([
  "png",
  "jpg",
  "jpeg",
  "gif",
  "webp",
  "svg",
  "avif",
  "bmp",
  "tif",
  "tiff",
]);
const audioExtensions = new Set(["mp3", "wav", "ogg", "flac", "m4a", "aac"]);
const textExtensions = new Set([
  "txt",
  "md",
  "mdx",
  "json",
  "log",
  "yaml",
  "yml",
  "xml",
  "csv",
  "ini",
  "sql",
  "js",
  "jsx",
  "ts",
  "tsx",
  "css",
  "scss",
  "sass",
  "html",
  "htm",
  "vue",
  "go",
  "py",
  "java",
  "php",
  "sh",
]);
const officeExtensions = new Set([
  "doc",
  "docm",
  "docx",
  "dot",
  "dotm",
  "dotx",
  "odt",
  "rtf",
  "xls",
  "xlsm",
  "xlsx",
  "xlt",
  "xltm",
  "xltx",
  "ods",
  "odp",
  "pot",
  "potm",
  "potx",
  "pps",
  "ppsm",
  "ppsx",
  "ppt",
  "pptm",
  "pptx",
]);

function extension(name: string) {
  return name.split(".").pop()?.toLowerCase() || "";
}

export function filePreviewKind(name: string): FilePreviewKind {
  const ext = extension(name);
  if (imageExtensions.has(ext)) return "image";
  if (isVideoFileName(name)) return "video";
  if (audioExtensions.has(ext)) return "audio";
  if (ext === "pdf") return "pdf";
  if (textExtensions.has(ext)) return "text";
  if (officeExtensions.has(ext)) return "office";
  return "unsupported";
}

export default function FilePreview({
  file,
  url,
  settings = {},
  text,
  loading,
  saving = false,
  editable = false,
  immersiveVideo = false,
  onTextChange,
  onSaveText,
}: {
  file: FileEntry;
  url: string;
  settings?: Record<string, string>;
  text?: string;
  loading?: boolean;
  saving?: boolean;
  editable?: boolean;
  immersiveVideo?: boolean;
  onTextChange?: (value: string) => void;
  onSaveText?: () => void;
}) {
  const kind = filePreviewKind(file.name);
  const ext = extension(file.name);
  const [internalText, setInternalText] = useState("");
  const [internalLoading, setInternalLoading] = useState(false);
  const [internalError, setInternalError] = useState("");
  const controlledText = text !== undefined;
  const absoluteUrl = useMemo(
    () => new URL(url, window.location.href).toString(),
    [url],
  );
  const externalPreviewUrl = useMemo(
    () =>
      buildExternalPreviewUrl(settings, {
        name: file.name,
        ext,
        url: absoluteUrl,
      }),
    [absoluteUrl, ext, file.name, settings],
  );

  useEffect(() => {
    if (kind !== "text" || controlledText) return;
    const controller = new AbortController();
    setInternalLoading(true);
    setInternalError("");
    setInternalText("");
    fetch(url, {
      credentials: "same-origin",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) throw new Error(await response.text());
        return response.text();
      })
      .then(setInternalText)
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError")
          return;
        setInternalError(
          error instanceof Error ? error.message : "无法加载文本预览",
        );
      })
      .finally(() => {
        if (!controller.signal.aborted) setInternalLoading(false);
      });
    return () => controller.abort();
  }, [controlledText, kind, url]);

  if (kind === "image") {
    return (
      <div className="preview-media preview-image">
        <img src={url} alt={file.name} decoding="async" />
      </div>
    );
  }
  if (kind === "video") {
    return (
      <div className="preview-media preview-video">
        <VideoPreview src={url} name={file.name} immersive={immersiveVideo} />
      </div>
    );
  }
  if (kind === "audio") {
    return (
      <div className="preview-audio">
        <MusicNote220Regular />
        <strong>{file.name}</strong>
        <audio src={url} controls preload="metadata" />
      </div>
    );
  }
  if (kind === "pdf") {
    return (
      <iframe
        className="preview-frame"
        title={file.name}
        src={url}
        allowFullScreen
      />
    );
  }
  if (kind === "text") {
    const previewLoading = loading ?? internalLoading;
    const previewText = text ?? internalText;
    if (internalError) {
      return (
        <PreviewFallback
          file={file}
          url={url}
          message={`文本预览加载失败：${internalError}`}
        />
      );
    }
    return (
      <div className="text-preview">
        {previewLoading ? (
          <Loading label="正在读取文件" />
        ) : (
          <>
            <textarea
              aria-label={`${file.name} 文本内容`}
              readOnly={!editable}
              value={previewText}
              onChange={(event) => onTextChange?.(event.target.value)}
            />
            {editable && onSaveText && (
              <Button
                variant="primary"
                icon={<Edit20Regular />}
                loading={saving}
                onClick={onSaveText}
              >
                保存文本
              </Button>
            )}
          </>
        )}
      </div>
    );
  }
  if (kind === "office") {
    const provider = normalizeExternalPreviewProvider(
      settings.externalPreviewProvider,
    );
    if (externalPreviewUrl) {
      return (
        <iframe
          className="preview-frame preview-office-frame"
          title={file.name}
          src={externalPreviewUrl}
          allowFullScreen
        />
      );
    }
    if (provider === "onlyoffice") {
      const server = normalizeExternalPreviewBaseUrl(
        settings.externalPreviewBaseUrl,
      );
      const documentType = onlyOfficeDocumentType(ext);
      if (server && documentType) {
        return (
          <OnlyOfficePreview
            src={absoluteUrl}
            name={file.name}
            ext={ext}
            server={server}
          />
        );
      }
    }
    return (
      <PreviewFallback
        file={file}
        url={url}
        message="当前未配置 Office 在线预览服务"
      />
    );
  }
  return (
    <PreviewFallback file={file} url={url} message="当前格式暂不支持在线预览" />
  );
}

function PreviewFallback({
  file,
  url,
  message,
}: {
  file: FileEntry;
  url: string;
  message: string;
}) {
  return (
    <div className="preview-placeholder">
      <Document20Regular />
      <strong>{message}</strong>
      <span>不会离开当前页面，可直接下载原文件。</span>
      <a className="button button-secondary" href={url} download={file.name}>
        <ArrowDownload20Regular />
        下载原文件
      </a>
    </div>
  );
}

interface OnlyOfficeEditor {
  destroyEditor?: () => void;
}

interface OnlyOfficeEditorConfig {
  documentType: "word" | "cell" | "slide";
  document: {
    fileType: string;
    key: string;
    title: string;
    url: string;
    permissions: { download: boolean; edit: boolean; print: boolean };
  };
  editorConfig: { lang: string; mode: "view" };
  height: string;
  type: "desktop";
  width: string;
}

declare global {
  interface Window {
    DocsAPI?: {
      DocEditor: new (
        elementId: string,
        config: OnlyOfficeEditorConfig,
      ) => OnlyOfficeEditor;
    };
  }
}

const onlyOfficeScriptRequests = new Map<string, Promise<void>>();

function loadOnlyOfficeScript(src: string) {
  if (window.DocsAPI) return Promise.resolve();
  const existing = onlyOfficeScriptRequests.get(src);
  if (existing) return existing;
  const request = new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = src;
    script.async = true;
    script.addEventListener("load", () => resolve(), { once: true });
    script.addEventListener(
      "error",
      () => reject(new Error("OnlyOffice API 加载失败")),
      { once: true },
    );
    document.head.appendChild(script);
  }).catch((error) => {
    onlyOfficeScriptRequests.delete(src);
    throw error;
  });
  onlyOfficeScriptRequests.set(src, request);
  return request;
}

function OnlyOfficePreview({
  src,
  name,
  ext,
  server,
}: {
  src: string;
  name: string;
  ext: string;
  server: string;
}) {
  const reactId = useId();
  const editorId = useMemo(
    () => `onlyoffice-${reactId.replace(/[^a-zA-Z0-9_-]/g, "")}`,
    [reactId],
  );
  const editorRef = useRef<OnlyOfficeEditor | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    const documentType = onlyOfficeDocumentType(ext);
    setLoading(true);
    setError("");
    void loadOnlyOfficeScript(`${server}/web-apps/apps/api/documents/api.js`)
      .then(() => {
        if (!active || !window.DocsAPI || !documentType) {
          throw new Error("OnlyOffice API 不可用");
        }
        editorRef.current = new window.DocsAPI.DocEditor(editorId, {
          documentType,
          document: {
            fileType: ext,
            key: stablePreviewKey(`${name}:${src}`),
            title: name,
            url: src,
            permissions: { download: true, edit: false, print: true },
          },
          editorConfig: { lang: "zh-CN", mode: "view" },
          height: "100%",
          type: "desktop",
          width: "100%",
        });
      })
      .catch((nextError: unknown) => {
        if (!active) return;
        setError(
          nextError instanceof Error
            ? nextError.message
            : "OnlyOffice 预览失败",
        );
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      editorRef.current?.destroyEditor?.();
      editorRef.current = null;
    };
  }, [editorId, ext, name, server, src]);

  return (
    <div className="onlyoffice-preview">
      <div id={editorId} className="onlyoffice-editor" />
      {loading && (
        <div className="preview-loading-overlay">
          <Loading label="正在启动 Office 预览" />
        </div>
      )}
      {error && (
        <div className="preview-loading-overlay preview-error-overlay">
          <Warning20Regular />
          <strong>{error}</strong>
        </div>
      )}
    </div>
  );
}
