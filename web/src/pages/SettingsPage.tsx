import {
  ArrowSync20Regular,
  Save20Regular,
  ShieldCheckmark20Regular,
} from "@fluentui/react-icons";
import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../api";
import {
  Button,
  Field,
  Loading,
  PageHeader,
  Select,
  Switch,
} from "../components/ui";
import { useToast } from "../state";

export type SettingsSection =
  | "site"
  | "view"
  | "webdav"
  | "link"
  | "upload"
  | "visibility"
  | "user-rules"
  | "security"
  | "access";

const defaults: Record<string, string> = {
  siteName: "XFile",
  rootName: "首页",
  externalPreviewProvider: "disabled",
  externalPreviewBaseUrl: "",
  externalPreviewTemplate: "",
  layout: "full",
  mobileLayout: "full",
  tableSize: "small",
  fileClickMode: "dbclick",
  mobileFileClickMode: "click",
  rootShowStorage: "enabled",
  showAnnouncement: "enabled",
  showLogin: "enabled",
  defaultSortField: "name",
  defaultSortOrder: "asc",
  maxShowSize: "1000",
  loadMoreSize: "50",
  enableNormalDownloadConfirm: "disabled",
  enablePackageDownloadConfirm: "enabled",
  enableBatchDownloadConfirm: "enabled",
  webdav: "disabled",
  webdavMountPath: "/dav",
  webdavUsername: "",
  webdavPassword: "",
  webdavReadOnly: "disabled",
  webdavAllowAnonymous: "disabled",
  showLinkButton: "enabled",
  showShortLink: "enabled",
  showPathLink: "enabled",
  refererProtection: "disabled",
  refererAllowList: "",
  downloadLimitPerMinute: "0",
  allowUpload: "enabled",
  maxUploadMB: "512",
  uploadAllowExtensions: "",
  uploadDenyExtensions: "",
  uploadPathAllowList: "",
  uploadPathDenyList: "",
  uploadOverwrite: "enabled",
  privatePathList: "",
  directoryPasswordRules: "",
  disabledOperations: "",
  loginLimitPerMinute: "5",
  loginCaptcha: "disabled",
  sharePasswordLimitPerMinute: "5",
  ipAllowList: "",
  ipDenyList: "",
};

let preferencesCache: Record<string, string> | null = null;
let preferencesRequest: Promise<Record<string, string>> | null = null;

function getPreferences(force = false) {
  if (!force && preferencesCache) return Promise.resolve(preferencesCache);
  if (!force && preferencesRequest) return preferencesRequest;

  const request = api
    .preferences()
    .then((preferences) => {
      preferencesCache = { ...defaults, ...preferences };
      return preferencesCache;
    })
    .finally(() => {
      if (preferencesRequest === request) preferencesRequest = null;
    });
  preferencesRequest = request;
  return request;
}

const sectionInfo: Record<
  SettingsSection,
  { eyebrow: string; title: string; description: string }
> = {
  site: {
    eyebrow: "系统设置",
    title: "站点设置",
    description: "配置站点名称、首页名称和文件预览服务。",
  },
  view: {
    eyebrow: "系统设置",
    title: "显示设置",
    description: "控制文件列表布局、点击方式、排序和前台按钮。",
  },
  webdav: {
    eyebrow: "系统设置",
    title: "WebDAV",
    description: "通过标准 WebDAV 协议挂载和管理 XFile 文件。",
  },
  link: {
    eyebrow: "链接与统计",
    title: "直/短链设置",
    description: "控制前台链接入口、防盗链和下载限频。",
  },
  upload: {
    eyebrow: "规则与权限",
    title: "上传规则",
    description: "限制上传大小、文件类型、目录和覆盖行为。",
  },
  visibility: {
    eyebrow: "规则与权限",
    title: "显示规则",
    description: "配置全局私有路径和目录密码规则。",
  },
  "user-rules": {
    eyebrow: "规则与权限",
    title: "用户规则",
    description: "设置所有普通用户共同遵循的文件操作限制。",
  },
  security: {
    eyebrow: "规则与权限",
    title: "安全设置",
    description: "控制登录验证码、登录限频和分享密码保护。",
  },
  access: {
    eyebrow: "规则与权限",
    title: "访问控制",
    description: "通过 IP 白名单和黑名单控制站点访问。",
  },
};

export default function SettingsPage({
  section,
}: {
  section: SettingsSection;
}) {
  const { show } = useToast();
  const [form, setForm] = useState<Record<string, string>>(
    () => preferencesCache || defaults,
  );
  const [loading, setLoading] = useState(() => preferencesCache === null);
  const [saving, setSaving] = useState(false);
  const info = useMemo(() => sectionInfo[section], [section]);

  const load = useCallback(
    async (force = false) => {
      if (force || !preferencesCache) setLoading(true);
      try {
        setForm(await getPreferences(force));
      } catch (error) {
        show(error instanceof Error ? error.message : "设置加载失败", "error");
      } finally {
        setLoading(false);
      }
    },
    [show],
  );

  useEffect(() => {
    void load();
  }, [load]);

  async function save() {
    setSaving(true);
    try {
      preferencesCache = {
        ...defaults,
        ...(await api.savePreferences(form)),
      };
      setForm(preferencesCache);
      show("设置已保存", "success");
    } catch (error) {
      show(error instanceof Error ? error.message : "保存失败", "error");
    } finally {
      setSaving(false);
    }
  }

  const set = (key: string, value: string) =>
    setForm((current) => ({ ...current, [key]: value }));

  if (loading) return <Loading />;

  return (
    <div className="page-stack">
      <PageHeader
        {...info}
        actions={
          <>
            <Button
              icon={<ArrowSync20Regular />}
              onClick={() => void load(true)}
            >
              重新加载
            </Button>
            <Button
              variant="primary"
              icon={<Save20Regular />}
              disabled={saving}
              onClick={save}
            >
              {saving ? "保存中…" : "保存设置"}
            </Button>
          </>
        }
      />
      <section className="settings-panel glass-panel">
        {section === "site" && (
          <>
            <SettingGroup title="站点信息">
              <div className="form-grid">
                <Field label="站点名称">
                  <input
                    value={form.siteName}
                    onChange={(event) => set("siteName", event.target.value)}
                  />
                </Field>
                <Field label="首页名称">
                  <input
                    value={form.rootName}
                    onChange={(event) => set("rootName", event.target.value)}
                  />
                </Field>
              </div>
            </SettingGroup>
            <SettingGroup title="预览服务">
              <div className="form-grid">
                <Field label="外部预览服务">
                  <Select
                    value={form.externalPreviewProvider}
                    onChange={(event) =>
                      set("externalPreviewProvider", event.target.value)
                    }
                  >
                    <option value="disabled">关闭</option>
                    <option value="onlyoffice">OnlyOffice</option>
                    <option value="kkfileview">kkFileView</option>
                    <option value="custom">自定义</option>
                  </Select>
                </Field>
                <Field label="预览服务地址">
                  <input
                    value={form.externalPreviewBaseUrl}
                    onChange={(event) =>
                      set("externalPreviewBaseUrl", event.target.value)
                    }
                    placeholder="https://preview.example.com"
                  />
                </Field>
                <Field className="span-2" label="自定义预览模板">
                  <input
                    value={form.externalPreviewTemplate}
                    onChange={(event) =>
                      set("externalPreviewTemplate", event.target.value)
                    }
                    placeholder="使用 {url} 作为文件地址占位符"
                  />
                </Field>
              </div>
            </SettingGroup>
          </>
        )}

        {section === "view" && (
          <>
            <SettingGroup title="布局与交互">
              <div className="form-grid">
                <Field label="桌面端布局">
                  <Select
                    value={form.layout}
                    onChange={(event) => set("layout", event.target.value)}
                  >
                    <option value="full">全宽</option>
                    <option value="center">居中</option>
                  </Select>
                </Field>
                <Field label="移动端布局">
                  <Select
                    value={form.mobileLayout}
                    onChange={(event) =>
                      set("mobileLayout", event.target.value)
                    }
                  >
                    <option value="full">全宽</option>
                    <option value="center">居中</option>
                  </Select>
                </Field>
                <Field label="表格密度">
                  <Select
                    value={form.tableSize}
                    onChange={(event) => set("tableSize", event.target.value)}
                  >
                    <option value="small">紧凑</option>
                    <option value="medium">标准</option>
                    <option value="large">宽松</option>
                  </Select>
                </Field>
                <div className="interaction-rule-card span-2">
                  <strong>文件点击规则</strong>
                  <span>
                    默认单击打开文件或文件夹；勾选任意项目后进入选择模式，单击项目将继续加入或取消选择。
                  </span>
                </div>
                <ToggleField
                  title="根目录显示存储源"
                  detail="与 XFile 根目录的存储源入口行为一致"
                  checked={form.rootShowStorage === "enabled"}
                  onChange={(value) =>
                    set("rootShowStorage", value ? "enabled" : "disabled")
                  }
                />
              </div>
            </SettingGroup>
            <SettingGroup title="前台内容">
              <div className="form-grid">
                <ToggleField
                  title="显示登录入口"
                  detail="允许访客从前台进入登录页"
                  checked={form.showLogin === "enabled"}
                  onChange={(value) =>
                    set("showLogin", value ? "enabled" : "disabled")
                  }
                />
              </div>
            </SettingGroup>
            <SettingGroup title="排序与加载">
              <div className="form-grid">
                <Field label="默认排序字段">
                  <Select
                    value={form.defaultSortField}
                    onChange={(event) =>
                      set("defaultSortField", event.target.value)
                    }
                  >
                    <option value="name">文件名</option>
                    <option value="size">大小</option>
                    <option value="modifiedAt">修改时间</option>
                  </Select>
                </Field>
                <Field label="默认排序方向">
                  <Select
                    value={form.defaultSortOrder}
                    onChange={(event) =>
                      set("defaultSortOrder", event.target.value)
                    }
                  >
                    <option value="asc">升序</option>
                    <option value="desc">降序</option>
                  </Select>
                </Field>
                <Field label="首次最多显示">
                  <input
                    type="number"
                    min="1"
                    value={form.maxShowSize}
                    onChange={(event) => set("maxShowSize", event.target.value)}
                  />
                </Field>
                <Field label="每次加载更多">
                  <input
                    type="number"
                    min="1"
                    value={form.loadMoreSize}
                    onChange={(event) =>
                      set("loadMoreSize", event.target.value)
                    }
                  />
                </Field>
              </div>
            </SettingGroup>
            <SettingGroup title="下载确认">
              <div className="form-grid">
                <ToggleField
                  title="普通文件下载确认"
                  detail="下载单个普通文件前二次确认"
                  checked={form.enableNormalDownloadConfirm === "enabled"}
                  onChange={(value) =>
                    set(
                      "enableNormalDownloadConfirm",
                      value ? "enabled" : "disabled",
                    )
                  }
                />
                <ToggleField
                  title="文件夹打包确认"
                  detail="下载文件夹压缩包前二次确认"
                  checked={form.enablePackageDownloadConfirm === "enabled"}
                  onChange={(value) =>
                    set(
                      "enablePackageDownloadConfirm",
                      value ? "enabled" : "disabled",
                    )
                  }
                />
                <ToggleField
                  title="批量下载确认"
                  detail="下载多个项目之前二次确认"
                  checked={form.enableBatchDownloadConfirm === "enabled"}
                  onChange={(value) =>
                    set(
                      "enableBatchDownloadConfirm",
                      value ? "enabled" : "disabled",
                    )
                  }
                />
              </div>
            </SettingGroup>
          </>
        )}

        {section === "webdav" && (
          <>
            <SettingGroup title="WebDAV 服务">
              <div className="form-grid">
                <ToggleField
                  title="启用 WebDAV"
                  detail="允许兼容客户端挂载 XFile"
                  checked={form.webdav === "enabled"}
                  onChange={(value) =>
                    set("webdav", value ? "enabled" : "disabled")
                  }
                />
                <ToggleField
                  title="只读模式"
                  detail="禁止客户端写入和删除"
                  checked={form.webdavReadOnly === "enabled"}
                  onChange={(value) =>
                    set("webdavReadOnly", value ? "enabled" : "disabled")
                  }
                />
                <Field label="挂载路径">
                  <input
                    value={form.webdavMountPath}
                    onChange={(event) =>
                      set("webdavMountPath", event.target.value)
                    }
                  />
                </Field>
                <ToggleField
                  title="允许匿名访问"
                  detail="仅建议用于受信任网络"
                  checked={form.webdavAllowAnonymous === "enabled"}
                  onChange={(value) =>
                    set("webdavAllowAnonymous", value ? "enabled" : "disabled")
                  }
                />
                <Field label="独立用户名">
                  <input
                    value={form.webdavUsername}
                    onChange={(event) =>
                      set("webdavUsername", event.target.value)
                    }
                    placeholder="留空则使用 XFile 账号"
                  />
                </Field>
                <Field label="独立密码">
                  <input
                    type="password"
                    value={form.webdavPassword}
                    onChange={(event) =>
                      set("webdavPassword", event.target.value)
                    }
                  />
                </Field>
              </div>
            </SettingGroup>
            <div className="webdav-address">
              <span>客户端地址</span>
              <code>
                {window.location.origin}
                {form.webdavMountPath || "/dav"}
              </code>
            </div>
          </>
        )}

        {section === "link" && (
          <>
            <SettingGroup title="前台链接入口">
              <div className="form-grid">
                <ToggleField
                  title="显示生成链接按钮"
                  detail="在文件列表和预览层显示链接操作"
                  checked={form.showLinkButton === "enabled"}
                  onChange={(value) =>
                    set("showLinkButton", value ? "enabled" : "disabled")
                  }
                />
                <ToggleField
                  title="显示短链"
                  detail="显示 XFile 分享链接"
                  checked={form.showShortLink === "enabled"}
                  onChange={(value) =>
                    set("showShortLink", value ? "enabled" : "disabled")
                  }
                />
                <ToggleField
                  title="显示路径直链"
                  detail="显示文件下载直链"
                  checked={form.showPathLink === "enabled"}
                  onChange={(value) =>
                    set("showPathLink", value ? "enabled" : "disabled")
                  }
                />
              </div>
            </SettingGroup>
            <SettingGroup title="防盗链与限频">
              <div className="form-grid">
                <ToggleField
                  title="启用 Referer 防盗链"
                  detail="只允许本站或允许列表中的来源访问"
                  checked={form.refererProtection === "enabled"}
                  onChange={(value) =>
                    set("refererProtection", value ? "enabled" : "disabled")
                  }
                />
                <Field label="每分钟下载次数">
                  <input
                    type="number"
                    min="0"
                    value={form.downloadLimitPerMinute}
                    onChange={(event) =>
                      set("downloadLimitPerMinute", event.target.value)
                    }
                  />
                </Field>
                <Field
                  className="span-2"
                  label="Referer 允许列表"
                  hint="每行一个域名，留空只允许当前站点"
                >
                  <textarea
                    rows={4}
                    value={form.refererAllowList}
                    onChange={(event) =>
                      set("refererAllowList", event.target.value)
                    }
                  />
                </Field>
              </div>
            </SettingGroup>
          </>
        )}

        {section === "upload" && (
          <SettingGroup title="上传限制">
            <div className="form-grid">
              <ToggleField
                title="允许上传"
                detail="关闭后所有账号都不能上传或新建文件"
                checked={form.allowUpload === "enabled"}
                onChange={(value) =>
                  set("allowUpload", value ? "enabled" : "disabled")
                }
              />
              <ToggleField
                title="允许覆盖同名文件"
                detail="关闭后同名文件上传会被拒绝"
                checked={form.uploadOverwrite === "enabled"}
                onChange={(value) =>
                  set("uploadOverwrite", value ? "enabled" : "disabled")
                }
              />
              <Field label="单文件上限（MB）">
                <input
                  type="number"
                  min="1"
                  value={form.maxUploadMB}
                  onChange={(event) => set("maxUploadMB", event.target.value)}
                />
              </Field>
              <Field label="允许的扩展名" hint="逗号或换行分隔，例如 pdf,docx">
                <textarea
                  rows={3}
                  value={form.uploadAllowExtensions}
                  onChange={(event) =>
                    set("uploadAllowExtensions", event.target.value)
                  }
                />
              </Field>
              <Field label="禁止的扩展名">
                <textarea
                  rows={3}
                  value={form.uploadDenyExtensions}
                  onChange={(event) =>
                    set("uploadDenyExtensions", event.target.value)
                  }
                />
              </Field>
              <Field label="允许上传的路径">
                <textarea
                  rows={3}
                  value={form.uploadPathAllowList}
                  onChange={(event) =>
                    set("uploadPathAllowList", event.target.value)
                  }
                />
              </Field>
              <Field label="禁止上传的路径">
                <textarea
                  rows={3}
                  value={form.uploadPathDenyList}
                  onChange={(event) =>
                    set("uploadPathDenyList", event.target.value)
                  }
                />
              </Field>
            </div>
          </SettingGroup>
        )}

        {section === "visibility" && (
          <>
            <SettingGroup title="路径显示规则">
              <div className="form-grid">
                <Field
                  className="span-2"
                  label="私有路径"
                  hint="每行一个相对路径；访客、公开分享和直链都不能访问"
                >
                  <textarea
                    rows={6}
                    value={form.privatePathList}
                    onChange={(event) =>
                      set("privatePathList", event.target.value)
                    }
                    placeholder={"secret\ninternal/contracts"}
                  />
                </Field>
                <Field
                  className="span-2"
                  label="目录密码规则"
                  hint="每行使用 路径 = 密码"
                >
                  <textarea
                    rows={6}
                    value={form.directoryPasswordRules}
                    onChange={(event) =>
                      set("directoryPasswordRules", event.target.value)
                    }
                    placeholder={
                      "vault = open\nvault/private = stronger-password"
                    }
                  />
                </Field>
              </div>
            </SettingGroup>
            <div className="security-callout">
              <ShieldCheckmark20Regular />
              <span>
                <strong>规则叠加</strong>
                <small>
                  全局显示规则会与每个存储源的隐藏、阻止路径同时生效。
                </small>
              </span>
            </div>
          </>
        )}

        {section === "user-rules" && (
          <SettingGroup title="全局操作限制">
            <div className="form-grid">
              <Field
                className="span-2"
                label="禁用操作"
                hint="逗号分隔：preview, download, upload, rename, move, copy, delete, share, directLinks"
              >
                <input
                  value={form.disabledOperations}
                  onChange={(event) =>
                    set("disabledOperations", event.target.value)
                  }
                />
              </Field>
            </div>
            <p className="setting-help">
              这里设置所有普通用户的共同限制；单个用户的额外限制在“用户管理”中配置。
            </p>
          </SettingGroup>
        )}

        {section === "security" && (
          <SettingGroup title="登录与分享安全">
            <div className="form-grid">
              <ToggleField
                title="登录验证码"
                detail="达到风控条件时要求完成验证码"
                checked={form.loginCaptcha === "enabled"}
                onChange={(value) =>
                  set("loginCaptcha", value ? "enabled" : "disabled")
                }
              />
              <Field label="每分钟登录尝试">
                <input
                  type="number"
                  min="0"
                  value={form.loginLimitPerMinute}
                  onChange={(event) =>
                    set("loginLimitPerMinute", event.target.value)
                  }
                />
              </Field>
              <Field label="每分钟分享密码尝试">
                <input
                  type="number"
                  min="0"
                  value={form.sharePasswordLimitPerMinute}
                  onChange={(event) =>
                    set("sharePasswordLimitPerMinute", event.target.value)
                  }
                />
              </Field>
            </div>
          </SettingGroup>
        )}

        {section === "access" && (
          <SettingGroup title="IP 访问控制">
            <div className="form-grid">
              <Field
                label="IP 白名单"
                hint="每行一个 IP 或 CIDR；留空表示不限制"
              >
                <textarea
                  rows={7}
                  value={form.ipAllowList}
                  onChange={(event) => set("ipAllowList", event.target.value)}
                  placeholder={"192.0.2.10\n10.0.0.0/8"}
                />
              </Field>
              <Field label="IP 黑名单" hint="黑名单优先于白名单">
                <textarea
                  rows={7}
                  value={form.ipDenyList}
                  onChange={(event) => set("ipDenyList", event.target.value)}
                />
              </Field>
            </div>
          </SettingGroup>
        )}
      </section>
    </div>
  );
}

function SettingGroup({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="setting-group">
      <h2>{title}</h2>
      {children}
    </section>
  );
}

function ToggleField({
  title,
  detail,
  checked,
  onChange,
}: {
  title: string;
  detail: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <div className="switch-field">
      <span>
        <strong>{title}</strong>
        <small>{detail}</small>
      </span>
      <Switch label={title} checked={checked} onChange={onChange} />
    </div>
  );
}
