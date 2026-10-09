#!/bin/sh
# 用 golang 容器跑測試、編譯並打包外掛：本機不用裝 Go，只要有 Docker。
# 產出 tw.com.cewolf.tiles-<版本>.tar.gz，放在這個目錄（已被 .gitignore 排除），到 System Console 上傳即可。
#
# 組裝與打包都在容器內的 /tmp 完成，原因有兩個：
# - macOS 的 tar 會把檔案的延伸屬性另存成 ._* 項目塞進壓縮檔，Mattermost 上傳時會報找不到 plugin.json
# - 直接對 macOS 掛進容器的目錄打包，偶爾會遇到「file changed as we read it」而失敗
set -e
cd "$(dirname "$0")"

ID=tw.com.cewolf.tiles
VERSION=$(sed -n 's/^  "version": "\(.*\)",$/\1/p' plugin.json)
if [ -z "$VERSION" ]; then
  echo "讀不到 plugin.json 的 version" >&2
  exit 1
fi
OUT="$ID-$VERSION.tar.gz"

docker run --rm -v "$PWD":/src -w /src/server -e ID="$ID" -e OUT="$OUT" golang:1.24 sh -ec '
  go vet ./...
  go test ./...
  P=/tmp/pkg/$ID
  mkdir -p "$P/server/dist" "$P/webapp/dist"
  for a in amd64 arm64; do
    CGO_ENABLED=0 GOOS=linux GOARCH=$a go build -buildvcs=false -trimpath -ldflags "-s -w" -o "$P/server/dist/plugin-linux-$a" .
  done
  cp /src/plugin.json "$P/"
  # Apache-2.0 要求散布時附上授權條款與 NOTICE
  cp /src/LICENSE /src/NOTICE "$P/"
  cp /src/webapp/dist/main.js "$P/webapp/dist/"
  tar -czf "/tmp/$OUT" -C /tmp/pkg "$ID"
  cp "/tmp/$OUT" "/src/$OUT"
'
echo "完成：$OUT"
