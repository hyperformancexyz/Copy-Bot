package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/gorilla/websocket"
	"github.com/itay747/hyperformance/models"
	"github.com/itay747/hyperformance/utils"
)

var (
	endpoint = "wss://api2.hyperliquid.xyz/ws"
	logger   utils.DualLogger
)

func (manager *Manager) StartCopyTradingSession(ctx context.Context, logChan chan string) {
	logger = *utils.NewDualLogger(logChan)
	logger.LogInfof("[StartCopyTradingSession] single session => copy=%s paste=%s",
		manager.CopyAddress, manager.PasteAddress)

	baseDelay := time.Second
	maxDelay := 16 * time.Second
	attempt := 0

	dialer := &websocket.Dialer{
		HandshakeTimeout:  15 * time.Second,
		ReadBufferSize:    int(8 * 1024 * 1024),
		WriteBufferSize:   int(1024 * 1024),
		EnableCompression: true,
	}

	for {
		select {
		case <-ctx.Done():
			logger.LogWarn("[StartCopyTradingSession] context canceled => stopping loop")
			return
		default:
		}

		logger.LogInfof("[StartCopyTradingSession] connecting => attempt %d", attempt+1)
		conn, _, err := dialer.Dial(endpoint, nil)
		if err != nil {
			delay := baseDelay << attempt
			if delay > maxDelay {
				delay = maxDelay
			}
			logger.LogWarnf("[StartCopyTradingSession] dial error => %v, backoff=%v", err, delay)
			attempt++
			time.Sleep(delay)
			continue
		}
		logger.LogInfo("[StartCopyTradingSession] connected successfully w/ Gorilla WS")
		manager.SubscribeAllStreams(conn, manager.PasteAddress)
		manager.SubscribeAllStreams(conn, manager.CopyAddress)

		go manager.keepConnectionAliveGorilla(conn, 15*time.Second, 30*time.Second)

		err = manager.readWsLoop(ctx, conn)
		conn.Close()

		if err != nil && ctx.Err() == nil {
			logger.LogWarnf("[StartCopyTradingSession] read loop => %v", err)
			attempt++
			time.Sleep(3 * time.Second)
			continue
		}
		if ctx.Err() != nil {
			logger.LogWarn("[StartCopyTradingSession] context done => stopping")
			return
		}
		attempt = 0
	}
}

// readWsLoop blocks reading inbound frames, dispatching them to manager.
func (manager *Manager) readWsLoop(ctx context.Context, conn *websocket.Conn) error {
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if msgType != websocket.TextMessage {
			continue
		}
		manager.handleWsRx(data)
	}
}

// handleWsRx routes inbound frames by `channel`.
func (manager *Manager) handleWsRx(rawData []byte) {
	var rawMap map[string]interface{}
	if err := json.Unmarshal(rawData, &rawMap); err != nil {
		return
	}
	ch, ok := rawMap["channel"].(string)
	if !ok {
		return
	}

	// If it's a subscribe ack
	if ch == "subscriptionResponse" {
		var resp models.SubscriptionResponse
		if json.Unmarshal(rawData, &resp) == nil {
			// Subscription response
			// logger.LogInfof("[inbound ws] copy paste subscriptionResponse => %#+v", resp)
		}
		return
	}

	switch ch {
	case "activeAssetData":
		manager.handleActiveAssetDataPayload(rawData)

	case "l2Book":
		manager.handleL2BookSnapshotPayload(rawData)

	case "webData2":
		manager.handleWebData2Payload(rawData)

	case "orderUpdates":
		if !manager.IsReady() {
			return
		}
		manager.handleOrderUpdates(rawData)
	}
}

func (manager *Manager) keepConnectionAliveGorilla(conn *websocket.Conn, interval time.Duration, readTimeout time.Duration) {
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))

	conn.SetPongHandler(func(appData string) error {
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	conn.SetPingHandler(func(appData string) error {
		deadline := time.Now().Add(2 * time.Second)
		if err := conn.WriteControl(websocket.PongMessage, []byte(appData), deadline); err != nil {
			logger.LogWarnf("[keepConnectionAliveGorilla] ping handler => failed writing pong: %v", err)
			_ = conn.Close()
		}
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer conn.Close()

		for range ticker.C {
			// 4a) Attempt to write a ping
			deadlineSec := 20
			deadline := time.Now().Add(20 * time.Second)
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), deadline); err != nil {
				logger.LogWarnf("[keepConnectionAliveGorilla] failed writing ping => %v in %vs", err, deadlineSec)
				return
			}
			// no “missedPongs++” needed here; the read deadline handles it.
		}
	}()
}

func (manager *Manager) SubscribeAllStreams(connection *websocket.Conn, userAddress string) error {
	manager.subConfirmChan = make(chan struct{})
	requiredSubscriptions := len(manager.AllowedSymbols) + 2
	atomic.StoreInt32(&manager.subNeeded, int32(requiredSubscriptions))

	for coinSymbol := range manager.MetaMap {
		if !manager.IsEnabledCoin(coinSymbol) {
			continue
		}
		userCoin := models.SubscriptionPayload{Coin: coinSymbol, User: userAddress}
		subRequest := models.NewSubcriptionRequest("activeAssetData", userCoin)
		writeErr := connection.WriteJSON(subRequest)
		if writeErr != nil {
			return writeErr
		}
	}
	userCoin := models.SubscriptionPayload{User: userAddress}
	webErr := connection.WriteJSON(models.NewSubcriptionRequest("webData2", userCoin))
	if webErr != nil {
		return webErr
	}
	if userAddress == manager.CopyAddress {
		orderErr := connection.WriteJSON(models.NewSubcriptionRequest("orderUpdates", userCoin))
		if orderErr != nil {
			return orderErr
		}

	}

	return nil
}

func (manager *Manager) ListenToWebSocketMessages(contextObject context.Context, connection *websocket.Conn) error {
	for {
		select {
		case <-contextObject.Done():
			return contextObject.Err()
		default:
		}
		_, rawData, readErr := connection.ReadMessage()
		if readErr != nil {
			return readErr
		}
		var rawMap map[string]interface{}
		unmarshalErr := json.Unmarshal(rawData, &rawMap)
		if unmarshalErr != nil {
			continue
		}
		channelName, channelOk := rawMap["channel"].(string)
		if !channelOk {
			continue
		}
		// if channelName == "subscribe" {
		// 	var subscriptionResp models.SubscriptionResponse
		// 	if json.Unmarshal(rawData, &subscriptionResp) == nil {
		// 		left := atomic.AddInt32(&manager.subNeeded, -1)
		// 		if left <= 0 {
		// 			select {
		// 			case <-manager.subConfirmChan:
		// 			default:
		// 				close(manager.subConfirmChan)
		// 			}
		// 		}
		// 	}
		// 	continue
		// }
		switch channelName {
		case "activeAssetData":
			manager.handleActiveAssetDataPayload(rawData)
		case "l2Book":
			manager.handleL2BookSnapshotPayload(rawData)
		case "webData2":
			manager.handleWebData2Payload(rawData)
		case "orderUpdates":
			manager.handleOrderUpdates(rawData)
		}
	}
}

func (manager *Manager) handleL2BookSnapshotPayload(rawData []byte) {
	var l2BookSnapshot *models.L2BookSnapshotMessage
	if json.Unmarshal(rawData, &l2BookSnapshot) != nil {
		return
	}
	manager.handleL2BookSnapshot(l2BookSnapshot)
}

func (manager *Manager) handleWebData2Payload(rawData []byte) {
	var wd2 *models.WebData2Message
	json.Unmarshal(rawData, &wd2)
	for _, symbol := range manager.AllowedSymbols {
		assetInfo, foundAsset := manager.MetaMap[symbol]
		if foundAsset {
			manager.AssetCtxStore.Store(symbol, wd2.Data.AssetCtxs[assetInfo.AssetID])
		}
	}
	if wd2.Data.User == manager.CopyAddress && manager.lastCopyWd2ChTime != wd2.ClearinghouseTime() {
		manager.CopyWd2 = wd2.AddPrev(manager.CopyWd2)
		if manager.PasteWd2 != nil && manager.CopyWd2.ClearinghouseTime().Equal(manager.PasteWd2.ClearinghouseTime()) {
			if !manager.CopyWd2.IsHead() && !manager.PasteWd2.IsHead() {
				manager.CopyWd2.AddOther(manager.PasteWd2)
			}
		}
		manager.CopyWebSocketReady = true
		manager.lastCopyWd2ChTime = wd2.ClearinghouseTime()
		manager.CopyWd2Chan <- wd2
	} else if wd2.Data.User == manager.PasteAddress && manager.lastPasteWd2ChTime != wd2.ClearinghouseTime() {
		manager.PasteWd2 = wd2.AddPrev(manager.PasteWd2)
		manager.PasteWebSocketReady = true
		manager.lastPasteWd2ChTime = wd2.ClearinghouseTime()
		manager.PasteWd2Chan <- wd2
	}
}

// func (m *Manager) updateInFlightForWD2(oldWd2, newWd2 models.WebData2Message) {
// 	if newWd2.Data.User != m.PasteAddress {
// 		return
// 	}
// 	oldMap := oldWd2.PositionsByCoin()
// 	newMap := newWd2.PositionsByCoin()
// 	for coin, newPos := range newMap {
// 		oldPos := oldMap[coin]
// 		delta := newPos.Szi - oldPos.Szi
// 		if math.Abs(delta) > 1e-8 {
// 			m.IocEngine.AdjustInFlight(coin, delta)
// 		}
// 	}
// }

func (engine *AloEngine) processNewAloOrders(copySideOrders map[string]hl.Order) {
	if len(copySideOrders) == 0 {
		return
	}
	var pasteRequests []hl.OrderRequest
	for cloid, copyOrderAsBase := range copySideOrders {
		_, foundCoin := engine.manager.MetaMap[copyOrderAsBase.Coin]
		if !foundCoin {
			logger.LogInfof("processNewAloOrders skipping unrecognized coin => coin=%s", copyOrderAsBase.Coin)
		}
		//virtualLeverage := manager.VirtualLeverage[copyOrder.Coin]

		isBuy := true
		if copyOrderAsBase.Side == "A" {
			isBuy = false
		}
		if copyOrderAsBase.Tif != hl.TifAlo {
			continue
		}
		pasteSz := engine.manager.scaleSize(copyOrderAsBase)
		decimals := engine.manager.Decimals(copyOrderAsBase.Coin)
		copyOrderAsBase.Sz = hl.SizeToFloat(pasteSz, decimals)
		if copyOrderAsBase.Sz == 0 {
			logger.LogErrorf("paste issue with scaledSz: %v", copyOrderAsBase.Sz)
			continue

		}
		orderRequest := hl.OrderRequest{
			Coin:       copyOrderAsBase.Coin,
			IsBuy:      isBuy,
			LimitPx:    copyOrderAsBase.LimitPx,
			Sz:         copyOrderAsBase.Sz,
			OrderType:  hl.OrderType{Limit: &hl.LimitOrderType{Tif: hl.TifAlo}},
			ReduceOnly: copyOrderAsBase.ReduceOnly,
			Cloid:      cloid,
		}
		notionalValue := orderRequest.Sz * orderRequest.LimitPx
		if notionalValue > minNotionalDiff {
			pasteRequests = append(pasteRequests, orderRequest)
		}
	}
	if len(pasteRequests) == 0 {
		return
	}
	bulkResponse, bulkErr := engine.manager.Client.BulkOrders(pasteRequests, hl.GroupingNa)
	if bulkErr != nil {
		logger.LogErrorf("paste BulkOrders error => %v", bulkErr)
	}
	if bulkResponse.Status != "ok" {
		logger.LogErrorf("paste BulkOrders status not ok => %s", bulkResponse.Status)
	}
	statuses := bulkResponse.Response.Data.Statuses
	for _, statusItem := range statuses {
		// status := statusItem.Status
		err := statusItem.Error
		if statusItem.Resting.Cloid == "" {
			logger.LogErrorf("paste alo order id was empty. status: %s | error: %s.", statusItem.Status, err)
		}
	}
}

func (manager *Manager) handleActiveAssetDataPayload(jsonData []byte) {
	var activeAssetMessage models.ActiveAssetDataMessage
	unmarshalErr := json.Unmarshal(jsonData, &activeAssetMessage)
	if unmarshalErr == nil {
		manager.handleActiveAssetData(activeAssetMessage.Data)
	}
}

func (eng *AloEngine) processCancelRequests(ordersToCancel map[string]hl.Order) {
	if len(ordersToCancel) == 0 {
		return
	}
	var byCloid []hl.CancelCloidWire

	for cloid, orderToCancel := range ordersToCancel {
		meta := eng.manager.MetaMap[orderToCancel.Coin]
		if orderToCancel.Cloid != "" {
			byCloid = append(byCloid, hl.CancelCloidWire{
				Asset: meta.AssetID,
				Cloid: cloid,
			})
		} else {
			logger.LogErrorf("Cancelling with an empty cloid: %#+v", orderToCancel)
		}
	}

	if len(byCloid) == 0 {
		return
	}

	resp, err := eng.manager.Client.BulkCancelOrdersByCloid(byCloid)
	if err != nil {
		logger.LogErrorf(fmt.Sprintf("[ERROR] paste BulkCancelByCloid: %v", err))
		return
	}
	if resp.Status != "ok" {
		logger.LogErrorf(fmt.Sprintf("[ERROR] paste BulkCancelOrdersByCloid status => %s", resp.Status))
		return
	}
	for i, st := range resp.Response.Data.Statuses {
		if st.Error != "" {
			cloid := byCloid[i].Cloid
			cloidInt, _ := hl.HexToInt(cloid)
			order := ordersToCancel[cloid]
			logger.LogErrorf(fmt.Sprintf("[aloCancel] paste cloid: %v | %s | Err: %s... ", cloidInt, order.Coin, st.Error[:10]))
		}
	}
}

// func (manager *Manager) processOrderModifications(modifications []models.OrderModification) {
// 	if len(modifications) == 0 {
// 		return
// 	}
// 	var requests []hl.OrderRequest
// 	for _, mod := range modifications {
// 		if mod.Mapping.PasteOid == 0 && mod.Order.Cloid == "" {
// 			continue
// 		}
// 		oldPasteOID := int(mod.Mapping.PasteOid)
// 		request := hl.OrderRequest{
// 			Coin:       mod.Order.Coin,
// 			IsBuy:      mod.Order.Side == "B",
// 			Sz:         mod.Order.Sz,
// 			LimitPx:    mod.Order.LimitPx,
// 			OrderType:  hl.OrderType{Limit: &hl.LimitOrderType{Tif: hl.TifAlo}},
// 			ReduceOnly: mod.Order.ReduceOnly,
// 			Cloid:      mod.Order.Cloid, // The NEW CLOID (for the modify)
// 		}
// 		if oldPasteOID != 0 {
// 			request.OrderID = &oldPasteOID
// 		}
// 		requests = append(requests, request)
// 	}

// 	if len(requests) == 0 {
// 		return
// 	}
// 	resp, err := manager.Client.BulkModifyOrdersByCloid(requests)
// 	if err != nil {
// 		logger.LogErrorf("[processOrderModifications] BulkModifyOrdersByCloid error => %v", err)
// 		cancelResp, cancelErr := manager.Client.CancelAllOrders()
// 		if cancelErr != nil {
// 			logger.LogErrorf("[processOrderModifications] fallback CancelAllOrders => %v", cancelErr)
// 		}
// 		if cancelResp != nil && cancelResp.Status != "ok" {
// 			logger.LogWarnf("[processOrderModifications] fallback CancelAllOrders => status=%s", cancelResp.Status)
// 		}
// 		return
// 	}
// 	if resp.Status != "ok" {
// 		logger.LogErrorf("[processOrderModifications] paste Non-ok status => %s", resp.Status)
// 		return
// 	}
// 	for i, statusItem := range resp.Response.Data.Statuses {
// 		if statusItem.Error != "" || statusItem.Resting.OrderID == 0 {
// 			continue
// 		}
// 		req := requests[i]
// 		newPasteOid := int64(statusItem.Resting.OrderID)
// 		// find matching mod
// 		var oldCopyOid, newCopyOid int64
// 		var oldCloid, newCloid string
// 		for _, mod := range modifications {
// 			sameCoin := (mod.Order.Coin == req.Coin)
// 			matchOid := (req.OrderID != nil && mod.Mapping.PasteOid == int64(*req.OrderID))
// 			matchCloid := (mod.Mapping.Cloid != "" && mod.Mapping.Cloid == mod.Order.Cloid)
// 			if sameCoin && (matchOid || matchCloid) {
// 				oldCopyOid = mod.Mapping.CopyOid
// 				newCopyOid = mod.Order.Oid
// 				oldCloid = mod.Mapping.Cloid
// 				newCloid = mod.Order.Cloid
// 				break
// 			}
// 		}
// 		if oldCopyOid == 0 {
// 			continue
// 		}
// 	}

// }

func (manager *Manager) makeModifiedOrder(openUpdate, canceledUpdate models.OrderUpdate) (*hl.Order, error) {
	if openUpdate.Status != "open" || canceledUpdate.Status != "canceled" {
		return nil, fmt.Errorf("[makeModifiedOrder] paste unexpected sequence: open->canceled not found")
	}

	if math.Abs(canceledUpdate.Order.Sz) < 1e-9 || math.Abs(openUpdate.Order.Sz) < 1e-9 {
		return nil, fmt.Errorf("[makeModifiedOrder] paste invalid sizes in makeModifiedOrder")
	}
	sizeRatio := openUpdate.Order.Sz / canceledUpdate.Order.Sz
	logger.LogInfof("copy paste Size ratio was %.2f", sizeRatio)
	scaledSize := manager.scaleSizeWithMultiplier(canceledUpdate.Order, sizeRatio)
	notionalValue := scaledSize * openUpdate.Order.LimitPx
	if notionalValue < minNotionalDiff {
		return nil, fmt.Errorf("[makeModifiedOrder] paste notional too small: $%v", notionalValue)
	}

	newOrder := openUpdate.Order
	newOrder.Sz = scaledSize
	newOrder.OrigSz = scaledSize
	newOrder.Cloid = openUpdate.Order.Cloid
	newOrder.Side = openUpdate.Order.Side
	return &newOrder, nil
}
