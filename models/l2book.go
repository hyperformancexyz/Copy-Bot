package models

import "math"

type BookLevel struct {
	Price float64
	Size  float64
	Count int
}

type L2BookSnapshotMessage struct {
	Channel string `json:"channel"`
	Data    struct {
		Coin   string `json:"coin"`
		Time   int64  `json:"time"`
		Levels [][]struct {
			Px float64 `json:"px,string"`
			Sz float64 `json:"sz,string"`
			N  int     `json:"n"`
		} `json:"levels"`
	} `json:"data"`
}

func (m *L2BookSnapshotMessage) Coin() string {
	return m.Data.Coin
}

func (m *L2BookSnapshotMessage) Timestamp() int64 {
	return m.Data.Time
}

func (m *L2BookSnapshotMessage) Bids() []BookLevel {
	if len(m.Data.Levels) == 0 {
		return nil
	}
	levels := make([]BookLevel, 0, len(m.Data.Levels[0]))
	for _, l := range m.Data.Levels[0] {
		levels = append(levels, BookLevel{l.Px, l.Sz, l.N})
	}
	return levels
}

func (m *L2BookSnapshotMessage) Asks() []BookLevel {
	if len(m.Data.Levels) < 2 {
		return nil
	}
	levels := make([]BookLevel, 0, len(m.Data.Levels[1]))
	for _, l := range m.Data.Levels[1] {
		levels = append(levels, BookLevel{l.Px, l.Sz, l.N})
	}
	return levels
}

func (m *L2BookSnapshotMessage) BestBid() BookLevel {
	bids := m.Bids()
	if len(bids) == 0 {
		return BookLevel{}
	}
	return bids[0]
}

func (m *L2BookSnapshotMessage) BestAsk() BookLevel {
	asks := m.Asks()
	if len(asks) == 0 {
		return BookLevel{}
	}
	return asks[0]
}

func (m *L2BookSnapshotMessage) Spread() float64 {
	bestBid := m.BestBid()
	bestAsk := m.BestAsk()
	if bestBid.Price == 0 || bestAsk.Price == 0 {
		return 0
	}
	return bestAsk.Price - bestBid.Price
}

func (m *L2BookSnapshotMessage) MidPrice() float64 {
	bestBid := m.BestBid()
	bestAsk := m.BestAsk()
	if bestBid.Price == 0 || bestAsk.Price == 0 {
		return 0
	}
	return (bestAsk.Price + bestBid.Price) / 2
}

func (m *L2BookSnapshotMessage) TotalVolume() float64 {
	var total float64
	for _, lvl := range m.Bids() {
		total += lvl.Size
	}
	for _, lvl := range m.Asks() {
		total += lvl.Size
	}
	return total
}

func (m *L2BookSnapshotMessage) WeightedMidPrice() float64 {
	bids, asks := m.Bids(), m.Asks()
	if len(bids) == 0 || len(asks) == 0 {
		return 0
	}
	var bidNotional, askNotional, bidVolume, askVolume float64
	for _, b := range bids {
		bidNotional += b.Price * b.Size
		bidVolume += b.Size
	}
	for _, a := range asks {
		askNotional += a.Price * a.Size
		askVolume += a.Size
	}
	if bidVolume == 0 || askVolume == 0 {
		return 0
	}
	bidVWAP := bidNotional / bidVolume
	askVWAP := askNotional / askVolume
	return (bidVWAP + askVWAP) / 2
}

func (m *L2BookSnapshotMessage) BestLevels(n int) ([]BookLevel, []BookLevel) {
	bids, asks := m.Bids(), m.Asks()
	if n > len(bids) {
		n = len(bids)
	}
	bestBids := bids[:n]
	if n > len(asks) {
		n = len(asks)
	}
	bestAsks := asks[:n]
	return bestBids, bestAsks
}

func (m *L2BookSnapshotMessage) NearestPriceLevel(target float64) BookLevel {
	diff := math.MaxFloat64
	var nearest BookLevel
	for _, lvl := range append(m.Bids(), m.Asks()...) {
		d := math.Abs(lvl.Price - target)
		if d < diff {
			diff = d
			nearest = lvl
		}
	}
	return nearest
}
