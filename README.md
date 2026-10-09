# 快速查詢（Mattermost tile 外掛）

在 Mattermost 頻道加入可自訂的 tile 連結選單，內容不限特定產品或用途。

- 電腦版／網頁版：頻道上方的燈泡按鈕 → 右側 tile 面板（可篩選）
- 手機 app：把「快速查詢助手」的私訊加到「我的最愛」當固定入口，或在任何頻道輸入 `/tiles [關鍵字]`

## 下載與安裝

1. 到 [Releases](https://github.com/cewolf2002/mattermost-tiles-plugin/releases) 下載 `tw.com.cewolf.tiles-<版本>.tar.gz`
2. System Console → 外掛管理 → 上傳外掛 → 選擇檔案 → 啟用

- 需要 Mattermost 10.0 以上，已在 10.11 Team Edition（免費版）實測；伺服器支援 linux amd64／arm64。
- 上傳按鈕是灰的：伺服器設定要開啟 `PluginSettings.EnableUploads`。

## 手機 app

手機 app 不會執行外掛的網頁程式，右側面板只在電腦版／網頁版出現。手機上改用 app 本來就支援的 bot 私訊、
訊息按鈕與互動式表單：

- **固定入口**：在面板底部或 `/tiles` 回覆按「📌 加到我的最愛」，跟「快速查詢助手」的私訊會出現在側邊欄的
  「我的最愛」（側邊欄分類存在伺服器上，手機會同步）。在私訊裡直接打關鍵字就回覆符合的 tile，輸入「全部」看完整選單。
- **斜線指令**：任何頻道輸入 `/tiles [關鍵字]`，回覆只有自己看得到（指令名稱可在設定改）。
- **每個 tile 是一顆按鈕**：Mattermost 的訊息按鈕只能回呼外掛、不能直接開網址，所以按下後 bot 會回一則
  只有自己看得到的連結（標題大小，好點），再點一下開啟。
- **我的捷徑**：回覆底下的「＋ 新增我的捷徑」「管理我的捷徑」會跳出表單，手機上也能新增、編輯、刪除。
- 不能像 Agents 那樣在側邊欄加專屬入口：那是官方直接寫進手機 app 的功能，外掛沒有對應的擴充點。

## 管理 tile

- 系統管理員在右側面板按鉛筆按鈕進入管理模式，可以新增、編輯、上下移動、刪除。
- 每個 tile 可設「顯示對象」：不選＝所有人看得到；選了頻道，就只有其中任一頻道的成員看得到。
  頻道清單列出所有公開頻道，私人頻道只列出管理員自己有加入的（外掛 API 沒有列出全部私人頻道的方法）。
- 顯示對象只是「顯示」上的區分，不是權限控管：公開頻道任何人都能自己加入；tile 只是連結，
  拿到網址的人照樣打得開，內容敏感時要在目標網站那邊設權限。
- 資料存在外掛的 KV store（Mattermost 資料庫裡），跟著資料庫一起備份，不在 `config.json`。

## 我的捷徑

- 每位同事可以在面板的「我的捷徑」自己新增 tile，只有自己看得到，別人讀不到也改不到。
- 每人上限由 System Console 的「每人「我的捷徑」上限」設定（預設 2，最多 20）；設 0 關閉這個功能。
  調低上限不會刪掉已存在的捷徑，只是不能再新增。
- 斜線指令的回覆也會列出自己的捷徑，並註明「只有你看得到」。

## System Console 設定

外掛 → 快速查詢：斜線指令、說明文字、每人「我的捷徑」上限。

## 結構

| 路徑 | 內容 |
|---|---|
| `plugin.json` | 外掛 manifest 與 System Console 設定欄位 |
| `server/` | Go：斜線指令、bot 私訊回覆、訊息按鈕與表單、面板用的 REST API（`/plugins/tw.com.cewolf.tiles/api/v1/...`）、KV 存取 |
| `webapp/dist/main.js` | 右側面板，免編譯（直接使用 Mattermost 提供的 React） |

## 重新編譯

建議用 `./build.sh`：只需要 Docker，會在容器裡跑 `go vet`、`go test`、編譯 amd64／arm64 並打包，
產出 `tw.com.cewolf.tiles-<版本>.tar.gz`，到 System Console → 外掛管理 上傳即可。

本機有 Go 的話也可以手動：

```bash
cd server
for a in amd64 arm64; do
  CGO_ENABLED=0 GOOS=linux GOARCH=$a go build -trimpath -ldflags "-s -w" -o dist/plugin-linux-$a .
done
go test ./...
cd .. && mkdir -p pkg/tw.com.cewolf.tiles && \
  cp -r plugin.json server webapp pkg/tw.com.cewolf.tiles/ && \
  COPYFILE_DISABLE=1 tar -czf tw.com.cewolf.tiles-2.1.0.tar.gz -C pkg tw.com.cewolf.tiles
```

在 macOS 手動打包一定要加 `COPYFILE_DISABLE=1`：macOS 的 tar 會把檔案的延伸屬性另存成 `._*` 項目塞進壓縮檔，
Mattermost 解開後會多出這些項目，上傳時就報「找不到資訊清單」（manifest not found）。

SDK 使用 `github.com/mattermost/mattermost/server/public v0.1.17`（Go 1.24）。
改版時記得同步調整 `plugin.json` 的 `version`。

## 授權

Apache License 2.0，條款見 [LICENSE](LICENSE)。Copyright 2026 cewolf2002。
