import type { AnnouncementEntry, AnnouncementInput } from "../types";
import {
  Add20Regular,
  ArrowSync20Regular,
  Delete20Regular,
  Edit20Regular,
  MegaphoneLoud20Regular,
  Save20Regular,
} from "@fluentui/react-icons";
import { useCallback, useEffect, useState } from "react";
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
  Switch,
} from "../components/ui";
import { useApp, useToast } from "../state";

const emptyDraft: AnnouncementInput = {
  title: "",
  content: "",
  published: true,
};

export default function AnnouncementsPage() {
  const { refresh } = useApp();
  const { show } = useToast();
  const [items, setItems] = useState<AnnouncementEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [editingID, setEditingID] = useState<number | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [draft, setDraft] = useState<AnnouncementInput>(emptyDraft);
  const [saving, setSaving] = useState(false);
  const contentLength = Array.from(draft.content).length;

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setItems(await api.announcements());
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : "公告加载失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  function openCreate() {
    setEditingID(null);
    setDraft(emptyDraft);
    setEditorOpen(true);
  }

  function openEdit(item: AnnouncementEntry) {
    setEditingID(item.id);
    setDraft({
      title: item.title,
      content: item.content,
      published: item.published,
    });
    setEditorOpen(true);
  }

  async function save() {
    setSaving(true);
    try {
      const saved = await api.saveAnnouncement(draft, editingID || undefined);
      setItems((current) => [
        saved,
        ...current.filter((item) => item.id !== saved.id),
      ]);
      setEditorOpen(false);
      await refresh();
      show(editingID ? "公告已更新" : "公告已创建", "success");
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "保存失败",
        "error",
      );
    } finally {
      setSaving(false);
    }
  }

  async function toggle(item: AnnouncementEntry, published: boolean) {
    setItems((current) =>
      current.map((entry) =>
        entry.id === item.id ? { ...entry, published } : entry,
      ),
    );
    try {
      const saved = await api.saveAnnouncement(
        { title: item.title, content: item.content, published },
        item.id,
      );
      setItems((current) =>
        current.map((entry) => (entry.id === saved.id ? saved : entry)),
      );
      await refresh();
      show(published ? "公告已公开" : "公告已转为草稿", "success");
    } catch (nextError) {
      setItems((current) =>
        current.map((entry) =>
          entry.id === item.id
            ? { ...entry, published: item.published }
            : entry,
        ),
      );
      show(
        nextError instanceof Error ? nextError.message : "状态更新失败",
        "error",
      );
    }
  }

  async function remove(item: AnnouncementEntry) {
    if (!confirm(`确定删除公告“${item.title}”吗？此操作不可撤销。`)) return;
    try {
      await api.deleteAnnouncement(item.id);
      setItems((current) => current.filter((entry) => entry.id !== item.id));
      await refresh();
      show("公告已删除", "success");
    } catch (nextError) {
      show(
        nextError instanceof Error ? nextError.message : "删除失败",
        "error",
      );
    }
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="系统设置"
        title="公告管理"
        description="发布多条网站公告，并控制每条公告是否向首页访客公开。"
        actions={
          <>
            <Button icon={<ArrowSync20Regular />} onClick={() => void load()}>
              刷新
            </Button>
            <Button
              variant="primary"
              icon={<Add20Regular />}
              onClick={openCreate}
            >
              新建公告
            </Button>
          </>
        }
      />
      {error && <ErrorBanner error={error} onRetry={() => void load()} />}
      {loading ? (
        <Loading />
      ) : items.length === 0 ? (
        <Empty
          title="还没有公告"
          description="创建第一条公告后，首页访客会看到公告入口和未读小红点。"
        />
      ) : (
        <section className="table-panel glass-panel">
          <table className="data-table">
            <thead>
              <tr>
                <th>公告</th>
                <th>状态</th>
                <th>更新时间</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td>
                    <div className="identity-cell announcement-identity">
                      <span>
                        <MegaphoneLoud20Regular />
                      </span>
                      <div>
                        <strong>{item.title}</strong>
                        <small>{item.content}</small>
                      </div>
                    </div>
                  </td>
                  <td>
                    <div className="inline-status">
                      <Switch
                        label={`${item.published ? "隐藏" : "公开"} ${item.title}`}
                        checked={item.published}
                        onChange={(value) => void toggle(item, value)}
                      />
                      <Badge tone={item.published ? "success" : "neutral"}>
                        {item.published ? "已公开" : "草稿"}
                      </Badge>
                    </div>
                  </td>
                  <td>{formatTime(item.updatedAt)}</td>
                  <td>
                    <div className="announcement-actions">
                      <Button
                        size="small"
                        icon={<Edit20Regular />}
                        onClick={() => openEdit(item)}
                      >
                        编辑
                      </Button>
                      <Button
                        size="small"
                        variant="danger"
                        icon={<Delete20Regular />}
                        onClick={() => void remove(item)}
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
        open={editorOpen}
        title={editingID ? "编辑公告" : "新建公告"}
        description="公开后的公告会立即出现在首页，访客无需登录即可查看。"
        size="large"
        onClose={() => !saving && setEditorOpen(false)}
        footer={
          <>
            <Button disabled={saving} onClick={() => setEditorOpen(false)}>
              取消
            </Button>
            <Button
              variant="primary"
              icon={<Save20Regular />}
              loading={saving}
              onClick={() => void save()}
            >
              {editingID ? "保存修改" : "创建公告"}
            </Button>
          </>
        }
      >
        <div className="announcement-editor">
          <Field label="公告标题" required>
            <input
              autoFocus
              maxLength={120}
              value={draft.title}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  title: event.target.value,
                }))
              }
              placeholder="例如：系统维护通知"
            />
          </Field>
          <Field
            label="公告内容"
            hint={`已填写 ${contentLength} / 10000 字`}
            required
          >
            <textarea
              rows={10}
              maxLength={10000}
              value={draft.content}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  content: event.target.value,
                }))
              }
              placeholder="请输入要向访问者展示的公告内容"
            />
          </Field>
          <div className="switch-field">
            <span>
              <strong>立即公开</strong>
              <small>关闭后将保存为草稿，不会在首页展示</small>
            </span>
            <Switch
              label="立即公开"
              checked={draft.published}
              onChange={(value) =>
                setDraft((current) => ({ ...current, published: value }))
              }
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
