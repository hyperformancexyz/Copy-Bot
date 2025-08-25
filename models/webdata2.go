package models

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
)

type WebData2Message struct {
	Channel string `json:"channel"`
	Data    struct {
		ClearinghouseState struct {
			MarginSummary              MarginSummary  `json:"marginSummary"`
			CrossMarginSummary         MarginSummary  `json:"crossMarginSummary"`
			CrossMaintenanceMarginUsed float64        `json:"crossMaintenanceMarginUsed,string"`
			Withdrawable               float64        `json:"withdrawable,string"`
			AssetPositions             AssetPositions `json:"assetPositions"`
			Time                       int64          `json:"time"`
		} `json:"clearinghouseState"`
		OpenOrders OpenOrders
		AssetCtxs  []AssetCtx `json:"assetCtxs"`
		ServerTime int64      `json:"serverTime"`
		User       string     `json:"user"`
	} `json:"data"`

	prev  *WebData2Message
	next  *WebData2Message
	other *WebData2Message
	n     int
}

func (currWd2 *WebData2Message) AddPrev(prevWd2 *WebData2Message) *WebData2Message {
	currWd2.prev = prevWd2
	if prevWd2 == nil {
		currWd2.n = 0
		return currWd2
	} else {
		currWd2.n = prevWd2.n + 1
		prevWd2.next = currWd2
	}
	if prevWd2.Data.User != currWd2.Data.User {
		msg := fmt.Errorf("mismatched user: current=%s, prev=%s", currWd2.Data.User, prevWd2.Data.User)
		panic(msg)
	}
	if !prevWd2.ServerTime().Before(currWd2.ServerTime()) {
		msg := fmt.Errorf(
			"[wd2] prev ServerTime is after current: current=%d, prev=%d",
			currWd2.Data.ServerTime,
			prevWd2.Data.ServerTime,
		)
		//prevWd2.Dump("ServerTime")
		//currWd2.Dump("ServerTime")
		panic(msg)

	}
	if prevWd2.ClearinghouseTime().After(currWd2.ClearinghouseTime()) {
		msg := fmt.Errorf(
			"[wd2] prev ClearinghouseState.Time after current: current=%d, prev=%d",
			currWd2.Data.ClearinghouseState.Time,
			prevWd2.Data.ClearinghouseState.Time,
		)
		//revWd2.Dump("ChTime")
		//currWd2.Dump("ChTime")
		panic(msg)

	}
	return currWd2
}
func (currWd2 *WebData2Message) IsHead() bool {
	return currWd2.prev == nil
}
func (currWd2 *WebData2Message) AddOther(otherWd2 *WebData2Message) *WebData2Message {
	if otherWd2 == nil {
		return currWd2
	}
	if currWd2.Data.User == otherWd2.Data.User {
		panic(fmt.Errorf(
			"user must not match: current=%s, other=%s",
			currWd2.Data.User,
			otherWd2.Data.User,
		))
	}
	if otherWd2.prev == nil {
		panic(fmt.Errorf(
			"other.prev must not be nil",
		))
	}
	if otherWd2.prev.Data.User != otherWd2.Data.User {
		panic(fmt.Errorf(
			"other.prev user mismatch: other.prev=%s, other=%s",
			otherWd2.prev.Data.User,
			otherWd2.Data.User,
		))
	}
	if currWd2.Data.ClearinghouseState.Time != otherWd2.Data.ClearinghouseState.Time {
		panic(fmt.Errorf(
			"clearinghouse time mismatch: current=%d, other=%d",
			currWd2.Data.ClearinghouseState.Time,
			otherWd2.Data.ClearinghouseState.Time,
		))
	}
	if currWd2.other != nil && currWd2.other != otherWd2 {
		panic(fmt.Errorf(
			"current.other is already set to a different WebData2Message",
		))
	}
	currWd2.other = otherWd2

	if otherWd2.other != nil && otherWd2.other != currWd2 {
		panic(fmt.Errorf(
			"otherWd2.other is already set to a different WebData2Message",
		))
	}
	if otherWd2.other == nil {
		otherWd2.other = currWd2
	}
	return currWd2
}
func (wd2 *WebData2Message) NewAloOrders(coinRiskMap map[string]float64) map[string]hl.Order {
	newOrders := make(map[string]hl.Order)

	if wd2.prev == nil {
		return newOrders
	}
	nextOrders := wd2.OrdersByCloid()
	prevOrders := wd2.prev.OrdersByCloid()

	for cloid, nextOrder := range nextOrders {
		if _, ok := coinRiskMap[nextOrder.Coin]; ok {
			if _, exists := prevOrders[cloid]; !exists {
				newOrders[cloid] = nextOrder
			}
		}

	}
	return newOrders
}

func (wd2 *WebData2Message) CancelledAloOrders(coinRiskMap map[string]float64) map[string]hl.Order {
	cancelledOrders := make(map[string]hl.Order)

	if wd2.prev == nil {
		return cancelledOrders
	}
	prevOrders := wd2.prev.OrdersByCloid()
	currentOrders := wd2.OrdersByCloid()

	for cloid, prevOrder := range prevOrders {
		if _, ok := coinRiskMap[prevOrder.Coin]; ok {
			if _, exists := currentOrders[cloid]; !exists {
				cancelledOrders[cloid] = prevOrder
			}
		}
	}
	return cancelledOrders
}
func (wd2 *WebData2Message) Dump(prefix string) error {
	filename := fmt.Sprintf(
		"%s_0x%s_%d_%d_%d.json",
		prefix,
		wd2.AddressShort(),
		wd2.N(),
		wd2.ServerTime().UnixMilli(),
		wd2.ClearinghouseTime().UnixMilli(),
	)
	wd2Short := *wd2
	wd2Short.Data.AssetCtxs = nil
	data, err := json.MarshalIndent(wd2Short, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func (wd2 *WebData2Message) N() int {
	return wd2.n
}
func (wd2 *WebData2Message) Last() *WebData2Message {
	return wd2.prev
}
func (wd2 *WebData2Message) ServerTime() time.Time {
	return time.UnixMilli(wd2.Data.ServerTime)
}
func (wd2 *WebData2Message) ClearinghouseTime() time.Time {
	return time.UnixMilli(wd2.Data.ClearinghouseState.Time)
}

func (wd2 *WebData2Message) PositionsToKey() string {
	positions := wd2.Positions()
	if len(positions) == 0 {
		return "{}"
	}
	acctVal := wd2.AccountValue()
	if acctVal <= 0 {
		return "{Free: 100}"
	}
	type entry struct {
		coin string
		val  float64
	}

	var entries []entry
	for _, p := range positions {
		rawPct := (p.MarginUsed / acctVal) * 100.0
		rounded := roundToPrecision(rawPct, 1)
		if rounded < 0 {
			rounded = 0
		}
		entries = append(entries, entry{p.Coin, rounded})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].coin < entries[j].coin
	})

	var total float64
	for _, e := range entries {
		total += e.val
	}
	total = roundToPrecision(total, 1)
	var parts []string
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%s: %v", e.coin, e.val))
	}
	if total < 100 {
		parts = append(parts, fmt.Sprintf("Free: %v", 100-total))
	}

	return fmt.Sprintf("{%s}", strings.Join(parts, ", "))
}
func roundToPrecision(value float64, precision int) float64 {
	scale := math.Pow10(precision)
	refined := math.Round(value*scale) / scale
	return refined
}

func (msg *WebData2Message) AddressShort() string {
	if len(msg.Data.User) == 0 {
		return ""
	}
	if len(msg.Data.User) <= 6 {
		return msg.Data.User
	}
	return msg.Data.User[:6]
}

func (msg *WebData2Message) Positions() []Position {
	assetPositions := msg.Data.ClearinghouseState.AssetPositions
	positions := make([]Position, len(assetPositions))
	for i, assetPosition := range msg.Data.ClearinghouseState.AssetPositions {
		positions[i] = assetPosition.Position
	}
	return positions
}
func (msg *WebData2Message) Orders() []hl.Order {
	return msg.Data.OpenOrders
}

func (msg *WebData2Message) AccountValue() float64 {
	if msg == nil {
		return 0
	}
	return msg.Data.ClearinghouseState.MarginSummary.AccountValue
}

func (wd2 *WebData2Message) OrdersByCloid() map[string]hl.Order {
	r := make(map[string]hl.Order)
	for _, o := range wd2.Orders() {
		if o.Cloid != "" {
			r[o.Cloid] = o
		}
	}
	return r
}

func (wd2 *WebData2Message) OrdersByOid() map[int64]hl.Order {
	r := make(map[int64]hl.Order)
	for _, o := range wd2.Orders() {
		r[o.Oid] = o
	}
	return r
}

func (wd2 *WebData2Message) OrdersByCoin() map[string]hl.Order {
	r := make(map[string]hl.Order)
	for _, o := range wd2.Orders() {
		r[o.Coin] = o
	}
	return r
}

func (wd2 *WebData2Message) PositionsByCoin() map[string]Position {
	r := make(map[string]Position)
	positions := wd2.Positions()
	for _, p := range positions {
		r[p.Coin] = p
	}
	return r
}

type AssetCtxMap map[string]AssetCtx

func (wd2 *WebData2Message) DistinctKey() string {
	return wd2.PositionsToKey()
}

// AssetCtx defines context information for an asset such as pricing and volume.
type OpenOrders []hl.Order
type OpenOrder struct {
	Coin             string  `json:"coin"`
	Side             string  `json:"side"`
	LimitPx          string  `json:"limitPx"`
	Sz               float64 `json:"sz,string"`
	Oid              int     `json:"oid"`
	Timestamp        int64   `json:"timestamp"`
	TriggerCondition string  `json:"triggerCondition"`
	IsTrigger        bool    `json:"isTrigger"`
	TriggerPx        float64 `json:"triggerPx,string"`
	Children         []any   `json:"children"`
	IsPositionTpsl   bool    `json:"isPositionTpsl"`
	ReduceOnly       bool    `json:"reduceOnly"`
	OrderType        string  `json:"orderType"`
	OrigSz           float64 `json:"origSz,string"`
	Tif              string  `json:"tif"`
	Cloid            string  `json:"cloid"`
}
type Position struct {
	Coin     string  `json:"coin"`
	Szi      float64 `json:"szi,string"`
	Leverage struct {
		Type  string `json:"type"`
		Value int    `json:"value"`
	} `json:"leverage"`
	EntryPx        float64 `json:"entryPx,string"`
	PositionValue  float64 `json:"positionValue,string"`
	UnrealizedPnl  float64 `json:"unrealizedPnl,string"`
	ReturnOnEquity float64 `json:"returnOnEquity,string"`
	LiquidationPx  float64 `json:"liquidationPx,string"`
	MarginUsed     float64 `json:"marginUsed,string"`
	MaxLeverage    int     `json:"maxLeverage"`
	CumFunding     struct {
		AllTime     float64 `json:"allTime,string"`
		SinceOpen   float64 `json:"sinceOpen,string"`
		SinceChange float64 `json:"sinceChange,string"`
	} `json:"cumFunding"`
}
type AssetCtx struct {
	Funding      float64  `json:"funding,string"`
	OpenInterest float64  `json:"openInterest,string"`
	PrevDayPx    float64  `json:"prevDayPx,string"`
	DayNtlVlm    float64  `json:"dayNtlVlm,string"`
	Premium      float64  `json:"premium,string"`
	OraclePx     float64  `json:"oraclePx,string"`
	MarkPx       float64  `json:"markPx,string"`
	MidPx        float64  `json:"midPx,string"`
	ImpactPxs    []string `json:"impactPxs"`
	DayBaseVlm   float64  `json:"dayBaseVlm,string"`
}
type AssetPositions []struct {
	Type     string   `json:"type"`
	Position Position `json:"position"`
}
type MarginSummary struct {
	AccountValue    float64 `json:"accountValue,string"`
	TotalNtlPos     float64 `json:"totalNtlPos,string"`
	TotalRawUsd     float64 `json:"totalRawUsd,string"`
	TotalMarginUsed float64 `json:"totalMarginUsed,string"`
}

func (wd2 *WebData2Message) SafeCopy() *WebData2Message {
	copyWd2 := *wd2
	copyWd2.Data.AssetCtxs = nil
	return &copyWd2
}
