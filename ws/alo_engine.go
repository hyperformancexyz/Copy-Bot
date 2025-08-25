package ws

import (
	"context"
	"sync"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/itay747/hyperformance/models"
)

type OrdersByCloidMap map[string]hl.Order

type AloEngine struct {
	mu              sync.Mutex
	manager         *Manager
	enabled         bool
	createdCloids   map[string]bool
	canceledCloids  map[string]bool
	copyOpenOrders  map[string]hl.Order
	pasteOpenOrders map[string]hl.Order
}

func NewAloEngine(ctx context.Context, m *Manager, enabled bool) *AloEngine {
	return &AloEngine{
		manager:         m,
		enabled:         enabled,
		copyOpenOrders:  make(map[string]hl.Order),
		pasteOpenOrders: make(map[string]hl.Order),
		createdCloids:   make(map[string]bool),
		canceledCloids:  make(map[string]bool),
	}
}

func (engine *AloEngine) Start(ctx context.Context,
	CopyWd2Chan <-chan *models.WebData2Message,
	PasteWd2Chan <-chan *models.WebData2Message,
	l2Book <-chan *models.L2BookSnapshotMessage) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-CopyWd2Chan:
				if engine.pasteOpenOrders != nil {
					engine.HandleAloReconcile()
				}
			case wd2 := <-PasteWd2Chan:
				engine.pasteOpenOrders = wd2.OrdersByCloid()

			case <-l2Book:

			}

		}
	}()
}
func (engine *AloEngine) HandleAloReconcile() {
	if !engine.enabled {
		return
	}
	orders, cancels := engine.RunAloReconcile()
	blockTime := engine.manager.CopyWd2.ClearinghouseTime()

	if len(orders) > 0 {
		go engine.processNewAloOrders(orders)
	}
	if len(cancels) > 0 {
		go engine.processCancelRequests(cancels)
	}
	engine.manager.UsedCopyAloCreates[blockTime.Unix()] = time.Now()

}

// , toCancelOid map[int64]hl.Order
func (engine *AloEngine) RunAloReconcile() (toCreateRaw, toCancelCloid map[string]hl.Order) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	toCreateRaw = engine.manager.CopyWd2.NewAloOrders(engine.manager.CoinRiskMap)
	toCreateFinal := make(map[string]hl.Order)
	for cloid, order := range toCreateRaw {
		if _, ok := engine.createdCloids[cloid]; !ok {
			toCreateFinal[cloid] = order
			engine.createdCloids[cloid] = true
		}
	}
	// orders missing copy prev, next should cancel cloids for paste
	toCancel := engine.manager.CopyWd2.CancelledAloOrders(engine.manager.CoinRiskMap)
	// orders missing from copy on paste should cancel
	for _, openPasteOrder := range engine.manager.PasteWd2.OrdersByCloid() {
		copyOpenOrders := engine.manager.CopyWd2.OrdersByCloid()
		if _, ok := copyOpenOrders[openPasteOrder.Cloid]; !ok {
			toCancel[openPasteOrder.Cloid] = openPasteOrder
		}
	}

	return toCreateFinal, toCancel
}

// func parseOpenOrders(openOrders []models.OpenOrder, mgr *Manager) map[string]hl.Order {
// 	result := make(map[string]hl.Order, len(openOrders))
// 	for _, o := range openOrders {
// 		if o.Cloid == "" {
// 			logger.LogErrorf("Found open paste order w/ no cloid %+v", o)
// 			continue
// 		}
// 		if !mgr.IsEnabledCoin(o.Coin) {
// 			continue
// 		}
// 		result[o.Cloid] = hl.Order{
// 			Oid:        int64(o.Oid),
// 			Coin:       o.Coin,
// 			Side:       o.Side,
// 			LimitPx:    o.LimitPx,
// 			Sz:         o.Sz,
// 			Timestamp:  o.Timestamp,
// 			OrderType:  o.OrderType,
// 			Cloid:      o.Cloid,
// 			Tif:        o.Tif,
// 			ReduceOnly: o.ReduceOnly,
// 		}
// 	}
// 	return result
// }

// func (eng *AloEngine) findCreateOrders(copyOrdersByCloid, pasteOrdersByCloid map[string]hl.Order) []hl.Order {
// 	var results []hl.Order
// 	for cloid, copyOrder := range copyOrdersByCloid {
// 		if !eng.manager.IsEnabledCoin(copyOrder.Coin) {
// 			continue
// 		}
// 		if eng.createdCloids[cloid] {
// 			continue
// 		}
// 		results = append(results, copyOrder)
// 		eng.createdCloids[cloid] = true
// 	}
// 	return nil
// }

// func (eng *AloEngine) findCancelOrders(copyOrdersByCloid, pasteOrdersByCloid map[string]hl.Order) []hl.Order {
// 	//var results []hl.Order
// 	// for _, ok := copyOrdersByCloid[cloid]; !ok {
// 	// 	if eng.canceledCloids[cloid] {
// 	// 		continue // We’ve already canceled this CLOID once, skip
// 	// 	}
// 	// 	results = append(results, pasteOrd)
// 	// 	eng.canceledCloids[cloid] = true
// 	// }
// 	// return results
// 	return nil
// }
