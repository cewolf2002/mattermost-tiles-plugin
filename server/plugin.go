package main

import (
	"net/http"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type configuration struct {
	Trigger          string
	IntroText        string
	MaxPersonalTiles int
	AppBarIconColor  string
}

type Plugin struct {
	plugin.MattermostPlugin

	mu        sync.RWMutex
	cfg       configuration
	trigger   string // 目前已註冊的斜線指令
	botUserID string

	routerOnce sync.Once
	router     *http.ServeMux
}

// pluginID 要跟 plugin.json 的 id 一致：按鈕與對話框的回呼網址是用它組出來的。
const pluginID = "tw.com.cewolf.tiles"

const defaultIntro = "#### 快速查詢"

// defaultIconColor 要跟 plugin.json 的預設值、webapp 的 DEFAULT_ICON_COLOR 一致。
const defaultIconColor = "#E8590C"

// iconColorEvent 是通知前端圖示換色的 WebSocket 事件；前端收到的事件名稱是 custom_<pluginID>_icon_color。
const iconColorEvent = "icon_color"

func normalizeTrigger(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "/"))
	if s == "" || strings.ContainsAny(s, " \t") {
		return "tiles"
	}
	return strings.ToLower(s)
}

// normalizeIconColor 把設定值整理成大寫 #RRGGBB；空白或格式不對（例如手動改 config.json 打錯）一律退回預設色。
func normalizeIconColor(s string) string {
	s = strings.TrimSpace(s)
	if !colorRe.MatchString(s) {
		return defaultIconColor
	}
	return strings.ToUpper(s)
}

func (p *Plugin) OnActivate() error {
	botID, err := p.API.EnsureBotUser(&model.Bot{
		Username:    "tiles-bot",
		DisplayName: "快速查詢助手",
		Description: "回覆快速查詢選單",
	})
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.botUserID = botID
	p.mu.Unlock()
	return p.OnConfigurationChange()
}

func (p *Plugin) OnConfigurationChange() error {
	var cfg configuration
	if err := p.API.LoadPluginConfiguration(&cfg); err != nil {
		return err
	}

	p.mu.Lock()
	oldColor := normalizeIconColor(p.cfg.AppBarIconColor)
	p.cfg = cfg
	oldTrigger := p.trigger
	newTrigger := normalizeTrigger(cfg.Trigger)
	activated := p.botUserID != ""
	p.mu.Unlock()

	// Mattermost 在 OnActivate 之前就會先呼叫一次；指令要等 OnActivate 再呼叫時才註冊
	if !activated {
		return nil
	}

	// 已開著的頁面不會重新載入外掛，要主動通知才會換色；外掛剛啟用時前端本來就會重新讀，不用通知
	if newColor := normalizeIconColor(cfg.AppBarIconColor); newColor != oldColor {
		p.API.PublishWebSocketEvent(iconColorEvent, map[string]any{"color": newColor}, &model.WebsocketBroadcast{})
	}

	if oldTrigger != newTrigger {
		if oldTrigger != "" {
			_ = p.API.UnregisterCommand("", oldTrigger)
		}
		if err := p.API.RegisterCommand(&model.Command{
			Trigger:          newTrigger,
			DisplayName:      "快速查詢",
			Description:      "開啟快速查詢選單",
			AutoComplete:     true,
			AutoCompleteDesc: "開啟快速查詢選單，可加關鍵字篩選",
			AutoCompleteHint: "[關鍵字]",
		}); err != nil {
			return err
		}
		p.mu.Lock()
		p.trigger = newTrigger
		p.mu.Unlock()
	}
	return nil
}

// snapshot 一次取出設定、指令名稱與 bot ID，避免讀到設定變更途中的半套狀態。
func (p *Plugin) snapshot() (configuration, string, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cfg, p.trigger, p.botUserID
}
