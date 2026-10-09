package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// 每個人的捷徑各存一筆，鍵裡帶使用者 ID，彼此讀不到也改不到。
const personalKeyPrefix = "personal_"

// 系統管理員可調整每人上限，但不讓它大到把面板塞爆。
const maxPersonalLimit = 20

var errPersonalDisabled = errors.New("系統管理員已關閉「我的捷徑」")

func personalKey(userID string) string {
	return personalKeyPrefix + userID
}

// personalLimit 回傳每人可新增的捷徑數量；0 代表關閉這個功能。
func (cfg configuration) personalLimit() int {
	return min(max(cfg.MaxPersonalTiles, 0), maxPersonalLimit)
}

// normalizePersonal 檢查欄位；個人捷徑只有自己看得到，用不到顯示對象。
// 先清掉顯示對象再檢查，免得送來的頻道值有誤時回一個跟個人捷徑無關的錯誤。
func normalizePersonal(in Tile) (Tile, error) {
	in.Channels = nil
	return normalizeTile(in)
}

// loadPersonal 讀取使用者的捷徑；功能關閉時回傳空清單，畫面上就不會出現。
func (p *Plugin) loadPersonal(userID string) ([]Tile, int, error) {
	cfg, _, _ := p.snapshot()
	limit := cfg.personalLimit()
	if limit == 0 {
		return []Tile{}, 0, nil
	}
	tiles, err := p.loadTiles(personalKey(userID))
	return tiles, limit, err
}

// addPersonal 新增一個捷徑，網頁面板與手機對話框共用。
// 超過上限時不刪舊的，只請使用者自己決定刪哪個。
func (p *Plugin) addPersonal(userID string, in Tile) (Tile, error) {
	cfg, _, _ := p.snapshot()
	limit := cfg.personalLimit()
	if limit == 0 {
		return Tile{}, errPersonalDisabled
	}
	t, err := normalizePersonal(in)
	if err != nil {
		return t, err
	}
	t.ID = model.NewId()
	_, err = p.updateTiles(personalKey(userID), func(tiles []Tile) ([]Tile, error) {
		if len(tiles) >= limit {
			return nil, &fieldError{"", fmt.Sprintf("我的捷徑最多 %d 個，請先刪除用不到的再新增", limit)}
		}
		return append(tiles, t), nil
	})
	return t, err
}

func (p *Plugin) updatePersonal(userID, id string, in Tile) (Tile, error) {
	t, err := normalizePersonal(in)
	if err != nil {
		return t, err
	}
	t.ID = id
	_, err = p.updateTiles(personalKey(userID), func(tiles []Tile) ([]Tile, error) {
		return replaceTile(tiles, t)
	})
	return t, err
}

func (p *Plugin) deletePersonal(userID, id string) ([]Tile, error) {
	return p.updateTiles(personalKey(userID), func(tiles []Tile) ([]Tile, error) {
		return removeTile(tiles, id)
	})
}

func (p *Plugin) handlePersonalCreate(w http.ResponseWriter, r *http.Request) {
	var in Tile
	if !decodeBody(w, r, &in) {
		return
	}
	t, err := p.addPersonal(userIDFrom(r), in)
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (p *Plugin) handlePersonalUpdate(w http.ResponseWriter, r *http.Request) {
	var in Tile
	if !decodeBody(w, r, &in) {
		return
	}
	t, err := p.updatePersonal(userIDFrom(r), r.PathValue("id"), in)
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (p *Plugin) handlePersonalDelete(w http.ResponseWriter, r *http.Request) {
	tiles, err := p.deletePersonal(userIDFrom(r), r.PathValue("id"))
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, personalListResponse{Tiles: tiles})
}

func (p *Plugin) handlePersonalMove(w http.ResponseWriter, r *http.Request) {
	var req moveRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Offset != -1 && req.Offset != 1 {
		writeError(w, http.StatusBadRequest, "offset 只能是 -1 或 1")
		return
	}
	tiles, err := p.updateTiles(personalKey(userIDFrom(r)), func(tiles []Tile) ([]Tile, error) {
		return moveTile(tiles, r.PathValue("id"), req.Offset)
	})
	if err != nil {
		p.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, personalListResponse{Tiles: tiles})
}

type personalListResponse struct {
	Tiles []Tile `json:"tiles"`
}
