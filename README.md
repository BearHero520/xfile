# xfile

xfile 是一个私有云盘系统原型，后端使用 Go，前端使用 Vite + React + Semi Design。默认服务端口为 `3008`，当前版本以内置本地存储为核心，适合继续扩展 WebDAV、对象存储、用户权限和离线下载。

## 功能点

- 文件索引：浏览本地存储目录，按文件夹和文件排序展示。
- 文件管理：上传、下载、新建文件夹、重命名、删除。
- 分享链接：为文件或文件夹创建临时分享 Key。
- 前端体验：Semi Design 管理台、当前目录搜索、面包屑导航、文件统计。
- 容器部署：Docker 镜像内置前端静态资源和 Go 服务。
- 自动构建：GitHub Actions 自动构建并推送 GHCR 镜像。

## 本地开发

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
GHCR_OWNER=your-github-username docker compose up -d
```

`docker-compose.yml` 默认使用：

```text
ghcr.io/${GHCR_OWNER:-your-github-username}/xfile:latest
```

数据会挂载到当前目录的 `./data`，容器内路径为 `/app/data`。

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
| `XFILE_WEB_DIR` | `web/dist` | 前端静态资源目录 |

## 下一步规划

- 用户登录、角色和存储源权限。
- WebDAV 服务端。
- S3、阿里云 OSS、WebDAV 等外部存储源。
- 分享密码、过期清理、访问统计。
- Office/文本/图片/音视频在线预览。
- 离线下载任务队列。
