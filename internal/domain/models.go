package domain

type FileEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modifiedAt"`
}

type Share struct {
	ID        int64  `json:"id"`
	Token     string `json:"token"`
	Path      string `json:"path"`
	URL       string `json:"url"`
	Protected bool   `json:"protected"`
	ExpiresAt string `json:"expiresAt,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type DirectLink struct {
	ID        int64  `json:"id"`
	Token     string `json:"token"`
	Path      string `json:"path"`
	URL       string `json:"url"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
}

type AccessLog struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Path      string `json:"path"`
	IP        string `json:"ip"`
	UserAgent string `json:"userAgent"`
	CreatedAt string `json:"createdAt"`
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
