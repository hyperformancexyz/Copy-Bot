package utils

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/charmbracelet/lipgloss"
	"github.com/itay747/hyperformance/models"
)

type DualLogger struct {
	logCh            chan string
	mu               sync.Mutex
	infoStyle        lipgloss.Style
	debugStyle       lipgloss.Style
	warningStyle     lipgloss.Style
	errorStyle       lipgloss.Style
	titleStyle       lipgloss.Style
	noEntriesStyle   lipgloss.Style
	boldStyle        lipgloss.Style
	faintStyle       lipgloss.Style
	underlineStyle   lipgloss.Style
	highlightPurple  lipgloss.Style
	highlightCyan    lipgloss.Style
	sideBuyStyle     lipgloss.Style
	sideSellStyle    lipgloss.Style
	sideUnknownStyle lipgloss.Style
	priceLowStyle    lipgloss.Style
	sizeTinyStyle    lipgloss.Style
	sizeSmallStyle   lipgloss.Style
	sizeMediumStyle  lipgloss.Style
	sizeLargeStyle   lipgloss.Style
}

func NewDualLogger(ch chan string) *DualLogger {
	return &DualLogger{
		logCh:            ch,
		infoStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Bold(true),
		debugStyle:       lipgloss.NewStyle().Faint(true),
		warningStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("228")).Bold(true),
		errorStyle:       lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		titleStyle:       lipgloss.NewStyle().Bold(true).Underline(true),
		noEntriesStyle:   lipgloss.NewStyle().Faint(true),
		boldStyle:        lipgloss.NewStyle().Bold(true),
		faintStyle:       lipgloss.NewStyle().Faint(true),
		underlineStyle:   lipgloss.NewStyle().Underline(true),
		highlightPurple:  lipgloss.NewStyle().Foreground(lipgloss.Color("165")),
		highlightCyan:    lipgloss.NewStyle().Foreground(lipgloss.Color("51")),
		sideBuyStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
		sideSellStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		sideUnknownStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),
		priceLowStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("99")),
		sizeTinyStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("99")),
		sizeSmallStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		sizeMediumStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		sizeLargeStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
	}
}

func (dl *DualLogger) LogInfo(message string) {
	stamp := time.Now().Format("15:04:05.000")
	prefix := fmt.Sprintf("[Info] [%s] ", stamp)
	line := dl.infoStyle.Render(prefix) + message
	if dl.logCh != nil {
		dl.logCh <- line
	}
}

func (dl *DualLogger) LogInfof(format string, args ...interface{}) {
	dl.LogInfo(fmt.Sprintf(format, args...))
}

func (dl *DualLogger) LogDebug(m string) {
	line := dl.debugStyle.Render("[DEBUG] ") + m
	if dl.logCh != nil {
		dl.logCh <- line
	}
}

func (dl *DualLogger) LogDebugf(format string, args ...interface{}) {
	dl.LogDebug(fmt.Sprintf(format, args...))
}

func (dl *DualLogger) LogWarn(m string) {
	line := dl.warningStyle.Render("[WARN] ") + m
	if dl.logCh != nil {
		dl.logCh <- line
	}
}

func (dl *DualLogger) LogWarnf(format string, args ...interface{}) {
	dl.LogWarn(fmt.Sprintf(format, args...))
}

func (dl *DualLogger) LogError(m string) {
	line := dl.errorStyle.Render("[ERROR] ") + m
	if dl.logCh != nil {
		dl.logCh <- line
	}
}

func (dl *DualLogger) LogErrorf(format string, args ...interface{}) {
	dl.LogError(fmt.Sprintf(format, args...))
}

func (dl *DualLogger) styleSide(sideString string, reduceOnly bool) string {
	upper := strings.ToUpper(sideString)
	switch upper {
	case "B":
		if reduceOnly {
			return dl.sideBuyStyle.Render("CLOSE SHORT")
		}
		return dl.sideBuyStyle.Render("BUY")
	case "A":
		if reduceOnly {
			return dl.sideSellStyle.Render("CLOSE LONG")
		}
		return dl.sideSellStyle.Render("SELL")
	default:
		return dl.sideUnknownStyle.Render("UNKNOWN_SIDE(" + sideString + ")")
	}
}

func (dl *DualLogger) styleOrderType(s string) string {
	switch strings.ToLower(s) {
	case "market":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("227")).Bold(true).Render("MARKET")
	case "limit":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("44")).Bold(true).Render("LIMIT")
	case "postonly":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true).Render("POST-ONLY")
	default:
		return dl.faintStyle.Render(s)
	}
}

func (dl *DualLogger) stylePrice(priceValue float64) string {
	if priceValue <= 0 {
		return dl.faintStyle.Render(fmt.Sprintf("%.2f", priceValue))
	}
	if priceValue < 1 {
		return dl.priceLowStyle.Render(fmt.Sprintf("%.4f", priceValue))
	}
	if priceValue < 1000 {
		return fmt.Sprintf("%.2f", priceValue)
	}
	return dl.boldStyle.Render(fmt.Sprintf("%.2f", priceValue))
}

func (dl *DualLogger) styleSize(sz float64) string {
	if sz == 0 {
		return dl.faintStyle.Render("0")
	}
	prefix := ""
	if sz > 0 {
		prefix = "+"
	}
	a := math.Abs(sz)
	switch {
	case a < 1:
		return dl.sizeTinyStyle.Render(fmt.Sprintf("%s%.4f", prefix, a))
	case a < 10:
		return dl.sizeSmallStyle.Render(fmt.Sprintf("%s%.2f", prefix, a))
	case a < 1000:
		return dl.sizeMediumStyle.Render(fmt.Sprintf("%s%.2f", prefix, a))
	default:
		return dl.sizeLargeStyle.Render(fmt.Sprintf("%s%.2f", prefix, a))
	}
}

func (dl *DualLogger) styleTimestamp(ts int64) string {
	if ts <= 0 {
		return dl.faintStyle.Render(fmt.Sprintf("%d", ts))
	}
	return time.Unix(ts/1000, 0).Local().Format("15:04:05")
}
func sideSign(side string) float64 {
	if side == "B" {
		return 1
	}
	return -1
}

func (dl *DualLogger) FormatOrder(o hl.Order) string {
	px := dl.stylePrice(o.LimitPx)
	sz := dl.styleSize(sideSign(o.Side) * (o.OrigSz - o.Sz))
	side := dl.styleSide(o.Side, o.ReduceOnly)

	return fmt.Sprintf(
		"%s %s - %v @ %s", side, o.Coin, sz, px)
}

func (dl *DualLogger) FormatOrderRequest(r hl.OrderRequest) string {
	lbl := "SHORT"
	if r.IsBuy {
		lbl = "LONG"
	}
	side := dl.styleSide(lbl, r.ReduceOnly)
	ot := dl.styleOrderType(r.OrderType.Limit.Tif)
	px := dl.stylePrice(r.LimitPx)
	sz := dl.styleSize(r.Sz)
	return fmt.Sprintf(
		"OrderRequest | Coin:%s | Side:%s | Type:%s | Px:%s | Sz:%s | TIF:%s",
		r.Coin, side, ot, px, sz, r.OrderType.Limit.Tif,
	)
}

func (dl *DualLogger) FormatCopyOrder(o hl.Order) string {
	return dl.highlightPurple.Render("[Copy] ") + dl.FormatOrder(o)
}
func (dl *DualLogger) FormatPasteOrder(o hl.Order) string {
	return dl.highlightPurple.Render("[Paste] ") + dl.FormatOrder(o)
}
func (dl *DualLogger) FormatPasteOrderRequest(r hl.OrderRequest) string {
	return dl.highlightCyan.Render("[Paste] ") + dl.FormatOrderRequest(r)
}

func (dl *DualLogger) FormatModification(before, after hl.Order) string {
	return fmt.Sprintf(
		"\n%s\n  Before: %s\n  After:  %s\n",
		dl.titleStyle.Render("---- Modification ----"),
		dl.FormatOrder(before),
		dl.FormatOrder(after),
	)
}

func (dl *DualLogger) FormatModificationCondensed(before, after hl.Order) string {
	return fmt.Sprintf("[MOD] Before => %s | After => %s", dl.FormatOrder(before), dl.FormatOrder(after))
}

func (dl *DualLogger) FormatOrderList(list []models.OrderUpdate, title string) string {
	if len(list) == 0 {
		return fmt.Sprintf("\n%s\n   %s\n",
			dl.titleStyle.Render("=== "+title+" ==="),
			dl.noEntriesStyle.Render("No entries"),
		)
	}
	var b strings.Builder
	b.WriteString("\n" + dl.titleStyle.Render("=== "+title+" ===") + "\n")
	for _, u := range list {
		b.WriteString(dl.FormatOrder(u.Order) + "\n")
	}
	return b.String()
}

func (dl *DualLogger) FormatHlOrderList(list []hl.Order, title string) string {
	if len(list) == 0 {
		return fmt.Sprintf("\n%s\n   %s\n",
			dl.titleStyle.Render("=== "+title+" ==="),
			dl.noEntriesStyle.Render("No entries"),
		)
	}
	var b strings.Builder
	b.WriteString("\n" + dl.titleStyle.Render("=== "+title+" ===") + "\n")
	for _, o := range list {
		b.WriteString(dl.FormatOrder(o) + "\n")
	}
	return b.String()
}

func (dl *DualLogger) FormatModificationsList(pairs [][2]hl.Order, title string) string {
	if len(pairs) == 0 {
		return fmt.Sprintf("\n%s\n   %s\n",
			dl.titleStyle.Render("=== "+title+" ==="),
			dl.noEntriesStyle.Render("No modifications"),
		)
	}
	var b strings.Builder
	b.WriteString("\n" + dl.titleStyle.Render("=== "+title+" ===") + "\n")
	for _, p := range pairs {
		b.WriteString(dl.FormatModification(p[0], p[1]))
	}
	return b.String()
}

func (dl *DualLogger) FormatPosition(p hl.Position) string {
	s := "B"
	if p.Szi < 0 {
		s = "A"
	}
	side := dl.styleSide(s, false)
	px := dl.stylePrice(p.EntryPx)
	sz := dl.styleSize(p.Szi)
	val := fmt.Sprintf("%.2f", p.PositionValue)
	pnl := fmt.Sprintf("%.2f", p.UnrealizedPnl)
	return fmt.Sprintf(
		"Coin:%s | Side:%s | EntryPx:%s | Szi:%s | PosVal:%s | UnPnl:%s | LiqPx:%.2f",
		dl.faintStyle.Render(p.Coin),
		side,
		px,
		sz,
		val,
		pnl,
		p.LiquidationPx,
	)
}

func (dl *DualLogger) FormatPositionList(list []hl.AssetPosition, title string) string {
	if len(list) == 0 {
		return fmt.Sprintf("\n%s\n   %s\n",
			dl.titleStyle.Render("=== "+title+" ==="),
			dl.noEntriesStyle.Render("No open positions"),
		)
	}
	var b strings.Builder
	b.WriteString("\n" + dl.titleStyle.Render("=== "+title+" ===") + "\n")
	for _, ap := range list {
		b.WriteString(dl.FormatPosition(ap.Position) + "\n")
	}
	return b.String()
}

func (dl *DualLogger) FormatFill(f struct {
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
}) string {
	side := dl.styleSide(f.Side, false)
	px := dl.stylePrice(f.Px)
	sz := dl.styleSize(f.Sz)
	pnl := fmt.Sprintf("%.4f", f.ClosedPnl)
	if f.ClosedPnl > 0 {
		pnl = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("+" + pnl)
	} else if f.ClosedPnl < 0 {
		pnl = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(pnl)
	}
	return fmt.Sprintf(
		"%s coin:%s px:%s sz:%s side:%s closedPnl:%s fee:%.4f feeToken:%s time:%s",
		dl.boldStyle.Render("Fill"),
		f.Coin,
		px,
		sz,
		side,
		pnl,
		f.Fee,
		f.FeeToken,
		dl.styleTimestamp(f.Time),
	)
}

func (dl *DualLogger) FormatFillList(fills []struct {
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
}, title string) string {
	if len(fills) == 0 {
		return fmt.Sprintf("\n%s\n   %s\n",
			dl.titleStyle.Render("=== "+title+" ==="),
			dl.noEntriesStyle.Render("No fills"),
		)
	}
	var b strings.Builder
	b.WriteString("\n" + dl.titleStyle.Render("=== "+title+" ===") + "\n")
	for _, fill := range fills {
		b.WriteString(dl.FormatFill(fill) + "\n")
	}
	return b.String()
}

func (dl *DualLogger) FormatActiveAssetData(a models.UserAssetData) string {
	lev := fmt.Sprintf("%.2f", a.Leverage.Value)
	var maxTrades []string
	for _, mv := range a.MaxTradeSzs {
		maxTrades = append(maxTrades, fmt.Sprintf("%.2f", mv))
	}
	var avail []string
	for _, av := range a.AvailableToTrade {
		avail = append(avail, fmt.Sprintf("%.2f", av))
	}
	return fmt.Sprintf(
		"%s user:%s coin:%s\n  Leverage: %s\n  MaxTradeSzs: %v\n  AvailableToTrade: %v\n",
		dl.boldStyle.Render("ActiveAssetData"),
		a.User, a.Coin,
		lev,
		maxTrades,
		avail,
	)
}

func (dl *DualLogger) FormatOrderUpdate(up models.OrderUpdate) string {
	lower := strings.ToLower(up.Status)
	var s string
	switch lower {
	case "open":
		s = dl.sideBuyStyle.Render("OPEN")
	case "filled":
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("44")).Bold(true).Render("FILLED")
	case "canceled", "rejected", "margincanceled":
		s = dl.sideSellStyle.Render(up.Status)
	default:
		s = dl.faintStyle.Render(up.Status)
	}
	return fmt.Sprintf(
		"Status:%s @%s => %s",
		s,
		dl.styleTimestamp(up.StatusTimestamp),
		dl.FormatOrder(up.Order),
	)
}

func (dl *DualLogger) FormatOrderUpdatesList(updates []models.OrderUpdate, title string) string {
	if len(updates) == 0 {
		return fmt.Sprintf("\n%s\n   %s\n",
			dl.titleStyle.Render("=== "+title+" ==="),
			dl.noEntriesStyle.Render("No updates"),
		)
	}
	var b strings.Builder
	b.WriteString("\n" + dl.titleStyle.Render("=== "+title+" ===") + "\n")
	for _, u := range updates {
		b.WriteString(dl.FormatOrderUpdate(u) + "\n")
	}
	return b.String()
}

func (dl *DualLogger) FancyBox(s string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(s)
}
