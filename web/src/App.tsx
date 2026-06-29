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
  SideSheet,
  Space,
  Spin,
  Table,
  Tag,
  Typography
} from '@douyinfe/semi-ui';
import {
  IconCloud,
  IconCopy,
  IconDelete,
  IconDownload,
  IconExit,
  IconEyeOpened,
  IconFolder,
  IconFolderOpen,
  IconLink,
  IconListView,
  IconLock,
  IconPlus,
  IconRefresh,
  IconSearch,
  IconShareStroked,
  IconUpload,
  IconUser
} from '@douyinfe/semi-icons';

type PreviewType = 'folder' | 'image' | 'video' | 'audio' | 'pdf' | 'text' | 'download';

type FileItem = {
  name: string;
  path: string;
  type: 'file' | 'folder';
  size: number;
  modified: string;
  mime: string;
  previewType: PreviewType;
};

type FileResponse = {
  path: string;
  items: FileItem[];
};

type Share = {
  key: string;
  path: string;
  hasPassword: boolean;
  expiresAt: string;
  createdAt: string;
};

type ShareResponse = {
  items: Share[];
};

type DirectLink = {
  key: string;
  path: string;
  name: string;
  createdAt: string;
  expiresAt: string | null;
  allowedReferers?: string[];
  allowedIPs?: string[];
  rateLimitKBps?: number;
  downloadCount: number;
  lastAccessAt: string | null;
};

type DirectLinkResponse = {
  items: DirectLink[];
};

type AccessLog = {
  id: string;
  type: 'share' | 'direct';
  key: string;
  path: string;
  ip: string;
  referer: string;
  userAgent: string;
  status: number;
  bytes: number;
  durationMs: number;
  message: string;
  createdAt: string;
};

type AccessLogResponse = {
  items: AccessLog[];
};

type SearchResponse = {
  query: string;
  items: FileItem[];
  limit: number;
};

type StorageStats = {
  fileCount: number;
  folderCount: number;
  totalSize: number;
  shareCount: number;
  directCount: number;
  logCount: number;
};

type AuthState = {
  authenticated: boolean;
  username?: string;
  expiresAt?: string;
};

const api = {
  async me(): Promise<AuthState> {
    return request('/api/auth/me');
  },
  async login(username: string, password: string): Promise<AuthState> {
    return request('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password })
    });
  },
  async logout() {
    return request('/api/auth/logout', { method: 'POST' });
  },
  async list(path: string): Promise<FileResponse> {
    return request(`/api/files?path=${encodeURIComponent(path)}`);
  },
  async search(path: string, keyword: string): Promise<SearchResponse> {
    return request(`/api/search?path=${encodeURIComponent(path)}&q=${encodeURIComponent(keyword)}`);
  },
  async stats(): Promise<StorageStats> {
    return request('/api/stats');
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
  async share(path: string, expiresInHours: number, password: string): Promise<Share> {
    return request('/api/share', {
      method: 'POST',
      body: JSON.stringify({ path, expiresInHours, password })
    });
  },
  async shares(): Promise<ShareResponse> {
    return request('/api/shares');
  },
  async deleteShare(key: string) {
    return request(`/api/shares/${encodeURIComponent(key)}`, { method: 'DELETE' });
  },
  async createDirectLink(path: string, expiresInHours: number, options: { allowedReferers: string[]; allowedIPs: string[]; rateLimitKBps: number }): Promise<DirectLink> {
    return request('/api/direct-links', {
      method: 'POST',
      body: JSON.stringify({ path, expiresInHours, ...options })
    });
  },
  async directLinks(): Promise<DirectLinkResponse> {
    return request('/api/direct-links');
  },
  async deleteDirectLink(key: string) {
    return request(`/api/direct-links/${encodeURIComponent(key)}`, { method: 'DELETE' });
  },
  async accessLogs(): Promise<AccessLogResponse> {
    return request('/api/access-logs?limit=300');
  },
  async clearAccessLogs() {
    return request('/api/access-logs', { method: 'DELETE' });
  }
};

class AuthError extends Error {}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
    ...init
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    if (response.status === 401) {
      throw new AuthError(payload.error || '请先登录');
    }
    throw new Error(payload.error || '请求失败');
  }
  return payload as T;
}

export default function App() {
  const [auth, setAuth] = useState<AuthState>({ authenticated: false });
  const [authLoading, setAuthLoading] = useState(true);
  const [path, setPath] = useState('');
  const [items, setItems] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyword, setKeyword] = useState('');
  const [isSearchMode, setIsSearchMode] = useState(false);
  const [searchResults, setSearchResults] = useState<FileItem[]>([]);
  const [stats, setStats] = useState<StorageStats | null>(null);
  const [shares, setShares] = useState<Share[]>([]);
  const [directLinks, setDirectLinks] = useState<DirectLink[]>([]);
  const [accessLogs, setAccessLogs] = useState<AccessLog[]>([]);
  const [shareUrl, setShareUrl] = useState('');
  const [directUrl, setDirectUrl] = useState('');
  const [shareSheetVisible, setShareSheetVisible] = useState(false);
  const [directSheetVisible, setDirectSheetVisible] = useState(false);
  const [logSheetVisible, setLogSheetVisible] = useState(false);
  const [previewItem, setPreviewItem] = useState<FileItem | null>(null);
  const [previewText, setPreviewText] = useState('');
  const [previewLoading, setPreviewLoading] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  function handleAuthError(error: unknown) {
    if (error instanceof AuthError) {
      setAuth({ authenticated: false });
      setItems([]);
      setShares([]);
      setDirectLinks([]);
      setAccessLogs([]);
      setStats(null);
      return true;
    }
    return false;
  }

  const filtered = useMemo(() => {
    const word = keyword.trim().toLowerCase();
    if (!word || isSearchMode) return items;
    return items.filter((item) => item.name.toLowerCase().includes(word));
  }, [items, keyword, isSearchMode]);

  const tableItems = isSearchMode ? searchResults : filtered;

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
      setIsSearchMode(false);
      setSearchResults([]);
      await loadStats();
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    } finally {
      setLoading(false);
    }
  }

  async function loadStats() {
    try {
      setStats(await api.stats());
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  async function loadShares() {
    try {
      const data = await api.shares();
      setShares(data.items);
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  async function loadDirectLinks() {
    try {
      const data = await api.directLinks();
      setDirectLinks(data.items);
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  async function loadAccessLogs() {
    try {
      const data = await api.accessLogs();
      setAccessLogs(data.items);
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  useEffect(() => {
    void bootstrap();
  }, []);

  async function bootstrap() {
    setAuthLoading(true);
    try {
      const data = await api.me();
      setAuth(data);
      if (data.authenticated) {
        await load('');
        await loadShares();
        await loadDirectLinks();
        await loadAccessLogs();
      }
    } catch (error) {
      setAuth({ authenticated: false });
    } finally {
      setAuthLoading(false);
      setLoading(false);
    }
  }

  async function handleLogin(username: string, password: string) {
    try {
      const data = await api.login(username, password);
      setAuth(data);
      await load('');
      await loadShares();
      await loadDirectLinks();
      await loadAccessLogs();
      Notification.success({ title: '登录成功' });
    } catch (error) {
      notifyError(error);
    }
  }

  async function handleLogout() {
    try {
      await api.logout();
    } catch {
      // Cookie cleanup is best-effort; clear local state even if the request fails.
    }
    setAuth({ authenticated: false });
    setItems([]);
    setShares([]);
    setDirectLinks([]);
    setAccessLogs([]);
    setStats(null);
    setPath('');
    setKeyword('');
  }

  async function createFolder() {
    let name = '';
    Modal.confirm({
      title: '新建文件夹',
      content: <Input autoFocus placeholder="文件夹名称" onChange={(value) => (name = value)} />,
      okText: '创建',
      onOk: async () => {
        if (!name.trim()) return Promise.reject(new Error('请输入文件夹名称'));
        try {
          await api.createFolder(path, name.trim());
          await load();
        } catch (error) {
          if (!handleAuthError(error)) {
            notifyError(error);
            return Promise.reject(error);
          }
        }
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
      if (response.status === 401) throw new AuthError(payload.error || '请先登录');
      if (!response.ok) throw new Error(payload.error || '上传失败');
      Notification.success({ title: '上传完成', content: `已上传 ${payload.count ?? files.length} 个文件` });
      await load();
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
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
        if (!newName.trim()) return Promise.reject(new Error('请输入新名称'));
        try {
          await api.rename(item.path, newName.trim());
          await load();
        } catch (error) {
          if (!handleAuthError(error)) {
            notifyError(error);
            return Promise.reject(error);
          }
        }
      }
    });
  }

  function shareItem(item: FileItem) {
    let hours = 72;
    let password = '';
    Modal.confirm({
      title: '创建分享链接',
      content: (
        <div className="share-form">
          <Typography.Text>分享对象：/{item.path}</Typography.Text>
          <label>
            有效期
            <select defaultValue="72" onChange={(event) => (hours = Number(event.currentTarget.value))}>
              <option value="24">24 小时</option>
              <option value="72">3 天</option>
              <option value="168">7 天</option>
              <option value="720">30 天</option>
            </select>
          </label>
          <label>
            访问密码
            <input placeholder="留空则不设置密码" onChange={(event) => (password = event.currentTarget.value)} />
          </label>
        </div>
      ),
      okText: '创建',
      onOk: async () => {
        try {
          const data = await api.share(item.path, hours, password);
          const url = `${window.location.origin}/s/${data.key}`;
          setShareUrl(url);
          await navigator.clipboard?.writeText(url).catch(() => undefined);
          await loadShares();
          await loadStats();
        } catch (error) {
          if (!handleAuthError(error)) {
            notifyError(error);
            return Promise.reject(error);
          }
        }
      }
    });
  }

  function createDirectLink(item: FileItem) {
    let hours = 0;
    let allowedReferers = '';
    let allowedIPs = '';
    let rateLimitKBps = 0;
    Modal.confirm({
      title: '创建直链',
      content: (
        <div className="share-form">
          <Typography.Text>直链对象：/{item.path}</Typography.Text>
          <label>
            有效期
            <select defaultValue="0" onChange={(event) => (hours = Number(event.currentTarget.value))}>
              <option value="0">永久有效</option>
              <option value="24">24 小时</option>
              <option value="168">7 天</option>
              <option value="720">30 天</option>
            </select>
          </label>
          <label>
            Referer 白名单
            <textarea placeholder="每行一个域名或 URL 片段，留空则不限制" onChange={(event) => (allowedReferers = event.currentTarget.value)} />
          </label>
          <label>
            IP / CIDR 白名单
            <textarea placeholder="每行一个 IP 或 CIDR，如 203.0.113.10 或 10.0.0.0/8" onChange={(event) => (allowedIPs = event.currentTarget.value)} />
          </label>
          <label>
            限速 KB/s
            <input type="number" min="0" placeholder="0 表示不限速" onChange={(event) => (rateLimitKBps = Number(event.currentTarget.value || 0))} />
          </label>
        </div>
      ),
      okText: '创建',
      onOk: async () => {
        try {
          const data = await api.createDirectLink(item.path, hours, {
            allowedReferers: parseRules(allowedReferers),
            allowedIPs: parseRules(allowedIPs),
            rateLimitKBps
          });
          const url = directLinkUrl(data);
          setDirectUrl(url);
          await navigator.clipboard?.writeText(url).catch(() => undefined);
          await loadDirectLinks();
          await loadStats();
        } catch (error) {
          if (!handleAuthError(error)) {
            notifyError(error);
            return Promise.reject(error);
          }
        }
      }
    });
  }

  async function runGlobalSearch() {
    const word = keyword.trim();
    if (!word) {
      Notification.warning({ title: '请输入搜索关键词' });
      return;
    }
    setLoading(true);
    try {
      const data = await api.search(path, word);
      setSearchResults(data.items);
      setIsSearchMode(true);
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    } finally {
      setLoading(false);
    }
  }

  async function openPreview(item: FileItem) {
    if (item.type === 'folder') {
      await load(item.path);
      return;
    }
    setPreviewItem(item);
    setPreviewText('');
    if (item.previewType !== 'text') return;

    setPreviewLoading(true);
    try {
      const response = await fetch(previewUrl(item.path));
      if (response.status === 401) throw new AuthError('请先登录');
      if (!response.ok) throw new Error('预览失败');
      const text = await response.text();
      setPreviewText(text.slice(0, 300000));
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    } finally {
      setPreviewLoading(false);
    }
  }

  async function removeItem(item: FileItem) {
    try {
      await api.remove(item.path);
      await load();
      await loadStats();
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  async function removeShare(key: string) {
    try {
      await api.deleteShare(key);
      await loadShares();
      await loadStats();
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  async function removeDirectLink(key: string) {
    try {
      await api.deleteDirectLink(key);
      await loadDirectLinks();
      await loadStats();
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  async function clearLogs() {
    try {
      await api.clearAccessLogs();
      await loadAccessLogs();
      await loadStats();
    } catch (error) {
      if (!handleAuthError(error)) notifyError(error);
    }
  }

  function downloadItem(item: FileItem) {
    window.location.href = `/api/download?path=${encodeURIComponent(item.path)}`;
  }

  async function copyText(value: string) {
    await navigator.clipboard?.writeText(value).catch(() => undefined);
    Notification.success({ title: '已复制' });
  }

  if (authLoading) {
    return (
      <div className="auth-shell">
        <Spin size="large" />
      </div>
    );
  }

  if (!auth.authenticated) {
    return <LoginView onLogin={handleLogin} />;
  }

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      render: (_: string, item: FileItem) => (
        <button className="file-name" onClick={() => openPreview(item)}>
          {item.type === 'folder' ? <IconFolderOpen /> : <IconCloud />}
          <span>{isSearchMode ? `/${item.path}` : item.name}</span>
        </button>
      )
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 120,
      render: (_: string, item: FileItem) => (
        <Tag color={item.type === 'folder' ? 'blue' : previewColor(item.previewType)}>
          {item.type === 'folder' ? '文件夹' : previewLabel(item.previewType)}
        </Tag>
      )
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
      width: 470,
      render: (_: unknown, item: FileItem) => (
        <Space wrap>
          {item.type === 'file' && (
            <Button icon={<IconEyeOpened />} onClick={() => openPreview(item)}>
              预览
            </Button>
          )}
          <Button icon={<IconDownload />} onClick={() => downloadItem(item)}>
            下载
          </Button>
          <Button icon={<IconShareStroked />} onClick={() => shareItem(item)}>
            分享
          </Button>
          <Button icon={<IconLink />} onClick={() => createDirectLink(item)}>
            直链
          </Button>
          <Button onClick={() => renameItem(item)}>重命名</Button>
          <Popconfirm
            title="确认删除？"
            content="删除后当前版本不会进入回收站。"
            onConfirm={() => removeItem(item)}
          >
            <Button type="danger" icon={<IconDelete />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      )
    }
  ];

  const shareColumns = [
    {
      title: '路径',
      dataIndex: 'path',
      render: (value: string) => <Typography.Text ellipsis={{ showTooltip: true }}>/{value}</Typography.Text>
    },
    {
      title: '链接',
      dataIndex: 'key',
      width: 120,
      render: (value: string) => (
        <Button icon={<IconCopy />} onClick={() => copyText(`${window.location.origin}/s/${value}`)}>
          复制
        </Button>
      )
    },
    {
      title: '密码',
      dataIndex: 'hasPassword',
      width: 90,
      render: (value: boolean) => <Tag color={value ? 'orange' : 'grey'}>{value ? '已设置' : '无'}</Tag>
    },
    {
      title: '过期时间',
      dataIndex: 'expiresAt',
      width: 190,
      render: (value: string) => new Date(value).toLocaleString()
    },
    {
      title: '操作',
      width: 100,
      render: (_: unknown, item: Share) => (
        <Popconfirm title="删除分享？" onConfirm={() => removeShare(item.key)}>
          <Button type="danger" icon={<IconDelete />} />
        </Popconfirm>
      )
    }
  ];

  const directColumns = [
    {
      title: '路径',
      dataIndex: 'path',
      render: (value: string) => <Typography.Text ellipsis={{ showTooltip: true }}>/{value}</Typography.Text>
    },
    {
      title: '直链',
      dataIndex: 'key',
      width: 120,
      render: (_: string, item: DirectLink) => (
        <Button icon={<IconCopy />} onClick={() => copyText(directLinkUrl(item))}>
          复制
        </Button>
      )
    },
    {
      title: '访问',
      dataIndex: 'downloadCount',
      width: 90
    },
    {
      title: '控制',
      width: 210,
      render: (_: unknown, item: DirectLink) => (
        <div className="link-controls">
          <Tag color={(item.allowedReferers?.length ?? 0) > 0 ? 'blue' : 'grey'}>Referer {item.allowedReferers?.length ?? 0}</Tag>
          <Tag color={(item.allowedIPs?.length ?? 0) > 0 ? 'green' : 'grey'}>IP {item.allowedIPs?.length ?? 0}</Tag>
          <Tag color={item.rateLimitKBps ? 'orange' : 'grey'}>{item.rateLimitKBps ? `${item.rateLimitKBps} KB/s` : '不限速'}</Tag>
        </div>
      )
    },
    {
      title: '最后访问',
      dataIndex: 'lastAccessAt',
      width: 190,
      render: (value: string | null) => (value ? new Date(value).toLocaleString() : '-')
    },
    {
      title: '过期时间',
      dataIndex: 'expiresAt',
      width: 190,
      render: (value: string | null) => (value ? new Date(value).toLocaleString() : '永久')
    },
    {
      title: '操作',
      width: 100,
      render: (_: unknown, item: DirectLink) => (
        <Popconfirm title="删除直链？" onConfirm={() => removeDirectLink(item.key)}>
          <Button type="danger" icon={<IconDelete />} />
        </Popconfirm>
      )
    }
  ];

  const logColumns = [
    {
      title: '时间',
      dataIndex: 'createdAt',
      width: 190,
      render: (value: string) => new Date(value).toLocaleString()
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 90,
      render: (value: AccessLog['type']) => <Tag color={value === 'direct' ? 'blue' : 'purple'}>{value === 'direct' ? '直链' : '分享'}</Tag>
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: number) => <Tag color={value >= 400 ? 'red' : 'green'}>{value}</Tag>
    },
    {
      title: '路径',
      dataIndex: 'path',
      render: (value: string) => <Typography.Text ellipsis={{ showTooltip: true }}>{value ? `/${value}` : '-'}</Typography.Text>
    },
    {
      title: 'IP',
      dataIndex: 'ip',
      width: 150
    },
    {
      title: 'Referer',
      dataIndex: 'referer',
      width: 220,
      render: (value: string) => <Typography.Text ellipsis={{ showTooltip: true }}>{value || '-'}</Typography.Text>
    },
    {
      title: '字节',
      dataIndex: 'bytes',
      width: 110,
      render: (value: number) => formatSize(value)
    },
    {
      title: '耗时',
      dataIndex: 'durationMs',
      width: 100,
      render: (value: number) => `${value} ms`
    }
  ];

  return (
    <Layout className="shell">
      <Layout.Sider className="sidebar">
        <div className="brand">
          <IconCloud size="extra-large" />
          <strong>xfile</strong>
        </div>
        <div className="session-card">
          <IconUser />
          <span>{auth.username || 'admin'}</span>
        </div>
        <Button block theme="solid" icon={<IconFolder />} onClick={() => load('')}>
          文件中心
        </Button>
        <Button block className="side-action" icon={<IconListView />} onClick={() => setShareSheetVisible(true)}>
          分享链接
        </Button>
        <Button block className="side-action" icon={<IconLink />} onClick={() => setDirectSheetVisible(true)}>
          直链管理
        </Button>
        <Button block className="side-action" icon={<IconListView />} onClick={() => {
          setLogSheetVisible(true);
          void loadAccessLogs();
        }}>
          访问日志
        </Button>
        <Button block className="side-action" icon={<IconExit />} onClick={handleLogout}>
          退出登录
        </Button>
        <div className="sidebar-panel">
          <Typography.Text strong>当前能力</Typography.Text>
          <ul>
            <li>文件索引与管理</li>
            <li>在线预览</li>
            <li>文件夹打包下载</li>
            <li>永久直链</li>
            <li>访问日志</li>
            <li>分享链接持久化</li>
            <li>全局搜索</li>
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
          <Space wrap>
            <Input prefix={<IconSearch />} placeholder="当前目录过滤 / 全局搜索关键词" value={keyword} onChange={setKeyword} />
            <Button icon={<IconSearch />} onClick={runGlobalSearch}>
              全局搜索
            </Button>
            {isSearchMode && <Button onClick={() => load(path)}>退出搜索</Button>}
            <Button icon={<IconRefresh />} onClick={() => load(path)} />
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
          description="当前版本已经具备本地存储、文件管理、在线预览、直链、分享链接、全局搜索和容器化部署能力。"
        />

        <section className="metrics">
          <Descriptions
            data={[
              { key: '当前目录', value: path || '/' },
              { key: '当前列表', value: `${items.filter((item) => item.type === 'file').length} 文件 / ${items.filter((item) => item.type === 'folder').length} 文件夹` },
              { key: '总文件', value: stats ? String(stats.fileCount) : '-' },
              { key: '总容量', value: stats ? formatSize(stats.totalSize) : '-' },
              { key: '分享链接', value: stats ? String(stats.shareCount) : '-' },
              { key: '直链', value: stats ? String(stats.directCount) : '-' },
              { key: '访问日志', value: stats ? String(stats.logCount) : '-' }
            ]}
            row
          />
        </section>

        {isSearchMode && (
          <div className="search-banner">
            正在显示 “{keyword}” 的全局搜索结果，共 {searchResults.length} 条。
          </div>
        )}

        <Spin spinning={loading}>
          <Table
            className="file-table"
            columns={columns}
            dataSource={tableItems}
            pagination={false}
            rowKey="path"
            empty={<Empty title="这里还没有文件" description="上传一些文件，xfile 就开始工作。" />}
          />
        </Spin>
      </Layout.Content>

      <Modal title={previewItem?.name || '文件预览'} visible={Boolean(previewItem)} footer={null} width={860} onCancel={() => setPreviewItem(null)}>
        <Spin spinning={previewLoading}>{renderPreview(previewItem, previewText)}</Spin>
      </Modal>

      <Modal title="分享链接已创建" visible={Boolean(shareUrl)} footer={null} onCancel={() => setShareUrl('')}>
        <Input value={shareUrl} readOnly />
        <Typography.Paragraph className="share-note">链接已尝试复制到剪贴板，也可以在左侧“分享链接”里继续管理。</Typography.Paragraph>
      </Modal>

      <Modal title="直链已创建" visible={Boolean(directUrl)} footer={null} onCancel={() => setDirectUrl('')}>
        <Input value={directUrl} readOnly />
        <Typography.Paragraph className="share-note">直链已尝试复制到剪贴板，可用于下载器、播放器或外部页面引用。</Typography.Paragraph>
      </Modal>

      <SideSheet
        title="分享链接"
        visible={shareSheetVisible}
        width={720}
        onCancel={() => setShareSheetVisible(false)}
        footer={<Button onClick={loadShares}>刷新</Button>}
      >
        <Table columns={shareColumns} dataSource={shares} pagination={false} rowKey="key" empty={<Empty title="暂无分享链接" />} />
      </SideSheet>

      <SideSheet
        title="直链管理"
        visible={directSheetVisible}
        width={920}
        onCancel={() => setDirectSheetVisible(false)}
        footer={<Button onClick={loadDirectLinks}>刷新</Button>}
      >
        <Table columns={directColumns} dataSource={directLinks} pagination={false} rowKey="key" empty={<Empty title="暂无直链" />} />
      </SideSheet>

      <SideSheet
        title="访问日志"
        visible={logSheetVisible}
        width={1080}
        onCancel={() => setLogSheetVisible(false)}
        footer={
          <Space>
            <Button onClick={loadAccessLogs}>刷新</Button>
            <Popconfirm title="清空访问日志？" content="清空后无法恢复。" onConfirm={clearLogs}>
              <Button type="danger">清空</Button>
            </Popconfirm>
          </Space>
        }
      >
        <Table columns={logColumns} dataSource={accessLogs} pagination={false} rowKey="id" empty={<Empty title="暂无访问日志" />} />
      </SideSheet>
    </Layout>
  );
}

function LoginView({ onLogin }: { onLogin: (username: string, password: string) => Promise<void> }) {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function submit() {
    if (!username.trim() || !password) {
      Notification.warning({ title: '请输入账号和密码' });
      return;
    }
    setSubmitting(true);
    try {
      await onLogin(username.trim(), password);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="auth-shell">
      <section className="login-panel">
        <div className="login-brand">
          <IconCloud size="extra-large" />
          <strong>xfile</strong>
        </div>
        <Typography.Title heading={3}>管理员登录</Typography.Title>
        <div className="login-fields">
          <Input prefix={<IconUser />} placeholder="用户名" value={username} onChange={setUsername} />
          <Input prefix={<IconLock />} mode="password" placeholder="密码" value={password} onChange={setPassword} onEnterPress={submit} />
          <Button block theme="solid" loading={submitting} onClick={submit}>
            登录
          </Button>
        </div>
      </section>
    </div>
  );
}

function renderPreview(item: FileItem | null, text: string) {
  if (!item) return null;
  const url = previewUrl(item.path);
  if (item.previewType === 'image') return <img className="preview-image" src={url} alt={item.name} />;
  if (item.previewType === 'video') return <video className="preview-media" src={url} controls />;
  if (item.previewType === 'audio') return <audio className="preview-audio" src={url} controls />;
  if (item.previewType === 'pdf') return <iframe className="preview-frame" src={url} title={item.name} />;
  if (item.previewType === 'text') return <pre className="text-preview">{text}</pre>;
  return <Empty title="暂不支持在线预览" description="可以先下载到本地查看。" />;
}

function previewUrl(path: string) {
  return `/api/preview?path=${encodeURIComponent(path)}`;
}

function directLinkUrl(item: DirectLink) {
  return `${window.location.origin}/dl/${item.key}/${encodeURIComponent(item.name || 'download')}`;
}

function parseRules(value: string) {
  return value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function notifyError(error: unknown) {
  Notification.error({
    title: '操作失败',
    content: error instanceof Error ? error.message : '未知错误'
  });
}

function previewLabel(type: PreviewType) {
  const labels: Record<PreviewType, string> = {
    folder: '文件夹',
    image: '图片',
    video: '视频',
    audio: '音频',
    pdf: 'PDF',
    text: '文本',
    download: '文件'
  };
  return labels[type] || '文件';
}

function previewColor(type: PreviewType) {
  const colors: Record<PreviewType, 'blue' | 'green' | 'orange' | 'purple' | 'cyan' | 'grey'> = {
    folder: 'blue',
    image: 'green',
    video: 'purple',
    audio: 'orange',
    pdf: 'cyan',
    text: 'blue',
    download: 'grey'
  };
  return colors[type] || 'grey';
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 * 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`;
  return `${(size / 1024 / 1024 / 1024).toFixed(1)} GB`;
}
