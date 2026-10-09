package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	cfg, trigger, botID := p.snapshot()
	query := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(args.Command), "/"+trigger))

	post := p.buildMenu(args.UserId, query, cfg, trigger, false)
	post.UserId = botID
	post.ChannelId = args.ChannelId
	post.RootId = args.RootId
	p.API.SendEphemeralPost(args.UserId, post)

	return &model.CommandResponse{}, nil
}

// buildMenu 組出選單訊息（說明文字＋使用者看得到且符合關鍵字的 tile＋按鈕）；頻道、發送者由呼叫端填。
// inDM 表示回覆在跟 bot 的私訊裡：提示文字改成直接打字，也不再顯示「加到我的最愛」。
func (p *Plugin) buildMenu(userID, query string, cfg configuration, trigger string, inDM bool) *model.Post {
	text := cfg.IntroText
	if text == "" {
		text = defaultIntro
	}

	tiles, err := p.loadTiles(sharedKey)
	if err != nil {
		p.API.LogError("load shared tiles", "err", err.Error())
		return &model.Post{Message: text + "\n\n:warning: 選單暫時讀取失敗，請稍後再試。"}
	}
	tiles = p.visibleTiles(userID, tiles)
	personal, limit, err := p.loadPersonal(userID)
	if err != nil {
		// 個人捷徑讀不到不影響共用選單，記下來就好
		p.API.LogError("load personal tiles", "err", err.Error())
	}

	matched := filterTiles(tiles, query)
	matchedPersonal := filterTiles(personal, query)
	switch {
	case len(tiles) == 0 && len(personal) == 0:
		text += "\n\n目前還沒有任何項目，請系統管理員到頻道右側的快速查詢面板新增。"
	case len(matched) == 0 && len(matchedPersonal) == 0 && inDM:
		text += "\n\n找不到符合「" + query + "」的項目，輸入「全部」看完整選單。"
	case len(matched) == 0 && len(matchedPersonal) == 0:
		text += "\n\n找不到符合「" + query + "」的項目，輸入 `/" + trigger + "` 看全部。"
	}

	var atts []*model.SlackAttachment
	if a := tileButtons(matched, scopeShared); a != nil {
		atts = append(atts, a)
	}
	if a := tileButtons(matchedPersonal, scopePersonal); a != nil {
		a.Footer = "我的捷徑（只有你看得到）"
		atts = append(atts, a)
	}
	if actions := menuActions(len(personal), limit, inDM); actions != nil {
		atts = append(atts, actions)
	}
	post := &model.Post{Message: text}
	model.ParseSlackAttachment(post, atts)
	return post
}
