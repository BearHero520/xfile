package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFavoriteRoutes(t *testing.T) {
	s, appStore := newAuthzTestServer(t)
	if _, err := appStore.CreateSuperAdmin("root", "password123"); err != nil {
		t.Fatalf("create super admin: %v", err)
	}
	root := appStoreRoot(t, appStore)
	if err := os.WriteFile(filepath.Join(root, "favorite.txt"), []byte("favorite"), 0o644); err != nil {
		t.Fatalf("write favorite: %v", err)
	}

	created := performJSONRequestAs(s, "root", http.MethodPost, "/api/v1/workspace/favorites", `{"path":"favorite.txt"}`)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"path":"favorite.txt"`) {
		t.Fatalf("create favorite failed: %d %s", created.Code, created.Body.String())
	}

	listed := performJSONRequestAs(s, "root", http.MethodGet, "/api/v1/workspace/favorites", "")
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"favorite.txt"`) {
		t.Fatalf("list favorites failed: %d %s", listed.Code, listed.Body.String())
	}
	var favorite struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &favorite); err != nil || favorite.ID == 0 {
		t.Fatalf("decode favorite: %v %s", err, created.Body.String())
	}
	deleted := performJSONRequestAs(s, "root", http.MethodDelete, "/api/v1/workspace/favorites/"+strconv.FormatInt(favorite.ID, 10), "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete favorite failed: %d %s", deleted.Code, deleted.Body.String())
	}
}
