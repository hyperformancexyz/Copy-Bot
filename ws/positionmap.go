package ws

import (
	"fmt"
	"math"
	"strings"
	"sync"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/itay747/hyperformance/models"
)

type MarginInfo struct {
	Coin               string
	CopyMarginPercent  float64
	PasteMarginPercent float64
}

type PositionMap struct {
	mutex      sync.Mutex
	positions  map[string][]hl.AssetPosition
	assetData  map[string]map[string]models.UserAssetData
	marginData map[string]MarginInfo
}

func NewPositionMap() *PositionMap {
	return &PositionMap{
		positions:  make(map[string][]hl.AssetPosition),
		assetData:  make(map[string]map[string]models.UserAssetData),
		marginData: make(map[string]MarginInfo),
	}
}

func (pm *PositionMap) OnWebData2Frame(address string, newPositions []hl.AssetPosition) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.positions[address] = newPositions
	pm.recomputeMarginUsage()
}

func (pm *PositionMap) OnUserAssetData(address string, userAssetData models.UserAssetData) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	addrMap, ok := pm.assetData[address]
	if !ok {
		addrMap = make(map[string]models.UserAssetData)
		pm.assetData[address] = addrMap
	}
	key := strings.ToUpper(userAssetData.Coin)

	addrMap[key] = userAssetData
	pm.recomputeMarginUsage()
}
func (pm *PositionMap) ArrowForCoin(rawCoin string) string {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	coin := strings.ToUpper(rawCoin)
	data, found := pm.marginData[coin]
	if !found {
		return "N/A"
	}
	diff := data.PasteMarginPercent - data.CopyMarginPercent
	s := ""
	if diff > 0 {
		s = "+"
	}
	return fmt.Sprintf("<=== %.2f%% == [%s%.2f%%] == %.2f%% ===>",
		data.CopyMarginPercent, s, math.Abs(diff), data.PasteMarginPercent)
}

func (pm *PositionMap) recomputeMarginUsage() {
	copyPos := pm.positions["copy"]
	pastePos := pm.positions["paste"]
	copyIdx := buildPositionIndex(copyPos)
	pasteIdx := buildPositionIndex(pastePos)
	unionCoins := make(map[string]struct{})
	for c := range copyIdx {
		unionCoins[c] = struct{}{}
	}
	for c := range pasteIdx {
		unionCoins[c] = struct{}{}
	}
	updated := make(map[string]MarginInfo, len(unionCoins))
	for coin := range unionCoins {
		copyVal := computeMarginPercent(copyIdx[coin], "copy", pm.assetData)
		pasteVal := computeMarginPercent(pasteIdx[coin], "paste", pm.assetData)
		updated[coin] = MarginInfo{Coin: coin, CopyMarginPercent: copyVal, PasteMarginPercent: pasteVal}
	}
	pm.marginData = updated
}

func buildPositionIndex(positions []hl.AssetPosition) map[string]hl.Position {
	index := make(map[string]hl.Position, len(positions))
	for _, ap := range positions {
		c := strings.ToUpper(ap.Position.Coin)
		index[c] = ap.Position
	}
	return index
}

func computeMarginPercent(pos hl.Position, address string, data map[string]map[string]models.UserAssetData) float64 {
	if math.Abs(pos.Szi) < 1e-9 {
		return 0
	}
	addrMap, hasAddr := data[address]
	if !hasAddr {
		return 0
	}
	coinKey := strings.ToUpper(pos.Coin)
	details, found := addrMap[coinKey]
	if !found {
		return 0
	}
	if len(details.AvailableToTrade) < 2 {
		return 0
	}
	availableToTrade := (details.AvailableToTrade[0] + details.AvailableToTrade[1]) / 2
	if availableToTrade <= 0 {
		return 0
	}
	u := (pos.MarginUsed / availableToTrade) * 100
	if u < 0 {
		u = 0
	}
	return u
}
