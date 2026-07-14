import type { FileEntry, StorageSource } from "../types";
import {
  AppsList20Regular,
  Archive20Regular,
  ArrowDownload20Regular,
  ArrowLeft20Regular,
  ArrowMove20Regular,
  ArrowReset20Regular,
  ArrowRotateClockwise20Regular,
  ArrowRotateCounterclockwise20Regular,
  ArrowSort20Regular,
  ArrowSync20Regular,
  ArrowUpload20Regular,
  BookOpen20Regular,
  Braces20Regular,
  Checkmark20Regular,
  ChevronRight20Regular,
  ClipboardLink20Regular,
  Copy20Regular,
  Delete20Regular,
  Document20Regular,
  DocumentAdd20Regular,
  DocumentCsv20Regular,
  DocumentCss20Regular,
  DocumentJavascript20Regular,
  DocumentMarkdown20Regular,
  DocumentPdf20Regular,
  DocumentTable20Regular,
  DocumentText20Regular,
  DocumentWord20Regular,
  Edit20Regular,
  Eye20Regular,
  Folder20Filled,
  FolderAdd20Regular,
  FolderOpen20Regular,
  Grid20Regular,
  GroupList20Regular,
  Image20Regular,
  Link20Regular,
  MegaphoneLoud20Regular,
  MoreHorizontal20Regular,
  MusicNote220Regular,
  Open20Regular,
  PersonArrowRight20Regular,
  Save20Regular,
  Search20Regular,
  Settings20Regular,
  Share20Regular,
  SignOut20Regular,
  SlideText20Regular,
  Star20Filled,
  Star20Regular,
  StarAdd20Regular,
  Table20Regular,
  TextBulletList20Regular,
  Video20Regular,
  WindowNew20Regular,
  ZoomIn20Regular,
  ZoomOut20Regular,
} from "@fluentui/react-icons";
import hljs from "highlight.js/lib/core";
import javascriptLanguage from "highlight.js/lib/languages/javascript";
import jsonLanguage from "highlight.js/lib/languages/json";
import markdownLanguage from "highlight.js/lib/languages/markdown";
import typescriptLanguage from "highlight.js/lib/languages/typescript";
import xmlLanguage from "highlight.js/lib/languages/xml";
import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import Editor from "react-simple-code-editor";
import { QRCodeSVG } from "qrcode.react";
import {
  api,
  contentUrl,
  downloadArchive,
  downloadPublicArchive,
  formatBytes,
  formatTime,
  publicContentUrl,
  upload,
} from "../api";
import AppearanceControl from "../components/AppearanceControl";
import BrandMark from "../components/BrandMark";
import FilePreview, { filePreviewKind } from "../components/FilePreview";
import VideoPreview, { isVideoFileName } from "../components/VideoPreview";
import {
  detectNovelDocument,
  novelExtensions,
  type NovelDocument,
} from "../novel";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Field,
  IconButton,
  Loading,
  Modal,
  Popover,
} from "../components/ui";
import { useApp, useToast } from "../state";

hljs.registerLanguage("json", jsonLanguage);
hljs.registerLanguage("javascript", javascriptLanguage);
hljs.registerLanguage("typescript", typescriptLanguage);
hljs.registerLanguage("markdown", markdownLanguage);
hljs.registerLanguage("xml", xmlLanguage);

type DialogKind =
  | "folder"
  | "document"
  | "rename"
  | "move"
  | "copy"
  | "metadata"
  | null;
type ViewMode = "list" | "gallery";
type SortField = "name" | "size" | "modifiedAt";
type GroupMode = "none" | "time" | "size" | "type";
type FileMenuActionKey =
  | "open"
  | "openNewTab"
  | "packageDownload"
  | "favorite"
  | "share"
  | "rename"
  | "move"
  | "copy"
  | "delete";
type Breadcrumb = { label: string; value: string; root?: boolean };
type GeneratedLink = {
  label: string;
  value: string;
  kind: "share" | "direct";
};
type QuickNavKey = "shares" | "delivery" | "favorites";

const fileMenuOptions: Array<{ key: FileMenuActionKey; label: string }> = [
  { key: "open", label: "打开" },
  { key: "openNewTab", label: "新标签打开" },
  { key: "packageDownload", label: "打包下载" },
  { key: "favorite", label: "收藏" },
  { key: "share", label: "创建分享" },
  { key: "rename", label: "重命名" },
  { key: "move", label: "移动" },
  { key: "copy", label: "复制" },
  { key: "delete", label: "删除" },
];

const documentPresets = [
  { label: "文本文档", extension: "txt", icon: <DocumentText20Regular /> },
  { label: "Word 文档", extension: "docx", icon: <DocumentWord20Regular /> },
  {
    label: "Markdown",
    extension: "md",
    icon: <DocumentMarkdown20Regular />,
  },
  { label: "JSON", extension: "json", icon: <Braces20Regular /> },
  { label: "CSV 表格", extension: "csv", icon: <DocumentCsv20Regular /> },
];

const blankDocxBase64 =
  "UEsDBBQAAAAIAMyS7Vx5bjPX6AAAAK0BAAATAAAAW0NvbnRlbnRfVHlwZXNdLnhtbH1QyU7DMBD9FWuuKHHggBCK0wPLETiUDxjZk8SqN3nc0v49Tlt6QIXjzFv1+tXeO7GjzDYGBbdtB4KCjsaGScHn+rV5AMEFg0EXAyk4EMNq6NeHRCyqNrCCuZT0KCXrmTxyGxOFiowxeyz1zJNMqDc4kbzrunupYygUSlMWDxj6Zxpx64p42df3qUcmxyCeTsQlSwGm5KzGUnG5C+ZXSnNOaKvyyOHZJr6pBJBXExbk74Cz7r0Ok60h8YG5vKGvLPkVs5Em6q2vyvZ/mys94zhaTRf94pZy1MRcF/euvSAebfjpL49zD99QSwMEFAAAAAgAzJLtXJv9N+qtAAAAKQEAAAsAAABfcmVscy8ucmVsc43POw7CMAwG4KtE3mlaBoRQ0y4IqSsqB7ASN61oHkrCo7cnAwNFDIy2f3+W6/ZpZnanECdnBVRFCYysdGqyWsClP232wGJCq3B2lgQsFKFt6jPNmPJKHCcfWTZsFDCm5A+cRzmSwVg4TzZPBhcMplwGzT3KK2ri27Lc8fBpwNpknRIQOlUB6xdP/9huGCZJRydvhmz6ceIrkWUMmpKAhwuKq3e7yCzwpuarF5sXUEsDBBQAAAAIAMyS7VxPy+5X3AAAAFMBAAARAAAAd29yZC9kb2N1bWVudC54bWxNkMFOwzAMhl8l8p0lHVMp1brduE1CAh4ga9ykUhNXibcynp5kCHUX259t/b/l/fHbT+KKMY0UOqg2CgSGnswYbAdfn29PDYjEOhg9UcAObpjgeNgvraH+4jGwyAIhtUsHjnlupUy9Q6/ThmYMeTZQ9JozRisXimaO1GNKWd9PcqtULb0eAxTJM5lbybMsMWHP7/HO9uNHLMWiql5VDbl2ua6b5wbk38JJx9xlmnN/t1NlJY7W8YpnYia/8oTDw9ShNhg7eFFNwYGIH9Be+I6q2Mn1NPl/s1z/cfgFUEsBAhQAFAAAAAgAzJLtXHluM9foAAAArQEAABMAAAAAAAAAAAAAAAAAAAAAAFtDb250ZW50X1R5cGVzXS54bWxQSwECFAAUAAAACADMku1cm/036q0AAAApAQAACwAAAAAAAAAAAAAAAAAZAQAAX3JlbHMvLnJlbHNQSwECFAAUAAAACADMku1cT8vuV9wAAABTAQAAEQAAAAAAAAAAAAAAAADvAQAAd29yZC9kb2N1bWVudC54bWxQSwUGAAAAAAMAAwC5AAAA+gIAAAAA";

function createBlankDocx(name: string) {
  const binary = atob(blankDocxBase64);
  const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
  return new File([bytes], name, {
    type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  });
}

const quickNavOptions: Array<{
  key: QuickNavKey;
  label: string;
  to: string;
  icon: React.ReactNode;
}> = [
  { key: "shares", label: "我的分享", to: "/shares", icon: <Share20Regular /> },
  {
    key: "delivery",
    label: "我的短链",
    to: "/delivery",
    icon: <Link20Regular />,
  },
  {
    key: "favorites",
    label: "我的收藏",
    to: "/favorites",
    icon: <Star20Regular />,
  },
];

function joinPath(parent: string, name: string) {
  return [parent, name]
    .filter(Boolean)
    .join("/")
    .replace(/\/{2,}/g, "/");
}

function parentPath(path: string) {
  const parts = path.split("/").filter(Boolean);
  parts.pop();
  return parts.join("/");
}

function extension(file: FileEntry) {
  return file.name.split(".").pop()?.toLowerCase() || "";
}

const textPreviewExtensions = [
  "txt",
  "md",
  "mdx",
  "json",
  "js",
  "jsx",
  "ts",
  "tsx",
  "log",
  "yaml",
  "yml",
  "xml",
  "csv",
];

const formattableExtensions = new Set([
  "json",
  "js",
  "jsx",
  "ts",
  "tsx",
  "md",
  "mdx",
]);

function codeLanguageLabel(ext: string) {
  return (
    {
      json: "JSON",
      js: "JavaScript",
      jsx: "JSX",
      ts: "TypeScript",
      tsx: "TSX",
      md: "Markdown",
      mdx: "MDX",
      yaml: "YAML",
      yml: "YAML",
      xml: "XML",
      csv: "CSV",
      log: "LOG",
      txt: "TEXT",
    }[ext] || ext.toUpperCase()
  );
}

function escapeCode(value: string) {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function decodeTextBuffer(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer);
  if (bytes[0] === 0xef && bytes[1] === 0xbb && bytes[2] === 0xbf) {
    return new TextDecoder("utf-8").decode(bytes.subarray(3));
  }
  if (bytes[0] === 0xff && bytes[1] === 0xfe) {
    return new TextDecoder("utf-16le").decode(bytes.subarray(2));
  }
  if (bytes[0] === 0xfe && bytes[1] === 0xff) {
    return new TextDecoder("utf-16be").decode(bytes.subarray(2));
  }
  try {
    return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return new TextDecoder("gb18030").decode(bytes);
  }
}

function highlightCode(value: string, ext: string) {
  const language =
    ext === "json"
      ? "json"
      : ["js", "jsx"].includes(ext)
        ? "javascript"
        : ["ts", "tsx"].includes(ext)
          ? "typescript"
          : ["md", "mdx"].includes(ext)
            ? "markdown"
            : ext === "xml"
              ? "xml"
              : undefined;
  return language
    ? hljs.highlight(value, { language, ignoreIllegals: true }).value
    : escapeCode(value);
}

async function formatTextContent(value: string, ext: string) {
  const prettier = await import("prettier/standalone");
  if (["md", "mdx"].includes(ext)) {
    const markdown = await import("prettier/plugins/markdown");
    return prettier.format(value, {
      parser: ext === "mdx" ? "mdx" : "markdown",
      plugins: [markdown],
      proseWrap: "preserve",
    });
  }

  const estree = await import("prettier/plugins/estree");
  if (["ts", "tsx"].includes(ext)) {
    const typescript = await import("prettier/plugins/typescript");
    return prettier.format(value, {
      parser: "typescript",
      plugins: [typescript, estree.default],
    });
  }

  const babel = await import("prettier/plugins/babel");
  return prettier.format(value, {
    parser: ext === "json" ? "json" : "babel",
    plugins: [babel, estree.default],
  });
}

const largeTextPreviewThreshold = 768 * 1024;
const longChapterThreshold = 160 * 1024;
const chapterNavigationWindow = 240;

type NovelAnalysisState = {
  source: string;
  ext: string;
  document: NovelDocument | null;
  pending: boolean;
};

function useNovelAnalysis(value: string, ext: string) {
  const eligible = novelExtensions.has(ext);
  const largeTextMode = value.length >= largeTextPreviewThreshold;
  const synchronousDocument = useMemo(
    () => (eligible && !largeTextMode ? detectNovelDocument(value, ext) : null),
    [eligible, ext, largeTextMode, value],
  );
  const [analysis, setAnalysis] = useState<NovelAnalysisState>({
    source: "",
    ext: "",
    document: null,
    pending: false,
  });

  useEffect(() => {
    if (!eligible || !largeTextMode) return;

    let disposed = false;
    let worker: Worker | null = null;
    setAnalysis({ source: value, ext, document: null, pending: true });
    const timer = window.setTimeout(() => {
      worker = new Worker(
        new URL("../workers/novelParser.worker.ts", import.meta.url),
        { type: "module" },
      );
      worker.onmessage = (
        event: MessageEvent<{ document: NovelDocument | null }>,
      ) => {
        if (disposed) return;
        setAnalysis({
          source: value,
          ext,
          document: event.data.document,
          pending: false,
        });
        worker?.terminate();
      };
      worker.onerror = () => {
        if (disposed) return;
        setAnalysis({ source: value, ext, document: null, pending: false });
        worker?.terminate();
      };
      worker.postMessage({ value, ext });
    }, 60);

    return () => {
      disposed = true;
      window.clearTimeout(timer);
      worker?.terminate();
    };
  }, [eligible, ext, largeTextMode, value]);

  const analysisIsCurrent = analysis.source === value && analysis.ext === ext;
  return {
    document: largeTextMode
      ? analysisIsCurrent
        ? analysis.document
        : null
      : synchronousDocument,
    pending:
      eligible && largeTextMode && (!analysisIsCurrent || analysis.pending),
    largeTextMode,
  };
}

function fileCategory(file: FileEntry) {
  if (file.type === "folder") return "文件夹";
  const ext = extension(file);
  if (["png", "jpg", "jpeg", "gif", "webp", "svg", "avif"].includes(ext))
    return "图片";
  if (isVideoFileName(file.name)) return "视频";
  if (["mp3", "wav", "ogg", "flac", "m4a"].includes(ext)) return "音频";
  if (["zip", "rar", "7z", "tar", "gz", "bz2", "xz"].includes(ext))
    return "压缩包";
  if (
    [
      "txt",
      "md",
      "json",
      "log",
      "yaml",
      "yml",
      "xml",
      "csv",
      "doc",
      "docx",
      "xls",
      "xlsx",
      "ppt",
      "pptx",
      "pdf",
    ].includes(ext)
  )
    return "文档";
  if (
    [
      "js",
      "jsx",
      "ts",
      "tsx",
      "css",
      "scss",
      "html",
      "go",
      "py",
      "java",
    ].includes(ext)
  )
    return "代码";
  return "其他";
}

function groupFiles(files: FileEntry[], mode: GroupMode) {
  if (mode === "none") return [{ key: "all", label: "", files }];

  const now = new Date();
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const definitions =
    mode === "time"
      ? [
          {
            key: "today",
            label: "今天",
            matches: (file: FileEntry) =>
              new Date(file.modifiedAt).getTime() >= todayStart.getTime(),
          },
          {
            key: "yesterday",
            label: "昨天",
            matches: (file: FileEntry) => {
              const age =
                todayStart.getTime() - new Date(file.modifiedAt).getTime();
              return age > 0 && age <= 24 * 60 * 60 * 1000;
            },
          },
          {
            key: "week",
            label: "本周",
            matches: (file: FileEntry) => {
              const age = now.getTime() - new Date(file.modifiedAt).getTime();
              return (
                age > 24 * 60 * 60 * 1000 && age <= 7 * 24 * 60 * 60 * 1000
              );
            },
          },
          {
            key: "month",
            label: "本月",
            matches: (file: FileEntry) => {
              const age = now.getTime() - new Date(file.modifiedAt).getTime();
              return (
                age > 7 * 24 * 60 * 60 * 1000 && age <= 30 * 24 * 60 * 60 * 1000
              );
            },
          },
          { key: "earlier", label: "更早", matches: () => true },
        ]
      : mode === "size"
        ? [
            {
              key: "folder",
              label: "文件夹",
              matches: (file: FileEntry) => file.type === "folder",
            },
            {
              key: "small",
              label: "小文件 · 小于 1 MB",
              matches: (file: FileEntry) =>
                file.type === "file" && file.size < 1024 * 1024,
            },
            {
              key: "medium",
              label: "中等文件 · 1 MB - 100 MB",
              matches: (file: FileEntry) =>
                file.type === "file" &&
                file.size >= 1024 * 1024 &&
                file.size < 100 * 1024 * 1024,
            },
            {
              key: "large",
              label: "大文件 · 100 MB 以上",
              matches: () => true,
            },
          ]
        : [
            "文件夹",
            "图片",
            "文档",
            "代码",
            "视频",
            "音频",
            "压缩包",
            "其他",
          ].map((label) => ({
            key: label,
            label,
            matches: (file: FileEntry) => fileCategory(file) === label,
          }));

  const remaining = [...files];
  return definitions.flatMap((definition) => {
    const matches = remaining.filter(definition.matches);
    if (!matches.length) return [];
    const matchedPaths = new Set(matches.map((file) => file.path));
    for (let index = remaining.length - 1; index >= 0; index -= 1) {
      if (matchedPaths.has(remaining[index].path)) remaining.splice(index, 1);
    }
    return [{ key: definition.key, label: definition.label, files: matches }];
  });
}

function FileGlyph({ file }: { file: FileEntry }) {
  if (file.type === "folder")
    return (
      <span className="file-glyph folder">
        <Folder20Filled />
      </span>
    );
  const ext = extension(file);
  if (["png", "jpg", "jpeg", "gif", "webp", "svg", "avif"].includes(ext))
    return (
      <span className="file-glyph image">
        <Image20Regular />
      </span>
    );
  if (ext === "pdf")
    return (
      <span className="file-glyph pdf">
        <DocumentPdf20Regular />
      </span>
    );
  if (["doc", "docx"].includes(ext))
    return (
      <span className="file-glyph word">
        <DocumentWord20Regular />
      </span>
    );
  if (["xls", "xlsx", "ods"].includes(ext))
    return (
      <span className="file-glyph spreadsheet">
        <DocumentTable20Regular />
      </span>
    );
  if (["ppt", "pptx", "odp"].includes(ext))
    return (
      <span className="file-glyph presentation">
        <SlideText20Regular />
      </span>
    );
  if (ext === "csv")
    return (
      <span className="file-glyph spreadsheet">
        <DocumentCsv20Regular />
      </span>
    );
  if (ext === "md")
    return (
      <span className="file-glyph markdown">
        <DocumentMarkdown20Regular />
      </span>
    );
  if (["txt", "log", "rtf"].includes(ext))
    return (
      <span className="file-glyph text">
        <DocumentText20Regular />
      </span>
    );
  if (ext === "json")
    return (
      <span className="file-glyph code">
        <Braces20Regular />
      </span>
    );
  if (["js", "jsx", "ts", "tsx"].includes(ext))
    return (
      <span className="file-glyph code">
        <DocumentJavascript20Regular />
      </span>
    );
  if (["css", "scss", "sass"].includes(ext))
    return (
      <span className="file-glyph code">
        <DocumentCss20Regular />
      </span>
    );
  if (["zip", "rar", "7z", "tar", "gz", "bz2", "xz"].includes(ext))
    return (
      <span className="file-glyph archive">
        <Archive20Regular />
      </span>
    );
  if (isVideoFileName(file.name))
    return (
      <span className="file-glyph video">
        <Video20Regular />
      </span>
    );
  if (["mp3", "wav", "ogg", "flac", "m4a"].includes(ext))
    return (
      <span className="file-glyph audio">
        <MusicNote220Regular />
      </span>
    );
  return (
    <span className="file-glyph document">
      <Document20Regular />
    </span>
  );
}

export default function ExplorerPage({
  publicMode = false,
}: {
  publicMode?: boolean;
}) {
  const { site, refresh, session } = useApp();
  const { show } = useToast();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const uploadInput = useRef<HTMLInputElement>(null);
  const dragDepth = useRef(0);
  const locationInitialized = useRef(false);
  const loadRequestId = useRef(0);
  const preferences = site?.preferences || {};
  const [sources, setSources] = useState<StorageSource[]>([]);
  const [sourceKey, setSourceKey] = useState("");
  const [path, setPath] = useState("");
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [keyword, setKeyword] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [viewMode, setViewMode] = useState<ViewMode>(() =>
    localStorage.getItem("xfile-view-mode") === "gallery" ? "gallery" : "list",
  );
  const [groupMode, setGroupMode] = useState<GroupMode>(() => {
    const saved = localStorage.getItem("xfile-group-mode") as GroupMode | null;
    return saved && ["none", "time", "size", "type"].includes(saved)
      ? saved
      : "none";
  });
  const [sortField, setSortField] = useState<SortField>("name");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc");
  const [displayLimit, setDisplayLimit] = useState(1000);
  const [dialog, setDialog] = useState<DialogKind>(null);
  const [dialogValue, setDialogValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [previewFile, setPreviewFile] = useState<FileEntry | null>(null);
  const [previewText, setPreviewText] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewSaving, setPreviewSaving] = useState(false);
  const [imageScale, setImageScale] = useState(1);
  const [imageRotation, setImageRotation] = useState(0);
  const [generatedLinks, setGeneratedLinks] = useState<GeneratedLink[]>([]);
  const [uploadProgress, setUploadProgress] = useState<Record<string, number>>(
    {},
  );
  const [dragActive, setDragActive] = useState(false);
  const [directoryPassword, setDirectoryPassword] = useState("");
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [docsOpen, setDocsOpen] = useState(false);
  const [customMenuOpen, setCustomMenuOpen] = useState(false);
  const [favoriteIds, setFavoriteIds] = useState<Record<string, number>>({});
  const [favoritePulse, setFavoritePulse] = useState<string[]>([]);
  const [fileMenuItems, setFileMenuItems] = useState<FileMenuActionKey[]>(
    () => {
      try {
        const saved = JSON.parse(
          localStorage.getItem("xfile-context-menu-items") || "[]",
        ) as FileMenuActionKey[];
        const valid = saved.filter((key) =>
          fileMenuOptions.some((option) => option.key === key),
        );
        return valid.length
          ? valid
          : fileMenuOptions.map((option) => option.key);
      } catch {
        return fileMenuOptions.map((option) => option.key);
      }
    },
  );
  const [quickNav, setQuickNav] = useState<QuickNavKey[]>(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("xfile-home-navigation") || "[]",
      ) as QuickNavKey[];
      const valid = saved.filter((key) =>
        quickNavOptions.some((option) => option.key === key),
      );
      return valid.length ? valid : quickNavOptions.map((option) => option.key);
    } catch {
      return quickNavOptions.map((option) => option.key);
    }
  });
  const [contextMenu, setContextMenu] = useState<{
    x: number;
    y: number;
    file?: FileEntry;
  } | null>(null);

  useEffect(() => {
    setImageScale(1);
    setImageRotation(0);
  }, [previewFile?.path]);

  useEffect(() => {
    const next = (site?.sources || []).filter(
      (source) => source.enabled && (!publicMode || source.public),
    );
    setSources(next);
    const rootShowsSources =
      (site?.preferences?.rootShowStorage || "enabled") === "enabled";
    if (next.length === 1 || !rootShowsSources)
      setSourceKey((current) => current || next[0]?.key || "");
    else if (sourceKey && !next.some((source) => source.key === sourceKey))
      setSourceKey("");
  }, [site, publicMode, sourceKey]);

  useEffect(() => {
    localStorage.setItem("xfile-home-navigation", JSON.stringify(quickNav));
  }, [quickNav]);

  useEffect(() => {
    localStorage.setItem("xfile-view-mode", viewMode);
  }, [viewMode]);

  useEffect(() => {
    localStorage.setItem("xfile-group-mode", groupMode);
  }, [groupMode]);

  useEffect(() => {
    const resetDragState = () => {
      dragDepth.current = 0;
      setDragActive(false);
    };
    window.addEventListener("dragend", resetDragState);
    window.addEventListener("drop", resetDragState);
    return () => {
      window.removeEventListener("dragend", resetDragState);
      window.removeEventListener("drop", resetDragState);
    };
  }, []);

  useEffect(() => {
    localStorage.setItem(
      "xfile-context-menu-items",
      JSON.stringify(fileMenuItems),
    );
  }, [fileMenuItems]);

  useEffect(() => {
    if (locationInitialized.current || !sources.length) return;
    const requestedSource = searchParams.get("source") || "";
    const requestedPath = searchParams.get("path") || "";
    if (
      requestedSource &&
      sources.some((source) => source.key === requestedSource)
    ) {
      setSourceKey(requestedSource);
      setPath(requestedPath);
    }
    locationInitialized.current = true;
  }, [searchParams, sources]);

  useEffect(() => {
    setSortField((preferences.defaultSortField as SortField) || "name");
    setSortOrder(preferences.defaultSortOrder === "desc" ? "desc" : "asc");
    setDisplayLimit(Number(preferences.maxShowSize) || 1000);
  }, [
    preferences.defaultSortField,
    preferences.defaultSortOrder,
    preferences.maxShowSize,
  ]);

  useEffect(() => {
    const close = () => setContextMenu(null);
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setContextMenu(null);
    };
    window.addEventListener("click", close);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("resize", close);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, []);

  const loadFavorites = useCallback(async () => {
    if (publicMode || !session?.authenticated || !sourceKey) {
      setFavoriteIds({});
      return;
    }
    try {
      const favorites = await api.favorites();
      setFavoriteIds(
        Object.fromEntries(
          favorites
            .filter((favorite) => favorite.storageKey === sourceKey)
            .map((favorite) => [favorite.path, favorite.id]),
        ),
      );
    } catch {
      setFavoriteIds({});
    }
  }, [publicMode, session?.authenticated, sourceKey]);

  useEffect(() => {
    void loadFavorites();
  }, [loadFavorites]);

  const load = useCallback(async () => {
    const requestId = ++loadRequestId.current;
    if (!sourceKey) {
      setLoading(false);
      setFiles([]);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const list = publicMode
        ? await api.publicEntries(sourceKey, path, directoryPassword)
        : await api.entries(sourceKey, path);
      if (requestId !== loadRequestId.current) return;
      setFiles(list);
      setSelected([]);
    } catch (nextError) {
      if (requestId !== loadRequestId.current) return;
      setError(
        nextError instanceof Error ? nextError.message : "文件列表加载失败",
      );
    } finally {
      if (requestId === loadRequestId.current) setLoading(false);
    }
  }, [sourceKey, path, publicMode, directoryPassword]);

  useEffect(() => {
    void load();
  }, [load]);

  const visibleFiles = useMemo(() => {
    const term = keyword.trim().toLowerCase();
    const filtered = term
      ? files.filter((file) =>
          `${file.name} ${file.description || ""}`.toLowerCase().includes(term),
        )
      : files;
    return [...filtered].sort((left, right) => {
      let result = 0;
      if (sortField === "size") result = left.size - right.size;
      else if (sortField === "modifiedAt")
        result =
          new Date(left.modifiedAt).getTime() -
          new Date(right.modifiedAt).getTime();
      else
        result = left.name.localeCompare(right.name, "zh-CN", {
          numeric: true,
        });
      if (left.type !== right.type) result = left.type === "folder" ? -1 : 1;
      return sortOrder === "asc" ? result : -result;
    });
  }, [files, keyword, sortField, sortOrder]);
  const displayedFiles = visibleFiles.slice(0, displayLimit);
  const displayedGroups = useMemo(
    () => groupFiles(displayedFiles, groupMode),
    [displayedFiles, groupMode],
  );
  const selectedFiles = useMemo(
    () => files.filter((file) => selected.includes(file.path)),
    [files, selected],
  );
  const activeSource = sources.find((source) => source.key === sourceKey);
  const showLinkActions =
    !publicMode && preferences.showLinkButton !== "disabled";
  const showShareAction =
    showLinkActions && preferences.showShortLink !== "disabled";
  const showDirectAction =
    showLinkActions && preferences.showPathLink !== "disabled";
  const breadcrumbs = useMemo<Breadcrumb[]>(
    () => [
      { label: site?.rootName || "首页", value: "", root: true },
      ...(sourceKey
        ? [{ label: activeSource?.name || sourceKey, value: "" }]
        : []),
      ...path
        .split("/")
        .filter(Boolean)
        .map((part, index, parts) => ({
          label: part,
          value: parts.slice(0, index + 1).join("/"),
        })),
    ],
    [path, site?.rootName, sourceKey, activeSource?.name],
  );

  useEffect(() => {
    setPreviewText("");
    if (
      !previewFile ||
      previewFile.type !== "file" ||
      !textPreviewExtensions.includes(extension(previewFile))
    )
      return;
    let disposed = false;
    const controller = new AbortController();
    setPreviewLoading(true);
    fetch(currentUrl(previewFile, true), {
      credentials: "same-origin",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) throw new Error(await response.text());
        return decodeTextBuffer(await response.arrayBuffer());
      })
      .then((value) => {
        if (!disposed) setPreviewText(value);
      })
      .catch((nextError) => {
        if (!disposed && (nextError as Error).name !== "AbortError") {
          setPreviewText("无法加载文本预览");
        }
      })
      .finally(() => {
        if (!disposed) setPreviewLoading(false);
      });
    return () => {
      disposed = true;
      controller.abort();
    };
  }, [previewFile?.path, sourceKey, publicMode, directoryPassword]);

  function currentUrl(file: FileEntry, preview = false) {
    return publicMode
      ? publicContentUrl(sourceKey, file.path, directoryPassword, preview)
      : contentUrl(sourceKey, file.path, preview);
  }

  function downloadFile(file: FileEntry) {
    const anchor = document.createElement("a");
    anchor.href = currentUrl(file);
    anchor.download = file.name;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
  }

  function toggle(file: FileEntry, checked?: boolean) {
    setSelected((current) => {
      const exists = current.includes(file.path);
      const shouldSelect = checked ?? !exists;
      return shouldSelect
        ? exists
          ? current
          : [...current, file.path]
        : current.filter((value) => value !== file.path);
    });
  }

  function open(file: FileEntry) {
    if (file.type === "folder") {
      setPath(file.path);
      return;
    }
    setPreviewFile(file);
  }

  function openInNewTab(file: FileEntry) {
    const url =
      file.type === "folder"
        ? `${location.origin}/?source=${encodeURIComponent(sourceKey)}&path=${encodeURIComponent(file.path)}`
        : currentUrl(file, true);
    window.open(url, "_blank", "noopener,noreferrer");
    setContextMenu(null);
  }

  function activate(file: FileEntry) {
    const mobile = window.matchMedia("(max-width: 760px)").matches;
    const mode = mobile
      ? preferences.mobileFileClickMode || "click"
      : preferences.fileClickMode || "dbclick";
    if (mode === "click") open(file);
    else toggle(file);
  }

  function sortBy(field: SortField) {
    if (sortField === field)
      setSortOrder((current) => (current === "asc" ? "desc" : "asc"));
    else {
      setSortField(field);
      setSortOrder("asc");
    }
  }

  async function handleUpload(list: FileList | File[] | null) {
    if (!list?.length) return;
    if (publicMode) {
      show("请先登录后再上传文件", "info");
      return;
    }
    if (!sourceKey) {
      show("请先选择一个存储空间", "info");
      return;
    }
    for (const file of Array.from(list)) {
      setUploadProgress((current) => ({ ...current, [file.name]: 1 }));
      try {
        await upload(sourceKey, path, file, (percent) =>
          setUploadProgress((current) => ({
            ...current,
            [file.name]: percent,
          })),
        );
        show(`${file.name} 上传完成`, "success");
      } catch (nextError) {
        show(
          nextError instanceof Error
            ? nextError.message
            : `${file.name} 上传失败`,
          "error",
        );
      } finally {
        window.setTimeout(
          () =>
            setUploadProgress((current) => {
              const next = { ...current };
              delete next[file.name];
              return next;
            }),
          1000,
        );
      }
    }
    await load();
  }

  function isFileDrag(dataTransfer: DataTransfer) {
    return (
      dataTransfer.files.length > 0 ||
      Array.from(dataTransfer.types).includes("Files")
    );
  }

  function includesDirectory(dataTransfer: DataTransfer) {
    return Array.from(dataTransfer.items).some((item) => {
      if (item.kind !== "file") return false;
      const entry = (
        item as DataTransferItem & {
          webkitGetAsEntry?: () => { isDirectory?: boolean } | null;
        }
      ).webkitGetAsEntry?.();
      return Boolean(entry?.isDirectory);
    });
  }

  function handleDragEnter(event: React.DragEvent<HTMLElement>) {
    if (!isFileDrag(event.dataTransfer)) return;
    event.preventDefault();
    event.stopPropagation();
    dragDepth.current += 1;
    setDragActive(true);
  }

  function handleDragOver(event: React.DragEvent<HTMLElement>) {
    if (!isFileDrag(event.dataTransfer)) return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = publicMode || !sourceKey ? "none" : "copy";
    setDragActive(true);
  }

  function handleDragLeave(event: React.DragEvent<HTMLElement>) {
    if (!isFileDrag(event.dataTransfer)) return;
    event.preventDefault();
    event.stopPropagation();
    dragDepth.current = Math.max(0, dragDepth.current - 1);
    if (dragDepth.current === 0) setDragActive(false);
  }

  async function handleDrop(event: React.DragEvent<HTMLElement>) {
    if (!isFileDrag(event.dataTransfer)) return;
    event.preventDefault();
    event.stopPropagation();
    dragDepth.current = 0;
    setDragActive(false);

    if (publicMode) {
      show("当前是公开浏览模式，请登录后上传", "info");
      return;
    }
    if (!sourceKey) {
      show("请先进入一个存储空间再上传", "info");
      return;
    }
    if (includesDirectory(event.dataTransfer)) {
      show("暂不支持直接拖入文件夹，请拖入一个或多个文件", "info");
      return;
    }

    const droppedFiles = Array.from(event.dataTransfer.files);
    if (!droppedFiles.length) {
      show("没有检测到可上传的文件", "info");
      return;
    }
    await handleUpload(droppedFiles);
  }

  async function submitDialog() {
    const value = dialogValue.trim();
    if (!dialog || (!value && !["move", "copy"].includes(dialog))) return;
    setSaving(true);
    try {
      if (dialog === "folder")
        await api.createFolder(sourceKey, joinPath(path, value));
      if (dialog === "document") {
        const targetPath = joinPath(path, value);
        const ext = value.split(".").pop()?.toLowerCase();
        if (ext === "docx") {
          const fileName = targetPath.split("/").pop() || value;
          await upload(
            sourceKey,
            parentPath(targetPath),
            createBlankDocx(fileName),
          );
        } else {
          await api.createDocument(sourceKey, targetPath);
          if (ext === "json") await api.saveText(sourceKey, targetPath, "{}\n");
          if (ext === "md")
            await api.saveText(sourceKey, targetPath, "# 新建文档\n");
        }
      }
      if (dialog === "rename" && selectedFiles[0])
        await api.moveEntry(
          sourceKey,
          selectedFiles[0].path,
          joinPath(parentPath(selectedFiles[0].path), value),
        );
      if (dialog === "move") await api.batchMove(sourceKey, selected, value);
      if (dialog === "copy") await api.batchCopy(sourceKey, selected, value);
      if (dialog === "metadata" && selectedFiles[0])
        await api.saveMetadata(sourceKey, selectedFiles[0].path, value);
      show("操作已完成", "success");
      setDialog(null);
      setDialogValue("");
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "操作失败",
        "error",
      );
    } finally {
      setSaving(false);
    }
  }

  function openDialog(kind: DialogKind, file?: FileEntry) {
    if (!kind) return;
    if (file) setSelected([file.path]);
    setDialog(kind);
    if (kind === "rename")
      setDialogValue(file?.name || selectedFiles[0]?.name || "");
    else if (kind === "metadata")
      setDialogValue(file?.description || selectedFiles[0]?.description || "");
    else setDialogValue("");
  }

  async function removeSelected(file?: FileEntry) {
    const targets = file ? [file] : selectedFiles;
    if (
      !targets.length ||
      !confirm(`确定删除选中的 ${targets.length} 项吗？此操作不可撤销。`)
    )
      return;
    try {
      for (const item of targets) await api.deleteEntry(sourceKey, item.path);
      show("文件已删除", "success");
      setPreviewFile(null);
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "删除失败",
        "error",
      );
    }
  }

  async function downloadFiles(targets: FileEntry[]) {
    if (!targets.length) return;
    const shouldConfirm =
      targets.length > 1
        ? preferences.enableBatchDownloadConfirm === "enabled"
        : targets[0].type === "folder"
          ? preferences.enablePackageDownloadConfirm === "enabled"
          : preferences.enableNormalDownloadConfirm === "enabled";
    if (shouldConfirm && !confirm(`确定下载 ${targets.length} 项吗？`)) return;
    if (targets.length === 1 && targets[0].type === "file") {
      downloadFile(targets[0]);
      return;
    }
    try {
      const paths = targets.map((item) => item.path);
      if (publicMode)
        await downloadPublicArchive(sourceKey, paths, directoryPassword);
      else await downloadArchive(sourceKey, paths);
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "下载失败",
        "error",
      );
    }
  }

  async function downloadPackage(targets: FileEntry[]) {
    if (!targets.length) return;
    setContextMenu(null);
    try {
      const paths = targets.map((item) => item.path);
      if (publicMode)
        await downloadPublicArchive(sourceKey, paths, directoryPassword);
      else await downloadArchive(sourceKey, paths);
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "打包下载失败",
        "error",
      );
    }
  }

  async function createShare(file?: FileEntry) {
    const targets = file
      ? [file]
      : selectedFiles.length
        ? selectedFiles
        : previewFile
          ? [previewFile]
          : [];
    if (!targets.length) return;
    try {
      const share =
        targets.length === 1
          ? await api.createShare({
              storageKey: sourceKey,
              path: targets[0].path,
            })
          : await api.batchShares(
              sourceKey,
              targets.map((target) => target.path),
            );
      setPreviewFile(null);
      setContextMenu(null);
      setGeneratedLinks([
        {
          label:
            targets.length > 1
              ? `聚合分享 · ${targets.length} 项`
              : `${share.path} · 分享链接`,
          value: `${location.origin}${share.url}`,
          kind: "share",
        },
      ]);
      show(
        targets.length > 1
          ? `已创建包含 ${targets.length} 项的聚合分享`
          : "分享链接已创建",
        "success",
      );
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "分享失败",
        "error",
      );
    }
  }

  async function createDirectLink(file?: FileEntry) {
    const targets = file
      ? [file]
      : selectedFiles.length
        ? selectedFiles
        : previewFile
          ? [previewFile]
          : [];
    if (!targets.length) return;
    try {
      const links =
        targets.length === 1
          ? [await api.createDeliveryLink(sourceKey, targets[0].path)]
          : await api.batchDeliveryLinks(
              sourceKey,
              targets.map((target) => target.path),
            );
      setPreviewFile(null);
      setContextMenu(null);
      setGeneratedLinks(
        links.map((link) => ({
          label: `${link.path} · 直链`,
          value: `${location.origin}${link.url}`,
          kind: "direct" as const,
        })),
      );
      show(`已创建 ${links.length} 个直链`, "success");
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "直链创建失败",
        "error",
      );
    }
  }

  async function toggleFavorite(file: FileEntry) {
    const existingId = favoriteIds[file.path];
    setContextMenu(null);
    setFavoritePulse((current) =>
      current.includes(file.path) ? current : [...current, file.path],
    );
    try {
      if (existingId) {
        await api.deleteFavorite(existingId);
        setFavoriteIds((current) => {
          const next = { ...current };
          delete next[file.path];
          return next;
        });
        show("已取消收藏", "success");
      } else {
        const favorite = await api.createFavorite(sourceKey, file.path);
        setFavoriteIds((current) => ({
          ...current,
          [file.path]: favorite.id,
        }));
        show("已加入我的收藏", "success");
      }
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "收藏失败",
        "error",
      );
    } finally {
      window.setTimeout(
        () =>
          setFavoritePulse((current) =>
            current.filter((pathValue) => pathValue !== file.path),
          ),
        360,
      );
    }
  }

  function openPresetDocument(label: string, ext: string) {
    const names = new Set(files.map((file) => file.name.toLowerCase()));
    let index = 1;
    let name = `新建${label}.${ext}`;
    while (names.has(name.toLowerCase())) {
      index += 1;
      name = `新建${label} (${index}).${ext}`;
    }
    setContextMenu(null);
    setDialog("document");
    setDialogValue(name);
  }

  async function refreshDirectory() {
    setContextMenu(null);
    await Promise.all([load(), loadFavorites()]);
    show("目录已刷新", "success");
  }

  function openContextMenuCustomizer() {
    setContextMenu(null);
    setCustomMenuOpen(true);
  }

  function menuItemEnabled(key: FileMenuActionKey) {
    return fileMenuItems.includes(key);
  }

  async function saveText() {
    if (!previewFile || publicMode) return;
    setPreviewSaving(true);
    try {
      await api.saveText(sourceKey, previewFile.path, previewText);
      show("文本已保存", "success");
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "保存失败",
        "error",
      );
    } finally {
      setPreviewSaving(false);
    }
  }

  async function logout() {
    await api.logout();
    await refresh();
    navigate("/");
  }

  function copy(value: string) {
    navigator.clipboard
      .writeText(value)
      .then(() => show("链接已复制", "success"));
  }

  const directoryContextActions = (
    <>
      {!publicMode && (
        <>
          <div className="context-menu-submenu">
            <button type="button" aria-haspopup="menu">
              <DocumentAdd20Regular />
              <span>新建</span>
              <ChevronRight20Regular className="context-menu-chevron" />
            </button>
            <div className="file-context-submenu glass" role="menu">
              <button
                type="button"
                onClick={() => {
                  setContextMenu(null);
                  openDialog("folder");
                }}
              >
                <FolderAdd20Regular />
                新建文件夹
              </button>
              <div className="context-menu-separator" role="separator" />
              {documentPresets.map((preset) => (
                <button
                  type="button"
                  key={preset.extension}
                  onClick={() =>
                    openPresetDocument(preset.label, preset.extension)
                  }
                >
                  {preset.icon}
                  {preset.label}
                  <small>.{preset.extension}</small>
                </button>
              ))}
              <button
                type="button"
                onClick={() => {
                  setContextMenu(null);
                  openDialog("document");
                }}
              >
                <DocumentAdd20Regular />
                新建文件…
              </button>
            </div>
          </div>
          <button
            type="button"
            onClick={() => {
              setContextMenu(null);
              uploadInput.current?.click();
            }}
          >
            <Open20Regular />
            上传
          </button>
        </>
      )}
      <div className="context-menu-submenu">
        <button type="button" aria-haspopup="menu">
          {viewMode === "list" ? <Table20Regular /> : <Grid20Regular />}
          <span>视图</span>
          <ChevronRight20Regular className="context-menu-chevron" />
        </button>
        <div className="file-context-submenu glass" role="menu">
          <button
            type="button"
            onClick={() => {
              setViewMode("list");
              setContextMenu(null);
            }}
          >
            <TextBulletList20Regular />
            列表
            {viewMode === "list" && <Checkmark20Regular />}
          </button>
          <button
            type="button"
            onClick={() => {
              setViewMode("gallery");
              setContextMenu(null);
            }}
          >
            <Grid20Regular />
            网格
            {viewMode === "gallery" && <Checkmark20Regular />}
          </button>
        </div>
      </div>
      <div className="context-menu-submenu">
        <button type="button" aria-haspopup="menu">
          <GroupList20Regular />
          <span>分组模式</span>
          <ChevronRight20Regular className="context-menu-chevron" />
        </button>
        <div className="file-context-submenu glass" role="menu">
          {(
            [
              ["none", "不分组"],
              ["time", "按时间"],
              ["size", "按大小"],
              ["type", "按类型"],
            ] as Array<[GroupMode, string]>
          ).map(([value, label]) => (
            <button
              type="button"
              key={value}
              onClick={() => {
                setGroupMode(value);
                setContextMenu(null);
              }}
            >
              <GroupList20Regular />
              {label}
              {groupMode === value && <Checkmark20Regular />}
            </button>
          ))}
        </div>
      </div>
      {!publicMode && (
        <button
          type="button"
          onClick={() => {
            setContextMenu(null);
            navigate("/settings/view");
          }}
        >
          <Settings20Regular />
          打开设置
        </button>
      )}
      <div className="context-menu-submenu">
        <button type="button" aria-haspopup="menu">
          <ArrowSync20Regular />
          <span>刷新</span>
          <ChevronRight20Regular className="context-menu-chevron" />
        </button>
        <div className="file-context-submenu glass" role="menu">
          <button type="button" onClick={() => void refreshDirectory()}>
            <ArrowSync20Regular />
            刷新目录
          </button>
          <button type="button" onClick={() => window.location.reload()}>
            <ArrowSync20Regular />
            刷新页面
          </button>
        </div>
      </div>
      {!publicMode && (
        <button type="button" onClick={openContextMenuCustomizer}>
          <AppsList20Regular />
          自定义菜单
        </button>
      )}
    </>
  );

  const dialogTitle = {
    folder: "新建文件夹",
    document: "新建文档",
    rename: "重命名",
    move: "移动到目录",
    copy: "复制到目录",
    metadata: "编辑文件说明",
  }[dialog || "folder"];
  const previewKind = previewFile ? filePreviewKind(previewFile.name) : null;
  const imagePreviewOpen = previewKind === "image";

  function adjustImageScale(delta: number) {
    setImageScale((current) =>
      Math.min(4, Math.max(0.25, Math.round((current + delta) * 100) / 100)),
    );
  }

  function resetImageView() {
    setImageScale(1);
    setImageRotation(0);
  }

  return (
    <div className="xfile-page">
      <header className="xfile-header glass">
        <Link to="/" className="brand">
          <BrandMark />
          <strong>XFile</strong>
        </Link>
        <span className="public-product-label">
          {site?.siteName || "公开网盘"}
        </span>
        {!publicMode && (
          <nav className="home-quick-nav" aria-label="个人资源导航">
            {quickNavOptions
              .filter((option) => quickNav.includes(option.key))
              .map((option) => (
                <Link key={option.key} to={option.to}>
                  {option.icon}
                  <span>{option.label}</span>
                </Link>
              ))}
            <Popover
              label="自定义首页导航"
              align="left"
              className="home-nav-customizer"
              trigger={({ open, toggle }) => (
                <IconButton label="自定义导航栏" active={open} onClick={toggle}>
                  <AppsList20Regular />
                </IconButton>
              )}
            >
              <div className="navigation-options">
                <strong>自定义导航栏</strong>
                <small>选择要显示在首页头部的入口。</small>
                {quickNavOptions.map((option) => (
                  <label key={option.key}>
                    <input
                      type="checkbox"
                      checked={quickNav.includes(option.key)}
                      onChange={(event) =>
                        setQuickNav((current) =>
                          event.target.checked
                            ? [...current, option.key]
                            : current.filter((key) => key !== option.key),
                        )
                      }
                    />
                    {option.icon}
                    <span>{option.label}</span>
                  </label>
                ))}
                <Button
                  size="small"
                  onClick={() =>
                    setQuickNav(quickNavOptions.map((option) => option.key))
                  }
                >
                  恢复默认
                </Button>
              </div>
            </Popover>
          </nav>
        )}
        <div className="xfile-global-search">
          <Search20Regular />
          <input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索文件或目录"
          />
        </div>
        <div className="xfile-header-actions">
          {preferences.showAnnouncement !== "disabled" && (
            <IconButton label="公告" onClick={() => setNoticeOpen(true)}>
              <MegaphoneLoud20Regular />
            </IconButton>
          )}
          {preferences.showDocument !== "disabled" && (
            <IconButton label="文档区" onClick={() => setDocsOpen(true)}>
              <BookOpen20Regular />
            </IconButton>
          )}
          <IconButton label="刷新" onClick={load}>
            <ArrowSync20Regular />
          </IconButton>
          {!publicMode && (
            <IconButton
              label="上传"
              onClick={() => uploadInput.current?.click()}
            >
              <Open20Regular />
            </IconButton>
          )}
          <IconButton
            label={viewMode === "list" ? "网格模式" : "列表模式"}
            onClick={() =>
              setViewMode((current) =>
                current === "list" ? "gallery" : "list",
              )
            }
          >
            {viewMode === "list" ? <Grid20Regular /> : <Table20Regular />}
          </IconButton>
          {!publicMode && (
            <Link
              className="icon-button"
              aria-label="管理设置"
              title="管理设置"
              to="/storage"
            >
              <Settings20Regular />
            </Link>
          )}
          <AppearanceControl />
          <select
            className="storage-selector"
            aria-label="选择存储源"
            value={sourceKey}
            onChange={(event) => {
              setSourceKey(event.target.value);
              setPath("");
            }}
          >
            <option value="">全部存储源</option>
            {sources.map((source) => (
              <option value={source.key} key={source.key}>
                {source.name}
              </option>
            ))}
          </select>
          {publicMode && preferences.showLogin !== "disabled" ? (
            <Link className="button button-primary" to="/login">
              <PersonArrowRight20Regular />
              登录
            </Link>
          ) : !publicMode ? (
            <>
              <span className="account-chip">
                {site?.loggedIn ? "管理账号" : "访客"}
              </span>
              <IconButton label="退出登录" onClick={logout}>
                <SignOut20Regular />
              </IconButton>
            </>
          ) : null}
        </div>
      </header>

      <main
        className={`xfile-browser ${preferences.layout === "center" ? "is-desktop-centered" : ""} ${preferences.mobileLayout === "center" ? "is-mobile-centered" : ""}`}
      >
        <section
          className={`xfile-surface glass-panel ${dragActive ? "is-drag-active" : ""}`}
          onDragEnter={handleDragEnter}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={(event) => void handleDrop(event)}
          onContextMenu={(event) => {
            if (publicMode || !sourceKey) return;
            const target = event.target as HTMLElement;
            if (
              target.closest(
                "tr, article, button, a, input, select, textarea, .file-context-menu",
              )
            )
              return;
            event.preventDefault();
            setContextMenu({ x: event.clientX, y: event.clientY });
          }}
        >
          {dragActive && (
            <div
              className={`xfile-drop-overlay ${publicMode || !sourceKey ? "is-blocked" : ""}`}
              role="status"
              aria-live="polite"
            >
              <div className="xfile-drop-card">
                <span className="xfile-drop-icon">
                  <ArrowUpload20Regular />
                </span>
                <strong>
                  {publicMode
                    ? "登录后可拖拽上传"
                    : !sourceKey
                      ? "先选择存储空间"
                      : "松开鼠标开始上传"}
                </strong>
                <span>
                  {publicMode
                    ? "当前是公开浏览模式，登录后回到此目录即可使用"
                    : !sourceKey
                      ? "进入一个存储目录后即可拖入文件"
                      : `上传到 ${path || activeSource?.name || "当前目录"} · 支持多文件`}
                </span>
              </div>
            </div>
          )}
          <div className="xfile-pathbar">
            <IconButton
              label="返回上级"
              disabled={!sourceKey}
              onClick={() => {
                if (path) setPath(parentPath(path));
                else setSourceKey("");
              }}
            >
              <ArrowLeft20Regular />
            </IconButton>
            <div className="xfile-breadcrumbs">
              {breadcrumbs.map((crumb, index) => (
                <span key={`${crumb.label}-${index}`}>
                  <button
                    onClick={() => {
                      if (crumb.root) {
                        setSourceKey("");
                        setPath("");
                      } else setPath(crumb.value);
                    }}
                  >
                    {crumb.label}
                  </button>
                  {index < breadcrumbs.length - 1 && <b>/</b>}
                </span>
              ))}
            </div>
            {selected.length > 0 && (
              <div className="xfile-batch-actions">
                <span>
                  <Checkmark20Regular />
                  已选择 {selected.length} 项
                </span>
                <Button
                  icon={<ArrowDownload20Regular />}
                  onClick={() => void downloadFiles(selectedFiles)}
                >
                  下载
                </Button>
                {!publicMode && (
                  <>
                    {showShareAction && (
                      <Button
                        icon={<Share20Regular />}
                        onClick={() => void createShare()}
                      >
                        分享
                      </Button>
                    )}
                    {showDirectAction && (
                      <Button
                        icon={<Link20Regular />}
                        onClick={() => void createDirectLink()}
                      >
                        直链
                      </Button>
                    )}
                    <Button
                      icon={<ArrowMove20Regular />}
                      onClick={() => openDialog("move")}
                    >
                      移动
                    </Button>
                    <Button
                      icon={<Copy20Regular />}
                      onClick={() => openDialog("copy")}
                    >
                      复制
                    </Button>
                    <Button
                      variant="danger"
                      icon={<Delete20Regular />}
                      onClick={() => void removeSelected()}
                    >
                      删除
                    </Button>
                  </>
                )}
              </div>
            )}
            {!publicMode && selected.length === 0 && sourceKey && (
              <div className="xfile-create-actions">
                <Button
                  icon={<FolderAdd20Regular />}
                  onClick={() => openDialog("folder")}
                >
                  新建文件夹
                </Button>
                <Button
                  icon={<DocumentAdd20Regular />}
                  onClick={() => openDialog("document")}
                >
                  新建文件
                </Button>
                <Button
                  variant="primary"
                  icon={<Open20Regular />}
                  onClick={() => uploadInput.current?.click()}
                >
                  上传
                </Button>
              </div>
            )}
          </div>

          <input
            ref={uploadInput}
            hidden
            multiple
            type="file"
            onChange={(event) => {
              const input = event.currentTarget;
              void handleUpload(input.files).finally(() => {
                input.value = "";
              });
            }}
          />
          {Object.keys(uploadProgress).length > 0 && (
            <div className="upload-progress-list">
              {Object.entries(uploadProgress).map(([name, percent]) => (
                <div key={name}>
                  <span>{name}</span>
                  <progress value={percent} max="100" />
                  <b>{percent}%</b>
                </div>
              ))}
            </div>
          )}
          {error && <ErrorBanner error={error} onRetry={load} />}

          {!sourceKey ? (
            <StorageRoot
              sources={sources}
              onOpen={(key) => setSourceKey(key)}
            />
          ) : loading ? (
            <Loading label="正在读取文件" />
          ) : displayedFiles.length === 0 ? (
            <Empty
              title="当前目录为空"
              description={
                publicMode
                  ? "此目录暂时没有可浏览的文件。"
                  : "可以拖拽文件到这里，或使用上传按钮开始使用。"
              }
            />
          ) : viewMode === "gallery" ? (
            <div className="xfile-gallery-groups">
              {displayedGroups.map((group) => (
                <section className="xfile-file-group" key={group.key}>
                  {group.label && (
                    <header className="xfile-group-heading">
                      <strong>{group.label}</strong>
                      <span>{group.files.length} 项</span>
                    </header>
                  )}
                  <div className="xfile-gallery">
                    {group.files.map((file) => {
                      const isSelected = selected.includes(file.path);
                      const isFavorite = Boolean(favoriteIds[file.path]);
                      const isPulsing = favoritePulse.includes(file.path);
                      return (
                        <article
                          key={file.path}
                          className={[
                            isSelected ? "is-selected" : "",
                            isFavorite ? "is-favorite" : "",
                            isPulsing ? "is-favorite-pulsing" : "",
                          ]
                            .filter(Boolean)
                            .join(" ")}
                          onContextMenu={(event) => {
                            event.preventDefault();
                            setContextMenu({
                              x: event.clientX,
                              y: event.clientY,
                              file,
                            });
                          }}
                        >
                          <input
                            type="checkbox"
                            aria-label={`选择 ${file.name}`}
                            checked={isSelected}
                            onChange={(event) =>
                              toggle(file, event.target.checked)
                            }
                          />
                          <button
                            className="gallery-file"
                            onClick={() => activate(file)}
                            onDoubleClick={() => open(file)}
                          >
                            {file.type === "file" &&
                            [
                              "png",
                              "jpg",
                              "jpeg",
                              "gif",
                              "webp",
                              "svg",
                              "avif",
                            ].includes(extension(file)) ? (
                              <img
                                src={currentUrl(file, true)}
                                alt={file.name}
                              />
                            ) : (
                              <FileGlyph file={file} />
                            )}
                            <strong>{file.name}</strong>
                            <small>
                              {file.type === "folder"
                                ? "文件夹"
                                : formatBytes(file.size)}
                            </small>
                          </button>
                          <div className="gallery-card-actions">
                            {!publicMode && (
                              <IconButton
                                className={`favorite-button ${isFavorite ? "is-active" : ""}`}
                                label={
                                  isFavorite
                                    ? `取消收藏 ${file.name}`
                                    : `收藏 ${file.name}`
                                }
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void toggleFavorite(file);
                                }}
                              >
                                {isFavorite ? (
                                  <Star20Filled />
                                ) : (
                                  <Star20Regular />
                                )}
                              </IconButton>
                            )}
                            <IconButton
                              label={`${file.name} 更多操作`}
                              onClick={(event) => {
                                event.stopPropagation();
                                const rect =
                                  event.currentTarget.getBoundingClientRect();
                                setContextMenu({
                                  x: rect.right,
                                  y: rect.bottom,
                                  file,
                                });
                              }}
                            >
                              <MoreHorizontal20Regular />
                            </IconButton>
                          </div>
                        </article>
                      );
                    })}
                  </div>
                </section>
              ))}
            </div>
          ) : (
            <div
              className={`xfile-table-wrap table-${preferences.tableSize || "small"}`}
            >
              <table className="file-table">
                <thead>
                  <tr>
                    <th className="check-cell">
                      <input
                        type="checkbox"
                        aria-label="全选"
                        checked={
                          selected.length === displayedFiles.length &&
                          !!displayedFiles.length
                        }
                        onChange={(event) =>
                          setSelected(
                            event.target.checked
                              ? displayedFiles.map((file) => file.path)
                              : [],
                          )
                        }
                      />
                    </th>
                    <th>
                      <button
                        className="sort-button"
                        onClick={() => sortBy("name")}
                      >
                        文件名 <ArrowSort20Regular />
                      </button>
                    </th>
                    <th>
                      <button
                        className="sort-button"
                        onClick={() => sortBy("modifiedAt")}
                      >
                        修改时间 <ArrowSort20Regular />
                      </button>
                    </th>
                    <th>
                      <button
                        className="sort-button"
                        onClick={() => sortBy("size")}
                      >
                        大小 <ArrowSort20Regular />
                      </button>
                    </th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {displayedGroups.map((group) => (
                    <Fragment key={group.key}>
                      {group.label && (
                        <tr className="file-table-group-row">
                          <th colSpan={5}>
                            <strong>{group.label}</strong>
                            <span>{group.files.length} 项</span>
                          </th>
                        </tr>
                      )}
                      {group.files.map((file) => {
                        const isSelected = selected.includes(file.path);
                        const isFavorite = Boolean(favoriteIds[file.path]);
                        const isPulsing = favoritePulse.includes(file.path);
                        return (
                          <tr
                            key={file.path}
                            className={[
                              isSelected ? "is-selected" : "",
                              isFavorite ? "is-favorite" : "",
                              isPulsing ? "is-favorite-pulsing" : "",
                            ]
                              .filter(Boolean)
                              .join(" ")}
                            onDoubleClick={() => open(file)}
                            onContextMenu={(event) => {
                              event.preventDefault();
                              setContextMenu({
                                x: event.clientX,
                                y: event.clientY,
                                file,
                              });
                            }}
                          >
                            <td className="check-cell">
                              <input
                                type="checkbox"
                                aria-label={`选择 ${file.name}`}
                                checked={isSelected}
                                onChange={(event) =>
                                  toggle(file, event.target.checked)
                                }
                              />
                            </td>
                            <td>
                              <button
                                className="file-name"
                                onClick={() => activate(file)}
                              >
                                <FileGlyph file={file} />
                                <span>
                                  <strong>{file.name}</strong>
                                  {file.description && (
                                    <small>{file.description}</small>
                                  )}
                                </span>
                                {isFavorite && (
                                  <Star20Filled
                                    className="file-favorite-mark"
                                    aria-label="已收藏"
                                  />
                                )}
                              </button>
                            </td>
                            <td>{formatTime(file.modifiedAt)}</td>
                            <td>
                              {file.type === "folder"
                                ? "—"
                                : formatBytes(file.size)}
                            </td>
                            <td>
                              <div className="file-row-actions">
                                {!publicMode && (
                                  <IconButton
                                    className={`favorite-button ${isFavorite ? "is-active" : ""}`}
                                    label={
                                      isFavorite
                                        ? `取消收藏 ${file.name}`
                                        : `收藏 ${file.name}`
                                    }
                                    onClick={(event) => {
                                      event.stopPropagation();
                                      void toggleFavorite(file);
                                    }}
                                  >
                                    {isFavorite ? (
                                      <Star20Filled />
                                    ) : (
                                      <Star20Regular />
                                    )}
                                  </IconButton>
                                )}
                                <IconButton
                                  label={`${file.name} 更多操作`}
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    const rect =
                                      event.currentTarget.getBoundingClientRect();
                                    setContextMenu({
                                      x: rect.right,
                                      y: rect.bottom,
                                      file,
                                    });
                                  }}
                                >
                                  <MoreHorizontal20Regular />
                                </IconButton>
                              </div>
                            </td>
                          </tr>
                        );
                      })}
                    </Fragment>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {sourceKey && visibleFiles.length > displayLimit && (
            <div className="load-more">
              <Button
                onClick={() =>
                  setDisplayLimit(
                    (current) =>
                      current + (Number(preferences.loadMoreSize) || 50),
                  )
                }
              >
                加载更多
              </Button>
            </div>
          )}
          <footer className="xfile-status">
            <span>
              {sourceKey
                ? `共 ${visibleFiles.length} 项${selected.length ? `（已选择 ${selected.length} 项）` : ""}`
                : `${sources.length} 个存储源`}
            </span>
            <span>
              {activeSource
                ? `${activeSource.name} · ${activeSource.typeLabel || activeSource.type}`
                : site?.siteName || "XFile"}
            </span>
          </footer>
        </section>
      </main>

      {contextMenu && (
        <div
          className={`file-context-menu glass ${contextMenu.x > window.innerWidth - 520 ? "submenu-left" : ""}`}
          style={{
            left: Math.max(8, Math.min(contextMenu.x, window.innerWidth - 252)),
            top: Math.max(8, Math.min(contextMenu.y, window.innerHeight - 520)),
          }}
          role="menu"
          aria-label={
            contextMenu.file
              ? `${contextMenu.file.name} 操作菜单`
              : "目录操作菜单"
          }
          onClick={(event) => event.stopPropagation()}
        >
          {contextMenu.file ? (
            <>
              {menuItemEnabled("open") && (
                <button
                  type="button"
                  onClick={() => {
                    if (contextMenu.file) open(contextMenu.file);
                    setContextMenu(null);
                  }}
                >
                  {contextMenu.file.type === "folder" ? (
                    <FolderOpen20Regular />
                  ) : (
                    <Eye20Regular />
                  )}
                  打开
                </button>
              )}
              {menuItemEnabled("openNewTab") && (
                <button
                  type="button"
                  onClick={() =>
                    contextMenu.file && openInNewTab(contextMenu.file)
                  }
                >
                  <WindowNew20Regular />
                  新标签打开
                </button>
              )}
              {menuItemEnabled("packageDownload") && (
                <button
                  type="button"
                  onClick={() =>
                    contextMenu.file && void downloadPackage([contextMenu.file])
                  }
                >
                  <Archive20Regular />
                  打包下载
                </button>
              )}
              {!publicMode && (
                <>
                  <div className="context-menu-separator" role="separator" />
                  {menuItemEnabled("favorite") && (
                    <button
                      type="button"
                      className={
                        contextMenu.file && favoriteIds[contextMenu.file.path]
                          ? "is-active"
                          : ""
                      }
                      onClick={() =>
                        contextMenu.file &&
                        void toggleFavorite(contextMenu.file)
                      }
                    >
                      {contextMenu.file &&
                      favoriteIds[contextMenu.file.path] ? (
                        <Star20Filled />
                      ) : (
                        <StarAdd20Regular />
                      )}
                      {contextMenu.file && favoriteIds[contextMenu.file.path]
                        ? "取消收藏"
                        : "收藏"}
                    </button>
                  )}
                  {menuItemEnabled("share") && (
                    <button
                      type="button"
                      disabled={!showShareAction}
                      title={
                        showShareAction
                          ? undefined
                          : "分享功能已在显示设置中关闭"
                      }
                      onClick={() =>
                        contextMenu.file && void createShare(contextMenu.file)
                      }
                    >
                      <Share20Regular />
                      创建分享
                    </button>
                  )}
                  <div className="context-menu-separator" role="separator" />
                  {menuItemEnabled("rename") && (
                    <button
                      type="button"
                      onClick={() => {
                        const file = contextMenu.file;
                        setContextMenu(null);
                        if (file) openDialog("rename", file);
                      }}
                    >
                      <Edit20Regular />
                      重命名
                    </button>
                  )}
                  {menuItemEnabled("move") && (
                    <button
                      type="button"
                      onClick={() => {
                        const file = contextMenu.file;
                        setContextMenu(null);
                        if (file) openDialog("move", file);
                      }}
                    >
                      <ArrowMove20Regular />
                      移动
                    </button>
                  )}
                  {menuItemEnabled("copy") && (
                    <button
                      type="button"
                      onClick={() => {
                        const file = contextMenu.file;
                        setContextMenu(null);
                        if (file) openDialog("copy", file);
                      }}
                    >
                      <Copy20Regular />
                      复制
                    </button>
                  )}
                  {menuItemEnabled("delete") && (
                    <button
                      type="button"
                      className="danger"
                      onClick={() => {
                        const file = contextMenu.file;
                        setContextMenu(null);
                        if (file) void removeSelected(file);
                      }}
                    >
                      <Delete20Regular />
                      删除
                    </button>
                  )}
                </>
              )}
              <div className="context-menu-separator" role="separator" />
              {directoryContextActions}
            </>
          ) : (
            directoryContextActions
          )}
        </div>
      )}

      <Modal
        open={customMenuOpen}
        title="自定义右键菜单"
        description="选择右键点击文件时显示的快捷操作。目录、视图和刷新命令会始终保留。"
        onClose={() => setCustomMenuOpen(false)}
        footer={
          <>
            <Button
              onClick={() =>
                setFileMenuItems(fileMenuOptions.map((option) => option.key))
              }
            >
              恢复默认
            </Button>
            <Button variant="primary" onClick={() => setCustomMenuOpen(false)}>
              完成
            </Button>
          </>
        }
      >
        <div className="context-menu-customizer">
          {fileMenuOptions.map((option) => (
            <label key={option.key}>
              <input
                type="checkbox"
                checked={fileMenuItems.includes(option.key)}
                onChange={(event) =>
                  setFileMenuItems((current) =>
                    event.target.checked
                      ? [...current, option.key]
                      : current.filter((key) => key !== option.key),
                  )
                }
              />
              <span>
                <strong>{option.label}</strong>
                <small>在文件右键菜单中显示</small>
              </span>
            </label>
          ))}
        </div>
      </Modal>

      <Modal
        open={!!previewFile}
        title={previewFile?.name || "文件预览"}
        description={
          previewFile
            ? `${formatBytes(previewFile.size)} · ${formatTime(previewFile.modifiedAt)}`
            : undefined
        }
        onClose={() => setPreviewFile(null)}
        size="large"
        className={`modal-preview ${
          previewKind === "video" ? "modal-preview-video" : ""
        } ${
          imagePreviewOpen ? "modal-preview-image" : ""
        }`}
        bodyClassName="modal-preview-body"
        footer={
          previewFile && (
            <>
              {imagePreviewOpen && (
                <>
                  <IconButton
                    label="缩小图片"
                    disabled={imageScale <= 0.25}
                    onClick={() => adjustImageScale(-0.25)}
                  >
                    <ZoomOut20Regular />
                  </IconButton>
                  <button
                    type="button"
                    className="image-zoom-value"
                    aria-label="恢复图片原始比例"
                    title="恢复到 100%"
                    onClick={resetImageView}
                  >
                    {Math.round(imageScale * 100)}%
                  </button>
                  <IconButton
                    label="放大图片"
                    disabled={imageScale >= 4}
                    onClick={() => adjustImageScale(0.25)}
                  >
                    <ZoomIn20Regular />
                  </IconButton>
                  <span className="image-toolbar-separator" aria-hidden="true" />
                  <IconButton
                    label="向左旋转"
                    onClick={() =>
                      setImageRotation((current) => (current + 270) % 360)
                    }
                  >
                    <ArrowRotateCounterclockwise20Regular />
                  </IconButton>
                  <IconButton
                    label="向右旋转"
                    onClick={() =>
                      setImageRotation((current) => (current + 90) % 360)
                    }
                  >
                    <ArrowRotateClockwise20Regular />
                  </IconButton>
                  <IconButton label="恢复图片视图" onClick={resetImageView}>
                    <ArrowReset20Regular />
                  </IconButton>
                  <span className="image-toolbar-separator" aria-hidden="true" />
                </>
              )}
              <Button
                size={imagePreviewOpen ? "small" : "medium"}
                icon={<ArrowDownload20Regular />}
                onClick={() => void downloadFiles([previewFile])}
              >
                <span>下载</span>
              </Button>
              {!publicMode && (
                <>
                  {showShareAction && (
                    <Button
                      size={imagePreviewOpen ? "small" : "medium"}
                      icon={<Share20Regular />}
                      onClick={() => void createShare(previewFile)}
                    >
                      <span>分享</span>
                    </Button>
                  )}
                  {showDirectAction && (
                    <Button
                      size={imagePreviewOpen ? "small" : "medium"}
                      icon={<Link20Regular />}
                      onClick={() => void createDirectLink(previewFile)}
                    >
                      <span>直链</span>
                    </Button>
                  )}
                </>
              )}
              {!imagePreviewOpen && (
                <Button variant="primary" onClick={() => setPreviewFile(null)}>
                  关闭
                </Button>
              )}
            </>
          )
        }
      >
        {previewFile && (
          <div className="xfile-preview">
            <Preview
              file={previewFile}
              url={currentUrl(previewFile, true)}
              settings={preferences}
              text={previewText}
              loading={previewLoading}
              saving={previewSaving}
              editable={!publicMode}
              readerKey={`${sourceKey}:${previewFile.path}`}
              immersiveVideo
              imageScale={imageScale}
              imageRotation={imageRotation}
              onImageZoom={adjustImageScale}
              onResetImage={resetImageView}
              onTextChange={setPreviewText}
              onSaveText={saveText}
            />
          </div>
        )}
      </Modal>
      <Modal
        open={generatedLinks.length > 0}
        title={
          generatedLinks[0]?.kind === "share" ? "分享链接已生成" : "直链已生成"
        }
        description={
          generatedLinks[0]?.kind === "share"
            ? "扫码即可打开，也可以复制链接发送给其他人。"
            : `共 ${generatedLinks.length} 个链接，可逐项复制。`
        }
        onClose={() => setGeneratedLinks([])}
        footer={
          <Button variant="primary" onClick={() => setGeneratedLinks([])}>
            完成
          </Button>
        }
      >
        <div
          className={`link-result-list ${generatedLinks[0]?.kind === "share" ? "has-qr" : ""}`}
        >
          {generatedLinks.map((link) => (
            <div className="generated-link" key={`${link.label}-${link.value}`}>
              {link.kind === "share" && (
                <div className="share-qr" aria-label="分享链接二维码">
                  <QRCodeSVG
                    value={link.value}
                    size={176}
                    level="M"
                    marginSize={2}
                    bgColor="#ffffff"
                    fgColor="#172033"
                    title="分享链接二维码"
                  />
                </div>
              )}
              <LinkField label={link.label} value={link.value} onCopy={copy} />
            </div>
          ))}
        </div>
      </Modal>
      <Modal
        open={!!dialog}
        title={dialogTitle}
        description={
          dialog === "move" || dialog === "copy"
            ? "请输入目标目录相对路径，留空表示根目录。"
            : undefined
        }
        onClose={() => setDialog(null)}
        footer={
          <>
            <Button onClick={() => setDialog(null)}>取消</Button>
            <Button
              variant="primary"
              disabled={
                saving ||
                (!dialogValue.trim() && dialog !== "move" && dialog !== "copy")
              }
              onClick={submitDialog}
            >
              {saving ? "处理中…" : "确认"}
            </Button>
          </>
        }
      >
        <Field
          label={
            dialog === "metadata"
              ? "文件说明"
              : dialog === "move" || dialog === "copy"
                ? "目标目录"
                : "名称"
          }
        >
          {dialog === "metadata" ? (
            <textarea
              rows={5}
              value={dialogValue}
              onChange={(event) => setDialogValue(event.target.value)}
            />
          ) : (
            <input
              autoFocus
              value={dialogValue}
              placeholder={
                dialog === "move" || dialog === "copy"
                  ? "例如：归档/2026"
                  : "请输入名称"
              }
              onChange={(event) => setDialogValue(event.target.value)}
              onKeyDown={(event) =>
                event.key === "Enter" && void submitDialog()
              }
            />
          )}
        </Field>
      </Modal>
      <Modal
        open={noticeOpen}
        title="网站公告"
        onClose={() => setNoticeOpen(false)}
      >
        <div className="document-content">
          <MegaphoneLoud20Regular />
          <p>
            {preferences.announcement ||
              "欢迎使用 XFile。管理员可以在显示设置中更新公告内容。"}
          </p>
        </div>
      </Modal>
      <Modal open={docsOpen} title="文档区" onClose={() => setDocsOpen(false)}>
        <div className="document-content">
          <BookOpen20Regular />
          <p>
            文件浏览支持列表、画廊、在线预览、分享、直链和批量操作。更多使用说明由管理员在显示设置中维护。
          </p>
        </div>
      </Modal>
      {publicMode && error.toLowerCase().includes("password") && (
        <div className="password-gate glass">
          <Field label="目录访问密码">
            <input
              type="password"
              value={directoryPassword}
              onChange={(event) => setDirectoryPassword(event.target.value)}
            />
          </Field>
          <Button variant="primary" onClick={load}>
            解锁目录
          </Button>
        </div>
      )}
    </div>
  );
}

function StorageRoot({
  sources,
  onOpen,
}: {
  sources: StorageSource[];
  onOpen: (key: string) => void;
}) {
  if (!sources.length) return <Empty title="暂无可用存储源" />;
  return (
    <div className="storage-root-grid">
      {sources.map((source) => (
        <button key={source.key} onClick={() => onOpen(source.key)}>
          <span>
            <Folder20Filled />
          </span>
          <div>
            <strong>{source.name}</strong>
            <small>{source.typeLabel || source.type}</small>
          </div>
          <Badge tone={source.public ? "success" : "info"}>
            {source.public ? "公开" : "登录可用"}
          </Badge>
        </button>
      ))}
    </div>
  );
}

function LinkField({
  label,
  value,
  onCopy,
}: {
  label: string;
  value: string;
  onCopy: (value: string) => void;
}) {
  return (
    <div className="link-field">
      <span>{label}</span>
      <div>
        <input readOnly value={value} />
        <IconButton label="复制" onClick={() => onCopy(value)}>
          <ClipboardLink20Regular />
        </IconButton>
      </div>
    </div>
  );
}

function NovelReader({
  file,
  document,
  sourceText,
  readerKey,
  editable,
  onEdit,
}: {
  file: FileEntry;
  document: NovelDocument;
  sourceText: string;
  readerKey: string;
  editable: boolean;
  onEdit: () => void;
}) {
  const contentRef = useRef<HTMLDivElement>(null);
  const saveTimerRef = useRef<number | null>(null);
  const [activeChapter, setActiveChapter] = useState(0);
  const storageKey = `xfile:novel-progress:${readerKey}`;
  const currentChapter =
    document.chapters[activeChapter] || document.chapters[0];
  const intro = useMemo(
    () =>
      activeChapter === 0 ? sourceText.slice(0, document.introEnd).trim() : "",
    [activeChapter, document.introEnd, sourceText],
  );
  const chapterBody = useMemo(
    () =>
      currentChapter
        ? sourceText
            .slice(currentChapter.bodyStart, currentChapter.bodyEnd)
            .trim()
        : "",
    [currentChapter, sourceText],
  );
  const chapterWindowStart = useMemo(() => {
    if (document.chapters.length <= chapterNavigationWindow) return 0;
    return Math.min(
      Math.max(0, activeChapter - Math.floor(chapterNavigationWindow / 2)),
      document.chapters.length - chapterNavigationWindow,
    );
  }, [activeChapter, document.chapters.length]);
  const visibleChapters = useMemo(
    () =>
      document.chapters.slice(
        chapterWindowStart,
        chapterWindowStart + chapterNavigationWindow,
      ),
    [chapterWindowStart, document.chapters],
  );

  const saveProgress = useCallback(
    (chapterIndex: number, scrollTop: number) => {
      try {
        window.localStorage.setItem(
          storageKey,
          JSON.stringify({ chapterIndex, scrollTop, updatedAt: Date.now() }),
        );
      } catch {
        // Browsing still works when storage is unavailable.
      }
    },
    [storageKey],
  );

  const jumpToChapter = useCallback(
    (index: number) => {
      const nextIndex = Math.min(
        Math.max(0, index),
        document.chapters.length - 1,
      );
      setActiveChapter(nextIndex);
      window.requestAnimationFrame(() => {
        const reader = contentRef.current;
        if (reader) reader.scrollTop = 0;
      });
      saveProgress(nextIndex, 0);
    },
    [document.chapters.length, saveProgress],
  );

  useEffect(() => {
    setActiveChapter(0);
    let saved: { chapterIndex?: number; scrollTop?: number } | null = null;
    try {
      saved = JSON.parse(window.localStorage.getItem(storageKey) || "null");
    } catch {
      saved = null;
    }

    const frame = window.requestAnimationFrame(() => {
      const reader = contentRef.current;
      if (!reader || !saved) return;
      const chapterIndex = Math.min(
        Math.max(0, saved.chapterIndex || 0),
        document.chapters.length - 1,
      );
      setActiveChapter(chapterIndex);
      if (typeof saved.scrollTop === "number") {
        reader.scrollTop = Math.min(
          Math.max(0, saved.scrollTop),
          Math.max(0, reader.scrollHeight - reader.clientHeight),
        );
      }
    });

    return () => window.cancelAnimationFrame(frame);
  }, [document.chapters.length, storageKey]);

  useEffect(
    () => () => {
      if (saveTimerRef.current !== null) {
        window.clearTimeout(saveTimerRef.current);
      }
    },
    [],
  );

  function handleScroll(event: React.UIEvent<HTMLDivElement>) {
    const reader = event.currentTarget;
    if (saveTimerRef.current !== null) {
      window.clearTimeout(saveTimerRef.current);
    }
    saveTimerRef.current = window.setTimeout(
      () => saveProgress(activeChapter, reader.scrollTop),
      180,
    );
  }

  function renderParagraphs(value: string) {
    if (value.length >= longChapterThreshold) {
      return <div className="novel-reader-long-copy">{value}</div>;
    }
    return value
      .split(/\n+/)
      .map((paragraph) => paragraph.trim())
      .filter(Boolean)
      .map((paragraph, index) => (
        <p key={`${index}-${paragraph.slice(0, 16)}`}>{paragraph}</p>
      ));
  }

  return (
    <div className="text-preview novel-reader">
      <div className="code-editor-toolbar novel-reader-toolbar">
        <span className="code-language-badge">
          <BookOpen20Regular />
          阅读模式
        </span>
        <span className="novel-reader-progress">
          自动记忆 · {activeChapter + 1}/{document.chapters.length} 章
        </span>
        <select
          className={`novel-chapter-select ${document.chapters.length > chapterNavigationWindow ? "is-large-book" : ""}`}
          aria-label="章节跳转"
          value={activeChapter}
          onChange={(event) => jumpToChapter(Number(event.target.value))}
        >
          {document.chapters.map((chapter, index) => (
            <option value={index} key={`${chapter.line}-${chapter.title}`}>
              {chapter.title}
            </option>
          ))}
        </select>
        <div className="code-editor-actions">
          <IconButton
            label="上一章"
            disabled={activeChapter === 0}
            onClick={() => jumpToChapter(activeChapter - 1)}
          >
            <ArrowLeft20Regular />
          </IconButton>
          <IconButton
            label="下一章"
            disabled={activeChapter === document.chapters.length - 1}
            onClick={() => jumpToChapter(activeChapter + 1)}
          >
            <ChevronRight20Regular />
          </IconButton>
          {editable && (
            <Button
              size="small"
              variant="ghost"
              icon={<Edit20Regular />}
              onClick={onEdit}
            >
              编辑
            </Button>
          )}
        </div>
      </div>
      <div className="novel-reader-layout">
        <aside className="novel-chapter-panel" aria-label="章节目录">
          <div className="novel-chapter-heading">
            <TextBulletList20Regular />
            <span>
              <strong>章节目录</strong>
              <small>
                已识别 {document.chapters.length} 章
                {document.chapters.length > chapterNavigationWindow &&
                  ` · 显示 ${chapterWindowStart + 1}–${chapterWindowStart + visibleChapters.length}`}
              </small>
            </span>
          </div>
          <nav>
            {visibleChapters.map((chapter, offset) => {
              const index = chapterWindowStart + offset;
              return (
                <button
                  type="button"
                  className={activeChapter === index ? "is-active" : ""}
                  aria-current={
                    activeChapter === index ? "location" : undefined
                  }
                  onClick={() => jumpToChapter(index)}
                  key={`${chapter.line}-${chapter.title}`}
                >
                  <span>{String(index + 1).padStart(2, "0")}</span>
                  <strong>{chapter.title}</strong>
                </button>
              );
            })}
          </nav>
        </aside>
        <div
          className="novel-reader-content"
          ref={contentRef}
          onScroll={handleScroll}
          tabIndex={0}
          aria-label={`${file.name} 阅读内容`}
        >
          {intro && (
            <section className="novel-reader-intro">
              {renderParagraphs(intro)}
            </section>
          )}
          {currentChapter && (
            <section
              className="novel-reader-chapter"
              key={`${currentChapter.line}-${currentChapter.title}`}
            >
              <span className="novel-chapter-number">
                第 {String(activeChapter + 1).padStart(2, "0")} 章
              </span>
              <h2>{currentChapter.title}</h2>
              <div className="novel-reader-copy">
                {chapterBody ? (
                  renderParagraphs(chapterBody)
                ) : (
                  <p>本章暂无正文。</p>
                )}
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  );
}

function Preview({
  file,
  url,
  text,
  loading,
  saving,
  editable,
  readerKey,
  settings,
  immersiveVideo,
  imageScale,
  imageRotation,
  onImageZoom,
  onResetImage,
  onTextChange,
  onSaveText,
}: {
  file: FileEntry;
  url: string;
  text: string;
  loading: boolean;
  saving: boolean;
  editable: boolean;
  readerKey: string;
  settings: Record<string, string>;
  immersiveVideo: boolean;
  imageScale: number;
  imageRotation: number;
  onImageZoom: (delta: number) => void;
  onResetImage: () => void;
  onTextChange: (value: string) => void;
  onSaveText: () => void | Promise<void>;
}) {
  const { show } = useToast();
  const [formatting, setFormatting] = useState(false);
  const [readerMode, setReaderMode] = useState(true);
  const ext = extension(file);
  const {
    document: novelDocument,
    pending: novelPending,
    largeTextMode,
  } = useNovelAnalysis(text, ext);
  const canFormat = formattableExtensions.has(ext) && !largeTextMode;

  useEffect(() => setReaderMode(true), [file.path]);

  async function formatCurrentText() {
    setFormatting(true);
    try {
      onTextChange(await formatTextContent(text, ext));
      show("代码已格式化", "success");
    } catch (nextError) {
      const message =
        nextError instanceof Error
          ? nextError.message.split("\n")[0]
          : "请检查文件语法";
      show(`格式化失败：${message}`, "error");
    } finally {
      setFormatting(false);
    }
  }
  if (filePreviewKind(file.name) === "image")
    return (
      <div
        className={`preview-media preview-image ${
          imageRotation % 180 === 0 ? "" : "is-rotated-sideways"
        }`}
      >
        <img
          src={url}
          alt={file.name}
          decoding="async"
          draggable={false}
          style={{
            transform: `rotate(${imageRotation}deg) scale(${imageScale})`,
          }}
          onDoubleClick={onResetImage}
          onWheel={(event) => {
            event.preventDefault();
            onImageZoom(event.deltaY < 0 ? 0.1 : -0.1);
          }}
        />
      </div>
    );
  if (isVideoFileName(file.name))
    return (
      <div className="preview-media preview-video">
        <VideoPreview src={url} name={file.name} immersive={immersiveVideo} />
      </div>
    );
  if (["mp3", "wav", "ogg", "flac", "m4a"].includes(ext))
    return (
      <div className="preview-audio">
        <MusicNote220Regular />
        <audio src={url} controls />
      </div>
    );
  if (ext === "pdf")
    return <iframe className="preview-frame" title={file.name} src={url} />;
  if (textPreviewExtensions.includes(ext)) {
    if (loading) {
      return (
        <div className="text-preview">
          <Loading />
        </div>
      );
    }
    if (novelPending) {
      return (
        <div className="text-preview novel-analysis-loading">
          <Loading label="正在后台识别章节" />
          <p>{formatBytes(file.size)} · 大文件性能模式不会阻塞页面操作</p>
        </div>
      );
    }
    if (novelDocument && (readerMode || !editable)) {
      return (
        <NovelReader
          file={file}
          document={novelDocument}
          sourceText={text}
          readerKey={readerKey}
          editable={editable}
          onEdit={() => setReaderMode(false)}
        />
      );
    }
    return (
      <div className="text-preview">
        <div className="code-editor-toolbar">
          <span className="code-language-badge">
            <Braces20Regular />
            {codeLanguageLabel(ext)}
          </span>
          {largeTextMode && (
            <span className="code-editor-performance-badge">
              大文件模式 · 已关闭高亮和格式化
            </span>
          )}
          <div className="code-editor-actions">
            {novelDocument && editable && (
              <Button
                size="small"
                variant="ghost"
                icon={<BookOpen20Regular />}
                onClick={() => setReaderMode(true)}
              >
                阅读
              </Button>
            )}
            {editable && canFormat && (
              <Button
                size="small"
                variant="ghost"
                loading={formatting}
                icon={<Braces20Regular />}
                onClick={() => void formatCurrentText()}
              >
                格式化
              </Button>
            )}
            {editable && (
              <Button
                size="small"
                variant="primary"
                loading={saving}
                icon={<Save20Regular />}
                onClick={() => void onSaveText()}
              >
                保存
              </Button>
            )}
          </div>
        </div>
        <label className="code-editor-label" htmlFor="preview-code-editor">
          编辑 {file.name}
        </label>
        <Editor
          className="code-editor"
          textareaId="preview-code-editor"
          textareaClassName="code-editor-input"
          preClassName="code-editor-highlight"
          value={text}
          readOnly={!editable}
          padding={16}
          tabSize={2}
          insertSpaces
          highlight={(value) =>
            largeTextMode ? escapeCode(value) : highlightCode(value, ext)
          }
          onValueChange={(value) => editable && onTextChange(value)}
          onKeyDown={(event) => {
            if (
              editable &&
              (event.ctrlKey || event.metaKey) &&
              event.key.toLowerCase() === "s"
            ) {
              event.preventDefault();
              void onSaveText();
            }
            if (
              editable &&
              canFormat &&
              event.altKey &&
              event.shiftKey &&
              event.key.toLowerCase() === "f"
            ) {
              event.preventDefault();
              void formatCurrentText();
            }
          }}
        />
      </div>
    );
  }
  return <FilePreview file={file} url={url} settings={settings} />;
}
