package main

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// 手機 app 不會執行外掛的網頁程式，右側面板在手機上不存在。
// 這裡改用手機也支援的三樣東西：bot 私訊、訊息上的按鈕、互動式對話框（表單）。

const (
	actionPath = "/plugins/" + pluginID + "/api/v1/action"
	dialogPath = "/plugins/" + pluginID + "/api/v1/dialog"

	defaultColorValue = "default" // 對話框的選項值不能是空字串，用這個代表「預設顏色」

	// 選單按鈕的 context 裡標明 tile 屬於哪一份清單
	scopeShared   = "shared"
	scopePersonal = "personal"
)

// 在私訊裡輸入這些字就顯示完整選單，不當成關鍵字篩選
var showAllWords = []string{"全部", "選單", "清單", "menu", "all", "help", "?", "？"}

var colorOptions = []*model.PostActionOptions{
	{Text: "預設", Value: defaultColorValue},
	{Text: "綠", Value: "#1D9E75"},
	{Text: "藍", Value: "#378ADD"},
	{Text: "橘", Value: "#D85A30"},
	{Text: "金", Value: "#BA7517"},
	{Text: "紫", Value: "#7F56D9"},
	{Text: "紅", Value: "#D24B4E"},
	{Text: "灰", Value: "#6B7280"},
}

func newAction(name, action, style string, extra map[string]any) *model.PostAction {
	ctx := map[string]any{"action": action}
	for k, v := range extra {
		ctx[k] = v
	}
	return &model.PostAction{
		Type:        model.PostActionTypeButton,
		Name:        name,
		Style:       style,
		Integration: &model.PostActionIntegration{URL: actionPath, Context: ctx},
	}
}

// tileButtons 把每個 tile 做成一顆按鈕（顏色沿用 tile 的強調色）。
// Mattermost 的訊息按鈕只能回呼外掛、不能直接開網址，所以按下後由 handleAction 回一則附連結的暫時訊息，再點一次開啟。
func tileButtons(tiles []Tile, scope string) *model.SlackAttachment {
	if len(tiles) == 0 {
		return nil
	}
	actions := make([]*model.PostAction, 0, len(tiles))
	for _, t := range tiles {
		style := t.Color
		if style == "" {
			style = "default"
		}
		actions = append(actions, newAction(tileHeading(t), "open", style, map[string]any{"tile_id": t.ID, "scope": scope}))
	}
	return &model.SlackAttachment{Actions: actions}
}

// findVisibleTile 找出按鈕對應的 tile；共用項目要再確認使用者看得到，避免繞過顯示對象拿到連結。
func (p *Plugin) findVisibleTile(userID, scope, id string) (Tile, bool) {
	var tiles []Tile
	var err error
	if scope == scopePersonal {
		tiles, _, err = p.loadPersonal(userID)
	} else {
		tiles, err = p.loadTiles(sharedKey)
		tiles = p.visibleTiles(userID, tiles)
	}
	if err != nil {
		return Tile{}, false
	}
	return findTile(tiles, id)
}

var (
	linkTextEscaper = strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)
	// 網址裡的空白與括號會讓 Markdown 連結提早結束，換成等價的百分比編碼
	linkURLEscaper = strings.NewReplacer(" ", "%20", "(", "%28", ")", "%29")
)

// openLinkText 是按下 tile 按鈕後的回覆：連結用標題大小，手機上比較好點。
func openLinkText(t Tile) string {
	text := "#### [" + linkTextEscaper.Replace(tileHeading(t)) + "](" + linkURLEscaper.Replace(t.URL) + ")"
	if t.Description != "" {
		text += "\n" + t.Description
	}
	return text + "\n點上面的連結開啟。"
}

// menuActions 是選單底下的按鈕列；在私訊裡已經是固定入口，就不再顯示「加到我的最愛」。
func menuActions(personalCount, limit int, inDM bool) *model.SlackAttachment {
	var actions []*model.PostAction
	if limit > 0 && personalCount < limit {
		actions = append(actions, newAction("＋ 新增我的捷徑", "add", "primary", nil))
	}
	if personalCount > 0 {
		actions = append(actions, newAction("管理我的捷徑", "manage", "default", nil))
	}
	if !inDM {
		actions = append(actions, newAction("📌 加到我的最愛", "pin", "default", nil))
	}
	if len(actions) == 0 {
		return nil
	}
	return &model.SlackAttachment{Actions: actions}
}

// managePost 列出使用者的捷徑，每則附「編輯」「刪除」按鈕；只用暫時訊息送出，只有本人看得到。
func (p *Plugin) managePost(userID, notice string) *model.Post {
	personal, limit, err := p.loadPersonal(userID)
	if err != nil {
		p.API.LogError("load personal tiles", "err", err.Error())
		return &model.Post{Message: ":warning: 讀取捷徑失敗，請稍後再試。"}
	}

	text := "#### 管理我的捷徑\n只有你看得到。"
	if notice != "" {
		text = notice + "\n\n" + text
	}
	if len(personal) == 0 {
		text += "\n\n目前沒有捷徑。"
	}
	atts := buildAttachments(personal)
	for i, t := range personal {
		atts[i].Actions = []*model.PostAction{
			newAction("編輯", "edit", "default", map[string]any{"tile_id": t.ID}),
			newAction("刪除", "delete", "danger", map[string]any{"tile_id": t.ID}),
		}
	}
	if limit > 0 && len(personal) < limit {
		atts = append(atts, &model.SlackAttachment{Actions: []*model.PostAction{newAction("＋ 新增我的捷徑", "add", "primary", nil)}})
	}
	post := &model.Post{Message: text}
	model.ParseSlackAttachment(post, atts)
	return post
}

// personalDialog 是手機上新增／編輯捷徑的表單。
// 欄位長度上限是以位元組計算（中文一字 3 位元組），網址與說明用多行欄位才放得下長內容。
func personalDialog(t *Tile) model.Dialog {
	d := model.Dialog{
		CallbackId:       "personal",
		Title:            "新增我的捷徑",
		IntroductionText: "只有你看得到。",
		SubmitLabel:      "儲存",
		Elements: []model.DialogElement{
			{DisplayName: "標題", Name: "title", Type: "text", MaxLength: maxTitleLen, Placeholder: "例如：請假系統"},
			{DisplayName: "網址", Name: "url", Type: "textarea", MaxLength: maxURLLen, Placeholder: "https://"},
			{DisplayName: "說明", Name: "description", Type: "textarea", Optional: true, MaxLength: maxDescriptionLen, Placeholder: "一句話說明，搜尋時也會比對"},
			{DisplayName: "圖示", Name: "icon", Type: "text", Optional: true, MaxLength: 16, Placeholder: "一個 emoji，例如 📘"},
			{DisplayName: "顏色", Name: "color", Type: "select", Optional: true, Options: colorOptions},
		},
	}
	if t != nil {
		d.Title = "編輯我的捷徑"
		d.State = t.ID
		d.Elements[0].Default = t.Title
		d.Elements[1].Default = t.URL
		d.Elements[2].Default = t.Description
		d.Elements[3].Default = t.Icon
		d.Elements[4].Default = t.Color
	}
	return d
}

func (p *Plugin) openPersonalDialog(triggerID string, t *Tile) error {
	if appErr := p.API.OpenInteractiveDialog(model.OpenDialogRequest{
		TriggerId: triggerID,
		URL:       dialogPath,
		Dialog:    personalDialog(t),
	}); appErr != nil {
		return appErr
	}
	return nil
}

func findTile(tiles []Tile, id string) (Tile, bool) {
	if i := indexOfTile(tiles, id); i >= 0 {
		return tiles[i], true
	}
	return Tile{}, false
}

// handleAction 處理選單訊息上的按鈕。
// 按鈕所在的通常是暫時訊息（不存在資料庫），所以不能用回應裡的 Update 改它，要自己呼叫 UpdateEphemeralPost。
// 回覆也不用回應裡的 EphemeralText：Mattermost 會把它當成按鈕那則訊息的討論串回覆，
// 而 10.x 預設開啟「收合討論串」，回覆就只出現在討論串裡，頻道上看不到。改成自己送一則不掛討論串的暫時訊息。
func (p *Plugin) handleAction(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r)
	var req model.PostActionIntegrationRequest
	if !decodeBody(w, r, &req) {
		return
	}
	action, _ := req.Context["action"].(string)
	tileID, _ := req.Context["tile_id"].(string)
	scope, _ := req.Context["scope"].(string)
	_, _, botID := p.snapshot()
	reply := ""

	switch action {
	case "open":
		if t, ok := p.findVisibleTile(userID, scope, tileID); ok {
			reply = openLinkText(t)
		} else {
			reply = errTileNotFound.Error()
		}

	case "add":
		personal, limit, err := p.loadPersonal(userID)
		switch {
		case err != nil:
			p.API.LogError("load personal tiles", "err", err.Error())
			reply = "讀取捷徑失敗，請稍後再試。"
		case limit == 0:
			reply = errPersonalDisabled.Error()
		case len(personal) >= limit:
			reply = fmt.Sprintf("我的捷徑最多 %d 個，請先按「管理我的捷徑」刪除用不到的。", limit)
		default:
			if err := p.openPersonalDialog(req.TriggerId, nil); err != nil {
				p.API.LogError("open dialog", "err", err.Error())
				reply = "表單開不起來，請再按一次。"
			}
		}

	case "edit":
		personal, _, err := p.loadPersonal(userID)
		t, ok := findTile(personal, tileID)
		switch {
		case err != nil || !ok:
			reply = errTileNotFound.Error()
		default:
			if err := p.openPersonalDialog(req.TriggerId, &t); err != nil {
				p.API.LogError("open dialog", "err", err.Error())
				reply = "表單開不起來，請再按一次。"
			}
		}

	case "delete":
		personal, _, _ := p.loadPersonal(userID)
		t, ok := findTile(personal, tileID)
		if !ok {
			reply = errTileNotFound.Error()
			break
		}
		if _, err := p.deletePersonal(userID, tileID); err != nil {
			reply = "刪除失敗：" + err.Error()
			break
		}
		// 直接更新這則管理訊息，被刪的項目就從清單上消失
		post := p.managePost(userID, "已刪除「"+t.Title+"」。")
		post.Id = req.PostId
		post.ChannelId = req.ChannelId
		post.UserId = botID
		p.API.UpdateEphemeralPost(userID, post)

	case "manage":
		post := p.managePost(userID, "")
		post.ChannelId = req.ChannelId
		post.UserId = botID
		p.API.SendEphemeralPost(userID, post)

	case "pin":
		already, err := p.pinForUser(userID)
		switch {
		case err != nil:
			p.API.LogError("pin bot DM", "err", err.Error())
			reply = "加入失敗，可以自己長按「快速查詢助手」的私訊 →「加入我的最愛」。"
		case already:
			reply = "「快速查詢助手」已經在你的「我的最愛」裡了。"
		default:
			reply = "已把「快速查詢助手」加到「我的最愛」，手機上也找得到；在私訊裡直接打關鍵字就能查。"
		}

	default:
		writeError(w, http.StatusBadRequest, "未知的動作")
		return
	}
	if reply != "" {
		p.API.SendEphemeralPost(userID, &model.Post{UserId: botID, ChannelId: req.ChannelId, Message: reply})
	}
	writeJSON(w, http.StatusOK, model.PostActionIntegrationResponse{})
}

// handleDialog 接收手機表單送出的內容；欄位錯誤回傳到表單上顯示，不關閉表單。
func (p *Plugin) handleDialog(w http.ResponseWriter, r *http.Request) {
	userID := userIDFrom(r)
	var req model.SubmitDialogRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Cancelled || req.CallbackId != "personal" {
		writeJSON(w, http.StatusOK, model.SubmitDialogResponse{})
		return
	}

	text := func(name string) string {
		s, _ := req.Submission[name].(string)
		return s
	}
	in := Tile{
		Title:       text("title"),
		URL:         text("url"),
		Description: text("description"),
		Icon:        text("icon"),
		Color:       text("color"),
	}
	if in.Color == defaultColorValue {
		in.Color = ""
	}

	var saved Tile
	var err error
	// State 只是自己清單裡的 ID；就算被竄改，也只會動到自己的捷徑
	if req.State == "" {
		saved, err = p.addPersonal(userID, in)
	} else {
		saved, err = p.updatePersonal(userID, req.State, in)
	}
	if err != nil {
		var fe *fieldError
		switch {
		case errors.As(err, &fe) && slices.Contains([]string{"title", "url", "description", "icon", "color"}, fe.Field):
			writeJSON(w, http.StatusOK, model.SubmitDialogResponse{Errors: map[string]string{fe.Field: fe.Msg}})
		case errors.As(err, &fe), errors.Is(err, errTileNotFound), errors.Is(err, errPersonalDisabled), errors.Is(err, errConflict):
			writeJSON(w, http.StatusOK, model.SubmitDialogResponse{Error: err.Error()})
		default:
			p.API.LogError("save personal tile from dialog", "err", err.Error())
			writeJSON(w, http.StatusOK, model.SubmitDialogResponse{Error: "儲存失敗，請稍後再試"})
		}
		return
	}

	// 表單關掉後，在原本的頻道留一則只有自己看得到的確認
	_, _, botID := p.snapshot()
	p.API.SendEphemeralPost(userID, &model.Post{
		UserId:    botID,
		ChannelId: req.ChannelId,
		Message:   "已儲存「" + saved.Title + "」到我的捷徑。",
	})
	writeJSON(w, http.StatusOK, model.SubmitDialogResponse{})
}

// MessageHasBeenPosted 讓跟 bot 的私訊變成查詢入口：同事傳什麼就當關鍵字回覆選單。
// 這個 hook 每則訊息都會觸發，所以先用最便宜的條件排除，再查頻道。
func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	cfg, trigger, botID := p.snapshot()
	if botID == "" || post.UserId == botID || post.IsSystemMessage() ||
		post.GetProp("from_bot") == "true" || post.GetProp("from_webhook") == "true" {
		return
	}
	ch, appErr := p.API.GetChannel(post.ChannelId)
	if appErr != nil || ch.Type != model.ChannelTypeDirect || ch.Name != model.GetDMNameFromIds(post.UserId, botID) {
		return
	}

	query := strings.TrimSpace(post.Message)
	if slices.Contains(showAllWords, strings.ToLower(query)) {
		query = ""
	}
	reply := p.buildMenu(post.UserId, query, cfg, trigger, true)
	reply.UserId = botID
	reply.ChannelId = post.ChannelId
	reply.RootId = post.RootId
	if _, appErr := p.API.CreatePost(reply); appErr != nil {
		p.API.LogError("reply in bot DM", "err", appErr.Error())
	}
}

// pinForUser 建立與 bot 的私訊並加到使用者每個團隊的「我的最愛」；已經在最愛裡就回傳 true、什麼都不改。
// 私訊在每個團隊的側邊欄都會出現，「我的最愛」卻是各團隊分開記，所以每個團隊都要加。
func (p *Plugin) pinForUser(userID string) (bool, error) {
	cfg, trigger, botID := p.snapshot()
	dm, appErr := p.API.GetDirectChannel(userID, botID)
	if appErr != nil {
		return false, appErr
	}
	teams, appErr := p.API.GetTeamsForUser(userID)
	if appErr != nil {
		return false, appErr
	}

	type change struct {
		teamID     string
		categories []*model.SidebarCategoryWithChannels
	}
	var changes []change
	for _, team := range teams {
		ordered, appErr := p.API.GetChannelSidebarCategories(userID, team.Id)
		if appErr != nil {
			return false, appErr
		}
		if changed := moveToFavorites(ordered.Categories, dm.Id); len(changed) > 0 {
			changes = append(changes, change{team.Id, changed})
		}
	}
	if len(changes) == 0 {
		return true, nil
	}

	// 先送一則歡迎選單：私訊裡有訊息，Mattermost 才會把它顯示在側邊欄
	welcome := p.buildMenu(userID, "", cfg, trigger, true)
	welcome.Message = "這裡是快速查詢的固定入口，已幫你加到「我的最愛」。\n" +
		"直接輸入關鍵字（例如「報價」）就會列出符合的項目；輸入「全部」看完整選單。按下項目會跳出連結，再點一下開啟。\n\n" + welcome.Message
	welcome.UserId = botID
	welcome.ChannelId = dm.Id
	if _, appErr := p.API.CreatePost(welcome); appErr != nil {
		return false, appErr
	}

	for _, c := range changes {
		if _, appErr := p.API.UpdateChannelSidebarCategories(userID, c.teamID, c.categories); appErr != nil {
			return false, appErr
		}
	}
	return false, nil
}

// moveToFavorites 把頻道放到「我的最愛」最上面，回傳需要送出更新的分類。
// Mattermost 規定頻道換分類時，來源與目的分類要一起更新，所以也要從自訂分類裡拿掉。
func moveToFavorites(categories []*model.SidebarCategoryWithChannels, channelID string) []*model.SidebarCategoryWithChannels {
	idx := slices.IndexFunc(categories, func(c *model.SidebarCategoryWithChannels) bool {
		return c.Type == model.SidebarCategoryFavorites
	})
	if idx < 0 || slices.Contains(categories[idx].Channels, channelID) {
		return nil
	}
	favorites := categories[idx]

	var changed []*model.SidebarCategoryWithChannels
	for _, c := range categories {
		// 私訊分類的順序不存資料庫，不用動它
		if c == favorites || c.Type == model.SidebarCategoryDirectMessages {
			continue
		}
		if i := slices.Index(c.Channels, channelID); i >= 0 {
			c.Channels = slices.Delete(c.Channels, i, i+1)
			changed = append(changed, c)
		}
	}
	favorites.Channels = append([]string{channelID}, favorites.Channels...)
	return append(changed, favorites)
}

func (p *Plugin) handlePin(w http.ResponseWriter, r *http.Request) {
	already, err := p.pinForUser(userIDFrom(r))
	if err != nil {
		p.API.LogError("pin bot DM", "err", err.Error())
		writeError(w, http.StatusInternalServerError, "加入失敗，可以自己長按「快速查詢助手」的私訊 →「加入我的最愛」。")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"already": already})
}
