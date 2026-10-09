package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeTile(t *testing.T) {
	got, err := normalizeTile(Tile{Title: " 手冊 ", URL: " https://example.com/a ", Color: "#1D9E75", Icon: " 📘 "})
	require.NoError(t, err)
	require.Equal(t, Tile{Title: "手冊", URL: "https://example.com/a", Color: "#1D9E75", Icon: "📘"}, got)

	_, err = normalizeTile(Tile{Title: "FAQ", URL: "http://example.com/b"})
	require.NoError(t, err)

	bad := map[string]struct {
		tile  Tile
		field string
	}{
		"no title":     {Tile{URL: "https://x.com"}, "title"},
		"long title":   {Tile{Title: strings.Repeat("字", maxTitleLen+1), URL: "https://x.com"}, "title"},
		"js url":       {Tile{Title: "x", URL: "javascript:alert(1)"}, "url"},
		"no host":      {Tile{Title: "x", URL: "https://"}, "url"},
		"no scheme":    {Tile{Title: "x", URL: "example.com"}, "url"},
		"long desc":    {Tile{Title: "x", URL: "https://x.com", Description: strings.Repeat("字", maxDescriptionLen+1)}, "description"},
		"icon is text": {Tile{Title: "x", URL: "https://x.com", Icon: "這不是一個圖示而是一句話"}, "icon"},
		"bad color":    {Tile{Title: "x", URL: "https://x.com", Color: "red"}, "color"},
	}
	for name, c := range bad {
		_, err := normalizeTile(c.tile)
		var fe *fieldError
		if !errors.As(err, &fe) || fe.Field != c.field {
			t.Errorf("%s: want error on %q, got %v", name, c.field, err)
		}
	}
}

func TestListOps(t *testing.T) {
	list := func() []Tile { return []Tile{{ID: "a"}, {ID: "b"}, {ID: "c"}} }
	ids := func(tiles []Tile) string {
		s := ""
		for _, t := range tiles {
			s += t.ID
		}
		return s
	}

	got, err := moveTile(list(), "b", -1)
	require.NoError(t, err)
	require.Equal(t, "bac", ids(got))
	got, _ = moveTile(list(), "a", -1)
	require.Equal(t, "abc", ids(got), "已在最前面就不動")
	got, _ = moveTile(list(), "c", 1)
	require.Equal(t, "abc", ids(got), "已在最後面就不動")

	got, err = removeTile(list(), "b")
	require.NoError(t, err)
	require.Equal(t, "ac", ids(got))

	got, err = replaceTile(list(), Tile{ID: "b", Title: "新"})
	require.NoError(t, err)
	require.Equal(t, "新", got[1].Title)

	_, err = removeTile(list(), "x")
	require.ErrorIs(t, err, errTileNotFound)
	_, err = moveTile(list(), "x", 1)
	require.ErrorIs(t, err, errTileNotFound)
	_, err = replaceTile(list(), Tile{ID: "x"})
	require.ErrorIs(t, err, errTileNotFound)
}

func TestFilterTiles(t *testing.T) {
	tiles := []Tile{
		{Title: "使用手冊", Description: "安裝與設定"},
		{Title: "常見問題", Description: "FAQ 與故障排除"},
	}
	if got := filterTiles(tiles, "使用"); len(got) != 1 || got[0].Title != "使用手冊" {
		t.Errorf("title filter: %v", got)
	}
	if got := filterTiles(tiles, "faq"); len(got) != 1 {
		t.Errorf("description match failed: %v", got)
	}
	if got := filterTiles(tiles, ""); len(got) != 2 {
		t.Errorf("empty query should return all")
	}
	if got := filterTiles(tiles, "設定"); len(got) != 1 {
		t.Errorf("chinese match failed")
	}
}

func TestNormalizeTrigger(t *testing.T) {
	cases := map[string]string{"": "tiles", "/Tiles": "tiles", "tiles q": "tiles", "sq": "sq"}
	for in, want := range cases {
		if got := normalizeTrigger(in); got != want {
			t.Errorf("normalizeTrigger(%q)=%q want %q", in, got, want)
		}
	}
}
