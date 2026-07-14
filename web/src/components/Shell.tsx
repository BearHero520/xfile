import {
  ArrowLeft20Regular,
  ArrowSync20Regular,
  ArrowUpload20Regular,
  CloudDatabase20Regular,
  Color20Regular,
  DataTrending20Regular,
  DocumentBulletList20Regular,
  Eye20Regular,
  Filter20Regular,
  GlobeShield20Regular,
  Info20Regular,
  Link20Regular,
  LockClosed20Regular,
  MegaphoneLoud20Regular,
  People20Regular,
  PanelLeftContract20Regular,
  PanelLeftExpand20Regular,
  Person20Regular,
  Search20Regular,
  Settings20Regular,
  Share20Regular,
  ShieldCheckmark20Regular,
  SignOut20Regular,
  Star20Regular,
} from "@fluentui/react-icons";
import { FormEvent, useEffect, useRef, useState } from "react";
import {
  Link,
  NavLink,
  useLocation,
  useNavigate,
  useOutlet,
} from "react-router-dom";
import { api } from "../api";
import { useApp, useToast } from "../state";
import AppearanceControl from "./AppearanceControl";
import BrandMark from "./BrandMark";
import { IconButton } from "./ui";

const groups = [
  {
    label: "系统设置",
    items: [
      {
        to: "/settings/site",
        label: "站点设置",
        icon: <Settings20Regular />,
        admin: true,
      },
      {
        to: "/storage",
        label: "存储源设置",
        icon: <CloudDatabase20Regular />,
        admin: true,
      },
      {
        to: "/settings/theme",
        label: "主题管理",
        icon: <Color20Regular />,
        admin: true,
      },
      {
        to: "/settings/announcement",
        label: "公告管理",
        icon: <MegaphoneLoud20Regular />,
        admin: true,
      },
      {
        to: "/settings/view",
        label: "显示设置",
        icon: <Eye20Regular />,
        admin: true,
      },
      {
        to: "/settings/webdav",
        label: "WebDAV",
        icon: <ArrowSync20Regular />,
        admin: true,
      },
    ],
  },
  {
    label: "链接与统计",
    items: [
      {
        to: "/settings/link",
        label: "直/短链设置",
        icon: <Link20Regular />,
        admin: true,
      },
      { to: "/delivery", label: "短链管理", icon: <Link20Regular /> },
      { to: "/shares", label: "分享管理", icon: <Share20Regular /> },
      { to: "/favorites", label: "我的收藏", icon: <Star20Regular /> },
      {
        to: "/audit",
        label: "下载日志",
        icon: <DocumentBulletList20Regular />,
      },
      {
        to: "/insights",
        label: "下载排行统计",
        icon: <DataTrending20Regular />,
      },
    ],
  },
  {
    label: "规则与权限",
    items: [
      {
        to: "/settings/upload",
        label: "上传规则",
        icon: <ArrowUpload20Regular />,
        admin: true,
      },
      {
        to: "/settings/visibility",
        label: "显示规则",
        icon: <Filter20Regular />,
        admin: true,
      },
      {
        to: "/settings/user-rules",
        label: "用户规则",
        icon: <People20Regular />,
        admin: true,
      },
      {
        to: "/accounts",
        label: "用户管理",
        icon: <People20Regular />,
        admin: true,
      },
      {
        to: "/settings/security",
        label: "安全设置",
        icon: <ShieldCheckmark20Regular />,
        admin: true,
      },
      {
        to: "/settings/access",
        label: "访问控制",
        icon: <GlobeShield20Regular />,
        admin: true,
      },
    ],
  },
  {
    label: "项目",
    items: [
      {
        to: "/about",
        label: "关于 XFile",
        icon: <Info20Regular />,
      },
    ],
  },
];

export default function Shell({ children }: { children?: React.ReactNode }) {
  const { session, site, refresh } = useApp();
  const { show } = useToast();
  const navigate = useNavigate();
  const location = useLocation();
  const outlet = useOutlet();
  const [search, setSearch] = useState("");
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem("xfile-sidebar-collapsed") === "true",
  );
  const nextOutlet = children || outlet;
  const nextOutletRef = useRef(nextOutlet);
  const routeKeyRef = useRef(location.key);
  const exitTimerRef = useRef<number | null>(null);
  const enterTimerRef = useRef<number | null>(null);
  const [displayedOutlet, setDisplayedOutlet] = useState(nextOutlet);
  const [routePhase, setRoutePhase] = useState("");
  const isAdmin = session?.user?.role === "super_admin";

  nextOutletRef.current = nextOutlet;

  useEffect(() => {
    localStorage.setItem("xfile-sidebar-collapsed", String(collapsed));
  }, [collapsed]);

  useEffect(() => {
    if (routeKeyRef.current === location.key) {
      setDisplayedOutlet(nextOutletRef.current);
      return;
    }
    const reduced =
      document.documentElement.dataset.motion === "reduced" ||
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (exitTimerRef.current !== null)
      window.clearTimeout(exitTimerRef.current);
    if (enterTimerRef.current !== null)
      window.clearTimeout(enterTimerRef.current);
    setRoutePhase("is-leaving");
    exitTimerRef.current = window.setTimeout(
      () => {
        routeKeyRef.current = location.key;
        setDisplayedOutlet(nextOutletRef.current);
        setRoutePhase("is-entering");
        enterTimerRef.current = window.setTimeout(
          () => setRoutePhase(""),
          reduced ? 0 : 210,
        );
      },
      reduced ? 0 : 130,
    );
    return () => {
      if (exitTimerRef.current !== null)
        window.clearTimeout(exitTimerRef.current);
      if (enterTimerRef.current !== null)
        window.clearTimeout(enterTimerRef.current);
    };
  }, [location.key]);

  function submitSearch(event: FormEvent) {
    event.preventDefault();
    const query = search.trim();
    if (query) navigate(`/search?q=${encodeURIComponent(query)}`);
  }

  async function logout() {
    try {
      await api.logout();
      await refresh();
      navigate("/login");
    } catch (error) {
      show(error instanceof Error ? error.message : "退出失败", "error");
    }
  }

  return (
    <div className={`app-shell ${collapsed ? "is-sidebar-collapsed" : ""}`}>
      <aside className="sidebar glass">
        <div className="sidebar-head">
          <Link to="/" className="brand">
            <BrandMark />
            <span>
              <strong>XFile</strong>
              <small>后台管理</small>
            </span>
          </Link>
          <IconButton
            className="sidebar-collapse"
            label={collapsed ? "展开侧边栏" : "折叠侧边栏"}
            onClick={() => setCollapsed((value) => !value)}
          >
            {collapsed ? (
              <PanelLeftExpand20Regular />
            ) : (
              <PanelLeftContract20Regular />
            )}
          </IconButton>
        </div>
        <Link to="/" className="nav-item admin-home-link">
          <ArrowLeft20Regular />
          <span>返回文件列表</span>
        </Link>
        <nav>
          {groups.map((group) => (
            <section key={group.label} className="nav-group">
              <div className="nav-label">{group.label}</div>
              {group.items
                .filter((item) => !item.admin || isAdmin)
                .map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    className={({ isActive }) =>
                      `nav-item ${isActive ? "is-active" : ""}`
                    }
                  >
                    {item.icon}
                    <span>{item.label}</span>
                  </NavLink>
                ))}
            </section>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div className="security-note">
            <ShieldCheckmark20Regular />
            <span>
              <strong>安全连接</strong>
              <small>会话与 CSRF 防护已启用</small>
            </span>
          </div>
          <div className="account-row">
            <div className="avatar">
              <Person20Regular />
            </div>
            <div>
              <strong>{session?.username || "访客"}</strong>
              <small>{isAdmin ? "超级管理员" : "工作区用户"}</small>
            </div>
            <IconButton label="退出登录" onClick={logout}>
              <SignOut20Regular />
            </IconButton>
          </div>
        </div>
      </aside>
      <div className="app-main">
        <header className="topbar glass">
          <div className="mobile-brand">
            <BrandMark />
            <strong>XFile 管理</strong>
          </div>
          <form className="topbar-search" onSubmit={submitSearch}>
            <Search20Regular aria-hidden="true" />
            <input
              aria-label="全局搜索"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="全局搜索文件、目录和说明"
            />
            <kbd>↵</kbd>
          </form>
          <div className="topbar-actions">
            <span className="site-pill">
              <LockClosed20Regular />
              {site?.siteName || "XFile"}
            </span>
            <AppearanceControl />
          </div>
        </header>
        <main className="content" tabIndex={-1}>
          <div className={`route-transition ${routePhase}`}>
            {displayedOutlet}
          </div>
        </main>
      </div>
    </div>
  );
}
