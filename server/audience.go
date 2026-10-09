package main

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/mattermost/mattermost/server/public/model"
)

// audienceInfo 描述一個頻道，給管理畫面顯示「顯示對象」與挑選頻道用。
type audienceInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	TeamName    string `json:"team_name,omitempty"`
	Private     bool   `json:"private,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
	Missing     bool   `json:"missing,omitempty"` // 頻道已被永久刪除
}

// adminTile 是管理清單上的一筆：多帶顯示對象的頻道名稱，前端不必再逐一查詢。
type adminTile struct {
	Tile
	Audience []audienceInfo `json:"audience,omitempty"`
}

// visibleTiles 只留下使用者看得到的 tile：沒設顯示對象的所有人都看得到，有設的要是其中任一頻道的成員。
// 這只是顯示上的區分，不是權限控管：tile 只是連結，內容本身的權限要由目標網站管。
func (p *Plugin) visibleTiles(userID string, tiles []Tile) []Tile {
	member := map[string]bool{}
	isMember := func(channelID string) bool {
		if v, ok := member[channelID]; ok {
			return v
		}
		_, appErr := p.API.GetChannelMember(channelID, userID)
		if appErr != nil && appErr.StatusCode != http.StatusNotFound {
			p.API.LogWarn("check channel membership", "err", appErr.Error())
		}
		// 查詢失敗時當作不是成員：寧可少顯示，也不要讓不該看到的人看到
		member[channelID] = appErr == nil
		return member[channelID]
	}

	out := make([]Tile, 0, len(tiles))
	for _, t := range tiles {
		if len(t.Channels) > 0 && !slices.ContainsFunc(t.Channels, isMember) {
			continue
		}
		// 一般畫面用不到顯示對象，也不必讓私人頻道的 ID 外流
		t.Channels = nil
		out = append(out, t)
	}
	return out
}

// describeAudience 把每個 tile 的頻道 ID 換成名稱；同一個頻道只查一次。
func (p *Plugin) describeAudience(tiles []Tile) []adminTile {
	infos := map[string]audienceInfo{}
	teams := map[string]string{}
	describe := func(id string) audienceInfo {
		if info, ok := infos[id]; ok {
			return info
		}
		info := audienceInfo{ID: id}
		if ch, appErr := p.API.GetChannel(id); appErr != nil {
			info.Missing = true
		} else {
			info.DisplayName = ch.DisplayName
			info.TeamName = p.teamName(ch.TeamId, teams)
			info.Private = ch.Type == model.ChannelTypePrivate
			info.Archived = ch.DeleteAt != 0
		}
		infos[id] = info
		return info
	}

	out := make([]adminTile, 0, len(tiles))
	for _, t := range tiles {
		at := adminTile{Tile: t}
		for _, id := range t.Channels {
			at.Audience = append(at.Audience, describe(id))
		}
		out = append(out, at)
	}
	return out
}

func (p *Plugin) teamName(teamID string, cache map[string]string) string {
	if name, ok := cache[teamID]; ok {
		return name
	}
	name := ""
	if team, appErr := p.API.GetTeam(teamID); appErr == nil {
		name = team.DisplayName
	}
	cache[teamID] = name
	return name
}

// checkAudience 確認顯示對象都是還存在的公開或私人頻道，避免存進打錯或私訊頻道的 ID。
func (p *Plugin) checkAudience(channelIDs []string) error {
	for _, id := range channelIDs {
		ch, appErr := p.API.GetChannel(id)
		if appErr != nil {
			return &fieldError{"channels", "找不到選擇的頻道，可能已被刪除，請重新選擇"}
		}
		if ch.Type != model.ChannelTypeOpen && ch.Type != model.ChannelTypePrivate {
			return &fieldError{"channels", "顯示對象只能選公開或私人頻道"}
		}
	}
	return nil
}

// 一次向伺服器要多少個公開頻道；頻道很多時會分頁要到完為止。
const channelPageSize = 200

// channelOptions 列出管理員可以挑的頻道：所有團隊的公開頻道，加上管理員自己有加入的私人頻道。
// 外掛 API 沒有「列出全部私人頻道」的方法，所以沒加入的私人頻道不會出現在清單裡。
func (p *Plugin) channelOptions(userID string) ([]audienceInfo, error) {
	teams, appErr := p.API.GetTeams()
	if appErr != nil {
		return nil, appErr
	}

	seen := map[string]bool{}
	out := []audienceInfo{}
	add := func(team *model.Team, ch *model.Channel) {
		if seen[ch.Id] || ch.DeleteAt != 0 || (ch.Type != model.ChannelTypeOpen && ch.Type != model.ChannelTypePrivate) {
			return
		}
		seen[ch.Id] = true
		out = append(out, audienceInfo{
			ID:          ch.Id,
			DisplayName: ch.DisplayName,
			TeamName:    team.DisplayName,
			Private:     ch.Type == model.ChannelTypePrivate,
		})
	}

	for _, team := range teams {
		for page := 0; ; page++ {
			chs, appErr := p.API.GetPublicChannelsForTeam(team.Id, page, channelPageSize)
			if appErr != nil {
				return nil, appErr
			}
			for _, ch := range chs {
				add(team, ch)
			}
			if len(chs) < channelPageSize {
				break
			}
		}
		// 不是這個團隊成員時會回 404，當作沒有頻道即可
		mine, appErr := p.API.GetChannelsForTeamForUser(team.Id, userID, false)
		if appErr != nil && appErr.StatusCode != http.StatusNotFound {
			return nil, appErr
		}
		for _, ch := range mine {
			add(team, ch)
		}
	}

	slices.SortStableFunc(out, func(a, b audienceInfo) int {
		return cmp.Or(cmp.Compare(a.TeamName, b.TeamName), cmp.Compare(a.DisplayName, b.DisplayName))
	})
	return out, nil
}
