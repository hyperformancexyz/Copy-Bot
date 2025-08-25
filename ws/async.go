package ws

// import (
// 	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
// 	"github.com/itay747/hyperformance/models"
// 	"github.com/itay747/hyperformance/utils"
// )

// type RequestFunc[T any] func() (T, error)

// type CallbackFunc[T any] func(T, error)

// func DoAsync[T any](reqFn RequestFunc[T], cb CallbackFunc[T]) {
// 	go func() {
// 		res, err := reqFn()
// 		cb(res, err)
// 	}()
// }

// func BulkOrders(
// 	manager *Manager,
// 	copyOrders []hl.Order,
// 	pasteRequests []hl.OrderRequest,
// 	logger *utils.DualLogger,
// ) {
// 	go func() {
// 		resp, err := manager.Client.BulkOrders(pasteRequests, hl.GroupingNa)
// 		if err != nil {
// 			logger.LogErrorf("BulkOrders error => %v", err)
// 			return
// 		}
// 		if resp.Status != "ok" {
// 			logger.LogErrorf("BulkOrders status not ok => %s", resp.Status)
// 			return
// 		}
// 		statuses := resp.Response.Data.Statuses
// 		for i, st := range statuses {
// 			if st.Error != "" {
// 				logger.LogErrorf("Skipping newOrder i=%d error=%s", i, st.Error)
// 				continue
// 			}
// 			cpy := copyOrders[i]
// 			filledID := int64(st.Filled.OrderID)
// 			restingID := int64(st.Resting.OrderID)

// 			switch {
// 			case filledID != 0:
// 				logger.LogInfof("New order fully filled => copyOid=%d pasteOid=%d coin=%s", cpy.Oid, filledID, cpy.Coin)
// 				manager.PasteOrderStatus.Store(filledID, cpy.Coin)
// 				manager.UpdateMap(cpy.Oid, filledID, cpy.Coin)
// 			case restingID != 0:
// 				logger.LogInfof("New order resting => copyOid=%d pasteOid=%d coin=%s", cpy.Oid, restingID, cpy.Coin)
// 				manager.PasteOrderStatus.Store(restingID, cpy.Coin)
// 				manager.UpdateMap(cpy.Oid, restingID, cpy.Coin)
// 			default:
// 				logger.LogWarnf("No resting/filled ID => i=%d coin=%s copyOid=%d", i, cpy.Coin, cpy.Oid)
// 			}
// 		}
// 	}()
// }

// func BulkModifyOrders(
// 	manager *Manager,
// 	modificationList []models.OrderModification,
// 	modifyRequests []hl.OrderRequest,
// 	logger *utils.DualLogger,
// ) {
// 	go func() {
// 		resp, err := manager.Client.BulkModifyOrders(modifyRequests)
// 		if err != nil {
// 			logger.LogErrorf("[processOrderModifications] BulkModifyOrders error => %v", err)
// 			cancelResp, cancelErr := manager.Client.CancelAllOrders()
// 			if cancelErr != nil {
// 				logger.LogErrorf("[processOrderModifications] fallback CancelAllOrders => %v", cancelErr)
// 			}
// 			if cancelResp != nil && cancelResp.Status != "ok" {
// 				logger.LogWarnf("[processOrderModifications] fallback CancelAllOrders => status=%s", cancelResp.Status)
// 			}
// 			return
// 		}
// 		if resp.Status != "ok" {
// 			return
// 		}
// 		for i, s := range resp.Response.Data.Statuses {
// 			if s.Error != "" || s.Resting.OrderID == 0 {
// 				continue
// 			}
// 			r := modifyRequests[i]
// 			var oldCopyOid, newCopyOid int64
// 			for _, mod := range modificationList {
// 				if mod.Mapping.PasteOid == int64(*r.OrderID) &&
// 					mod.Order.Coin == r.Coin &&
// 					mod.Order.Cloid == r.Cloid {
// 					oldCopyOid = mod.Mapping.CopyOid
// 					newCopyOid = mod.Order.Oid
// 					break
// 				}
// 			}
// 			if oldCopyOid == 0 {
// 				continue
// 			}
// 			newPasteOid := int64(s.Resting.OrderID)
// 			manager.PasteOrderStatus.Store(newPasteOid, r.Coin)
// 			manager.mapLock.Lock()
// 			oldValue, foundValue := manager.mappingStore.Load(oldCopyOid)
// 			if foundValue {
// 				oldMapping := oldValue
// 				oldMapping.PasteOid = newPasteOid
// 				manager.mappingStore.Save(oldMapping)
// 				if newCopyOid != 0 && newCopyOid != oldCopyOid {
// 					d := oldMapping
// 					d.CopyOid = newCopyOid
// 					manager.mappingStore.Save(d)
// 				}
// 			} else {
// 				manager.UpdateMap(oldCopyOid, newPasteOid, r.Coin)
// 				if newCopyOid != 0 && newCopyOid != oldCopyOid {
// 					newMapping := models.Mapping{
// 						CopyOid:   newCopyOid,
// 						PasteOid:  newPasteOid,
// 						AssetID:   manager.MetaMap[r.Coin].AssetID,
// 						AssetName: r.Coin,
// 					}
// 					manager.mappingStore.Save(newMapping)
// 				}
// 			}
// 			manager.mapLock.Unlock()
// 		}
// 	}()
// }
