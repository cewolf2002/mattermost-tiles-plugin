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

func normalizeTrigger(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "/"))
	if s == "" || strings.ContainsAny(s, " \t") {
		return "tiles"
	}
	return strings.ToLower(s)
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
	p.cfg = cfg
	oldTrigger := p.trigger
	newTrigger := normalizeTrigger(cfg.Trigger)
	activated := p.botUserID != ""
	p.mu.Unlock()

	// Mattermost 在 OnActivate 之前就會先呼叫一次；指令要等 OnActivate 再呼叫時才註冊
	if !activated {
		return nil
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
