package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/charmbracelet/lipgloss"
	"github.com/itay747/hyperformance/models"
	"github.com/itay747/hyperformance/ws"
)

var (
	positionsMemoryCache = make(map[string]PositionMemory)
)

type PositionMemory struct {
	MarkPrice float64
	PnL       float64
}

type PositionsRenderer interface {
	RenderPane(isCopy bool, positionsByCoin map[string]models.Position, renderWidth int) string
}

type ColumnSpec struct {
	Title   string
	Ratio   float64
	Align   lipgloss.Position
	ColorFn func(position models.Position) lipgloss.Color
	ValueFn func(position models.Position, positionKey string) string
}

type DefaultPositionsRenderer struct {
	columns    []ColumnSpec
	manager    *ws.Manager
	minWidth   int
	titleStyle lipgloss.Style
	rowStyle   lipgloss.Style
}

func NewPositionsRenderer(manager *ws.Manager) *DefaultPositionsRenderer {
	return &DefaultPositionsRenderer{
		minWidth:   70,
		manager:    manager,
		titleStyle: DefaultStyle.Bold(true),
		rowStyle:   DefaultStyle,
		columns: []ColumnSpec{
			{
				Title: "Coin",
				Ratio: 0.15,
				Align: lipgloss.Left,
				ColorFn: func(position models.Position) lipgloss.Color {
					if position.Szi < 0 {
						return lipgloss.Color("196")
					}
					return lipgloss.Color("46")
				},
				ValueFn: func(position models.Position, positionKey string) string {

					// If this is the "PASTE" pane, append italic virtual leverage
					leverage := float64(position.Leverage.Value)
					if strings.HasPrefix(strings.ToUpper(positionKey), "PASTE:") {
						leverage *= manager.CoinRiskMap[position.Coin]
					}
					leverageStr := fmt.Sprintf("%vx %s", int(leverage), position.Coin)
					return leverageStr
				},
			},
			// {
			// 	Title: "Value",
			// 	Ratio: 0.10,
			// 	Align: lipgloss.Left,
			// 	ColorFn: func(position models.Position) lipgloss.Color {
			// 		if position.Szi < 0 {
			// 			return lipgloss.Color("196")
			// 		}
			// 		return lipgloss.Color("46")
			// 	},
			// 	ValueFn: func(position models.Position, positionKey string) string {

			// 		value := position.PositionValue
			// 		return fmt.Sprintf("$%.2f", value)
			// 	},
			// },
			{
				Title: "Margin %",
				Ratio: 0.15,
				Align: lipgloss.Left,
				ColorFn: func(position models.Position) lipgloss.Color {
					if position.MarginUsed < 0 {
						return lipgloss.Color("196")
					}
					return lipgloss.Color("#b2b2b2")
				},
				ValueFn: func(position models.Position, positionKey string) string {
					marginVal := position.MarginUsed
					var accountVal float64
					if strings.HasPrefix(strings.ToUpper(positionKey), "COPY") {
						accountVal = manager.CopyWd2.AccountValue()
					} else {
						accountVal = manager.PasteWd2.AccountValue()
					}
					var pct float64
					if accountVal > 0 {
						pct = (marginVal / accountVal) * 100
					}
					return fmt.Sprintf("%.1f%%", pct)
				},
			},
			// {
			// 	Title: "Margin",
			// 	Ratio: 0.125,
			// 	Align: lipgloss.Right,
			// 	ColorFn: func(position models.Position) lipgloss.Color {
			// 		return lipgloss.Color("#b2b2b2")
			// 	},
			// 	ValueFn: func(position models.Position, positionKey string) string {
			// 		marginVal := position.MarginUsed

			// 		return fmt.Sprintf("$%.2f", marginVal)
			// 	},
			// },
			{
				Title: "PnL",
				Ratio: 0.15,
				Align: lipgloss.Left,
				ColorFn: func(position models.Position) lipgloss.Color {
					if position.UnrealizedPnl < 0 {
						return lipgloss.Color("196")
					}
					return lipgloss.Color("46")
				},
				ValueFn: func(position models.Position, positionKey string) string {
					uPnL := position.UnrealizedPnl
					oldPnl, changed := handlePnLFlash(positionKey+"_pnl", position.UnrealizedPnl)
					if changed {
						if position.UnrealizedPnl > oldPnl {
							flashes.setFlash(positionKey+"_pnl", lipgloss.Color("46"))
						} else {
							flashes.setFlash(positionKey+"_pnl", lipgloss.Color("196"))
						}
					}
					return flashes.style(positionKey + "_pnl").
						Render(FormatSigned(uPnL, "$", ""))
				},
			},
			{
				Title: "RoE",
				Ratio: 0.15,
				Align: lipgloss.Left,
				ColorFn: func(position models.Position) lipgloss.Color {
					if position.UnrealizedPnl < 0 {
						return lipgloss.Color("196")
					}
					return lipgloss.Color("46")
				},
				ValueFn: func(position models.Position, positionKey string) string {
					sign := ""
					if position.UnrealizedPnl < 0 {
						sign = "-"
					} else {
						sign = "+"
					}
					returnOnEquity := math.Abs(position.ReturnOnEquity) * 100
					oldPnl, changed := handlePnLFlash(positionKey+"_roe", position.UnrealizedPnl)
					if changed {
						if position.UnrealizedPnl > oldPnl {
							flashes.setFlash(positionKey+"_roe", lipgloss.Color("46"))
						} else {
							flashes.setFlash(positionKey+"_roe", lipgloss.Color("196"))
						}
					}
					return flashes.style(positionKey + "_roe").
						Render(fmt.Sprintf("%s%.2f%%", sign, returnOnEquity))
				},
			},
			{
				Title: "Entry Px",
				Ratio: 0.125,
				Align: lipgloss.Left,
				ColorFn: func(_ models.Position) lipgloss.Color {
					return lipgloss.Color("244")
				},
				ValueFn: func(position models.Position, _ string) string {
					decimals := manager.Decimals(position.Coin)
					return "$" + hl.PriceToWire(position.EntryPx, 6, decimals)
				},
			},
			{
				Title: "Mid Px",
				Ratio: 0.125,
				Align: lipgloss.Right,
				ColorFn: func(position models.Position) lipgloss.Color {
					price := manager.GetMidPrice(position.Coin)
					if price == 0 {
						return lipgloss.Color("244")
					}
					if position.UnrealizedPnl < 0 {
						return lipgloss.Color("196")
					} else if position.UnrealizedPnl > 0 {
						return lipgloss.Color("46")
					}
					return lipgloss.Color("244")
				},
				ValueFn: func(position models.Position, positionKey string) string {
					price := manager.GetMidPrice(position.Coin)
					if price == 0 {
						return "$0 (0.00%)"
					}
					//deltaPercent := 0 //getMarkDeltaPercent(position.EntryPx, price)
					// sign := ""
					// if deltaPercent > 0 {
					// 	sign = "+"
					// } else if deltaPercent < 0 {
					// 	sign = "-"
					// }
					oldMarkValue, changed := handleMarkPriceFlash(positionKey+"_mark", price)
					if changed {
						if price > oldMarkValue {
							flashes.setFlash(positionKey+"_mark", lipgloss.Color("46"))
						} else {
							flashes.setFlash(positionKey+"_mark", lipgloss.Color("196"))
						}
					}
					decimals := manager.Decimals(position.Coin)
					formattedPrice := hl.PriceToWire(price, 6, decimals)
					return flashes.style(positionKey + "_mark").
						Render(fmt.Sprintf("$%s", formattedPrice))
				},
			},
		},
	}
}
func FormatSigned(val float64, prefix, suffix string) string {
	var sign string
	if val < 0 {
		sign = "-"
	} else {
		sign = "+"
	}
	var valStr string
	if prefix == "$" {
		valStr = fmt.Sprintf("%.2f", math.Abs(val))
	} else {
		valStr = strconv.FormatFloat(math.Abs(val), 'f', -1, 64)
	}
	return fmt.Sprintf("%s%s%s%s", sign, prefix, valStr, suffix)

}
func (renderer *DefaultPositionsRenderer) RenderPane(
	isCopy bool,
	assetPositions map[string]models.Position,
	renderWidth int,
) string {
	paneLabel := "PASTE"
	if isCopy {
		paneLabel = "COPY"
	}
	if len(assetPositions) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			renderer.titleStyle.Align(lipgloss.Center).Render(paneLabel),
			"No positions",
		)
	}
	if renderWidth < renderer.minWidth {
		renderWidth = renderer.minWidth
	}
	header := renderer.renderHeader(renderWidth)
	body := renderer.renderRows(paneLabel, assetPositions, renderWidth)
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (renderer *DefaultPositionsRenderer) renderHeader(totalWidth int) string {
	var headerCells []string
	usedWidth := 0
	for i, column := range renderer.columns {
		isLast := i == len(renderer.columns)-1
		columnWidth := int(math.Round(float64(totalWidth) * column.Ratio))
		if isLast {
			columnWidth = totalWidth - usedWidth
		}
		if columnWidth < 5 {
			columnWidth = 5
		}
		usedWidth += columnWidth
		title := DefaultStyle.
			Bold(true).
			Align(column.Align + 3).
			Width(columnWidth).
			Render(strings.ToUpper(column.Title))
		headerCells = append(headerCells, title)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)
}

func (renderer *DefaultPositionsRenderer) renderRows(
	paneLabel string,
	positionsByCoin map[string]models.Position,
	totalWidth int,
) string {
	columnWidths := make([]int, len(renderer.columns))
	usedWidth := 0
	for i, column := range renderer.columns {
		isLast := i == len(renderer.columns)-1
		columnWidth := int(math.Round(float64(totalWidth) * column.Ratio))
		if isLast {
			columnWidth = totalWidth - usedWidth
		}
		if columnWidth < 5 {
			columnWidth = 5
		}
		usedWidth += columnWidth
		columnWidths[i] = columnWidth
	}
	// if _, ok := copySet[cloid]; !ok {
	// 		if eng.canceledCloids[cloid] {
	// 			continue
	// 		}
	// 		results = append(results, pasteOrd)
	// 		eng.canceledCloids[cloid] = true
	// 	}
	var rows []string
	for _, coin := range renderer.manager.AllowedSymbols {
		positionKey := fmt.Sprintf("%s:%s", paneLabel, coin)
		position, ok := positionsByCoin[coin]

		if !ok {
			rows = append(rows, "-")
			continue
		}
		var cellsForRow []string
		for i, column := range renderer.columns {
			columnWidth := columnWidths[i]
			value := column.ValueFn(position, positionKey)
			styledValue := renderer.rowStyle.
				Foreground(column.ColorFn(position)).
				Align(column.Align + 3).
				Width(columnWidth).
				Render(value)
			cellsForRow = append(cellsForRow, styledValue)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cellsForRow...))
	}
	return strings.Join(rows, "\n")

}

func getMarkPrice(position models.Position) float64 {
	if position.Szi == 0 {
		return 0
	}
	return math.Abs(position.PositionValue / position.Szi)
}

func handleMarkPriceFlash(key string, newMarkPrice float64) (float64, bool) {
	positionMemory, found := positionsMemoryCache[key]
	if !found {
		positionsMemoryCache[key] = PositionMemory{MarkPrice: newMarkPrice}
		return 0, false
	}
	oldMarkPrice := positionMemory.MarkPrice
	if newMarkPrice != oldMarkPrice {
		positionMemory.MarkPrice = newMarkPrice
		positionsMemoryCache[key] = positionMemory
		return oldMarkPrice, true
	}
	return oldMarkPrice, false
}

func handlePnLFlash(key string, newPnl float64) (float64, bool) {
	positionMemory, found := positionsMemoryCache[key]
	if !found {
		positionsMemoryCache[key] = PositionMemory{PnL: newPnl}
		return 0, false
	}
	oldPnl := positionMemory.PnL
	if newPnl != oldPnl {
		positionMemory.PnL = newPnl
		positionsMemoryCache[key] = positionMemory
		return oldPnl, true
	}
	return oldPnl, false
}
