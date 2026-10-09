package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCommandSendsEphemeralTiles(t *testing.T) {
	e := setup(t, configuration{Trigger: "/tiles", IntroText: "hi"})
	seedShared(t, e.p,
		Tile{Icon: "📘", Title: "使用手冊", Description: "安裝與設定", URL: "https://a.example", Color: "#1D9E75"},
		Tile{Title: "常見問題", Description: "FAQ 與故障排除", URL: "https://b.example"},
	)

	var sent *model.Post
	e.api.On("SendEphemeralPost", "u1", mock.Anything).Run(func(a mock.Arguments) {
		sent = a.Get(1).(*model.Post)
	}).Return(&model.Post{})

	_, appErr := e.p.ExecuteCommand(nil, &model.CommandArgs{UserId: "u1", ChannelId: "c1", Command: "/tiles faq"})
	require.Nil(t, appErr)
	require.NotNil(t, sent)
	require.Equal(t, "bot123", sent.UserId)
	require.Equal(t, "c1", sent.ChannelId)
	atts := tileAttachments(sent)
	require.Len(t, atts, 1)
	require.Equal(t, "常見問題", atts[0].Title)
	require.Equal(t, "https://b.example", atts[0].TitleLink)
	e.api.AssertExpectations(t)
}

func TestCommandWithoutTilesExplainsWhy(t *testing.T) {
	e := setup(t, configuration{})

	var sent *model.Post
	e.api.On("SendEphemeralPost", "u1", mock.Anything).Run(func(a mock.Arguments) {
		sent = a.Get(1).(*model.Post)
	}).Return(&model.Post{})

	_, appErr := e.p.ExecuteCommand(nil, &model.CommandArgs{UserId: "u1", ChannelId: "c1", Command: "/tiles"})
	require.Nil(t, appErr)
	require.Contains(t, sent.Message, defaultIntro)
	require.Contains(t, sent.Message, "目前還沒有任何項目")
	require.Empty(t, tileAttachments(sent))
}
