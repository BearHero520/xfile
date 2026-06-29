import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Banner,
  Button,
  Descriptions,
  Empty,
  Input,
  Layout,
  Modal,
  Notification,
  Popconfirm,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconCloud,
  IconDelete,
  IconDownload,
  IconFolder,
  IconFolderOpen,
  IconPlus,
  IconRefresh,
  IconSearch,
  IconShareStroked,
  IconUpload
} from '@douyinfe/semi-icons';

type FileItem = {
  name: string;
  path: string;
  type: 'file' | 'folder';
  size: number;
  modified: string;
};

type FileResponse = {
  path: string;
  items: FileItem[];
};

type Share = {
  key: string;
  path: string;
  expiresAt: string;
  createdAt: string;
};

const api = {
  async list(path: string): Promise<FileResponse> {
    return request(`/api/files?path=${encodeURIComponent(path)}`);
  },
  async createFolder(path: string, name: string) {
    return request('/api/folders', {
      method: 'POST',
      body: JSON.stringify({ path, name })
    });
  },
  async remove(path: string) {
    return request(`/api/files?path=${encodeURIComponent(path)}`, { method: 'DELETE' });
  },
  async rename(path: string, newName: string) {
    return request('/api/rename', {
      method: 'POST',
      body: JSON.stringify({ path, newName })
    });
  },
  async share(path: string, expiresInHours: number): Promise<Share> {
    return request('/api/share', {
      method: 'POST',
      body: JSON.stringify({ path, expiresInHours })
    });
  }
};

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    ...init
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || '请求失败');
  }
  return payload as T;
}

export default function App() {
  const [path, setPath] = useState('');
  const [items, setItems] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyword, setKeyword] = useState('');
  const [shareUrl, setShareUrl] = useState('');
  const fileInput = useRef<HTMLInputElement>(null);

  const filtered = useMemo(() => {
    const word = keyword.trim().toLowerCase();
    if (!word) return items;
    return items.filter((item) => item.name.toLowerCase().includes(word));
  }, [items, keyword]);

  const breadcrumb = useMemo(() => {
    const parts = path.split('/').filter(Boolean);
    return [{ name: '首页', path: '' }].concat(
      parts.map((part, index) => ({
        name: part,
        path: parts.slice(0, index + 1).join('/')
      }))
    );
  }, [path]);

  async function load(nextPath = path) {
    setLoading(true);
    try {
      const data = await api.list(nextPath);
      setPath(data.path);
      setItems(data.items);
    } catch (error) {
      notifyError(error);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load('');
  }, []);

  async function createFolder() {
    let name = '';
    Modal.confirm({
      title: '新建文件夹',
      content: <Input autoFocus placeholder="文件夹名称" onChange={(value) => (name = value)} />,
      okText: '创建',
      onOk: async () => {
        if (!name.trim()) return Promise.reject(new Error('请输入文件夹名称'));
        await api.createFolder(path, name.trim());
        await load();
      }
    });
  }

  async function uploadFiles(files: FileList | null) {
    if (!files?.length) return;
    const form = new FormData();
    Array.from(files).forEach((file) => form.append('files', file));
    setLoading(true);
    try {
      const response = await fetch(`/api/upload?path=${encodeURIComponent(path)}`, {
        method: 'POST',
        body: form
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload.error || '上传失败');
      Notification.success({ title: '上传完成', content: `已上传 ${payload.count ?? files.length} 个文件` });
      await load();
    } catch (error) {
      notifyError(error);
    } finally {
      setLoading(false);
      if (fileInput.current) fileInput.current.value = '';
    }
  }

  async function renameItem(item: FileItem) {
    let newName = item.name;
    Modal.confirm({
      title: '重命名',
      content: <Input defaultValue={item.name} onChange={(value) => (newName = value)} />,
      okText: '保存',
      onOk: async () => {
        await api.rename(item.path, newName.trim());
        await load();
      }
    });
  }

  async function shareItem(item: FileItem) {
    try {
      const data = await api.share(item.path, 72);
      const url = `${window.location.origin}/s/${data.key}`;
      setShareUrl(url);
      await navigator.clipboard?.writeText(url).catch(() => undefined);
    } catch (error) {
      notifyError(error);
    }
  }

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      render: (_: string, item: FileItem) => (
        <button className="file-name" onClick={() => (item.type === 'folder' ? load(item.path) : undefined)}>
          {item.type === 'folder' ? <IconFolderOpen /> : <IconCloud />}
          <span>{item.name}</span>
        </button>
      )
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 110,
      render: (type: FileItem['type']) => <Tag color={type === 'folder' ? 'blue' : 'green'}>{type === 'folder' ? '文件夹' : '文件'}</Tag>
    },
    {
      title: '大小',
      dataIndex: 'size',
      width: 130,
      render: (size: number, item: FileItem) => (item.type === 'folder' ? '-' : formatSize(size))
    },
    {
      title: '修改时间',
      dataIndex: 'modified',
      width: 210,
      render: (value: string) => new Date(value).toLocaleString()
    },
    {
      title: '操作',
      width: 330,
      render: (_: unknown, item: FileItem) => (
        <Space>
          {item.type === 'file' && (
            <Button icon={<IconDownload />} onClick={() => (window.location.href = `/api/download?path=${encodeURIComponent(item.path)}`)}>
              下载
            </Button>
          )}
          <Button icon={<IconShareStroked />} onClick={() => shareItem(item)}>
            分享
          </Button>
          <Button onClick={() => renameItem(item)}>重命名</Button>
          <Popconfirm
            title="确认删除？"
            content="删除后当前版本不会进入回收站。"
            onConfirm={async () => {
              await api.remove(item.path);
              await load();
            }}
          >
            <Button type="danger" icon={<IconDelete />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      )
    }
  ];

  return (
    <Layout className="shell">
      <Layout.Sider className="sidebar">
        <div className="brand">
          <IconCloud size="extra-large" />
          <strong>xfile</strong>
        </div>
        <Button block theme="solid" icon={<IconFolder />} onClick={() => load('')}>
          文件中心
        </Button>
        <div className="sidebar-panel">
          <Typography.Text strong>能力清单</Typography.Text>
          <ul>
            <li>本地文件索引</li>
            <li>上传下载管理</li>
            <li>分享链接</li>
            <li>Docker 部署</li>
          </ul>
        </div>
      </Layout.Sider>
      <Layout.Content className="content">
        <header className="topbar">
          <div>
            <Typography.Title heading={3}>文件中心</Typography.Title>
            <div className="crumbs">
              {breadcrumb.map((part, index) => (
                <button key={part.path || 'root'} onClick={() => load(part.path)}>
                  {part.name}
                  {index < breadcrumb.length - 1 && <span>/</span>}
                </button>
              ))}
            </div>
          </div>
          <Space>
            <Input prefix={<IconSearch />} placeholder="搜索当前目录" value={keyword} onChange={setKeyword} />
            <Button icon={<IconRefresh />} onClick={() => load()} />
            <Button icon={<IconPlus />} onClick={createFolder}>
              新建文件夹
            </Button>
            <Button theme="solid" icon={<IconUpload />} onClick={() => fileInput.current?.click()}>
              上传
            </Button>
            <input ref={fileInput} className="hidden-input" multiple type="file" onChange={(event) => uploadFiles(event.target.files)} />
          </Space>
        </header>

        <Banner
          type="info"
          closeIcon={null}
          title="xfile 私有云盘"
          description="当前版本提供本地存储、文件管理、分享链接和容器化部署，后续可扩展用户权限、WebDAV、离线下载与对象存储。"
        />

        <section className="metrics">
          <Descriptions
            data={[
              { key: '当前目录', value: path || '/' },
              { key: '文件数量', value: String(items.filter((item) => item.type === 'file').length) },
              { key: '文件夹数量', value: String(items.filter((item) => item.type === 'folder').length) }
            ]}
            row
          />
        </section>

        <Spin spinning={loading}>
          <Table
            className="file-table"
            columns={columns}
            dataSource={filtered}
            pagination={false}
            rowKey="path"
            empty={<Empty title="这里还没有文件" description="上传一些文件，xfile 就开始工作。" />}
          />
        </Spin>
      </Layout.Content>

      <Modal title="分享链接已创建" visible={Boolean(shareUrl)} footer={null} onCancel={() => setShareUrl('')}>
        <Input value={shareUrl} readonly />
        <Typography.Paragraph className="share-note">链接有效期 72 小时，已尝试复制到剪贴板。</Typography.Paragraph>
      </Modal>
    </Layout>
  );
}

function notifyError(error: unknown) {
  Notification.error({
    title: '操作失败',
    content: error instanceof Error ? error.message : '未知错误'
  });
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 * 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`;
  return `${(size / 1024 / 1024 / 1024).toFixed(1)} GB`;
}
