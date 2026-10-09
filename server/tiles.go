package main

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

// Tile 是右側面板上的一格，也是斜線指令回覆裡的一則附件。
type Tile struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
	Icon        string `json:"icon,omitempty"`  // 一個 emoji，例如 "📘"
	Color       string `json:"color,omitempty"` // #RRGGBB，tile 上緣的強調色
	// Channels 是顯示對象（頻道 ID）：空的代表所有人都看得到，有值時只有其中任一頻道的成員看得到
	Channels []string `json:"channels,omitempty"`
}

const (
	maxTitleLen       = 50
	maxDescriptionLen = 100
	// 國旗、膚色等組合 emoji 由多個字元組成，留一點空間
	maxIconLen     = 8
	maxURLLen      = 2000
	maxSharedTiles = 100
	maxAudience    = 20
)

var (
	colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

	errTileNotFound = errors.New("找不到這個項目，可能已被刪除，請重新整理")
)

// fieldError 帶著欄位名稱，讓網頁表單能把錯誤訊息顯示在對應欄位旁邊。
type fieldError struct {
	Field string
	Msg   string
}

func (e *fieldError) Error() string { return e.Msg }

// normalizeTile 去掉前後空白並檢查欄位；ID 由呼叫端決定，這裡不動。
// 錯誤訊息直接顯示給同事看，所以用中文並指出是哪個欄位。
func normalizeTile(t Tile) (Tile, error) {
	t.Title = strings.TrimSpace(t.Title)
	t.Description = strings.TrimSpace(t.Description)
	t.URL = strings.TrimSpace(t.URL)
	t.Icon = strings.TrimSpace(t.Icon)
	t.Color = strings.TrimSpace(t.Color)

	if t.Title == "" {
		return t, &fieldError{"title", "請填寫標題"}
	}
	if utf8.RuneCountInString(t.Title) > maxTitleLen {
		return t, &fieldError{"title", fmt.Sprintf("標題最多 %d 個字", maxTitleLen)}
	}
	if len(t.URL) > maxURLLen {
		return t, &fieldError{"url", "網址太長"}
	}
	// 只收 http/https，擋掉 javascript: 之類點了會在 Mattermost 頁面裡執行程式的網址
	u, err := url.Parse(t.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return t, &fieldError{"url", "網址必須是 http:// 或 https:// 開頭的完整網址"}
	}
	if utf8.RuneCountInString(t.Description) > maxDescriptionLen {
		return t, &fieldError{"description", fmt.Sprintf("說明最多 %d 個字", maxDescriptionLen)}
	}
	if utf8.RuneCountInString(t.Icon) > maxIconLen {
		return t, &fieldError{"icon", "圖示請填一個 emoji"}
	}
	if t.Color != "" && !colorRe.MatchString(t.Color) {
		return t, &fieldError{"color", "顏色必須是 #RRGGBB 格式"}
	}

	// 去掉重複；不是 Mattermost ID 格式的值直接擋掉，頻道是否真的存在由呼叫端再查
	var channels []string
	for _, id := range t.Channels {
		id = strings.TrimSpace(id)
		if id == "" || slices.Contains(channels, id) {
			continue
		}
		if !model.IsValidId(id) {
			return t, &fieldError{"channels", "顯示對象裡有無效的頻道"}
		}
		channels = append(channels, id)
	}
	if len(channels) > maxAudience {
		return t, &fieldError{"channels", fmt.Sprintf("顯示對象最多選 %d 個頻道", maxAudience)}
	}
	t.Channels = channels
	return t, nil
}

func indexOfTile(tiles []Tile, id string) int {
	for i, t := range tiles {
		if t.ID == id {
			return i
		}
	}
	return -1
}

// replaceTile 用新內容取代同 ID 的項目，位置不變。
func replaceTile(tiles []Tile, t Tile) ([]Tile, error) {
	i := indexOfTile(tiles, t.ID)
	if i < 0 {
		return nil, errTileNotFound
	}
	tiles[i] = t
	return tiles, nil
}

func removeTile(tiles []Tile, id string) ([]Tile, error) {
	i := indexOfTile(tiles, id)
	if i < 0 {
		return nil, errTileNotFound
	}
	return append(tiles[:i], tiles[i+1:]...), nil
}

// moveTile 跟前一個（offset -1）或後一個（offset 1）交換位置；已在最前或最後就維持原樣。
func moveTile(tiles []Tile, id string, offset int) ([]Tile, error) {
	i := indexOfTile(tiles, id)
	if i < 0 {
		return nil, errTileNotFound
	}
	j := i + offset
	if j < 0 || j >= len(tiles) {
		return tiles, nil
	}
	tiles[i], tiles[j] = tiles[j], tiles[i]
	return tiles, nil
}

// filterTiles 保留標題或說明包含所有關鍵字的項目（不分大小寫）。
func filterTiles(tiles []Tile, query string) []Tile {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return tiles
	}
	out := make([]Tile, 0, len(tiles))
	for _, t := range tiles {
		hay := strings.ToLower(t.Title + " " + t.Description)
		match := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				match = false
				break
			}
		}
		if match {
			out = append(out, t)
		}
	}
	return out
}

func tileHeading(t Tile) string {
	if t.Icon == "" {
		return t.Title
	}
	return t.Icon + " " + t.Title
}

// buildAttachments 把 tile 轉成訊息附件；手機 app 也看得懂附件，所以斜線指令的回覆在手機上可用。
func buildAttachments(tiles []Tile) []*model.SlackAttachment {
	atts := make([]*model.SlackAttachment, 0, len(tiles))
	for _, t := range tiles {
		color := t.Color
		if color == "" {
			color = "#1C58D9"
		}
		atts = append(atts, &model.SlackAttachment{
			Color:     color,
			Title:     tileHeading(t),
			TitleLink: t.URL,
			Text:      t.Description,
		})
	}
	return atts
}
