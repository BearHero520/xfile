import type {
  AccountInput,
  SessionEntry,
  StorageSource,
  UserEntry,
} from "../types";
import {
  Add20Regular,
  ArrowSync20Regular,
  Delete20Regular,
  Edit20Regular,
  PersonAccounts20Regular,
  PlugDisconnected20Regular,
} from "@fluentui/react-icons";
import { useEffect, useMemo, useState } from "react";
import { api, formatTime } from "../api";
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

const operations = [
  "preview",
  "download",
  "upload",
  "rename",
  "move",
  "copy",
  "delete",
  "share",
  "directLinks",
];
const operationLabels: Record<string, string> = {
  preview: "预览",
  download: "下载",
  upload: "上传/新建",
  rename: "重命名",
  move: "移动",
  copy: "复制",
  delete: "删除",
  share: "分享",
  directLinks: "直链",
};
const blank: AccountInput = {
  username: "",
  password: "",
  role: "admin",
  enabled: true,
  storageSourceKeys: [],
  storageSourceRoots: {},
  disabledOperations: [],
};

export default function AccountsPage() {
  const { show } = useToast();
  const [users, setUsers] = useState<UserEntry[]>([]);
  const [sources, setSources] = useState<StorageSource[]>([]);
  const [sessions, setSessions] = useState<SessionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState<number | undefined>();
  const [form, setForm] = useState<AccountInput>(blank);
  const [modal, setModal] = useState<"account" | "sessions" | null>(null);
  const [activeUser, setActiveUser] = useState<UserEntry | null>(null);

  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const [nextUsers, nextSources] = await Promise.all([
        api.accounts(),
        api.storageNodes(),
      ]);
      setUsers(nextUsers);
      setSources(nextSources);
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "用户加载失败");
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);
  const stats = useMemo(
    () => ({
      admins: users.filter((user) => user.role === "super_admin").length,
      disabled: users.filter((user) => !user.enabled).length,
      sessions: users.reduce(
        (sum, user) => sum + (user.activeSessionCount || 0),
        0,
      ),
    }),
    [users],
  );

  function create() {
    setEditing(undefined);
    setForm(blank);
    setModal("account");
  }
  function edit(user: UserEntry) {
    setEditing(user.id);
    setForm({
      username: user.username,
      password: "",
      role: user.role,
      enabled: user.enabled,
      storageSourceKeys: user.storageSourceKeys || [],
      storageSourceRoots: user.storageSourceRoots || {},
      disabledOperations: user.disabledOperations || [],
    });
    setModal("account");
  }

  async function save() {
    try {
      await api.saveAccount(form, editing);
      show(editing ? "用户已更新" : "用户已创建", "success");
      setModal(null);
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "保存失败",
        "error",
      );
    }
  }
  async function remove(user: UserEntry) {
    if (!confirm(`确定删除用户“${user.username}”吗？`)) return;
    try {
      await api.deleteAccount(user.id);
      show("用户已删除", "success");
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "删除失败",
        "error",
      );
    }
  }
  async function toggle(user: UserEntry, enabled: boolean) {
    try {
      await api.saveAccount(
        {
          username: user.username,
          password: "",
          role: user.role,
          enabled,
          storageSourceKeys: user.storageSourceKeys || [],
          storageSourceRoots: user.storageSourceRoots || {},
          disabledOperations: user.disabledOperations || [],
        },
        user.id,
      );
      await load();
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "状态更新失败",
        "error",
      );
    }
  }
  async function openSessions(user: UserEntry) {
    setActiveUser(user);
    setModal("sessions");
    try {
      setSessions(await api.accountSessions(user.id));
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "会话加载失败",
        "error",
      );
    }
  }
  async function revoke(session: SessionEntry) {
    if (!activeUser || !confirm("确定让这个浏览器会话下线吗？")) return;
    await api.revokeAccountSession(activeUser.id, session.id);
    setSessions(await api.accountSessions(activeUser.id));
    await load();
  }
  async function revokeAll() {
    if (
      !activeUser ||
      !confirm(`确定让“${activeUser.username}”的所有会话下线吗？`)
    )
      return;
    await api.revokeAccountSessions(activeUser.id);
    setSessions([]);
    await load();
  }
  function toggleSource(key: string) {
    setForm((current) => ({
      ...current,
      storageSourceKeys: current.storageSourceKeys.includes(key)
        ? current.storageSourceKeys.filter((value) => value !== key)
        : [...current.storageSourceKeys, key],
    }));
  }
  function toggleOperation(value: string) {
    setForm((current) => ({
      ...current,
      disabledOperations: current.disabledOperations.includes(value)
        ? current.disabledOperations.filter((item) => item !== value)
        : [...current.disabledOperations, value],
    }));
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="安全与系统"
        title="用户管理"
        description="维护账号、角色、存储范围、操作权限和在线会话。"
        actions={
          <>
            <Button icon={<ArrowSync20Regular />} onClick={load}>
              刷新
            </Button>
            <Button variant="primary" icon={<Add20Regular />} onClick={create}>
              新建用户
            </Button>
          </>
        }
      />
      <section className="metric-grid">
        <Metric label="用户总数" value={users.length} />
        <Metric label="超级管理员" value={stats.admins} />
        <Metric label="已停用" value={stats.disabled} />
        <Metric label="活跃会话" value={stats.sessions} />
      </section>
      {error && <ErrorBanner error={error} onRetry={load} />}
      {loading ? (
        <Loading />
      ) : users.length === 0 ? (
        <Empty title="还没有用户" />
      ) : (
        <section className="table-panel glass-panel">
          <table className="data-table">
            <thead>
              <tr>
                <th>用户</th>
                <th>角色</th>
                <th>存储范围</th>
                <th>状态</th>
                <th>活跃会话</th>
                <th>创建时间</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id}>
                  <td>
                    <div className="identity-cell">
                      <span>
                        <PersonAccounts20Regular />
                      </span>
                      <div>
                        <strong>{user.username}</strong>
                        <small>ID {user.id}</small>
                      </div>
                    </div>
                  </td>
                  <td>
                    <Badge
                      tone={user.role === "super_admin" ? "success" : "info"}
                    >
                      {user.role === "super_admin"
                        ? "超级管理员"
                        : "工作区用户"}
                    </Badge>
                  </td>
                  <td>
                    {user.role === "super_admin"
                      ? "全部存储源"
                      : `${user.storageSourceKeys?.length || 0} 个存储源`}
                  </td>
                  <td>
                    <Switch
                      label={`启用 ${user.username}`}
                      checked={user.enabled}
                      onChange={(value) => void toggle(user, value)}
                    />
                  </td>
                  <td>
                    <Button
                      variant="ghost"
                      onClick={() => void openSessions(user)}
                    >
                      {user.activeSessionCount || 0} 个
                    </Button>
                  </td>
                  <td>{formatTime(user.createdAt)}</td>
                  <td>
                    <div className="row-actions">
                      <Button
                        variant="ghost"
                        icon={<Edit20Regular />}
                        onClick={() => edit(user)}
                      >
                        编辑
                      </Button>
                      <Button
                        variant="ghost"
                        icon={<Delete20Regular />}
                        onClick={() => void remove(user)}
                      >
                        删除
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}
      <Modal
        open={modal === "account"}
        title={editing ? "编辑用户" : "新建用户"}
        onClose={() => setModal(null)}
        size="large"
        footer={
          <>
            <Button onClick={() => setModal(null)}>取消</Button>
            <Button variant="primary" onClick={save}>
              保存用户
            </Button>
          </>
        }
      >
        <div className="form-grid">
          <Field label="用户名">
            <input
              value={form.username}
              onChange={(event) =>
                setForm({ ...form, username: event.target.value })
              }
            />
          </Field>
          <Field label={editing ? "新密码（留空不修改）" : "密码"}>
            <input
              type="password"
              value={form.password}
              onChange={(event) =>
                setForm({ ...form, password: event.target.value })
              }
            />
          </Field>
          <Field label="角色">
            <Select
              value={form.role}
              onChange={(event) =>
                setForm({ ...form, role: event.target.value })
              }
            >
              <option value="admin">工作区用户</option>
              <option value="super_admin">超级管理员</option>
            </Select>
          </Field>
          <div className="switch-field">
            <span>
              <strong>启用账号</strong>
              <small>停用后现有会话会失效</small>
            </span>
            <Switch
              label="启用账号"
              checked={form.enabled}
              onChange={(value) => setForm({ ...form, enabled: value })}
            />
          </div>
          {form.role !== "super_admin" && (
            <>
              <div className="span-2 option-section">
                <strong>可访问存储源</strong>
                <div className="check-grid">
                  {sources.map((source) => (
                    <label key={source.key}>
                      <input
                        type="checkbox"
                        checked={form.storageSourceKeys.includes(source.key)}
                        onChange={() => toggleSource(source.key)}
                      />
                      {source.name}
                    </label>
                  ))}
                </div>
              </div>
              <div className="span-2 option-section">
                <strong>禁用操作</strong>
                <div className="check-grid">
                  {operations.map((value) => (
                    <label key={value}>
                      <input
                        type="checkbox"
                        checked={form.disabledOperations.includes(value)}
                        onChange={() => toggleOperation(value)}
                      />
                      {operationLabels[value]}
                    </label>
                  ))}
                </div>
              </div>
              {form.storageSourceKeys.map((key) => (
                <Field
                  key={key}
                  label={`${sources.find((source) => source.key === key)?.name || key} 根路径`}
                  hint="每行一个允许访问的根目录，留空表示整个存储源"
                >
                  <textarea
                    rows={3}
                    value={(form.storageSourceRoots[key] || []).join("\n")}
                    onChange={(event) =>
                      setForm({
                        ...form,
                        storageSourceRoots: {
                          ...form.storageSourceRoots,
                          [key]: event.target.value
                            .split(/\r?\n/)
                            .map((item) => item.trim())
                            .filter(Boolean),
                        },
                      })
                    }
                  />
                </Field>
              ))}
            </>
          )}
        </div>
      </Modal>
      <Modal
        open={modal === "sessions"}
        title={`${activeUser?.username || ""} 的在线会话`}
        onClose={() => setModal(null)}
        size="large"
        footer={
          <>
            <Button onClick={() => setModal(null)}>关闭</Button>
            <Button
              variant="danger"
              icon={<PlugDisconnected20Regular />}
              onClick={revokeAll}
            >
              全部下线
            </Button>
          </>
        }
      >
        <div className="session-list">
          {sessions.length === 0 ? (
            <Empty title="没有活跃会话" />
          ) : (
            sessions.map((session) => (
              <article key={session.id}>
                <div>
                  <strong>{session.current ? "当前会话" : session.ip}</strong>
                  <small>{session.userAgent || "未知客户端"}</small>
                  <small>最后活动：{formatTime(session.lastSeenAt)}</small>
                </div>
                <Button variant="ghost" onClick={() => void revoke(session)}>
                  下线
                </Button>
              </article>
            ))
          )}
        </div>
      </Modal>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <article className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>实时账号状态</small>
    </article>
  );
}
