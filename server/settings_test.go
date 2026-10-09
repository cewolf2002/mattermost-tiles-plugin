package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNormalizeIconColor(t *testing.T) {
	require.Equal(t, defaultIconColor, normalizeIconColor(""))
	require.Equal(t, defaultIconColor, normalizeIconColor("red"), "只接受 #RRGGBB")
	require.Equal(t, defaultIconColor, normalizeIconColor("#FFF"), "三碼簡寫也不接受，前後端規則要一致")
	require.Equal(t, "#1C58D9", normalizeIconColor(" #1c58d9 "))
}

func TestSettingsReturnsIconColor(t *testing.T) {
	e := setup(t, configuration{AppBarIconColor: "#1c58d9"})
	w := doRequest(t, e.p, "u1", http.MethodGet, "/api/v1/settings", nil)
	require.Equal(t, http.StatusOK, w.Code, "一般同事也要讀得到，不然右側圖示沒辦法套用顏色")
	require.Equal(t, "#1C58D9", decode[settingsResponse](t, w).IconColor)

	w = doRequest(t, e.p, "", http.MethodGet, "/api/v1/settings", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestIconColorChangeNotifiesOpenPages(t *testing.T) {
	e := setup(t, configuration{})
	e.api.On("PublishWebSocketEvent", iconColorEvent, map[string]any{"color": "#7F56D9"}, mock.Anything).Return().Once()

	// 只是大小寫跟預設色不同，其實是同一個顏色，不該通知
	e.cfg.AppBarIconColor = "#e8590c"
	require.NoError(t, e.p.OnConfigurationChange())
	e.api.AssertNotCalled(t, "PublishWebSocketEvent", mock.Anything, mock.Anything, mock.Anything)

	e.cfg.AppBarIconColor = "#7f56d9"
	require.NoError(t, e.p.OnConfigurationChange())
	e.api.AssertNumberOfCalls(t, "PublishWebSocketEvent", 1)

	// 改別的設定、顏色沒動，也不該再通知
	e.cfg.IntroText = "hi"
	require.NoError(t, e.p.OnConfigurationChange())
	e.api.AssertNumberOfCalls(t, "PublishWebSocketEvent", 1)
}
