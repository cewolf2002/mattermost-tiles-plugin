package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// captureEphemeral 記下送給某人的暫時訊息（新送出的與更新的分開記）。
type ephemeralLog struct {
	sent    []*model.Post
	updated []*model.Post
}

func captureEphemeral(e *testEnv, user string) *ephemeralLog {
	log := &ephemeralLog{}
	e.api.On("SendEphemeralPost", user, mock.Anything).Run(func(a mock.Arguments) {
		log.sent = append(log.sent, a.Get(1).(*model.Post))
	}).Return(&model.Post{}).Maybe()
	e.api.On("UpdateEphemeralPost", user, mock.Anything).Run(func(a mock.Arguments) {
		log.updated = append(log.updated, a.Get(1).(*model.Post))
	}).Return(&model.Post{}).Maybe()
	return log
}

func captureDialogs(e *testEnv) *[]model.OpenDialogRequest {
	var opened []model.OpenDialogRequest
	e.api.On("OpenInteractiveDialog", mock.Anything).Run(func(a mock.Arguments) {
		opened = append(opened, a.Get(0).(model.OpenDialogRequest))
	}).Return(nil).Maybe()
	return &opened
}

// doAction 模擬按下訊息上的按鈕，回傳 bot 因此送給使用者的文字回覆（沒有就是空字串）。
func doAction(t *testing.T, e *testEnv, log *ephemeralLog, user string, ctx map[string]any) string {
	t.Helper()
	before := len(log.sent)
	w := doRequest(t, e.p, user, http.MethodPost, "/api/v1/action", model.PostActionIntegrationRequest{
		UserId: "someone-else", TriggerId: "trigger1", ChannelId: "c1", PostId: "post1", Context: ctx,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	resp := decode[model.PostActionIntegrationResponse](t, w)
	require.Empty(t, resp.EphemeralText, "不能用 EphemeralText：收合討論串時會看不到")
	if len(log.sent) == before {
		return ""
	}
	last := log.sent[len(log.sent)-1]
	require.Empty(t, last.RootId, "回覆不能掛在討論串裡")
	require.Equal(t, "c1", last.ChannelId)
	return last.Message
}

func submitDialog(t *testing.T, e *testEnv, user, state string, submission map[string]any) model.SubmitDialogResponse {
	t.Helper()
	w := doRequest(t, e.p, user, http.MethodPost, "/api/v1/dialog", model.SubmitDialogRequest{
		CallbackId: "personal", State: state, ChannelId: "c1", Submission: submission,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return decode[model.SubmitDialogResponse](t, w)
}

func TestMenuButtons(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 1})
	log := captureEphemeral(e, "alice")
	run := func() *model.Post {
		_, appErr := e.p.ExecuteCommand(nil, &model.CommandArgs{UserId: "alice", ChannelId: "c1", Command: "/tiles"})
		require.Nil(t, appErr)
		return log.sent[len(log.sent)-1]
	}

	require.Equal(t, []string{"＋ 新增我的捷徑", "📌 加到我的最愛"}, buttonNames(run()))
	addPersonalReq(t, e, "alice", Tile{Title: "我的報表", URL: "https://a.example"})
	require.Equal(t, []string{"管理我的捷徑", "📌 加到我的最愛"}, buttonNames(run()), "到上限就不再顯示新增")

	cfg, trigger, _ := e.p.snapshot()
	require.Equal(t, []string{"管理我的捷徑"}, buttonNames(e.p.buildMenu("alice", "", cfg, trigger, true)), "私訊裡不需要「加到我的最愛」")

	for _, a := range run().Attachments() {
		for _, act := range a.Actions {
			require.Equal(t, "/plugins/tw.com.cewolf.tiles/api/v1/action", act.Integration.URL)
		}
	}
}

func TestAddButtonOpensDialogAndSubmitSaves(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 1})
	opened := captureDialogs(e)
	log := captureEphemeral(e, "alice")

	reply := doAction(t, e, log, "alice", map[string]any{"action": "add"})
	require.Empty(t, reply)
	require.Len(t, *opened, 1)
	req := (*opened)[0]
	require.Equal(t, "trigger1", req.TriggerId)
	require.Equal(t, "/plugins/tw.com.cewolf.tiles/api/v1/dialog", req.URL)
	require.Equal(t, "新增我的捷徑", req.Dialog.Title)
	require.Empty(t, req.Dialog.State)

	// 欄位錯誤回到表單上，不新增
	r := submitDialog(t, e, "alice", "", map[string]any{"title": "報表", "url": "報表網址"})
	require.Contains(t, r.Errors["url"], "http")
	require.Empty(t, viewOf(t, e, "alice").Personal)

	r = submitDialog(t, e, "alice", "", map[string]any{"title": " 報表 ", "url": "https://a.example", "color": defaultColorValue})
	require.Empty(t, r.Errors)
	require.Empty(t, r.Error)
	personal := viewOf(t, e, "alice").Personal
	require.Equal(t, []string{"報表"}, titles(personal))
	require.Empty(t, personal[0].Color, "選「預設」要存成空字串")
	require.Contains(t, log.sent[len(log.sent)-1].Message, "已儲存「報表」")
	require.Equal(t, "c1", log.sent[len(log.sent)-1].ChannelId)

	// 到上限後按新增：不開表單，直接告訴使用者
	reply = doAction(t, e, log, "alice", map[string]any{"action": "add"})
	require.Contains(t, reply, "最多 1 個")
	require.Len(t, *opened, 1)

	// 表單送出時才超過上限（例如同時在電腦上新增），錯誤顯示在表單上
	r = submitDialog(t, e, "alice", "", map[string]any{"title": "第二個", "url": "https://b.example"})
	require.Contains(t, r.Error, "最多 1 個")

	r = submitDialog(t, e, "alice", "", map[string]any{"title": "x"})
	require.NotEmpty(t, r.Errors["url"])

	w := doRequest(t, e.p, "alice", http.MethodPost, "/api/v1/dialog", model.SubmitDialogRequest{CallbackId: "personal", Cancelled: true})
	require.Equal(t, http.StatusOK, w.Code, "按取消不做任何事")
}

func TestEditAndDeleteFromManageList(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 2})
	opened := captureDialogs(e)
	log := captureEphemeral(e, "alice")
	a := addPersonalReq(t, e, "alice", Tile{Title: "報表", URL: "https://a.example", Description: "每週", Color: "#1D9E75", Icon: "📊"})
	b := addPersonalReq(t, e, "alice", Tile{Title: "請假", URL: "https://b.example"})

	doAction(t, e, log, "alice", map[string]any{"action": "manage"})
	manage := log.sent[len(log.sent)-1]
	require.Equal(t, "bot123", manage.UserId)
	require.Equal(t, "c1", manage.ChannelId)
	require.Equal(t, []string{"📊 報表", "請假"}, titlesOf(tileAttachments(manage)), "附件標題前面會加上圖示")
	require.Equal(t, []string{"編輯", "刪除", "編輯", "刪除"}, buttonNames(manage))

	doAction(t, e, log, "alice", map[string]any{"action": "edit", "tile_id": a.ID})
	d := (*opened)[0].Dialog
	require.Equal(t, "編輯我的捷徑", d.Title)
	require.Equal(t, a.ID, d.State)
	require.Equal(t, []string{"報表", "https://a.example", "每週", "📊", "#1D9E75"},
		[]string{d.Elements[0].Default, d.Elements[1].Default, d.Elements[2].Default, d.Elements[3].Default, d.Elements[4].Default})

	// 真正的表單會把預先帶入的欄位一起送回來
	r := submitDialog(t, e, "alice", a.ID, map[string]any{"title": "業績報表", "url": "https://a.example", "icon": "📊"})
	require.Empty(t, r.Error)
	require.Equal(t, []string{"業績報表", "請假"}, titles(viewOf(t, e, "alice").Personal), "編輯後位置不變")

	// 刪除：直接更新原本那則管理訊息，而不是另外發一則
	require.Empty(t, doAction(t, e, log, "alice", map[string]any{"action": "delete", "tile_id": b.ID}))
	require.Len(t, log.updated, 1)
	updated := log.updated[0]
	require.Equal(t, "post1", updated.Id)
	require.Equal(t, "c1", updated.ChannelId)
	require.Contains(t, updated.Message, "已刪除「請假」")
	require.Equal(t, []string{"📊 業績報表"}, titlesOf(tileAttachments(updated)))
	require.Contains(t, buttonNames(updated), "＋ 新增我的捷徑", "刪掉後又可以新增")

	require.Contains(t, doAction(t, e, log, "alice", map[string]any{"action": "delete", "tile_id": b.ID}), "找不到")
	bobLog := captureEphemeral(e, "bob")
	require.Contains(t, doAction(t, e, bobLog, "bob", map[string]any{"action": "edit", "tile_id": a.ID}), "找不到", "不能編輯別人的捷徑")

	w := doRequest(t, e.p, "alice", http.MethodPost, "/api/v1/action", model.PostActionIntegrationRequest{Context: map[string]any{"action": "hack"}})
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func titlesOf(atts []*model.SlackAttachment) []string {
	out := make([]string, 0, len(atts))
	for _, a := range atts {
		out = append(out, a.Title)
	}
	return out
}

func TestPersonalDialogFitsMattermostLimits(t *testing.T) {
	// 對話框限制以位元組計算：中文一字 3 位元組，標題最多 24 位元組、單行欄位預設值最多 150 位元組
	long := Tile{
		ID:          model.NewId(),
		Title:       strings.Repeat("報", maxTitleLen),
		URL:         "https://example.com/" + strings.Repeat("a", maxURLLen-20),
		Description: strings.Repeat("說", maxDescriptionLen),
		Icon:        "🛠️",
		Color:       "#1D9E75",
	}
	for _, d := range []model.Dialog{personalDialog(nil), personalDialog(&long)} {
		require.NoError(t, d.IsValid(), d.Title)
	}
}

func TestBotDMRepliesWithMenu(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 2})
	seedShared(t, e.p,
		Tile{Title: "報價單", URL: "https://quote.example"},
		Tile{Title: "請假規定", URL: "https://leave.example"},
	)
	dm := e.dir.addChannel(model.GetDMNameFromIds("alice", "bot123"), model.ChannelTypeDirect, "alice", "bot123")
	other := e.dir.addChannel(model.GetDMNameFromIds("alice", "bob"), model.ChannelTypeDirect, "alice", "bob")
	town := e.dir.addChannel("town-square", model.ChannelTypeOpen, "alice")

	var replies []*model.Post
	e.api.On("CreatePost", mock.Anything).Run(func(a mock.Arguments) {
		replies = append(replies, a.Get(0).(*model.Post))
	}).Return(&model.Post{}, nil).Maybe()

	e.p.MessageHasBeenPosted(nil, &model.Post{UserId: "alice", ChannelId: dm, Message: "報價"})
	require.Len(t, replies, 1)
	require.Equal(t, "bot123", replies[0].UserId)
	require.Equal(t, dm, replies[0].ChannelId)
	require.Equal(t, []string{"報價單"}, openButtonNames(replies[0]))
	require.NotContains(t, buttonNames(replies[0]), "📌 加到我的最愛")

	e.p.MessageHasBeenPosted(nil, &model.Post{UserId: "alice", ChannelId: dm, Message: "全部"})
	require.Len(t, openButtons(replies[1]), 2, "輸入「全部」顯示完整選單")

	e.p.MessageHasBeenPosted(nil, &model.Post{UserId: "alice", ChannelId: dm, Message: "沒有這個"})
	require.Contains(t, replies[2].Message, "輸入「全部」看完整選單")

	// 不該回覆的情況
	e.p.MessageHasBeenPosted(nil, &model.Post{UserId: "bot123", ChannelId: dm, Message: "報價"})
	e.p.MessageHasBeenPosted(nil, &model.Post{UserId: "alice", ChannelId: other, Message: "報價"})
	e.p.MessageHasBeenPosted(nil, &model.Post{UserId: "alice", ChannelId: town, Message: "報價"})
	e.p.MessageHasBeenPosted(nil, &model.Post{UserId: "alice", ChannelId: dm, Message: "報價", Type: model.PostTypeJoinChannel})
	webhook := &model.Post{UserId: "alice", ChannelId: dm, Message: "報價"}
	webhook.AddProp("from_webhook", "true")
	e.p.MessageHasBeenPosted(nil, webhook)
	require.Len(t, replies, 3)
}

func TestPinAddsBotDMToFavorites(t *testing.T) {
	e := setup(t, configuration{})
	dmID := model.NewId()
	teamID := e.dir.team.Id
	e.api.On("GetDirectChannel", "alice", "bot123").Return(&model.Channel{Id: dmID, Type: model.ChannelTypeDirect}, nil)
	e.api.On("GetTeamsForUser", "alice").Return([]*model.Team{e.dir.team}, nil)

	// 側邊欄：我的最愛、一般頻道、一個自訂分類（私訊被放在這裡）、私訊分類
	favorites := &model.SidebarCategoryWithChannels{SidebarCategory: model.SidebarCategory{Id: "fav", Type: model.SidebarCategoryFavorites}, Channels: []string{"c-old"}}
	custom := &model.SidebarCategoryWithChannels{SidebarCategory: model.SidebarCategory{Id: "custom", Type: model.SidebarCategoryCustom}, Channels: []string{"c1", dmID}}
	channels := &model.SidebarCategoryWithChannels{SidebarCategory: model.SidebarCategory{Id: "ch", Type: model.SidebarCategoryChannels}, Channels: []string{"c2"}}
	dms := &model.SidebarCategoryWithChannels{SidebarCategory: model.SidebarCategory{Id: "dm", Type: model.SidebarCategoryDirectMessages}}
	e.api.On("GetChannelSidebarCategories", "alice", teamID).Return(func(string, string) (*model.OrderedSidebarCategories, *model.AppError) {
		return &model.OrderedSidebarCategories{Categories: model.SidebarCategoriesWithChannels{favorites, channels, custom, dms}}, nil
	})
	var sentUpdate []*model.SidebarCategoryWithChannels
	e.api.On("UpdateChannelSidebarCategories", "alice", teamID, mock.Anything).Run(func(a mock.Arguments) {
		sentUpdate = a.Get(2).([]*model.SidebarCategoryWithChannels)
	}).Return(nil, nil)
	var welcome *model.Post
	e.api.On("CreatePost", mock.Anything).Run(func(a mock.Arguments) {
		welcome = a.Get(0).(*model.Post)
	}).Return(&model.Post{}, nil)

	w := doRequest(t, e.p, "alice", http.MethodPost, "/api/v1/pin", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, map[string]bool{"already": false}, decode[map[string]bool](t, w))

	require.NotNil(t, welcome)
	require.Equal(t, dmID, welcome.ChannelId)
	require.Equal(t, "bot123", welcome.UserId)
	require.Contains(t, welcome.Message, "固定入口")

	// 來源分類（自訂）與目的分類（我的最愛）都要送出；一般頻道與私訊分類不動
	require.Len(t, sentUpdate, 2)
	require.Equal(t, "custom", sentUpdate[0].Id)
	require.Equal(t, []string{"c1"}, sentUpdate[0].Channels)
	require.Equal(t, "fav", sentUpdate[1].Id)
	require.Equal(t, []string{dmID, "c-old"}, sentUpdate[1].Channels)

	// 已經在我的最愛：什麼都不改，也不再發歡迎訊息
	welcome = nil
	log := captureEphemeral(e, "alice")
	require.Contains(t, doAction(t, e, log, "alice", map[string]any{"action": "pin"}), "已經在")
	require.Nil(t, welcome)
}

func TestOpenButtonRepliesWithLink(t *testing.T) {
	e := setup(t, configuration{MaxPersonalTiles: 2})
	sales := e.dir.addChannel("業務部", model.ChannelTypePrivate, "bob")
	seeded := seedShared(t, e.p,
		Tile{Icon: "📘", Title: "使用手冊 [新版]", Description: "安裝與設定", URL: "https://example.com/wiki/A_(b) c", Color: "#1D9E75"},
		Tile{Title: "業務報價", URL: "https://sales.example", Channels: []string{sales}},
	)
	manual, quote := seeded[0], seeded[1]
	mine := addPersonalReq(t, e, "alice", Tile{Title: "我的報表", URL: "https://mine.example"})
	alice := captureEphemeral(e, "alice")
	bob := captureEphemeral(e, "bob")

	// 選單上的按鈕帶著 tile 的顏色與 ID
	_, appErr := e.p.ExecuteCommand(nil, &model.CommandArgs{UserId: "alice", ChannelId: "c1", Command: "/tiles"})
	require.Nil(t, appErr)
	btn := openButtons(alice.sent[len(alice.sent)-1])[0]
	require.Equal(t, "📘 使用手冊 [新版]", btn.Name)
	require.Equal(t, "#1D9E75", btn.Style)
	require.Equal(t, manual.ID, btn.Integration.Context["tile_id"])

	// 按下後回一則附連結的暫時訊息；標題的中括號與網址的括號、空白都要跳脫，連結才不會斷掉
	reply := doAction(t, e, alice, "alice", map[string]any{"action": "open", "scope": scopeShared, "tile_id": manual.ID})
	require.Contains(t, reply, `#### [📘 使用手冊 \[新版\]](https://example.com/wiki/A_%28b%29%20c)`)
	require.Contains(t, reply, "安裝與設定")

	reply = doAction(t, e, alice, "alice", map[string]any{"action": "open", "scope": scopePersonal, "tile_id": mine.ID})
	require.Contains(t, reply, "(https://mine.example)")

	// 繞過顯示對象或拿別人的捷徑都拿不到連結
	reply = doAction(t, e, alice, "alice", map[string]any{"action": "open", "scope": scopeShared, "tile_id": quote.ID})
	require.Contains(t, reply, "找不到")
	require.NotContains(t, reply, "sales.example")
	reply = doAction(t, e, bob, "bob", map[string]any{"action": "open", "scope": scopeShared, "tile_id": quote.ID})
	require.Contains(t, reply, "(https://sales.example)", "業務部成員拿得到")
	reply = doAction(t, e, bob, "bob", map[string]any{"action": "open", "scope": scopePersonal, "tile_id": mine.ID})
	require.Contains(t, reply, "找不到")
}
