package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestViewRequiresLogin(t *testing.T) {
	e := setup(t, configuration{})
	w := doRequest(t, e.p, "", http.MethodGet, "/api/v1/tiles", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminManagesSharedTiles(t *testing.T) {
	e := setup(t, configuration{IntroText: "hi"})

	w := doRequest(t, e.p, "u1", http.MethodPost, "/api/v1/admin/tiles", Tile{Title: "x", URL: "https://x.example"})
	require.Equal(t, http.StatusForbidden, w.Code, "一般同事不能管理共用項目")
	w = doRequest(t, e.p, "u1", http.MethodGet, "/api/v1/admin/tiles", nil)
	require.Equal(t, http.StatusForbidden, w.Code)

	w = doRequest(t, e.p, "admin", http.MethodPost, "/api/v1/admin/tiles", Tile{Title: "手冊", URL: "ftp://x"})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "url", decode[errorResponse](t, w).Field, "錯誤要指出是哪個欄位")

	seeded := seedShared(t, e.p,
		Tile{Title: " 使用手冊 ", URL: "https://a.example"},
		Tile{ID: "client-chosen", Title: "常見問題", URL: "https://b.example"},
	)
	a, b := seeded[0], seeded[1]
	require.Equal(t, "使用手冊", a.Title, "前後空白要去掉")
	require.NotEmpty(t, a.ID)
	require.NotEqual(t, "client-chosen", b.ID, "ID 一律由伺服器產生")

	w = doRequest(t, e.p, "admin", http.MethodPost, "/api/v1/admin/tiles/"+b.ID+"/move", moveRequest{Offset: -1})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"常見問題", "使用手冊"}, adminTitles(decode[adminListResponse](t, w).Tiles))

	w = doRequest(t, e.p, "admin", http.MethodPost, "/api/v1/admin/tiles/"+b.ID+"/move", moveRequest{Offset: 3})
	require.Equal(t, http.StatusBadRequest, w.Code)

	w = doRequest(t, e.p, "admin", http.MethodPut, "/api/v1/admin/tiles/"+a.ID, Tile{Title: "手冊", URL: "https://a2.example"})
	require.Equal(t, http.StatusOK, w.Code)

	view := decode[viewResponse](t, doRequest(t, e.p, "u1", http.MethodGet, "/api/v1/tiles", nil))
	require.False(t, view.IsAdmin)
	require.Equal(t, "hi", view.Intro)
	require.Equal(t, []string{"常見問題", "手冊"}, titles(view.Tiles), "編輯後位置不變")
	require.Equal(t, "https://a2.example", view.Tiles[1].URL)

	w = doRequest(t, e.p, "admin", http.MethodDelete, "/api/v1/admin/tiles/"+b.ID, nil)
	require.Equal(t, http.StatusOK, w.Code)
	view = decode[viewResponse](t, doRequest(t, e.p, "admin", http.MethodGet, "/api/v1/tiles", nil))
	require.True(t, view.IsAdmin)
	require.Equal(t, []string{"手冊"}, titles(view.Tiles))

	w = doRequest(t, e.p, "admin", http.MethodDelete, "/api/v1/admin/tiles/nope", nil)
	require.Equal(t, http.StatusNotFound, w.Code)
	w = doRequest(t, e.p, "admin", http.MethodPut, "/api/v1/admin/tiles/nope", Tile{Title: "x", URL: "https://x.example"})
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestEmptyStoreReturnsEmptyArray(t *testing.T) {
	e := setup(t, configuration{})
	w := doRequest(t, e.p, "u1", http.MethodGet, "/api/v1/tiles", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"tiles":[]`, "沒有資料時要回傳 [] 而不是 null")
}

func TestUpdateRetriesWhenSomeoneElseSavedFirst(t *testing.T) {
	e := setup(t, configuration{})

	e.kv.failNextCAS = 2
	seedShared(t, e.p, Tile{Title: "a", URL: "https://a.example"})

	e.kv.failNextCAS = maxUpdateAttempts
	w := doRequest(t, e.p, "admin", http.MethodPost, "/api/v1/admin/tiles", Tile{Title: "b", URL: "https://b.example"})
	require.Equal(t, http.StatusConflict, w.Code, "一直衝突就放棄並請使用者重試")

	view := decode[viewResponse](t, doRequest(t, e.p, "u1", http.MethodGet, "/api/v1/tiles", nil))
	require.Equal(t, []string{"a"}, titles(view.Tiles))
}
