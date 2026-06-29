# xfile

xfile 是一个私有云盘系统原型，后端使用 Go + SQLite3，前端使用 Vite + React + Semi Design。默认服务端口为 `3008`，当前版本以内置本地存储为核心，适合继续扩展 WebDAV、对象存储、用户权限和离线下载。

默认管理员账号为 `admin`，默认密码为 `xfile-admin`。公开部署前请务必通过环境变量修改密码。

## 功能点

- 管理员登录：基于 HttpOnly Cookie 的 session 登录，文件管理 API 默认需要认证。
- 文件索引：浏览本地存储目录，按文件夹和文件排序展示。
- 文件管理：上传、下载、新建文件夹、重命名、删除，文件夹下载会自动打包为 `.zip`。
- 在线预览：支持图片、视频、音频、PDF、文本和代码类文件预览。
- 直链：为文件或文件夹创建公开直链，文件输出原始资源，文件夹输出 zip，支持永久或限时有效。
- 直链访问控制：支持 Referer 白名单、IP/CIDR 白名单和 KB/s 限速。
- 全局搜索：可从当前目录向下递归搜索文件和文件夹。
- 分享链接：为文件或文件夹创建临时分享 Key，可设置访问密码，并持久化到 SQLite。
- 链接管理：前端可查看、复制、删除分享链接和直链。
- 访问日志：记录分享/直链访问时间、状态码、IP、Referer、UserAgent、字节数和耗时。
- 存储统计：统计文件数、文件夹数、总容量、分享链接数量和直链数量。
- 前端体验：Semi Design 管理台、当前目录过滤、全局搜索、面包屑导航、文件统计。
- 容器部署：Docker 镜像内置前端静态资源和 Go 服务。
- 自动构建：GitHub Actions 自动构建并推送 GHCR 镜像。

## API 概览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/health` | 健康检查 |
| `GET` | `/api/auth/me` | 当前登录状态 |
| `POST` | `/api/auth/login` | 管理员登录 |
| `POST` | `/api/auth/logout` | 退出登录 |
| `GET` | `/api/stats` | 存储统计 |
| `GET` | `/api/files?path=` | 获取目录文件列表 |
| `GET` | `/api/search?path=&q=` | 递归搜索 |
| `POST` | `/api/upload?path=` | 上传文件 |
| `POST` | `/api/folders` | 新建文件夹 |
| `POST` | `/api/rename` | 重命名 |
| `DELETE` | `/api/files?path=` | 删除文件或文件夹 |
| `GET` | `/api/download?path=` | 下载文件或文件夹 zip |
| `GET` | `/api/preview?path=` | 在线预览 |
| `POST` | `/api/direct-links` | 创建直链 |
| `GET` | `/api/direct-links` | 直链列表 |
| `DELETE` | `/api/direct-links/{key}` | 删除直链 |
| `GET` | `/api/access-logs` | 访问日志 |
| `DELETE` | `/api/access-logs` | 清空访问日志 |
| `POST` | `/api/share` | 创建分享 |
| `GET` | `/api/shares` | 分享列表 |
| `DELETE` | `/api/shares/{key}` | 删除分享 |
| `GET` | `/dl/{key}` | 公开直链访问 |
| `GET` | `/s/{key}` | 打开分享链接 |

直链与分享链接的区别：

- 直链 `/dl/{key}`：面向下载器、播放器、外部页面引用，直接返回文件内容；文件夹返回 zip；可配置 Referer/IP/限速。
- 分享 `/s/{key}`：面向人为访问，当前版本打开文件或目录列表；可配置访问密码。

直链访问控制说明：

- Referer 白名单：每行一个域名或 URL 片段；配置后，请求 Referer 必须包含其中一项。
- IP/CIDR 白名单：每行一个 IP 或 CIDR，例如 `203.0.113.10`、`10.0.0.0/8`。
- 反向代理：默认不信任 `X-Forwarded-For` / `X-Real-IP`；如部署在可信反代后并要按真实客户端 IP 判断，设置 `XFILE_TRUST_PROXY=true`。
- 限速：单位为 KB/s，`0` 表示不限速。

数据文件：

- SQLite 数据库：`data/xfile.db`
- 文件存储目录：`data/files`

## 本地开发

需要 Go `1.24+`。

后端：

```bash
go run ./cmd/xfile
```

前端：

```bash
cd web
npm install
npm run dev
```

开发时访问前端地址 `http://localhost:5173`，Vite 会把 `/api` 请求代理到 `http://localhost:3008`。

## Docker 构建

```bash
docker build -t xfile:local .
docker run --rm -p 3008:3008 -v ./data:/app/data xfile:local
```

访问：

```text
http://localhost:3008
```

## 使用 docker-compose 拉取镜像

镜像由 GitHub Actions 推送到 GHCR。将下面命令里的 `your-github-username` 换成你的 GitHub 用户名或组织名：

```bash
GHCR_OWNER=your-github-username XFILE_ADMIN_PASSWORD=change-me docker compose up -d
```

`docker-compose.yml` 默认使用：

```text
ghcr.io/${GHCR_OWNER:-your-github-username}/xfile:latest
```

数据会挂载到当前目录的 `./data`，容器内路径为 `/app/data`。SQLite 数据库默认位于 `./data/xfile.db`。

## GitHub 镜像自动构建

仓库推送到 GitHub 后，`.github/workflows/docker.yml` 会在以下场景构建镜像：

- 推送到 `main` 或 `master`
- 推送 `v*` 标签
- 手动触发 workflow
- Pull Request 构建校验，不推送镜像

默认镜像地址：

```text
ghcr.io/<你的 GitHub 用户名或组织名>/<仓库名>:latest
```

如果仓库名就是 `xfile`，镜像地址通常是：

```text
ghcr.io/<你的 GitHub 用户名或组织名>/xfile:latest
```

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `XFILE_PORT` | `3008` | HTTP 服务端口 |
| `XFILE_DATA_DIR` | `data` | 数据目录 |
| `XFILE_DB_PATH` | `$XFILE_DATA_DIR/xfile.db` | SQLite3 数据库路径 |
| `XFILE_WEB_DIR` | `web/dist` | 前端静态资源目录 |
| `XFILE_ADMIN_USER` | `admin` | 管理员用户名 |
| `XFILE_ADMIN_PASSWORD` | `xfile-admin` | 管理员密码 |
| `XFILE_TRUST_PROXY` | `false` | 是否信任反向代理传入的客户端 IP 请求头 |

## 下一步规划

- 多用户、角色和存储源权限。
- WebDAV 服务端。
- S3、阿里云 OSS、WebDAV 等外部存储源。
- 分享访问页美化、分享目录下载、下载排行统计。
- Office 文档预览或 OnlyOffice/kkFileView 集成。
- 离线下载任务队列。
