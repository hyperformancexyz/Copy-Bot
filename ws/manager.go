package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/bitfield/script"
	"github.com/itay747/hyperformance/config"
	"github.com/itay747/hyperformance/models"
)

// Manager is the main struct that manages the Hyperformance bot
type Manager struct {
	AddLogFunc func(address, message string)

	Client *hl.Hyperliquid

	//ArchEngine ArchEngine
	AloEngine *AloEngine
	IocEngine *IocEngine

	CopyAddress  string
	PasteAddress string

	CopyWd2Chan  chan *models.WebData2Message
	PasteWd2Chan chan *models.WebData2Message

	UsedPasteWd2       map[int64]time.Time
	UsedCopyAloCreates map[int64]time.Time

	CopyWd2  *models.WebData2Message
	PasteWd2 *models.WebData2Message

	OrderUpdatesChan chan *models.OrderMessage

	L2BookSnapshotChan chan *models.L2BookSnapshotMessage

	AllowedSymbols         []string
	MetaMap                map[string]hl.AssetInfo
	AssetDetailsStore      sync.Map
	AssetCtxStore          sync.Map
	CopyWebSocketReady     bool
	PasteWebSocketReady    bool
	CopyAssetDataReady     bool
	PasteAssetDataReady    bool
	AssetSubscriptionCount sync.Map
	lastCopyWd2ChTime      time.Time
	lastPasteWd2ChTime     time.Time
	subNeeded              int32
	subConfirmChan         chan struct{}
	logStore               sync.Map
	CoinRiskMap            map[string]float64

	i int64
}

func (m *Manager) defaultAddLog(address, message string) {
	now := time.Now().Format("15:04:05")
	line := "[" + now + "] " + message
	store, ok := m.logStore.Load(strings.ToLower(address))
	if !ok {
		return
	}
	store.(*models.RingBuffer).Push(line)
}
func loadInternalConfig() (*config.HyperformanceConfig, error) {
	var loadedConfiguration config.HyperformanceConfig
	configBytes, loadErr := script.File("../config.json").Bytes()
	if loadErr != nil {
		return nil, loadErr
	}
	_ = json.Unmarshal(configBytes, &loadedConfiguration)
	return &loadedConfiguration, nil
}

// NewManager creates a new Manager instance
func NewManager(ctx context.Context) *Manager {

	botConfig, configErr := loadInternalConfig()
	if configErr != nil {
		panic(configErr)
	}
	managerConfig, configErr := config.LoadConfig()
	if configErr != nil {
		panic(configErr)
	}
	hClient := hl.NewHyperliquid(&hl.HyperliquidClientConfig{
		AccountAddress: botConfig.PasteAddress,
		PrivateKey:     botConfig.SecretKey,
		IsMainnet:      true,
	})
	hClient.CancelAllOrders()
	for coin := range botConfig.CoinRiskMap {
		hClient.ClosePosition(coin)
	}
	metaMapData, metaErr := hClient.BuildMetaMap()
	if metaErr != nil {
		panic(metaErr)
	}
	var permittedAssets []string
	for assetSymbol, virtualLeverage := range managerConfig.CoinRiskMap {
		_, assetFound := metaMapData[assetSymbol]
		if assetFound && virtualLeverage > 0 {
			permittedAssets = append(permittedAssets, assetSymbol)
		}
	}
	sort.Slice(permittedAssets, func(i, j int) bool {
		return permittedAssets[i] < permittedAssets[j]
	})
	m := &Manager{
		Client:             hClient,
		CopyAddress:        strings.ToLower(managerConfig.CopyAddress),
		PasteAddress:       strings.ToLower(managerConfig.PasteAddress),
		AllowedSymbols:     permittedAssets,
		MetaMap:            metaMapData,
		logStore:           sync.Map{},
		CoinRiskMap:        managerConfig.CoinRiskMap,
		CopyWd2Chan:        make(chan *models.WebData2Message, 256),
		PasteWd2Chan:       make(chan *models.WebData2Message, 256),
		OrderUpdatesChan:   make(chan *models.OrderMessage, 256),
		UsedPasteWd2:       make(map[int64]time.Time),
		UsedCopyAloCreates: make(map[int64]time.Time),
		i:                  0,
	}
	m.AloEngine = NewAloEngine(ctx, m, !managerConfig.DisableAloEngine)
	m.IocEngine = NewIocEngine(ctx, m, !managerConfig.DisableIocEngine)
	//m.ArchEngine = NewArchEngine(ctx, m)

	m.AddLogFunc = m.defaultAddLog
	m.logStore.Store("copy", models.NewRingBuffer(10000))
	m.logStore.Store("paste", models.NewRingBuffer(10000))
	return m
}

func (m *Manager) StartLogging(ringCapacity int) {
	m.logStore.Store(m.CopyAddress, models.NewRingBuffer(ringCapacity))
	m.logStore.Store(m.PasteAddress, models.NewRingBuffer(ringCapacity))
}

func (m *Manager) AddLog(address, message string) {
	now := time.Now().Format("15:04:05")
	line := "[" + now + "] " + message
	store, ok := m.logStore.Load(strings.ToLower(address))
	if !ok {
		return
	}
	store.(*models.RingBuffer).Push(line)
}

func (m *Manager) GetLogs(address string, requestedCount int) []string {
	stored, ok := m.logStore.Load(strings.ToLower(address))
	if !ok {
		return nil
	}
	return stored.(*models.RingBuffer).LastN(requestedCount)
}
func (m *Manager) handleL2BookSnapshot(l2BookSnapshot *models.L2BookSnapshotMessage) {
	m.L2BookSnapshotChan <- l2BookSnapshot
}

// func (m *Manager) postBroadcastWD2(address string, wd2 *models.WebData2Message) {
// 	copyOk := m.CopyWebSocketReady && m.CopyAssetDataReady
// 	pasteOk := m.PasteWebSocketReady && m.PasteAssetDataReady
// 	if copyOk && pasteOk && !m.alreadySynced {
// 		m.positionsSyncedOnce.Do(func() { close(m.PositionsSyncedChannel) })
// 		m.alreadySynced = true
// 	}

// }

func (manager *Manager) handleOrderUpdates(rawData []byte) {
	var orderUpdatesMessage = &models.OrderMessage{}
	err := json.Unmarshal(rawData, orderUpdatesMessage)
	if err != nil {
		logger.LogErrorf("failed to parse paste orderUpdates")
	}
	// By convention,
	// manager.CopyOrderChan <- &orderUpdatesMessage
	manager.OrderUpdatesChan <- orderUpdatesMessage

}
func (manager *Manager) handleActiveAssetData(userAssetData models.UserAssetData) {
	address := userAssetData.User
	key := address + ":" + userAssetData.Coin
	manager.AssetDetailsStore.Store(key, models.AssetDetails{
		LeverageValue:    userAssetData.Leverage.Value,
		MaxTradeAmounts:  userAssetData.MaxTradeSzs,
		AvailableToTrade: userAssetData.AvailableToTrade,
	})
	v, _ := manager.AssetSubscriptionCount.LoadOrStore(address, 0)
	n := v.(int) + 1
	manager.AssetSubscriptionCount.Store(address, n)
	if strings.EqualFold(address, manager.CopyAddress) {
		if !manager.CopyAssetDataReady && n >= len(manager.AllowedSymbols) {
			manager.CopyAssetDataReady = true
		}
	} else {
		if !manager.PasteAssetDataReady && n >= len(manager.AllowedSymbols) {
			manager.PasteAssetDataReady = true
		}
	}
}

// func (m *Manager) sendIocOrders(orders []hl.Order) {
// 	requests := m.ConvertOrdersToOrderRequests(orders)
// 	resp, err := m.Client.BulkOrders(requests, hl.GroupingNa)

// 	if err != nil || resp.Status != "ok" {
// 		for _, o := range orders {
// 			revert := sideSign(o.Side) * o.Sz
// 			m.iocSzInFlight[o.Coin] -= revert
// 			if math.Abs(m.iocSzInFlight[o.Coin]) < 1e-8 {
// 				m.iocSzInFlight[o.Coin] = 0
// 			}
// 		}
// 		return
// 	}

// 	for i, st := range resp.Response.Data.Statuses {
// 		o := orders[i]
// 		if st.Error != "" || st.Resting.OrderID == 0 {
// 			revert := sideSign(o.Side) * o.Sz
// 			m.iocSzInFlight[o.Coin] -= revert
// 		} else {
// 			filled := sideSign(o.Side) * st.Filled.TotalSz
// 			m.iocSzInFlight[o.Coin] -= filled
// 		}
// 		if math.Abs(m.iocSzInFlight[o.Coin]) < 1e-8 {
// 			m.iocSzInFlight[o.Coin] = 0
// 		}
// 	}
// }

// func (manager *Manager) handleWebData2(webData models.WebData2Message, address string) {
// 	manager.WebData2Chan <- webData

// 	var copyOk, pasteOk bool
// 	nowTime := time.Now()

// 	if strings.EqualFold(address, manager.CopyAddress) {
// 		for _, symbol := range manager.AllowedSymbols {
// 			assetInfo, foundAsset := manager.MetaMap[symbol]
// 			if foundAsset {
// 				manager.AssetCtxStore.Store(symbol, webData.Data.AssetCtxs[assetInfo.AssetID])
// 			}
// 		}
// 		manager.CopyWebSocketReady = true
// 		manager.lastCopyWebData2 = nowTime

// 	} else {
// 		manager.PasteWebSocketReady = true
// 		manager.lastPasteWebData2 = nowTime

// 	}
// 	copyOk = manager.CopyWebSocketReady && manager.CopyAssetDataReady
// 	pasteOk = manager.PasteWebSocketReady && manager.PasteAssetDataReady

// 	if copyOk && pasteOk && !manager.alreadySynced {
// 		logger.LogInfo("[handleWebData2] Both COPY & PASTE ready => positionsSyncedChannel close once")
// 		manager.positionsSyncedOnce.Do(func() { close(manager.PositionsSyncedChannel) })
// 		manager.alreadySynced = true
// 	}
// 	hasBothWD2s := !manager.lastCopyWebData2.IsZero() && !manager.lastPasteWebData2.IsZero()
// 	if !hasBothWD2s {
// 		return
// 	}
// 	nowTime = time.Now()
// 	timeSinceReconcile := time.Since(manager.lastReconcile).Abs().Seconds()

// 	if strings.EqualFold(address, manager.PasteAddress) {
// 		if copyOk && pasteOk && manager.lastReconcile.IsZero() {
// 			orders := manager.GetIocReconcileOrders()
// 			manager.processNewOrders(orders)
// 			manager.lastReconcile = nowTime
// 		} else if timeSinceReconcile > reconcileEvery {
// 			manager.lastReconcile = nowTime
// 			orders := manager.GetIocReconcileOrders()
// 			manager.processNewOrders(orders)
// 		}

//		}
//	}
func (manager *Manager) IsReady() bool {
	copySideReady := manager.CopyWebSocketReady && manager.CopyAssetDataReady
	pasteSideReady := manager.PasteWebSocketReady && manager.PasteAssetDataReady

	return copySideReady && pasteSideReady
}

func (manager *Manager) IsEnabledCoin(symbol string) bool {
	for _, allowedSymbol := range manager.AllowedSymbols {
		if strings.EqualFold(symbol, allowedSymbol) {
			return true
		}
	}
	return false
}

func (manager *Manager) GetMidPrice(symbol string) float64 {
	assetContextValue, contextFound := manager.AssetCtxStore.Load(symbol)
	if assetContextValue == nil || !contextFound {
		logger.LogWarnf("[getMidPrice] no assetctx => %s", symbol)
		return 0
	}
	return assetContextValue.(models.AssetCtx).MidPx
}

func (manager *Manager) getMarketPrice(order hl.Order) float64 {
	midPrice := manager.GetMidPrice(order.Coin)
	if order.Side == "B" {
		midPrice *= 1.08
	} else {
		midPrice *= 0.92
	}
	return manager.SnapPrice(order.Coin, midPrice)
}

func (manager *Manager) SnapPrice(coinSymbol string, originalPrice float64) float64 {
	if originalPrice <= 0 {
		logger.LogWarnf("[snapPrice] <=0 => %s px=%.2f", coinSymbol, originalPrice)
		return originalPrice
	}
	assetInformation, infoFound := manager.MetaMap[coinSymbol]
	if !infoFound {
		logger.LogWarnf("[snapPrice] no meta => %s px=%.2f", coinSymbol, originalPrice)
		return originalPrice
	}
	if originalPrice >= 100000 {
		return math.Round(originalPrice)
	}
	priceDecimals := 6 - assetInformation.SzDecimals
	if priceDecimals < 0 {
		priceDecimals = 0
	}
	floatString := strconv.FormatFloat(originalPrice, 'g', 5, 64)
	parsedFloat, parseErr := strconv.ParseFloat(floatString, 64)
	if parseErr != nil {
		logger.LogWarnf("[snapPrice] parse error => %s px=%.2f err=%v", coinSymbol, originalPrice, parseErr)
		return originalPrice
	}
	powerValue := math.Pow10(priceDecimals)
	snappedPrice := math.Round(parsedFloat*powerValue) / powerValue
	return snappedPrice
}

// scaleSize returns the signed size.
func (manager *Manager) scaleSize(order hl.Order) float64 {
	scaleFactor := manager.deriveScaleFactor(order.Coin, order.Side)
	scaledSize := RoundToPrecision(order.Sz*scaleFactor, manager.Decimals(order.Coin))
	//logger.LogInfof("[scaleSize] => coin=%s side=%s value=%.6f scale=%.6fx", order.Coin, order.Side, order.Sz, scaleFactor)
	return scaledSize
}
func (manager *Manager) scaleSizeWithMultiplier(orderObject hl.Order, multiplier float64) float64 {
	scaleFactor := manager.deriveScaleFactor(orderObject.Coin, orderObject.Side)
	scaledValue := RoundToPrecision(orderObject.Sz*scaleFactor*multiplier, manager.Decimals(orderObject.Coin))
	logger.LogInfof("[scaleSizeWithMultiplier] => c=%s side=%s o=%.6f r=%.6f sf=%.6f sz=%.6f",
		orderObject.Coin,
		orderObject.Side,
		orderObject.Sz,
		multiplier,
		scaleFactor,
		scaledValue,
	)
	return scaledValue
}

func (m *Manager) NewPasteIocCloid(uniqueId int64) string {
	pasteCloidVal, _ := strconv.Atoi(fmt.Sprintf("1337%v", uniqueId))
	nextCloidValue := big.NewInt(int64(pasteCloidVal))
	return hl.IntToHex(nextCloidValue)
}
func (manager *Manager) deriveScaleFactor(symbol, side string) float64 {
	if manager.CopyWd2 == nil || manager.PasteWd2 == nil {
		logger.LogErrorf("[deriveScaleFactor] copy copyWd2 or pasteWd2 was nil")
		logger.LogErrorf("[deriveScaleFactor] copy copyWd2: %#+v", manager.CopyWd2)
		logger.LogErrorf("[deriveScaleFactor] paste pasteWd2: %#+v", manager.PasteWd2)
		return 0
	}
	copyAccountValue := manager.CopyWd2.AccountValue()
	pasteAccountValue := manager.PasteWd2.AccountValue()
	// copyLeverage := copyAsset.LeverageValue
	// pasteLeverage := pasteAsset.LeverageValue

	if copyAccountValue == 0 || pasteAccountValue == 0 {
		logger.LogErrorf("[deriveScaleFactor] copy paste copyAvailable= $%v == 0 || pasteAvailable= $%v == 0", copyAccountValue, pasteAccountValue)
		return 0
	}
	virtualLeverage := manager.CoinRiskMap[symbol]
	pasteScaleFactor := pasteAccountValue
	copyScaleFactor := copyAccountValue
	scaleFactor := pasteScaleFactor / copyScaleFactor
	scaleFactor *= virtualLeverage
	return scaleFactor
}

func (manager *Manager) Decimals(symbol string) int {
	assetDetails, foundOk := manager.MetaMap[symbol]
	if foundOk {
		return assetDetails.SzDecimals
	}
	return 2
}
func Round2(value float64) float64 {
	return RoundToPrecision(value, 2)
}
func RoundToPrecision(value float64, precision int) float64 {
	scale := math.Pow10(precision)
	refined := math.Round(value*scale) / scale
	return refined
}

func (m *Manager) OrderDir(order hl.Order, wd2 *models.WebData2Message) string {
	positions := wd2.PositionsByCoin()
	newSide := "Long"
	if order.Side == "A" {
		newSide = "Short"
	}
	pos, found := positions[order.Coin]
	if !found {
		return "Open " + newSide
	}

	currentSide := "Long"
	if pos.Szi < 0 {
		currentSide = "Short"
	} else if pos.Szi == 0 {
		return "Open " + newSide
	}

	if currentSide == newSide {
		return "Open " + newSide
	}

	if math.Abs(order.OrigSz) <= math.Abs(pos.Szi) {
		return "Close " + currentSide
	}
	return currentSide + " > " + newSide
}
