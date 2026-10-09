package main

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func addPersonalReq(t *testing.T, e *testEnv, user string, in Tile) Tile {
	t.Helper()
	w := doRequest(t, e.p, user, http.MethodPost, "/api/v1/personal", in)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return decode[Tile](t, w)
}

func viewOf(t *testing.T, e *testEnv, user string) viewResponse {
	t.Helper()
	w := doRequest(t, e.p, user, http.MethodGet, "/api/v1/tiles", nil)
	require.Equal(t, http.StatusOK, w.Code)
	return decode[viewResponse](t, w)
}

func TestPersonalTilesAreLimitedAndPrivate(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 2})
	ch := e.dir.addChannel("工程部", model.ChannelTypeOpen, "alice")

	a := addPersonalReq(t, e, "alice", Tile{Title: "我的報表", URL: "https://a.example", Channels: []string{ch}})
	require.Empty(t, a.Channels, "個人捷徑不需要顯示對象")
	_, err := normalizePersonal(Tile{Title: "x", URL: "https://x.example", Channels: []string{"不是ID"}})
	require.NoError(t, err, "顯示對象的值有誤也不影響個人捷徑")
	b := addPersonalReq(t, e, "alice", Tile{Title: "我的表單", URL: "https://b.example"})

	w := doRequest(t, e.p, "alice", http.MethodPost, "/api/v1/personal", Tile{Title: "第三個", URL: "https://c.example"})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, decode[errorResponse](t, w).Error, "最多 2 個")

	v := viewOf(t, e, "alice")
	require.Equal(t, 2, v.MaxPersonal)
	require.Equal(t, []string{"我的報表", "我的表單"}, titles(v.Personal))
	require.Empty(t, viewOf(t, e, "bob").Personal, "別人的捷徑看不到")

	// 別人改不到也刪不到：每個人的清單是分開的，找不到就是 404
	w = doRequest(t, e.p, "bob", http.MethodPut, "/api/v1/personal/"+a.ID, Tile{Title: "改掉", URL: "https://x.example"})
	require.Equal(t, http.StatusNotFound, w.Code)
	w = doRequest(t, e.p, "bob", http.MethodDelete, "/api/v1/personal/"+a.ID, nil)
	require.Equal(t, http.StatusNotFound, w.Code)

	w = doRequest(t, e.p, "alice", http.MethodPost, "/api/v1/personal/"+b.ID+"/move", moveRequest{Offset: -1})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"我的表單", "我的報表"}, titles(decode[personalListResponse](t, w).Tiles))

	w = doRequest(t, e.p, "alice", http.MethodPut, "/api/v1/personal/"+a.ID, Tile{Title: "報表", URL: "ftp://x"})
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "url", decode[errorResponse](t, w).Field)
	w = doRequest(t, e.p, "alice", http.MethodPut, "/api/v1/personal/"+a.ID, Tile{Title: "報表", URL: "https://a2.example"})
	require.Equal(t, http.StatusOK, w.Code)

	w = doRequest(t, e.p, "alice", http.MethodDelete, "/api/v1/personal/"+b.ID, nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"報表"}, titles(viewOf(t, e, "alice").Personal))

	// 刪掉一個之後又可以新增
	addPersonalReq(t, e, "alice", Tile{Title: "新的", URL: "https://n.example"})
}

func TestPersonalTilesCanBeDisabled(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 0})

	w := doRequest(t, e.p, "alice", http.MethodPost, "/api/v1/personal", Tile{Title: "x", URL: "https://x.example"})
	require.Equal(t, http.StatusForbidden, w.Code)

	v := viewOf(t, e, "alice")
	require.Equal(t, 0, v.MaxPersonal)
	require.NotNil(t, v.Personal)
	require.Empty(t, v.Personal)
}

func TestPersonalLimitIsClamped(t *testing.T) {
	require.Equal(t, 0, configuration{MaxPersonalTiles: -3}.personalLimit())
	require.Equal(t, 2, configuration{MaxPersonalTiles: 2}.personalLimit())
	require.Equal(t, maxPersonalLimit, configuration{MaxPersonalTiles: 999}.personalLimit())
}

func TestCommandIncludesPersonalTiles(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 2})
	seedShared(t, e.p, Tile{Title: "公告", URL: "https://all.example"})
	addPersonalReq(t, e, "alice", Tile{Title: "我的報表", Description: "每週業績", URL: "https://mine.example"})

	var sent *model.Post
	e.api.On("SendEphemeralPost", "alice", mock.Anything).Run(func(a mock.Arguments) {
		sent = a.Get(1).(*model.Post)
	}).Return(&model.Post{})

	_, appErr := e.p.ExecuteCommand(nil, &model.CommandArgs{UserId: "alice", ChannelId: "c1", Command: "/tiles"})
	require.Nil(t, appErr)
	require.Equal(t, []string{"公告", "我的報表"}, openButtonNames(sent))
	atts := sent.Attachments()
	require.Empty(t, atts[0].Footer, "共用項目那組按鈕")
	require.Contains(t, atts[1].Footer, "我的捷徑", "個人捷徑那組按鈕要註明只有自己看得到")
	require.Equal(t, scopePersonal, atts[1].Actions[0].Integration.Context["scope"])

	_, appErr = e.p.ExecuteCommand(nil, &model.CommandArgs{UserId: "alice", ChannelId: "c1", Command: "/tiles 業績"})
	require.Nil(t, appErr)
	require.Equal(t, []string{"我的報表"}, openButtonNames(sent), "關鍵字也比對個人捷徑的說明")
}
