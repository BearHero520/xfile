package domain

type FileEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modifiedAt"`
}

type StorageSource struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	Type         string `json:"type"`
	TypeLabel    string `json:"typeLabel"`
	RootPath     string `json:"rootPath,omitempty"`
	HiddenPaths  string `json:"hiddenPaths,omitempty"`
	BlockedPaths string `json:"blockedPaths,omitempty"`
	Public       bool   `json:"public"`
	Enabled      bool   `json:"enabled"`
	OrderNum     int    `json:"orderNum"`
	CreatedAt    string `json:"createdAt"`
}

type StorageSourceInput struct {
	Name         string `json:"name"`
	Key          string `json:"key"`
	Type         string `json:"type"`
	RootPath     string `json:"rootPath"`
	HiddenPaths  string `json:"hiddenPaths"`
	BlockedPaths string `json:"blockedPaths"`
	Public       bool   `json:"public"`
	Enabled      bool   `json:"enabled"`
	OrderNum     int    `json:"orderNum"`
}

type PublicSite struct {
	SiteName    string          `json:"siteName"`
	RootName    string          `json:"rootName"`
	Initialized bool            `json:"initialized"`
	LoggedIn    bool            `json:"loggedIn"`
	Sources     []StorageSource `json:"sources"`
}

type Share struct {
	ID            int64  `json:"id"`
	Token         string `json:"token"`
	Path          string `json:"path"`
	URL           string `json:"url"`
	Protected     bool   `json:"protected"`
	ExpiresAt     string `json:"expiresAt,omitempty"`
	ViewCount     int    `json:"viewCount"`
	DownloadCount int    `json:"downloadCount"`
	LastAccessAt  string `json:"lastAccessAt,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

type ShareDetail struct {
	Token       string      `json:"token"`
	Path        string      `json:"path"`
	CurrentPath string      `json:"currentPath,omitempty"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Size        int64       `json:"size"`
	Protected   bool        `json:"protected"`
	ExpiresAt   string      `json:"expiresAt,omitempty"`
	CreatedAt   string      `json:"createdAt"`
	Files       []FileEntry `json:"files,omitempty"`
}

type User struct {
	ID                 int64               `json:"id"`
	Username           string              `json:"username"`
	Role               string              `json:"role"`
	StorageSourceKeys  []string            `json:"storageSourceKeys,omitempty"`
	StorageSourceRoots map[string][]string `json:"storageSourceRoots,omitempty"`
	DisabledOperations []string            `json:"disabledOperations,omitempty"`
	CreatedAt          string              `json:"createdAt"`
}

type DirectLink struct {
	ID           int64  `json:"id"`
	Token        string `json:"token"`
	Path         string `json:"path"`
	URL          string `json:"url"`
	Enabled      bool   `json:"enabled"`
	AccessCount  int    `json:"accessCount"`
	LastAccessAt string `json:"lastAccessAt,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type AccessLog struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Path      string `json:"path"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	CreatedAt string `json:"createdAt"`
}

type AccessLogPage struct {
	Items    []AccessLog `json:"items"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

type Dashboard struct {
	SiteName       string      `json:"siteName"`
	StorageRoot    string      `json:"storageRoot"`
	FileCount      int         `json:"fileCount"`
	FolderCount    int         `json:"folderCount"`
	TotalBytes     int64       `json:"totalBytes"`
	ShareCount     int         `json:"shareCount"`
	RecentFiles    []FileEntry `json:"recentFiles"`
	RecentLogs     []AccessLog `json:"recentLogs"`
	StorageSources []string    `json:"storageSources"`
}
