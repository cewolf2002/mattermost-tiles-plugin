package main

import (
	"cmp"
	"net/http"
	"slices"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAudienceLimitsWhoSeesTile(t *testing.T) {
	e := setup(t, configuration{})
	eng := e.dir.addChannel("工程部", model.ChannelTypeOpen, "alice", "admin")
	sales := e.dir.addChannel("業務部", model.ChannelTypePrivate, "bob")

	seedShared(t, e.p,
		Tile{Title: "公告", URL: "https://all.example"},
		Tile{Title: "工程手冊", URL: "https://eng.example", Channels: []string{eng}},
		Tile{Title: "業務報價", URL: "https://sales.example", Channels: []string{sales}},
		Tile{Title: "跨部門", URL: "https://both.example", Channels: []string{eng, sales, eng}},
	)

	visible := func(user string) []string {
		w := doRequest(t, e.p, user, http.MethodGet, "/api/v1/tiles", nil)
		require.Equal(t, http.StatusOK, w.Code)
		require.NotContains(t, w.Body.String(), sales, "一般畫面不能帶出私人頻道的 ID")
		return titles(decode[viewResponse](t, w).Tiles)
	}
	require.Equal(t, []string{"公告", "工程手冊", "跨部門"}, visible("alice"))
	require.Equal(t, []string{"公告", "業務報價", "跨部門"}, visible("bob"))
	require.Equal(t, []string{"公告"}, visible("carol"), "不在任何頻道的人只看到給所有人的項目")
	require.Equal(t, []string{"公告", "工程手冊", "跨部門"}, visible("admin"), "管理員平常看到的也依自己的頻道過濾")

	// 管理清單要看到全部，並附上頻道名稱；重複的頻道只留一個
	list := decode[adminListResponse](t, doRequest(t, e.p, "admin", http.MethodGet, "/api/v1/admin/tiles", nil)).Tiles
	require.Equal(t, []string{"公告", "工程手冊", "業務報價", "跨部門"}, adminTitles(list))
	require.Empty(t, list[0].Audience)
	require.Equal(t, []string{eng, sales}, list[3].Channels)
	require.Equal(t, "業務部", list[2].Audience[0].DisplayName)
	require.True(t, list[2].Audience[0].Private)
	require.Equal(t, "Cewolf", list[2].Audience[0].TeamName)

	// 頻道被刪除後：管理清單標示出來，原本的成員也不再看得到
	delete(e.dir.channels, sales)
	delete(e.dir.members, sales)
	list = decode[adminListResponse](t, doRequest(t, e.p, "admin", http.MethodGet, "/api/v1/admin/tiles", nil)).Tiles
	require.True(t, list[2].Audience[0].Missing)
	require.Equal(t, []string{"公告"}, visible("bob"), "業務部被刪掉後，bob 已不在任何指定頻道")
}

func TestAudienceIsFilteredInSlashCommand(t *testing.T) {
	e := setup(t, configuration{})
	sales := e.dir.addChannel("業務部", model.ChannelTypePrivate, "bob")
	seedShared(t, e.p,
		Tile{Title: "公告", URL: "https://all.example"},
		Tile{Title: "業務報價", URL: "https://sales.example", Channels: []string{sales}},
	)

	sent := map[string]*model.Post{}
	e.api.On("SendEphemeralPost", mock.Anything, mock.Anything).Run(func(a mock.Arguments) {
		sent[a.String(0)] = a.Get(1).(*model.Post)
	}).Return(&model.Post{})

	for _, user := range []string{"alice", "bob"} {
		_, appErr := e.p.ExecuteCommand(nil, &model.CommandArgs{UserId: user, ChannelId: "c1", Command: "/tiles"})
		require.Nil(t, appErr)
	}
	require.Equal(t, []string{"公告"}, openButtonNames(sent["alice"]))
	require.Equal(t, []string{"公告", "業務報價"}, openButtonNames(sent["bob"]))
}

func TestAudienceValidation(t *testing.T) {
	e := setup(t, configuration{})
	dm := e.dir.addChannel("dm", model.ChannelTypeDirect, "admin", "alice")

	cases := map[string][]string{
		"格式不對":  {"abc"},
		"頻道不存在": {model.NewId()},
		"私訊頻道":  {dm},
	}
	for name, channels := range cases {
		w := doRequest(t, e.p, "admin", http.MethodPost, "/api/v1/admin/tiles", Tile{Title: "x", URL: "https://x.example", Channels: channels})
		require.Equal(t, http.StatusBadRequest, w.Code, name)
		require.Equal(t, "channels", decode[errorResponse](t, w).Field, name)
	}

	tooMany := make([]string, maxAudience+1)
	for i := range tooMany {
		tooMany[i] = model.NewId()
	}
	_, err := normalizeTile(Tile{Title: "x", URL: "https://x.example", Channels: tooMany})
	require.ErrorContains(t, err, "最多選")
}

func TestChannelOptions(t *testing.T) {
	e := setup(t, configuration{})
	e.dir.addChannel("閒聊", model.ChannelTypeOpen)
	e.dir.addChannel("工程部", model.ChannelTypeOpen, "admin")
	e.dir.addChannel("業務部", model.ChannelTypePrivate, "admin")
	e.dir.addChannel("人資部", model.ChannelTypePrivate, "carol") // 管理員沒加入，外掛 API 列不到
	e.dir.addChannel("dm", model.ChannelTypeDirect, "admin", "alice")
	archived := e.dir.addChannel("舊專案", model.ChannelTypeOpen, "admin")
	e.dir.channels[archived].DeleteAt = model.GetMillis()

	w := doRequest(t, e.p, "alice", http.MethodGet, "/api/v1/admin/channels", nil)
	require.Equal(t, http.StatusForbidden, w.Code)

	w = doRequest(t, e.p, "admin", http.MethodGet, "/api/v1/admin/channels", nil)
	require.Equal(t, http.StatusOK, w.Code)
	opts := decode[channelsResponse](t, w).Channels
	names := make([]string, 0, len(opts))
	for _, o := range opts {
		names = append(names, o.DisplayName)
	}
	require.ElementsMatch(t, []string{"閒聊", "工程部", "業務部"}, names)
	require.True(t, slices.IsSortedFunc(opts, func(a, b audienceInfo) int { return cmp.Compare(a.DisplayName, b.DisplayName) }))
}
