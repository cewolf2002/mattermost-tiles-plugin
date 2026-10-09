package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

// plugin.json 寫錯時上傳才會失敗，提早在測試抓出來。
func TestManifestIsValidAndMatchesCode(t *testing.T) {
	data, err := os.ReadFile("../plugin.json")
	require.NoError(t, err)
	var m model.Manifest
	require.NoError(t, json.Unmarshal(data, &m))
	require.NoError(t, m.IsValid())
	require.Equal(t, pluginID, m.Id, "按鈕與對話框的回呼網址用 pluginID 組成，必須跟 plugin.json 的 id 一致")
}
