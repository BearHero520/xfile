import {
  ArrowReset20Regular,
  ArrowSync20Regular,
  CheckmarkCircle20Filled,
  CloudArrowUp24Filled,
  Delete20Regular,
  Image20Regular,
  Save20Regular,
} from "@fluentui/react-icons";
import { useCallback, useEffect, useId, useState } from "react";
import { api } from "../api";
import { Button, Field, Loading, PageHeader } from "../components/ui";
import { useApp, useToast } from "../state";
import type { ThemeSettings } from "../types";

const defaultTheme: ThemeSettings = {
  themePreset: "ocean",
  brandLogo: "",
  brandFavicon: "",
  brandingVersion: "0",
};

const presets = [
  {
    id: "ocean",
    name: "海洋蓝",
    description: "清爽、可靠，适合通用管理后台",
    colors: ["#087cf0", "#08aaa5", "#101827"],
  },
  {
    id: "violet",
    name: "星云紫",
    description: "更具品牌感，层次柔和克制",
    colors: ["#7c3aed", "#c026d3", "#21143a"],
  },
  {
    id: "emerald",
    name: "翡翠绿",
    description: "自然稳健，强调安全与效率",
    colors: ["#059669", "#0d9488", "#0c241d"],
  },
  {
    id: "sunset",
    name: "日落橙",
    description: "温暖醒目，适合活跃型团队",
    colors: ["#ea580c", "#e11d48", "#32180f"],
  },
  {
    id: "graphite",
    name: "石墨灰",
    description: "中性专业，让内容成为主角",
    colors: ["#475569", "#64748b", "#111827"],
  },
  {
    id: "sky",
    name: "晴空青",
    description: "明亮通透，适合轻快的内容空间",
    colors: ["#0891b2", "#0ea5e9", "#083344"],
  },
  {
    id: "rose",
    name: "樱花粉",
    description: "柔亮醒目，带来更鲜明的品牌气质",
    colors: ["#db2777", "#f43f5e", "#3b0a24"],
  },
  {
    id: "sunflower",
    name: "向日葵黄",
    description: "温暖明快，突出重要操作与状态",
    colors: ["#d97706", "#facc15", "#30220a"],
  },
] as const;

const allowedTypes = new Set([
  "image/png",
  "image/jpeg",
  "image/webp",
  "image/gif",
  "image/x-icon",
  "image/vnd.microsoft.icon",
]);

function fileToDataURL(file: File, maxBytes: number) {
  if (!allowedTypes.has(file.type)) {
    return Promise.reject(new Error("仅支持 PNG、JPEG、WebP、GIF 或 ICO 图片"));
  }
  if (file.size > maxBytes) {
    return Promise.reject(
      new Error(`图片不能超过 ${Math.round(maxBytes / 1024)} KB`),
    );
  }
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(new Error("图片读取失败"));
    reader.readAsDataURL(file);
  });
}

function isUploadedImage(value: string) {
  return value.startsWith("data:image/");
}

export default function ThemePage() {
  const { site, refresh } = useApp();
  const { show } = useToast();
  const [form, setForm] = useState<ThemeSettings>(() => ({
    ...defaultTheme,
    themePreset: site?.preferences.themePreset || "ocean",
  }));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const savedPreset = site?.preferences.themePreset || "ocean";

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setForm(await api.themeSettings());
    } catch (error) {
      show(error instanceof Error ? error.message : "主题设置加载失败", "error");
    } finally {
      setLoading(false);
    }
  }, [show]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    document.documentElement.dataset.brandTheme =
      form.themePreset || savedPreset || "ocean";
    return () => {
      document.documentElement.dataset.brandTheme = savedPreset;
    };
  }, [form.themePreset, savedPreset]);

  async function save() {
    setSaving(true);
    try {
      const next = await api.saveThemeSettings(form);
      setForm(next);
      await refresh();
      show("主题与品牌资源已保存，所有浏览器将使用新配置", "success");
    } catch (error) {
      show(error instanceof Error ? error.message : "保存失败", "error");
    } finally {
      setSaving(false);
    }
  }

  function reset() {
    setForm((current) => ({
      ...current,
      themePreset: "ocean",
      brandLogo: "",
      brandFavicon: "",
    }));
  }

  if (loading) {
    return (
      <div className="page-stack">
        <PageHeader eyebrow="系统设置" title="主题管理" />
        <section className="glass-panel settings-panel">
          <Loading label="正在加载主题设置" />
        </section>
      </div>
    );
  }

  return (
    <div className="page-stack theme-page">
      <PageHeader
        eyebrow="系统设置"
        title="主题管理"
        description="统一设置全站配色、Logo 与浏览器图标，保存后对所有设备生效。"
        actions={
          <>
            <Button icon={<ArrowReset20Regular />} onClick={reset}>
              恢复默认
            </Button>
            <Button icon={<ArrowSync20Regular />} onClick={() => void load()}>
              重新加载
            </Button>
            <Button
              variant="primary"
              icon={<Save20Regular />}
              loading={saving}
              onClick={() => void save()}
            >
              保存并应用
            </Button>
          </>
        }
      />

      <section className="glass-panel settings-panel">
        <div className="setting-group">
          <div className="theme-section-heading">
            <div>
              <h2>预设主题</h2>
              <p>主题只统一品牌色，不会覆盖每位用户自己的明暗模式、圆角和动效偏好。</p>
            </div>
            <span className="theme-live-badge">
              <CheckmarkCircle20Filled /> 当前预览
            </span>
          </div>
          <div className="theme-preset-grid" role="radiogroup" aria-label="预设主题">
            {presets.map((preset) => {
              const selected = form.themePreset === preset.id;
              return (
                <button
                  key={preset.id}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  className={`theme-preset-card ${selected ? "is-selected" : ""}`}
                  onClick={() =>
                    setForm((current) => ({
                      ...current,
                      themePreset: preset.id,
                    }))
                  }
                >
                  <span className="theme-swatches" aria-hidden="true">
                    {preset.colors.map((color) => (
                      <i key={color} style={{ backgroundColor: color }} />
                    ))}
                  </span>
                  <span>
                    <strong>{preset.name}</strong>
                    <small>{preset.description}</small>
                  </span>
                  <span className="theme-selected-mark">
                    {selected && <CheckmarkCircle20Filled />}
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        <div className="setting-group">
          <div className="theme-section-heading">
            <div>
              <h2>品牌资源</h2>
              <p>可以上传图片，也可以填写以 / 开头的站内路径或 http/https 地址。</p>
            </div>
          </div>
          <div className="brand-asset-grid">
            <AssetEditor
              kind="logo"
              title="站点 Logo"
              description="显示在前台、登录页、分享页和后台导航中。"
              value={form.brandLogo}
              maxBytes={2 * 1024 * 1024}
              onChange={(brandLogo) =>
                setForm((current) => ({ ...current, brandLogo }))
              }
              onError={(message) => show(message, "error")}
            />
            <AssetEditor
              kind="favicon"
              title="浏览器图标 favicon"
              description="用于页面标签和浏览器收藏夹，建议使用正方形 PNG 或 ICO。"
              value={form.brandFavicon}
              maxBytes={512 * 1024}
              onChange={(brandFavicon) =>
                setForm((current) => ({ ...current, brandFavicon }))
              }
              onError={(message) => show(message, "error")}
            />
          </div>
        </div>

        <div className="setting-group">
          <h2>效果预览</h2>
          <div className="brand-preview">
            <aside>
              <PreviewLogo value={form.brandLogo} />
              <span>
                <strong>{site?.siteName || "XFile"}</strong>
                <small>后台管理</small>
              </span>
            </aside>
            <div>
              <span className="brand-preview-label">主题强调色</span>
              <strong>{presets.find((item) => item.id === form.themePreset)?.name}</strong>
              <p>保存后，其他浏览器刷新或重新打开站点即可读取同一份服务端配置。</p>
              <button type="button">主要操作</button>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

function AssetEditor({
  kind,
  title,
  description,
  value,
  maxBytes,
  onChange,
  onError,
}: {
  kind: "logo" | "favicon";
  title: string;
  description: string;
  value: string;
  maxBytes: number;
  onChange: (value: string) => void;
  onError: (message: string) => void;
}) {
  const inputId = useId();
  const uploaded = isUploadedImage(value);

  async function chooseFile(file?: File) {
    if (!file) return;
    try {
      onChange(await fileToDataURL(file, maxBytes));
    } catch (error) {
      onError(error instanceof Error ? error.message : "图片读取失败");
    }
  }

  return (
    <article className="brand-asset-card">
      <div className={`brand-asset-preview is-${kind}`}>
        {value ? (
          <img src={value} alt={`${title} 预览`} />
        ) : kind === "logo" ? (
          <CloudArrowUp24Filled />
        ) : (
          <span>XF</span>
        )}
      </div>
      <div className="brand-asset-content">
        <header>
          <div>
            <strong>{title}</strong>
            <p>{description}</p>
          </div>
          {value && (
            <Button
              size="small"
              variant="ghost"
              icon={<Delete20Regular />}
              onClick={() => onChange("")}
            >
              清除
            </Button>
          )}
        </header>
        <Field
          label="资源地址"
          hint={
            uploaded
              ? "已选择本地图片，保存后将写入服务端数据库。"
              : `支持站内路径或远程地址；上传上限 ${Math.round(maxBytes / 1024)} KB。`
          }
        >
          <input
            value={uploaded ? "" : value}
            placeholder={uploaded ? "已上传图片" : kind === "logo" ? "/logo.png" : "/favicon.png"}
            onChange={(event) => onChange(event.target.value)}
          />
        </Field>
        <input
          id={inputId}
          className="brand-file-input"
          type="file"
          accept=".png,.jpg,.jpeg,.webp,.gif,.ico,image/png,image/jpeg,image/webp,image/gif,image/x-icon"
          onChange={(event) => {
            void chooseFile(event.target.files?.[0]);
            event.currentTarget.value = "";
          }}
        />
        <label className="button button-secondary button-medium" htmlFor={inputId}>
          <Image20Regular />
          选择图片
        </label>
      </div>
    </article>
  );
}

function PreviewLogo({ value }: { value: string }) {
  return (
    <span className={`brand-mark ${value ? "has-image" : ""}`} aria-hidden="true">
      {value ? <img src={value} alt="" /> : <CloudArrowUp24Filled />}
    </span>
  );
}
