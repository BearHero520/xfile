import type { StorageSource, StorageSourceInput } from "../types";
import {
  Add20Regular,
  ArrowSync20Regular,
  Cloud20Regular,
  Delete20Regular,
  Edit20Regular,
  Folder20Filled,
  Globe20Regular,
  HardDrive20Regular,
} from "@fluentui/react-icons";
import { useEffect, useMemo, useState } from "react";
import { api } from "../api";
import {
  Badge,
  Button,
  Empty,
  ErrorBanner,
  Field,
  Loading,
  Modal,
  PageHeader,
  Select,
  Switch,
} from "../components/ui";
import { useToast } from "../state";

const blank: StorageSourceInput = {
  name: "",
  key: "",
  type: "local",
  rootPath: "data/files",
  hiddenPaths: "",
  blockedPaths: "",
  public: false,
  enabled: true,
  orderNum: 0,
};
const providers = [
  {
    type: "local",
    label: "本地存储",
    icon: <HardDrive20Regular />,
    ready: true,
  },
  { type: "s3", label: "S3 / MinIO", icon: <Cloud20Regular />, ready: true },
  {
    type: "aliyun_oss",
    label: "阿里云 OSS",
    icon: <Cloud20Regular />,
    ready: true,
  },
  {
    type: "tencent_cos",
    label: "腾讯云 COS",
    icon: <Cloud20Regular />,
    ready: true,
  },
  { type: "webdav", label: "WebDAV", icon: <Globe20Regular />, ready: true },
];

export default function StoragePage() {
  const { show } = useToast();
  const [items, setItems] = useState<StorageSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<number | undefined>();
  const [form, setForm] = useState<StorageSourceInput>(blank);
  const [saving, setSaving] = useState(false);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await api.storageNodes());
    } catch (nextError) {
      setError(
        nextError instanceof Error ? nextError.message : "存储源加载失败",
      );
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);

  const stats = useMemo(
    () => ({
      enabled: items.filter((item) => item.enabled).length,
      public: items.filter((item) => item.public).length,
      rules: items.filter((item) => item.hiddenPaths || item.blockedPaths)
        .length,
    }),
    [items],
  );

  function create(type = "local") {
    setEditing(undefined);
    setForm({
      ...blank,
      type,
      name: providers.find((item) => item.type === type)?.label || "新存储源",
      key: `${type.replace(/_/g, "-")}-${items.length + 1}`,
      rootPath:
        type === "local"
          ? "data/files"
          : type === "webdav"
            ? JSON.stringify(
                { url: "", username: "", password: "", root: "" },
                null,
                2,
              )
            : JSON.stringify(
                {
                  endpoint: "",
                  bucket: "",
                  region: "",
                  accessKey: "",
                  secretKey: "",
                  prefix: "",
                  secure: true,
                },
                null,
                2,
              ),
      orderNum: items.length,
    });
    setOpen(true);
  }

  function edit(item: StorageSource) {
    setEditing(item.id);
    setForm({
      name: item.name,
      key: item.key,
      type: item.type,
      rootPath: prettyConfig(item.rootPath || ""),
      hiddenPaths: item.hiddenPaths || "",
      blockedPaths: item.blockedPaths || "",
      public: item.public,
      enabled: item.enabled,
      orderNum: item.orderNum,
    });
    setOpen(true);
  }

  async function save() {
    if (!form.name.trim() || !form.key.trim()) {
      show("名称和唯一标识不能为空", "error");
      return;
    }
    setSaving(true);
    try {
      const payload = {
        ...form,
        rootPath:
          form.type === "local"
            ? form.rootPath.trim()
            : compactConfig(form.rootPath),
      };
      await api.saveStorageNode(payload, editing);
      show(editing ? "存储源已更新" : "存储源已创建", "success");
      setOpen(false);
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "保存失败",
        "error",
      );
    } finally {
      setSaving(false);
    }
  }

  async function remove(item: StorageSource) {
    if (!confirm(`确定删除存储源“${item.name}”吗？`)) return;
    try {
      await api.deleteStorageNode(item.id);
      show("存储源已删除", "success");
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "删除失败",
        "error",
      );
    }
  }

  async function toggle(item: StorageSource, enabled: boolean) {
    try {
      await api.saveStorageNode(
        {
          name: item.name,
          key: item.key,
          type: item.type,
          rootPath: item.rootPath || "",
          hiddenPaths: item.hiddenPaths || "",
          blockedPaths: item.blockedPaths || "",
          public: item.public,
          enabled,
          orderNum: item.orderNum,
        },
        item.id,
      );
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "状态更新失败",
        "error",
      );
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="资源管理"
        title="存储源"
        description="管理本地目录、对象存储和 WebDAV，把不同存储统一呈现在文件工作台。"
        actions={
          <>
            <Button icon={<ArrowSync20Regular />} onClick={load}>
              刷新
            </Button>
            <Button
              variant="primary"
              icon={<Add20Regular />}
              onClick={() => create()}
            >
              添加存储源
            </Button>
          </>
        }
      />
      <section className="metric-grid">
        <Metric
          label="存储源总数"
          value={items.length}
          detail={`启用 ${stats.enabled}`}
        />
        <Metric
          label="公开入口"
          value={stats.public}
          detail="显示在访客文件页"
        />
        <Metric label="路径规则" value={stats.rules} detail="隐藏或禁止访问" />
        <Metric
          label="健康状态"
          value={
            items.length && stats.enabled === items.length ? "正常" : "需检查"
          }
          detail="按启停状态检查"
        />
      </section>
      <section className="provider-strip">
        {providers.map((provider) => (
          <button key={provider.type} onClick={() => create(provider.type)}>
            {provider.icon}
            <span>
              <strong>{provider.label}</strong>
              <small>立即接入</small>
            </span>
            <Add20Regular />
          </button>
        ))}
      </section>
      {error && <ErrorBanner error={error} onRetry={load} />}
      {loading ? (
        <Loading />
      ) : items.length === 0 ? (
        <Empty
          title="还没有存储源"
          description="添加本地目录或对象存储后即可开始管理文件。"
        />
      ) : (
        <section className="storage-grid">
          {items.map((item) => (
            <article className="storage-card glass-panel" key={item.id}>
              <header>
                <span className="storage-icon">
                  <Folder20Filled />
                </span>
                <div>
                  <strong>{item.name}</strong>
                  <small>
                    {item.typeLabel || item.type} · {item.key}
                  </small>
                </div>
                <Switch
                  label={`启用 ${item.name}`}
                  checked={item.enabled}
                  onChange={(value) => void toggle(item, value)}
                />
              </header>
              <div className="storage-card-body">
                <div>
                  <span>访问状态</span>
                  <Badge tone={item.public ? "success" : "neutral"}>
                    {item.public ? "公开" : "私有"}
                  </Badge>
                </div>
                <div>
                  <span>路径/配置</span>
                  <code>{summarizePath(item.rootPath)}</code>
                </div>
                <div>
                  <span>路径规则</span>
                  <strong>
                    {item.hiddenPaths || item.blockedPaths
                      ? "已配置"
                      : "未配置"}
                  </strong>
                </div>
              </div>
              <footer>
                <Button
                  variant="ghost"
                  icon={<Edit20Regular />}
                  onClick={() => edit(item)}
                >
                  编辑
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
      )}
      <Modal
        open={open}
        title={editing ? "编辑存储源" : "添加存储源"}
        description="接口参数采用 XFile 自有模型，不依赖旧系统配置命名。"
        onClose={() => setOpen(false)}
        size="large"
        footer={
          <>
            <Button onClick={() => setOpen(false)}>取消</Button>
            <Button variant="primary" disabled={saving} onClick={save}>
              {saving ? "保存中…" : "保存存储源"}
            </Button>
          </>
        }
      >
        <div className="form-grid">
          <Field label="存储类型">
            <Select
              value={form.type}
              onChange={(event) => create(event.target.value)}
            >
              {providers.map((provider) => (
                <option key={provider.type} value={provider.type}>
                  {provider.label}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="显示名称">
            <input
              value={form.name}
              onChange={(event) =>
                setForm({ ...form, name: event.target.value })
              }
            />
          </Field>
          <Field label="唯一标识" hint="创建后建议不要修改">
            <input
              value={form.key}
              onChange={(event) =>
                setForm({ ...form, key: event.target.value })
              }
            />
          </Field>
          <Field label="排序">
            <input
              type="number"
              value={form.orderNum}
              onChange={(event) =>
                setForm({ ...form, orderNum: Number(event.target.value) })
              }
            />
          </Field>
          <Field
            className="span-2"
            label={form.type === "local" ? "本地根目录" : "连接配置 JSON"}
          >
            <textarea
              rows={8}
              value={form.rootPath}
              onChange={(event) =>
                setForm({ ...form, rootPath: event.target.value })
              }
            />
          </Field>
          <Field label="隐藏路径" hint="每行一条规则">
            <textarea
              rows={5}
              value={form.hiddenPaths}
              onChange={(event) =>
                setForm({ ...form, hiddenPaths: event.target.value })
              }
            />
          </Field>
          <Field label="禁止访问路径" hint="每行一条规则">
            <textarea
              rows={5}
              value={form.blockedPaths}
              onChange={(event) =>
                setForm({ ...form, blockedPaths: event.target.value })
              }
            />
          </Field>
          <div className="switch-field">
            <span>
              <strong>公开访问</strong>
              <small>允许访客在文件页浏览</small>
            </span>
            <Switch
              label="公开访问"
              checked={form.public}
              onChange={(value) => setForm({ ...form, public: value })}
            />
          </div>
          <div className="switch-field">
            <span>
              <strong>启用存储源</strong>
              <small>停用后不会出现在文件工作台</small>
            </span>
            <Switch
              label="启用"
              checked={form.enabled}
              onChange={(value) => setForm({ ...form, enabled: value })}
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}

function Metric({
  label,
  value,
  detail,
}: {
  label: string;
  value: string | number;
  detail: string;
}) {
  return (
    <article className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </article>
  );
}
function prettyConfig(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}
function compactConfig(value: string) {
  return JSON.stringify(JSON.parse(value));
}
function summarizePath(value?: string) {
  if (!value) return "未配置";
  if (value.startsWith("{")) {
    try {
      const parsed = JSON.parse(value);
      return parsed.endpoint || parsed.url || parsed.bucket || "连接配置";
    } catch {
      return "连接配置";
    }
  }
  return value;
}
