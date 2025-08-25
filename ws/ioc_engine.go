package ws

import (
	"context"
	"math"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/itay747/hyperformance/models"
)

var (
	minNotionalDiff = 20.0
)

type IocEngine struct {
	manager              *Manager
	localPastePositions  map[string]models.Position
	lastPasteWd2Rebase   time.Time
	enabled              bool
	ctx                  context.Context
	startupReconcileDone bool
}

func NewIocEngine(ctx context.Context, m *Manager, enabled bool) *IocEngine {
	return &IocEngine{
		manager:              m,
		enabled:              enabled,
		ctx:                  ctx,
		startupReconcileDone: false,
	}
}

func (r *IocEngine) Start(
	ctx context.Context,
	copyWd2Stream <-chan *models.WebData2Message,
	pasteWd2Stream <-chan *models.WebData2Message,
	orderUpdatesChan <-chan *models.OrderMessage,
) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case copyWd2 := <-copyWd2Stream:
				if !r.startupReconcileDone && r.enabled && r.manager.IsReady() && r.localPastePositions != nil {
					r.handleIocReconcile(copyWd2, r.localPastePositions)
					r.startupReconcileDone = true
				}

			case pasteWd2 := <-pasteWd2Stream:
				if r.localPastePositions == nil {
					r.localPastePositions = pasteWd2.PositionsByCoin()
					r.lastPasteWd2Rebase = time.Now()
				}
			case orderUpdate := <-orderUpdatesChan:
				if r.manager.IsReady() {
					r.handleOrderUpdates(orderUpdate)
				}
			}

		}
	}()
}

func (r *IocEngine) handleOrderUpdates(orderUpdates *models.OrderMessage) {
	//logger.LogInfof("Received order updates: %#+v", orderUpdates)
	ordersOut := make([]hl.Order, 0)

	byCoin := make(map[string][]models.OrderUpdate)
	for _, updateEntry := range orderUpdates.Data {
		if !r.manager.IsEnabledCoin(updateEntry.Order.Coin) {
			continue
		}
		byCoin[updateEntry.Order.Coin] = append(byCoin[updateEntry.Order.Coin], updateEntry)
	}
	for _, grouped := range byCoin {
		index := 0
		for index < len(grouped) {
			currentUpdate := grouped[index]
			if index+1 < len(grouped) {
				nextUpdate := grouped[index+1]
				hasSameCoin := (currentUpdate.Order.Coin == nextUpdate.Order.Coin)
				openFilled := (currentUpdate.Status == "open" && nextUpdate.Status == "filled")
				hasSameOids := (currentUpdate.Order.Oid == nextUpdate.Order.Oid)
				hasFill := (nextUpdate.Order.OrigSz - nextUpdate.Order.Sz) != 0
				if hasSameCoin && hasSameOids && openFilled && hasFill {
					// the cancelled one will be the base order template for ioc we build
					order := nextUpdate.Order
					// skip if unfilled ioc
					if order.Sz == order.OrigSz {
						continue
					}

					// only partially filled (therefore cancelled)
					// or fully filled COPY order's from here
					filledSz := order.OrigSz - order.Sz
					pasteOrder := hl.Order{
						Coin:       order.Coin,
						Sz:         filledSz,
						Side:       order.Side,
						Cloid:      r.manager.NewPasteIocCloid(order.Oid),
						ReduceOnly: order.ReduceOnly,
						Tif:        hl.TifFrontendMarket,
						OrderType:  hl.TifFrontendMarket,
						LimitPx:    0,
					}
					// Compute scaled size when this fill is applied to paste
					pasteOrderNotional := Round2(pasteOrder.Sz * r.manager.GetMidPrice(order.Coin))
					if pasteOrderNotional < minNotionalDiff {
						continue
					}

					ordersOut = append(ordersOut, pasteOrder)
					logger.LogInfof("\n%s\n%s", logger.FormatCopyOrder(order), logger.FormatPasteOrder(pasteOrder))

				}
			}
			index++
		}
	}

	if len(ordersOut) > 0 {
		r.SendIocOrders(ordersOut)
	}

}

func (r *IocEngine) IocOrdersToRequests(orders []hl.Order) []hl.OrderRequest {
	var requests []hl.OrderRequest
	for _, order := range orders {
		isBuy := true
		if order.Side == "A" {
			isBuy = false
		}

		if order.Tif == hl.TifFrontendMarket || order.OrderType == hl.TifFrontendMarket {
			order.LimitPx = r.manager.getMarketPrice(order)
			order.Tif = hl.TifFrontendMarket
		} else {
			scaled := r.manager.scaleSize(order)
			sz := hl.SizeToFloat(scaled, r.manager.Decimals(order.Coin))
			order.Sz = sz
		}
		if order.Sz <= 1e-9 {
			continue
		}
		req := hl.OrderRequest{
			Coin:       order.Coin,
			IsBuy:      isBuy,
			LimitPx:    order.LimitPx,
			Sz:         order.Sz,
			OrderType:  hl.OrderType{Limit: &hl.LimitOrderType{Tif: order.Tif}},
			ReduceOnly: order.ReduceOnly,
			Cloid:      order.Cloid,
		}
		notionalValue := Round2(req.Sz * req.LimitPx)
		if notionalValue < minNotionalDiff {
			continue
		}
		requests = append(requests, req)
	}
	return requests
}
func (r *IocEngine) handlePasteOrderUpdate(orderUpdate models.OrderUpdate) {
	logger.LogInfof("paste %s", logger.FormatOrderUpdate(orderUpdate))
}

func (r *IocEngine) handleIocReconcile(copyWd2 *models.WebData2Message, pastePositionsModelled map[string]models.Position) {
	if !r.manager.IsReady() {
		logger.LogWarn("[IOC] paste handleIocReconcile without manager being `isReady`")
		return
	}
	//var orders []hl.Order
	//pasteWd2Positions := pasteWd2.PositionsByCoin()
	//modelHasError := r.manager.GetIocReconcileOrders(pasteWd2Positions, pastePositionsModelled, false, true)
	// rebasedRecently := time.Since(r.lastPasteWd2Rebase).Seconds() < 30
	// pasteHasError := len(modelHasError) > 0
	// if !rebasedRecently && pasteHasError {
	// 	orders = r.manager.GetIocReconcileOrders(copyWd2.PositionsByCoin(), pasteWd2Positions, true, true)
	// 	r.localPastePositions = pasteWd2Positions
	// 	r.lastPasteWd2Rebase = time.Now()
	// 	for _, order := range orders {
	// 		position := r.localPastePositions[order.Coin]
	// 		position.Szi += sideSign(order.Side) * order.Sz
	// 		r.localPastePositions[order.Coin] = position
	// 	}

	// } else {
	orders := r.manager.GetIocReconcileOrders(copyWd2.PositionsByCoin(), pastePositionsModelled, true, false)
	// We now update pastePositionsByCoin immediately with the new sizes, assuming orders will succeed
	for _, order := range orders {
		position := pastePositionsModelled[order.Coin]
		position.Szi += sideSign(order.Side) * order.Sz
		pastePositionsModelled[order.Coin] = position
	}
	if len(orders) == 0 {
		return
	}

	r.SendIocOrders(orders)

}
func (r *IocEngine) SendIocOrders(orders []hl.Order) {
	requests := r.IocOrdersToRequests(orders)
	if len(requests) == 0 {
		logger.LogInfo("[IOC] paste Reconcile produced no valid request => skipping")
		return
	}
	resp, err := r.manager.Client.BulkOrders(requests, hl.GroupingNa)
	if err != nil {
		logger.LogErrorf("[IOC] paste BulkOrders error => %v", err)
		return
	}
	if resp.Status != "ok" {
		logger.LogErrorf("[IOC] paste BulkOrders returned status %q => skipping", resp.Status)
		return
	}
	for i, st := range resp.Response.Data.Statuses {
		isBuy := requests[i].IsBuy
		side := "LONG"
		if !isBuy {
			side = "SHORT"
		}
		logger.LogInfof("[IOC] paste %s %s %v", side, requests[i].Coin, st.Filled.TotalSz)
	}
}
func sideSign(side string) float64 {
	if side == "B" {
		return 1
	}
	return -1
}

// The Ioc Copy PositionMap <-> Paste PositionMap reconciliation algo.
// Returns no orders if no diff exists, any orders mean a notional diff was found.
func (manager *Manager) GetIocReconcileOrders(copyPosMap map[string]models.Position, pastePosMap map[string]models.Position, scale bool, bypassCheck bool) []hl.Order {

	copyPosKey := manager.CopyWd2.PositionsToKey()

	logger.LogInfof("copy: %s", copyPosKey)
	var newOrders []hl.Order

	if len(copyPosMap) == 0 && len(pastePosMap) == 0 {
		return newOrders
	}
	for _, symbol := range manager.AllowedSymbols {
		copyPos := copyPosMap[symbol]
		pastePos := pastePosMap[symbol]

		if copyPos.PositionValue < minNotionalDiff && pastePos.PositionValue < minNotionalDiff {
			continue
		}
		midPrice := manager.GetMidPrice(symbol)
		if midPrice <= 0 {
			continue
		}

		sideOfCopy := "B"
		if copyPos.Szi < 0 {
			sideOfCopy = "A"
		}
		decimals := manager.Decimals(symbol)
		// copySzi and copyNotional are both scaled from here until end of func
		// Variable ending with Szi are signed
		//  (ie, short position will have negative size, long positive)
		// Variables ending with Sz are unsigned (always +)
		copySzi := copyPos.Szi
		if scale {
			copySzi = manager.scaleSize(hl.Order{Coin: symbol, Side: sideOfCopy, Sz: copyPos.Szi})
		}
		pasteSzi := pastePos.Szi

		copyNotional := RoundToPrecision(math.Abs(copySzi)*midPrice, 2)
		pasteNotional := RoundToPrecision(math.Abs(pasteSzi)*midPrice, 2)
		diffSz := hl.SizeToFloat(math.Abs(pasteSzi-copySzi), decimals)
		notionalDiff := RoundToPrecision(diffSz*midPrice, 2)

		if notionalDiff < minNotionalDiff {
			continue
		}

		// Build a base order with default fields we might change
		baseOrder := hl.Order{
			Coin:  symbol,
			Tif:   hl.TifFrontendMarket,
			Cloid: manager.NewPasteIocCloid(manager.CopyWd2.Data.ClearinghouseState.Time),
		}

		var finalSide string
		var finalDiffSz float64
		// We'll default to 1x size, and only scale if we want
		finalDiffSz = diffSz

		// Use the switch to decide side & reduceOnly, maybe adjust finalSz
		switch {
		// 1) If copy has notional but paste is under min => open a new position
		case copyNotional > minNotionalDiff && pasteNotional < minNotionalDiff:
			if copySzi < 0 {
				finalSide = "A"
			} else {
				finalSide = "B"
			}
			finalDiffSz = math.Abs(copySzi) // fully replicate

		// 2) If copy is below min but paste is above => close paste
		case copyNotional < minNotionalDiff && pasteNotional > minNotionalDiff:
			if pasteSzi < 0 {
				finalSide = "B"
			} else {
				finalSide = "A"
			}
			finalDiffSz = math.Abs(pasteSzi)
			baseOrder.ReduceOnly = true

		// 3a) Both sides are long
		case copySzi > 0 && pasteSzi > 0:
			// Paste pos is smaller, increase size by longing
			if copyNotional > pasteNotional-minNotionalDiff {
				finalSide = "B"
				// Paste pos is larger => decrease size by shorting
			} else if copyNotional < pasteNotional+minNotionalDiff {
				finalSide = "A"
				baseOrder.ReduceOnly = true
			} else {
				continue // no significant difference
			}
		// 3b) Both sides are SHORT (both > 0 or both < 0)
		case copySzi < 0 && pasteSzi < 0:
			// Paste pos is smaller => increase size by shorting
			if copyNotional > pasteNotional-minNotionalDiff {
				finalSide = "A"
				// Paste pos is larger => decrease size by longing
			} else if copyNotional < pasteNotional+minNotionalDiff {
				finalSide = "B"
				baseOrder.ReduceOnly = true
			} else {
				continue
			}

		// 4a) Copy long, Paste short
		case copySzi > 0 && pasteSzi < 0:
			finalSide = "B"
			finalDiffSz = math.Abs(pasteSzi) + math.Abs(copySzi)
		// 4b) Copy short, Paste long
		case copySzi < 0 && pasteSzi > 0:
			finalSide = "A"
			finalDiffSz = math.Abs(pasteSzi) + math.Abs(copySzi)
		}

		// If we never set finalSide, skip
		if finalSide == "" || finalDiffSz < 1e-9 {
			continue
		}
		baseOrder.Side = finalSide
		baseOrder.Sz = finalDiffSz
		pasteWd2Time := manager.PasteWd2.Data.ClearinghouseState.Time
		if _, ok := manager.UsedPasteWd2[pasteWd2Time]; !ok {
			manager.UsedPasteWd2[pasteWd2Time] = time.Now()
			newOrders = append(newOrders, baseOrder)
		} else {
			logger.LogWarnf("paste Tried reconciling on prev reconciled wd2 clearinghouse timestamp: %v", pasteWd2Time)
		}
	}

	if len(newOrders) == 0 {
		return newOrders
	}
	return newOrders
}

func (manager *Manager) HasMargin(iocOrder hl.Order) bool {
	// We assume all IOC orders are placed on the Paste side
	// so we retrieve AssetDetails for manager.PasteAddress + ":" + iocOrder.Coin
	key := manager.PasteAddress + ":" + iocOrder.Coin
	value, ok := manager.AssetDetailsStore.Load(key)
	if !ok {
		logger.LogWarnf("[HasMargin] No AssetDetails found => coin=%s key=%s", iocOrder.Coin, key)
		return false
	}
	details := value.(models.AssetDetails)
	midPx := manager.GetMidPrice(iocOrder.Coin)
	orderNotional := Round2(iocOrder.Sz * midPx)
	i := 0
	if iocOrder.Side == "A" {
		i = 1
	}
	marginAvailable := details.AvailableToTrade[i]
	notionalAvailable := Round2(marginAvailable * details.LeverageValue * midPx)
	if notionalAvailable < orderNotional {
		logger.LogWarnf("[HasMargin] Not enough margin => wanted notional=$%.2f available=$%.2f coin=%s",
			orderNotional, notionalAvailable, iocOrder.Coin)
		return false
	}
	return true
}

//	func (r *IocEngine) handlePasteOrderUpdates(orderUpdates models.OrderUpdatesMessage) {
//		for _, upd := range orderUpdates.Data {
//			if upd.Status != "filled" {
//				continue
//			}
//			order := upd.Order
//			if !r.manager.IsEnabledCoin(order.Coin) {
//				continue
//			}
//			// // If order was TifFrontendMarket => we know it's an IOC from our code
//			// if order.Tif == hl.TifFrontendMarket {
//			// 	if upd.Status == "filled" {
//			// 		delta := order.Sz
//			// 		if order.Side == "A" {
//			// 			delta = -delta
//			// 		}
//			// 		r.AdjustInFlight(order.Coin, delta)
//			// 	}
//			// }
//		}
//	}
