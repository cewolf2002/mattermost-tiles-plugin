package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeKV 用記憶體模擬外掛 KV store，讓測試跑到真正的讀寫與 compare-and-set 邏輯。
type fakeKV struct {
	mu          sync.Mutex
	data        map[string][]byte
	failNextCAS int // 讓接下來幾次 compare-and-set 失敗，模擬別人剛好同時存檔
}

func newFakeKV(api *plugintest.API) *fakeKV {
	kv := &fakeKV{data: map[string][]byte{}}
	api.On("KVGet", mock.Anything).Return(func(key string) ([]byte, *model.AppError) {
		kv.mu.Lock()
		defer kv.mu.Unlock()
		return kv.data[key], nil
	}).Maybe()
	api.On("KVCompareAndSet", mock.Anything, mock.Anything, mock.Anything).Return(func(key string, oldValue, newValue []byte) (bool, *model.AppError) {
		kv.mu.Lock()
		defer kv.mu.Unlock()
		if kv.failNextCAS > 0 {
			kv.failNextCAS--
			return false, nil
		}
		cur, exists := kv.data[key]
		if (oldValue == nil && exists) || (oldValue != nil && !bytes.Equal(cur, oldValue)) {
			return false, nil
		}
		kv.data[key] = newValue
		return true, nil
	}).Maybe()
	return kv
}

// fakeDir 用記憶體模擬團隊、頻道與頻道成員，給顯示對象相關的測試用。
type fakeDir struct {
	team     *model.Team
	channels map[string]*model.Channel
	members  map[string]map[string]bool // 頻道 ID → 成員的使用者 ID
	order    []string                   // 頻道建立順序，讓列表結果固定
}

func notFound(where string) *model.AppError {
	return model.NewAppError(where, "not_found", nil, "", http.StatusNotFound)
}

func newFakeDir(api *plugintest.API) *fakeDir {
	d := &fakeDir{
		team:     &model.Team{Id: model.NewId(), DisplayName: "Cewolf"},
		channels: map[string]*model.Channel{},
		members:  map[string]map[string]bool{},
	}
	api.On("GetChannel", mock.Anything).Return(func(id string) (*model.Channel, *model.AppError) {
		if ch, ok := d.channels[id]; ok {
			return ch, nil
		}
		return nil, notFound("GetChannel")
	}).Maybe()
	api.On("GetChannelMember", mock.Anything, mock.Anything).Return(func(channelID, userID string) (*model.ChannelMember, *model.AppError) {
		if d.members[channelID][userID] {
			return &model.ChannelMember{ChannelId: channelID, UserId: userID}, nil
		}
		return nil, notFound("GetChannelMember")
	}).Maybe()
	api.On("GetTeam", mock.Anything).Return(func(id string) (*model.Team, *model.AppError) {
		if id == d.team.Id {
			return d.team, nil
		}
		return nil, notFound("GetTeam")
	}).Maybe()
	api.On("GetTeams").Return(func() ([]*model.Team, *model.AppError) {
		return []*model.Team{d.team}, nil
	}).Maybe()
	api.On("GetPublicChannelsForTeam", mock.Anything, mock.Anything, mock.Anything).Return(func(teamID string, page, perPage int) ([]*model.Channel, *model.AppError) {
		var out []*model.Channel
		if page == 0 {
			for _, id := range d.order {
				if ch := d.channels[id]; ch.TeamId == teamID && ch.Type == model.ChannelTypeOpen {
					out = append(out, ch)
				}
			}
		}
		return out, nil
	}).Maybe()
	api.On("GetChannelsForTeamForUser", mock.Anything, mock.Anything, mock.Anything).Return(func(teamID, userID string, _ bool) ([]*model.Channel, *model.AppError) {
		var out []*model.Channel
		for _, id := range d.order {
			if ch := d.channels[id]; d.members[id][userID] && (ch.TeamId == teamID || ch.TeamId == "") {
				out = append(out, ch)
			}
		}
		return out, nil
	}).Maybe()
	return d
}

// addChannel 建立頻道並加入成員，回傳頻道 ID。
func (d *fakeDir) addChannel(name string, typ model.ChannelType, members ...string) string {
	id := model.NewId()
	teamID := d.team.Id
	if typ == model.ChannelTypeDirect {
		teamID = ""
	}
	d.channels[id] = &model.Channel{Id: id, TeamId: teamID, Name: name, DisplayName: name, Type: typ}
	d.members[id] = map[string]bool{}
	for _, u := range members {
		d.members[id][u] = true
	}
	d.order = append(d.order, id)
	return id
}

type testEnv struct {
	p   *Plugin
	api *plugintest.API
	kv  *fakeKV
	dir *fakeDir
}

// setup 建立已啟用的外掛；"admin" 是系統管理員，其他使用者都是一般同事。
func setup(t *testing.T, cfg configuration) *testEnv {
	t.Helper()
	api := &plugintest.API{}
	api.On("EnsureBotUser", mock.Anything).Return("bot123", nil)
	api.On("LoadPluginConfiguration", mock.Anything).Run(func(args mock.Arguments) {
		*args.Get(0).(*configuration) = cfg
	}).Return(nil)
	api.On("RegisterCommand", mock.MatchedBy(func(c *model.Command) bool {
		return c.Trigger == normalizeTrigger(cfg.Trigger) && c.AutoComplete
	})).Return(nil).Once()
	for _, level := range []string{"LogError", "LogWarn"} {
		api.On(level, mock.Anything).Return().Maybe()
		api.On(level, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	}
	api.On("HasPermissionTo", "admin", model.PermissionManageSystem).Return(true).Maybe()
	api.On("HasPermissionTo", mock.Anything, model.PermissionManageSystem).Return(false).Maybe()
	kv := newFakeKV(api)
	dir := newFakeDir(api)

	p := &Plugin{}
	p.SetAPI(api)
	require.NoError(t, p.OnConfigurationChange()) // 啟用前的這次呼叫不能註冊指令
	require.NoError(t, p.OnActivate())
	return &testEnv{p: p, api: api, kv: kv, dir: dir}
}

func doRequest(t *testing.T, p *Plugin, userID, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, reader)
	if userID != "" {
		r.Header.Set("Mattermost-User-ID", userID)
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(nil, w, r)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v), w.Body.String())
	return v
}

func titles(tiles []Tile) []string {
	out := make([]string, 0, len(tiles))
	for _, t := range tiles {
		out = append(out, t.Title)
	}
	return out
}

// tileAttachments 取有標題的附件（管理我的捷徑清單用的是附件，不是按鈕）。
func tileAttachments(post *model.Post) []*model.SlackAttachment {
	var out []*model.SlackAttachment
	for _, a := range post.Attachments() {
		if a.Title != "" {
			out = append(out, a)
		}
	}
	return out
}

func isOpenAction(act *model.PostAction) bool {
	return act.Integration != nil && act.Integration.Context["action"] == "open"
}

// buttonNames 回傳 tile 以外的按鈕（新增、管理、加到我的最愛、編輯、刪除）文字，依出現順序。
func buttonNames(post *model.Post) []string {
	var out []string
	for _, a := range post.Attachments() {
		for _, act := range a.Actions {
			if !isOpenAction(act) {
				out = append(out, act.Name)
			}
		}
	}
	return out
}

// openButtons 回傳選單裡代表 tile 的按鈕，依出現順序。
func openButtons(post *model.Post) []*model.PostAction {
	var out []*model.PostAction
	for _, a := range post.Attachments() {
		for _, act := range a.Actions {
			if isOpenAction(act) {
				out = append(out, act)
			}
		}
	}
	return out
}

func openButtonNames(post *model.Post) []string {
	var out []string
	for _, act := range openButtons(post) {
		out = append(out, act.Name)
	}
	return out
}

func adminTitles(tiles []adminTile) []string {
	out := make([]string, 0, len(tiles))
	for _, t := range tiles {
		out = append(out, t.Title)
	}
	return out
}

// seedShared 以管理員身分透過 API 新增共用 tile，回傳伺服器產生 ID 後的結果。
func seedShared(t *testing.T, p *Plugin, tiles ...Tile) []Tile {
	t.Helper()
	out := make([]Tile, 0, len(tiles))
	for _, in := range tiles {
		w := doRequest(t, p, "admin", "POST", "/api/v1/admin/tiles", in)
		require.Equal(t, 200, w.Code, w.Body.String())
		out = append(out, decode[Tile](t, w))
	}
	return out
}
