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
import { useApp, useToast } from "../state";

type ObjectStorageType = "s3" | "aliyun_oss" | "tencent_cos";
type ObjectStorageConfig = {
  endpoint: string;
  bucket: string;
  region: string;
  accessKey: string;
  secretKey: string;
  prefix: string;
  secure: boolean;
};
type WebDAVStorageConfig = {
  url: string;
  username: string;
  password: string;
  root: string;
};
type FieldErrors = Record<string, string>;

const emptyObjectConfig: ObjectStorageConfig = {
  endpoint: "",
  bucket: "",
  region: "",
  accessKey: "",
  secretKey: "",
  prefix: "",
  secure: true,
};

const emptyWebDAVConfig: WebDAVStorageConfig = {
  url: "",
  username: "",
  password: "",
  root: "",
};

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
  const { refresh } = useApp();
  const { show } = useToast();
  const [items, setItems] = useState<StorageSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<number | undefined>();
  const [form, setForm] = useState<StorageSourceInput>(blank);
  const [saving, setSaving] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const objectConfig = useMemo(
    () => parseObjectConfig(form.rootPath),
    [form.rootPath],
  );
  const webDAVConfig = useMemo(
    () => parseWebDAVConfig(form.rootPath),
    [form.rootPath],
  );

  const syncHome = () => {
    void refresh().catch((nextError) =>
      show(
        nextError instanceof Error
          ? nextError.message
          : "首页存储源状态同步失败",
        "error",
      ),
    );
  };

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

  function clearFieldError(key: string) {
    setFieldErrors((current) => {
      if (!current[key]) return current;
      const next = { ...current };
      delete next[key];
      return next;
    });
  }

  function setInputField<K extends keyof StorageSourceInput>(
    key: K,
    value: StorageSourceInput[K],
  ) {
    clearFieldError(String(key));
    setForm((current) => ({ ...current, [key]: value }));
  }

  function setObjectConfigField<K extends keyof ObjectStorageConfig>(
    key: K,
    value: ObjectStorageConfig[K],
  ) {
    clearFieldError(`config.${String(key)}`);
    setForm((current) => {
      const next = { ...parseObjectConfig(current.rootPath), [key]: value };
      if (key === "region" && current.type !== "s3") next.endpoint = "";
      return {
        ...current,
        rootPath: JSON.stringify(next, null, 2),
      };
    });
  }

  function setWebDAVConfigField<K extends keyof WebDAVStorageConfig>(
    key: K,
    value: WebDAVStorageConfig[K],
  ) {
    clearFieldError(`config.${String(key)}`);
    setForm((current) => ({
      ...current,
      rootPath: JSON.stringify(
        { ...parseWebDAVConfig(current.rootPath), [key]: value },
        null,
        2,
      ),
    }));
  }

  function create(type = "local") {
    setEditing(undefined);
    setFieldErrors({});
    setForm({
      ...blank,
      type,
      name: providers.find((item) => item.type === type)?.label || "新存储源",
      key: `${type.replace(/_/g, "-")}-${items.length + 1}`,
      rootPath: defaultRootPath(type),
      orderNum: items.length,
    });
    setOpen(true);
  }

  function changeType(type: string) {
    setFieldErrors({});
    setForm((current) => ({
      ...current,
      type,
      name: editing
        ? current.name
        : providers.find((item) => item.type === type)?.label || current.name,
      key: editing
        ? current.key
        : `${type.replace(/_/g, "-")}-${items.length + 1}`,
      rootPath: defaultRootPath(type),
    }));
  }

  function edit(item: StorageSource) {
    setEditing(item.id);
    setFieldErrors({});
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
    const validation = validateStorageInput(form);
    if (Object.keys(validation.errors).length) {
      setFieldErrors(validation.errors);
      show("请检查标红的必填项和格式", "error");
      return;
    }
    setSaving(true);
    try {
      const payload = {
        ...form,
        name: form.name.trim(),
        key: form.key.trim(),
        rootPath: validation.rootPath,
      };
      const saved = await api.saveStorageNode(payload, editing);
      setItems((current) =>
        sortSources(
          editing
            ? current.map((item) => (item.id === saved.id ? saved : item))
            : [...current, saved],
        ),
      );
      show(editing ? "存储源已更新" : "存储源已创建", "success");
      setOpen(false);
      syncHome();
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
      setItems((current) => current.filter((source) => source.id !== item.id));
      show("存储源已删除", "success");
      syncHome();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "删除失败",
        "error",
      );
    }
  }

  async function toggle(item: StorageSource, enabled: boolean) {
    setItems((current) =>
      current.map((source) =>
        source.id === item.id ? { ...source, enabled } : source,
      ),
    );
    try {
      const saved = await api.saveStorageNode(
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
      setItems((current) =>
        current.map((source) => (source.id === saved.id ? saved : source)),
      );
      syncHome();
    } catch (nextError) {
      setItems((current) =>
        current.map((source) => (source.id === item.id ? item : source)),
      );
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
        description="必填项已标记；云厂商 Endpoint 与 HTTPS 会根据地域自动配置。"
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
              onChange={(event) => changeType(event.target.value)}
            >
              {providers.map((provider) => (
                <option key={provider.type} value={provider.type}>
                  {provider.label}
                </option>
              ))}
            </Select>
          </Field>
          <Field
            label="显示名称"
            required
            error={fieldErrors.name}
            hint="显示在首页和存储源卡片中"
          >
            <input
              value={form.name}
              placeholder="例如：团队文件"
              onChange={(event) => setInputField("name", event.target.value)}
            />
          </Field>
          <Field
            label="唯一标识"
            required
            error={fieldErrors.key}
            hint="2–64 位字母、数字、下划线或短横线，创建后建议不要修改"
          >
            <input
              value={form.key}
              placeholder="例如：team-files"
              onChange={(event) => setInputField("key", event.target.value)}
            />
          </Field>
          <Field label="排序" hint="数值越小越靠前">
            <input
              type="number"
              value={form.orderNum}
              onChange={(event) =>
                setInputField("orderNum", Number(event.target.value))
              }
            />
          </Field>
          {form.type === "local" && (
            <Field
              className="span-2"
              label="本地根目录"
              required
              error={fieldErrors.rootPath}
              hint="XFile 服务器上的目录路径；不存在时会自动创建"
            >
              <input
                value={form.rootPath}
                placeholder="例如：data/files 或 D:\\xfile\\files"
                onChange={(event) =>
                  setInputField("rootPath", event.target.value)
                }
              />
            </Field>
          )}
          {isObjectStorageType(form.type) && (
            <ObjectStorageFields
              type={form.type}
              config={objectConfig}
              errors={fieldErrors}
              onChange={setObjectConfigField}
            />
          )}
          {form.type === "webdav" && (
            <WebDAVStorageFields
              config={webDAVConfig}
              errors={fieldErrors}
              onChange={setWebDAVConfigField}
            />
          )}
          <Field label="隐藏路径" hint="每行一条规则">
            <textarea
              rows={5}
              value={form.hiddenPaths}
              onChange={(event) =>
                setInputField("hiddenPaths", event.target.value)
              }
            />
          </Field>
          <Field label="禁止访问路径" hint="每行一条规则">
            <textarea
              rows={5}
              value={form.blockedPaths}
              onChange={(event) =>
                setInputField("blockedPaths", event.target.value)
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

function ObjectStorageFields({
  type,
  config,
  errors,
  onChange,
}: {
  type: ObjectStorageType;
  config: ObjectStorageConfig;
  errors: FieldErrors;
  onChange: (key: keyof ObjectStorageConfig, value: string | boolean) => void;
}) {
  const isAliyun = type === "aliyun_oss";
  const isTencent = type === "tencent_cos";
  const vendor = isAliyun
    ? "阿里云 OSS"
    : isTencent
      ? "腾讯云 COS"
      : "S3 / MinIO";
  const accessKeyLabel = isAliyun
    ? "AccessKey ID"
    : isTencent
      ? "SecretId"
      : "Access Key";
  const secretKeyLabel = isAliyun
    ? "AccessKey Secret"
    : isTencent
      ? "SecretKey"
      : "Secret Key";
  const regionHint = isAliyun
    ? "从 OSS Bucket 概览页获取，例如 cn-hangzhou"
    : isTencent
      ? "从 COS 存储桶基本配置获取，例如 ap-guangzhou"
      : "选填；AWS S3 等服务可能需要，例如 us-east-1";
  const bucketHint = isAliyun
    ? "填写 Bucket 名称，不包含 Endpoint"
    : isTencent
      ? "填写完整存储桶名称，通常包含 APPID，例如 files-1250000000"
      : "填写已经创建好的 Bucket 名称";

  return (
    <>
      {type === "s3" ? (
        <Field
          className="span-2"
          label="Endpoint"
          required
          error={errors["config.endpoint"]}
          hint="填写完整服务地址，必须包含 http:// 或 https://；例如 http://127.0.0.1:9000"
        >
          <input
            value={config.endpoint}
            placeholder="https://s3.example.com"
            onChange={(event) => onChange("endpoint", event.target.value)}
          />
        </Field>
      ) : (
        <Field
          className="span-2"
          label="Endpoint（自动生成）"
          hint={`根据地域自动生成 ${vendor} 的 HTTPS 地址，无需手动填写`}
        >
          <input
            readOnly
            value={config.endpoint || providerEndpoint(type, config.region)}
            placeholder="填写地域后自动生成"
          />
        </Field>
      )}
      <Field
        label="Bucket"
        required
        error={errors["config.bucket"]}
        hint={bucketHint}
      >
        <input
          value={config.bucket}
          placeholder={isTencent ? "files-1250000000" : "my-bucket"}
          onChange={(event) => onChange("bucket", event.target.value)}
        />
      </Field>
      <Field
        label={type === "s3" ? "Region（选填）" : "地域"}
        required={type !== "s3"}
        error={errors["config.region"]}
        hint={regionHint}
      >
        <input
          value={config.region}
          placeholder={
            isAliyun ? "cn-hangzhou" : isTencent ? "ap-guangzhou" : "us-east-1"
          }
          onChange={(event) => onChange("region", event.target.value)}
        />
      </Field>
      <Field
        label={accessKeyLabel}
        required
        error={errors["config.accessKey"]}
        hint={`从${vendor}的访问密钥管理页面获取`}
      >
        <input
          value={config.accessKey}
          autoComplete="off"
          onChange={(event) => onChange("accessKey", event.target.value)}
        />
      </Field>
      <Field
        label={secretKeyLabel}
        required
        error={errors["config.secretKey"]}
        hint="仅保存在服务器配置中，请使用权限最小化的专用密钥"
      >
        <input
          type="password"
          value={config.secretKey}
          autoComplete="new-password"
          onChange={(event) => onChange("secretKey", event.target.value)}
        />
      </Field>
      <Field
        className="span-2"
        label="存储目录前缀（选填）"
        error={errors["config.prefix"]}
        hint="限制 XFile 使用 Bucket 内的某个子目录；留空表示使用 Bucket 根目录"
      >
        <input
          value={config.prefix}
          placeholder="例如：xfile/files"
          onChange={(event) => onChange("prefix", event.target.value)}
        />
      </Field>
    </>
  );
}

function WebDAVStorageFields({
  config,
  errors,
  onChange,
}: {
  config: WebDAVStorageConfig;
  errors: FieldErrors;
  onChange: (key: keyof WebDAVStorageConfig, value: string) => void;
}) {
  return (
    <>
      <Field
        className="span-2"
        label="WebDAV 地址"
        required
        error={errors["config.url"]}
        hint="填写远程 WebDAV 根地址，必须包含 http:// 或 https://"
      >
        <input
          value={config.url}
          placeholder="https://dav.example.com/remote.php/dav/files/user"
          onChange={(event) => onChange("url", event.target.value)}
        />
      </Field>
      <Field label="用户名（选填）" hint="匿名 WebDAV 可以留空">
        <input
          value={config.username}
          autoComplete="username"
          onChange={(event) => onChange("username", event.target.value)}
        />
      </Field>
      <Field label="密码（选填）" hint="支持 Basic Auth 账号密码">
        <input
          type="password"
          value={config.password}
          autoComplete="new-password"
          onChange={(event) => onChange("password", event.target.value)}
        />
      </Field>
      <Field
        className="span-2"
        label="远程根目录（选填）"
        error={errors["config.root"]}
        hint="只挂载 WebDAV 中的某个子目录；留空表示使用地址对应的根目录"
      >
        <input
          value={config.root}
          placeholder="例如：documents/team"
          onChange={(event) => onChange("root", event.target.value)}
        />
      </Field>
    </>
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

function defaultRootPath(type: string) {
  if (type === "local") return "data/files";
  if (type === "webdav") return JSON.stringify(emptyWebDAVConfig, null, 2);
  return JSON.stringify(emptyObjectConfig, null, 2);
}

function isObjectStorageType(value: string): value is ObjectStorageType {
  return value === "s3" || value === "aliyun_oss" || value === "tencent_cos";
}

function parseObjectConfig(value: string): ObjectStorageConfig {
  try {
    const parsed = JSON.parse(value) as Partial<ObjectStorageConfig>;
    const secure = parsed.secure !== false;
    const rawEndpoint = stringValue(parsed.endpoint);
    return {
      endpoint:
        rawEndpoint && !/^https?:\/\//i.test(rawEndpoint)
          ? `${secure ? "https" : "http"}://${rawEndpoint}`
          : rawEndpoint,
      bucket: stringValue(parsed.bucket),
      region: stringValue(parsed.region),
      accessKey: stringValue(parsed.accessKey),
      secretKey: stringValue(parsed.secretKey),
      prefix: stringValue(parsed.prefix),
      secure,
    };
  } catch {
    return { ...emptyObjectConfig };
  }
}

function parseWebDAVConfig(value: string): WebDAVStorageConfig {
  try {
    const parsed = JSON.parse(value) as Partial<WebDAVStorageConfig>;
    return {
      url: stringValue(parsed.url),
      username: stringValue(parsed.username),
      password: stringValue(parsed.password),
      root: stringValue(parsed.root),
    };
  } catch {
    return { ...emptyWebDAVConfig };
  }
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

function providerEndpoint(type: ObjectStorageType, region: string) {
  region = region.trim();
  if (!region) return "";
  if (type === "aliyun_oss") return `https://oss-${region}.aliyuncs.com`;
  if (type === "tencent_cos") return `https://cos.${region}.myqcloud.com`;
  return "";
}

function validateStorageInput(form: StorageSourceInput): {
  errors: FieldErrors;
  rootPath: string;
} {
  const errors: FieldErrors = {};
  const name = form.name.trim();
  const key = form.key.trim();
  if (!name) errors.name = "请输入显示名称";
  if (!key) errors.key = "请输入唯一标识";
  else if (!/^[A-Za-z0-9_-]{2,64}$/.test(key))
    errors.key = "只能使用 2–64 位字母、数字、下划线或短横线";

  if (form.type === "local") {
    const rootPath = form.rootPath.trim();
    if (!rootPath) errors.rootPath = "请输入本地根目录";
    return { errors, rootPath };
  }

  if (form.type === "webdav") {
    const config = parseWebDAVConfig(form.rootPath);
    const url = config.url.trim();
    const root = normalizeRemotePath(config.root);
    if (!url) errors["config.url"] = "请输入 WebDAV 地址";
    else if (!isHTTPURL(url, true))
      errors["config.url"] = "请输入有效的 http:// 或 https:// 地址";
    if (hasParentSegment(config.root))
      errors["config.root"] = "根目录不能包含 .. 路径";
    return {
      errors,
      rootPath: JSON.stringify({
        url,
        username: config.username.trim(),
        password: config.password,
        root,
      }),
    };
  }

  if (isObjectStorageType(form.type)) {
    const config = parseObjectConfig(form.rootPath);
    const bucket = config.bucket.trim();
    const region = config.region.trim();
    const accessKey = config.accessKey.trim();
    const secretKey = config.secretKey.trim();
    const prefix = normalizeRemotePath(config.prefix);
    const endpoint =
      form.type === "s3"
        ? config.endpoint.trim()
        : config.endpoint.trim() || providerEndpoint(form.type, region);

    if (form.type === "s3" && !endpoint)
      errors["config.endpoint"] = "请输入 Endpoint";
    else if (endpoint && !isHTTPURL(endpoint, false))
      errors["config.endpoint"] =
        "Endpoint 必须是有效的 http:// 或 https:// 地址，且不能包含路径";
    if (form.type !== "s3" && !region) errors["config.region"] = "请输入地域";
    else if (region && !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(region))
      errors["config.region"] = "地域格式不正确，请使用小写字母、数字和短横线";
    if (!bucket) errors["config.bucket"] = "请输入 Bucket 名称";
    else if (
      bucket.length < 3 ||
      bucket.length > 63 ||
      !/^[a-z0-9][a-z0-9.-]*[a-z0-9]$/.test(bucket)
    )
      errors["config.bucket"] = "Bucket 应为 3–63 位小写字母、数字、点或短横线";
    if (!accessKey) errors["config.accessKey"] = "请输入访问密钥 ID";
    if (!secretKey) errors["config.secretKey"] = "请输入访问密钥 Secret";
    if (hasParentSegment(config.prefix))
      errors["config.prefix"] = "目录前缀不能包含 .. 路径";

    let secure = true;
    if (endpoint) {
      try {
        secure = new URL(endpoint).protocol === "https:";
      } catch {
        secure = true;
      }
    }
    return {
      errors,
      rootPath: JSON.stringify({
        endpoint,
        bucket,
        region,
        accessKey,
        secretKey,
        prefix,
        secure,
      }),
    };
  }

  return { errors, rootPath: form.rootPath.trim() };
}

function isHTTPURL(value: string, allowPath: boolean) {
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:")
      return false;
    if (!parsed.hostname || parsed.username || parsed.password) return false;
    if (
      !allowPath &&
      (parsed.pathname !== "/" || parsed.search !== "" || parsed.hash !== "")
    )
      return false;
    return true;
  } catch {
    return false;
  }
}

function normalizeRemotePath(value: string) {
  return value
    .trim()
    .replaceAll("\\", "/")
    .replace(/^\/+|\/+$/g, "");
}

function hasParentSegment(value: string) {
  return normalizeRemotePath(value).split("/").includes("..");
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

function sortSources(items: StorageSource[]) {
  return [...items].sort(
    (left, right) => left.orderNum - right.orderNum || left.id - right.id,
  );
}
