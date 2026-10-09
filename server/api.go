package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// 一個 tile 不到 3 KB，這個上限只是避免被塞入超大請求。
const maxBodyBytes = 64 * 1024

type viewResponse struct {
	Intro       string `json:"intro"`
	Trigger     string `json:"trigger"`
	IsAdmin     bool   `json:"is_admin"`
	Tiles       []Tile `json:"tiles"`
	Personal    []Tile `json:"personal"`
	MaxPersonal int    `json:"max_personal"` // 0 代表「我的捷徑」已關閉
}

type adminListResponse struct {
	Tiles []adminTile `json:"tiles"`
}

type channelsResponse struct {
	Channels []audienceInfo `json:"channels"`
}

type moveRequest struct {
	Offset int `json:"offset"`
}

type errorResponse struct {
	Error string `json:"error"`
	Field string `json:"field,omitempty"`
}

func userIDFrom(r *http.Request) string {
	return r.Header.Get("Mattermost-User-ID")
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	// 這個標頭只有在 Mattermost 驗證過登入（含 CSRF 檢查）後才會帶上，外部送來的同名標頭會先被清掉
	if userIDFrom(r) == "" {
		writeError(w, http.StatusUnauthorized, "請先登入")
		return
	}
	p.routerOnce.Do(func() { p.router = p.newRouter() })
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) newRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tiles", p.handleView)
	mux.HandleFunc("GET /api/v1/admin/tiles", p.adminOnly(p.handleAdminList))
	mux.HandleFunc("POST /api/v1/admin/tiles", p.adminOnly(p.handleAdminCreate))
	mux.HandleFunc("PUT /api/v1/admin/tiles/{id}", p.adminOnly(p.handleAdminUpdate))
	mux.HandleFunc("DELETE /api/v1/admin/tiles/{id}", p.adminOnly(p.handleAdminDelete))
	mux.HandleFunc("POST /api/v1/admin/tiles/{id}/move", p.adminOnly(p.handleAdminMove))
	mux.HandleFunc("GET /api/v1/admin/channels", p.adminOnly(p.handleAdminChannels))
	mux.HandleFunc("POST /api/v1/personal", p.handlePersonalCreate)
	mux.HandleFunc("PUT /api/v1/personal/{id}", p.handlePersonalUpdate)
	mux.HandleFunc("DELETE /api/v1/personal/{id}", p.handlePersonalDelete)
	mux.HandleFunc("POST /api/v1/personal/{id}/move", p.handlePersonalMove)
	mux.HandleFunc("POST /api/v1/pin", p.handlePin)
	mux.HandleFunc("POST /api/v1/action", p.handleAction)
	mux.HandleFunc("POST /api/v1/dialog", p.handleDialog)
	return mux
}

func (p *Plugin) isAdmin(userID string) bool {
	return p.API.HasPermissionTo(userID, model.PermissionManageSystem)
}

func (p *Plugin) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.isAdmin(userIDFrom(r)) {
			writeError(w, http.StatusForbidden, "只有系統管理員可以管理共用項目")
			return
		}
		next(w, r)
	}
}

func (p *Plugin) handleView(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r)
	cfg, trigger, _ := p.snapshot()

	shared, err := p.loadTiles(sharedKey)
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	personal, limit, err := p.loadPersonal(userID)
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, viewResponse{
		Intro:       cfg.IntroText,
		Trigger:     trigger,
		IsAdmin:     p.isAdmin(userID),
		Tiles:       p.visibleTiles(userID, shared),
		Personal:    personal,
		MaxPersonal: limit,
	})
}

func (p *Plugin) handleAdminList(w http.ResponseWriter, _ *http.Request) {
	tiles, err := p.loadTiles(sharedKey)
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, adminListResponse{Tiles: p.describeAudience(tiles)})
}

func (p *Plugin) handleAdminChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := p.channelOptions(userIDFrom(r))
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, channelsResponse{Channels: channels})
}

func (p *Plugin) handleAdminCreate(w http.ResponseWriter, r *http.Request) {
	var in Tile
	if !decodeBody(w, r, &in) {
		return
	}
	t, err := p.normalizeShared(in)
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	// ID 一律由伺服器產生，不採用前端送來的值
	t.ID = model.NewId()
	_, err = p.updateTiles(sharedKey, func(tiles []Tile) ([]Tile, error) {
		if len(tiles) >= maxSharedTiles {
			return nil, &fieldError{"", "共用項目已達上限，請先刪除用不到的項目"}
		}
		return append(tiles, t), nil
	})
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (p *Plugin) handleAdminUpdate(w http.ResponseWriter, r *http.Request) {
	var in Tile
	if !decodeBody(w, r, &in) {
		return
	}
	t, err := p.normalizeShared(in)
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	t.ID = r.PathValue("id")
	_, err = p.updateTiles(sharedKey, func(tiles []Tile) ([]Tile, error) {
		return replaceTile(tiles, t)
	})
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (p *Plugin) handleAdminDelete(w http.ResponseWriter, r *http.Request) {
	tiles, err := p.updateTiles(sharedKey, func(tiles []Tile) ([]Tile, error) {
		return removeTile(tiles, r.PathValue("id"))
	})
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, adminListResponse{Tiles: p.describeAudience(tiles)})
}

func (p *Plugin) handleAdminMove(w http.ResponseWriter, r *http.Request) {
	var req moveRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Offset != -1 && req.Offset != 1 {
		writeError(w, http.StatusBadRequest, "offset 只能是 -1 或 1")
		return
	}
	tiles, err := p.updateTiles(sharedKey, func(tiles []Tile) ([]Tile, error) {
		return moveTile(tiles, r.PathValue("id"), req.Offset)
	})
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, adminListResponse{Tiles: p.describeAudience(tiles)})
}

// normalizeShared 檢查共用 tile 的欄位，並確認顯示對象的頻道都存在。
func (p *Plugin) normalizeShared(in Tile) (Tile, error) {
	t, err := normalizeTile(in)
	if err != nil {
		return t, err
	}
	return t, p.checkAudience(t.Channels)
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "資料格式錯誤")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// writeStoreError 把內部錯誤轉成前端看得懂的訊息；非預期的錯誤只記 log，不把細節回給使用者。
func (p *Plugin) writeStoreError(w http.ResponseWriter, err error) {
	var fe *fieldError
	switch {
	case errors.As(err, &fe):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: fe.Msg, Field: fe.Field})
	case errors.Is(err, errTileNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, errConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, errPersonalDisabled):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		p.API.LogError("tiles store error", "err", err.Error())
		writeError(w, http.StatusInternalServerError, "儲存失敗，請稍後再試")
	}
}
