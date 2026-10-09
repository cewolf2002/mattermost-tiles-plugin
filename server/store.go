package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

// sharedKey 是共用 tile 在外掛 KV store 裡的鍵；整份清單存成一筆，陣列順序就是顯示順序。
const sharedKey = "shared_tiles"

// 寫入衝突時最多重試幾次；同時編輯的人很少，幾次就夠。
const maxUpdateAttempts = 5

var errConflict = errors.New("資料剛好被其他人修改，請重新整理後再試一次")

func decodeTiles(raw []byte) ([]Tile, error) {
	var tiles []Tile
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &tiles); err != nil {
			return nil, fmt.Errorf("decode tiles: %w", err)
		}
	}
	// 回傳空陣列而不是 nil，前端拿到的才會是 [] 而不是 null
	if tiles == nil {
		tiles = []Tile{}
	}
	return tiles, nil
}

func (p *Plugin) loadTiles(key string) ([]Tile, error) {
	raw, appErr := p.API.KVGet(key)
	if appErr != nil {
		return nil, appErr
	}
	return decodeTiles(raw)
}

// updateTiles 讀出清單交給 fn 修改，再用 compare-and-set 寫回。
// 兩個人同時存檔時，後寫的一方會因為內容已經變了而寫入失敗，於是重讀最新清單再套用一次，
// 不會把對方剛存的修改蓋掉。
func (p *Plugin) updateTiles(key string, fn func([]Tile) ([]Tile, error)) ([]Tile, error) {
	for range maxUpdateAttempts {
		raw, appErr := p.API.KVGet(key)
		if appErr != nil {
			return nil, appErr
		}
		tiles, err := decodeTiles(raw)
		if err != nil {
			return nil, err
		}
		next, err := fn(tiles)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(next)
		if err != nil {
			return nil, err
		}
		// raw 是 nil 代表這個鍵還不存在，此時 KVCompareAndSet 只在鍵不存在時才寫入
		ok, appErr := p.API.KVCompareAndSet(key, raw, data)
		if appErr != nil {
			return nil, appErr
		}
		if ok {
			return next, nil
		}
	}
	return nil, errConflict
}
