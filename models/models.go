// Package models defines the data structures and types used in the bot application.
package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	gabs "github.com/Jeffail/gabs/v2"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
)

type SubscriptionPayload struct {
	Coin     string
	User     string
	NSigFigs *int
}

type Subscription struct {
	Type     string `json:"type"`
	Coin     string `json:"coin,omitempty"`
	User     string `json:"user,omitempty"`
	NSigFigs *int   `json:"nSigFigs,omitempty"`
}

type SubscriptionRequest struct {
	Method       string       `json:"method"`
	Subscription Subscription `json:"subscription"`
}

func NewSubcriptionRequest(channel string, payload SubscriptionPayload) SubscriptionRequest {
	if channel == "l2Book" {
		return SubscriptionRequest{
			Method: "subscribe",
			Subscription: Subscription{
				Type:     channel,
				Coin:     payload.Coin,
				NSigFigs: payload.NSigFigs,
			},
		}
	}
	return SubscriptionRequest{
		Method: "subscribe",
		Subscription: Subscription{
			Type: channel,
			Coin: payload.Coin,
			User: payload.User,
		},
	}
}

// SubscriptionResponse represents a subscription response message.
type SubscriptionResponse struct {
	Channel string `json:"channel"`
	Data    struct {
		Subscription Subscription `json:"subscription"`
	} `json:"data"`
}

func (orderMessage *OrderMessage) DistinctKey() string {
	return fmt.Sprintf("%v", time.Now().UnixMilli())
}

// OrderUpdate represents an update for a particular order.
type OrderUpdate struct {
	Status          string   `json:"status"`
	StatusTimestamp int64    `json:"statusTimestamp"`
	Order           hl.Order `json:"order"`
}

// OrderMessage contains order update messages.
type OrderMessage struct {
	Channel string        `json:"channel"`
	Data    []OrderUpdate `json:"data"`
}

// UserAssetData holds the active asset trading data for a user.
type UserAssetData struct {
	User     string `json:"user"`
	Coin     string `json:"coin"`
	Leverage struct {
		Type  string  `json:"type"`
		Value float64 `json:"value"`
	} `json:"leverage"`
	MaxTradeSzs      []float64 `json:"maxTradeSzs"`
	AvailableToTrade []float64 `json:"availableToTrade"`
}

// WireActiveAssetData is used for intermediate JSON representation of ActiveAssetData.
type WireActiveAssetData struct {
	User     string `json:"user"`
	Coin     string `json:"coin"`
	Leverage struct {
		Type  string  `json:"type"`
		Value float64 `json:"value"`
	} `json:"leverage"`
	MaxTradeSzs      []string `json:"maxTradeSzs"`
	AvailableToTrade []string `json:"availableToTrade"`
}

// UnmarshalJSON implements the json.Unmarshaler interface for ActiveAssetData.
func (a *UserAssetData) UnmarshalJSON(data []byte) error {
	parsed, _ := gabs.ParseJSON(data)
	wire := WireActiveAssetData{}
	if parsed.Exists("method") {
		fmt.Println("method: ", parsed.StringIndent("", "  "))
		json.Unmarshal(data, &a)

	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	a.User = wire.User
	a.Coin = wire.Coin
	a.Leverage = wire.Leverage

	a.MaxTradeSzs = make([]float64, len(wire.MaxTradeSzs))
	for i, v := range wire.MaxTradeSzs {
		num, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		a.MaxTradeSzs[i] = num
	}

	a.AvailableToTrade = make([]float64, len(wire.AvailableToTrade))
	for i, v := range wire.AvailableToTrade {
		num, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return err
		}
		a.AvailableToTrade[i] = num
	}
	return nil
}

// ActiveAssetDataMessage encapsulates active asset data with associated channel.
type ActiveAssetDataMessage struct {
	Channel string        `json:"channel"`
	Data    UserAssetData `json:"data"`
}

// BotMeta holds metadata information about the bot.
type BotMeta struct {
	Name        string
	MaxLeverage float64
	SzDecimals  int
}

// UserFillsMessage represents fill details for user trades.
type UserFillsMessage struct {
	Channel string `json:"channel"`
	Data    struct {
		IsSnapshot bool   `json:"isSnapshot"`
		User       string `json:"user"`
		Fills      []struct {
			Coin          string  `json:"coin"`
			Px            float64 `json:"px,string"`
			Sz            float64 `json:"sz,string"`
			Side          string  `json:"side"`
			Time          int64   `json:"time"`
			StartPosition string  `json:"startPosition"`
			Dir           string  `json:"dir"`
			ClosedPnl     float64 `json:"closedPnl,string"`
			Hash          string  `json:"hash"`
			Oid           int     `json:"oid"`
			Crossed       bool    `json:"crossed"`
			Fee           float64 `json:"fee,string"`
			Tid           int64   `json:"tid"`
			Cloid         string  `json:"cloid"`
			FeeToken      string  `json:"feeToken"`
		} `json:"fills"`
	} `json:"data"`
}

// OrderUpdates aggregates various types of order updates.
type OrderUpdates struct {
	Orders        []OrderUpdate
	Cancellations []OrderUpdate
	Modifications []OrderUpdate
}

// OrderMapping defines the mapping between copied and pasted orders.
type OrderMapping struct {
	CopyOid  int64
	PasteOid int64
	Cloid    string

	AssetID   int
	AssetName string
}

// OrderModification represents a modification to an order along with its mapping.
type OrderModification struct {
	Mapping OrderMapping
	Order   hl.Order
}

// OrderStreams contains collections of modifications, new orders, and cancellations.
type OrderStreams struct {
	Modifies  []OrderModification
	NewOrders []hl.Order
	Cancels   []hl.Order
}

// AssetDetails provides detailed information about an asset including leverage and trade limits.
type AssetDetails struct {
	LeverageValue    float64
	MaxTradeAmounts  []float64
	AvailableToTrade []float64
}

/* --------------------------------------------------------------------------
   Color & Styling Constants
   -------------------------------------------------------------------------- */

// Standard ANSI escape sequences
var (
	reset     = "\u001B[0m"
	bold      = "\u001B[1m"
	faint     = "\u001B[2m"
	underline = "\u001B[4m"

	/* Basic 8-colors (foreground) */
	black  = "\u001B[30m"
	red    = "\u001B[31m"
	green  = "\u001B[32m"
	yellow = "\u001B[33m"
	blue   = "\u001B[34m"
	purple = "\u001B[35m"
	cyan   = "\u001B[36m"
	white  = "\u001B[37m"

	/* Bright variants */
	brightBlack  = "\u001B[90m"
	brightRed    = "\u001B[91m"
	brightGreen  = "\u001B[92m"
	brightYellow = "\u001B[93m"
	brightBlue   = "\u001B[94m"
	brightPurple = "\u001B[95m"
	brightCyan   = "\u001B[96m"
	brightWhite  = "\u001B[97m"
)

/*
Helper function for side-based coloring:

  - A standard "Buy" (long) is green.
  - A standard "Sell" (short) is red.
  - If reduceOnly, we highlight with the inverse color
    signifying it's "closing" that side.
*/
func colorSide(side string, reduceOnly bool) string {
	sideUp := strings.ToUpper(side)
	switch sideUp {
	case "B":
		if reduceOnly {
			// Closing a short = green text but with a red-ish accent
			return fmt.Sprintf("%sCLOSE SHORT%s", brightGreen, reset)
		}
		return fmt.Sprintf("%sBUY%s", brightGreen, reset)
	case "A":
		if reduceOnly {
			// Closing a long = red text but with a green-ish accent
			return fmt.Sprintf("%sCLOSE LONG%s", brightRed, reset)
		}
		return fmt.Sprintf("%sSELL%s", brightRed, reset)
	}
	// Fallback
	return fmt.Sprintf("%sUNKNOWN_SIDE(%s)%s", yellow, side, reset)
}
