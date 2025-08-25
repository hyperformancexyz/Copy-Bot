package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	"github.com/charmbracelet/lipgloss"
	"github.com/itay747/hyperformance/ws"
)

type OrdersRenderer interface {
	RenderPane(isCopy bool, orders map[int]hl.Order, copyOrders map[int]hl.Order, width int) string
}

type OrderColumnSpec struct {
	Title   string
	Ratio   float64
	Align   lipgloss.Position
	ValueFn func(order hl.Order, isCopy bool) string
}

type DefaultOrdersRenderer struct {
	manager  *ws.Manager
	columns  []OrderColumnSpec
	minWidth int
	titleSty lipgloss.Style
	rowSty   lipgloss.Style
}

func NewOrdersRenderer(manager *ws.Manager) *DefaultOrdersRenderer {
	return &DefaultOrdersRenderer{
		manager:  manager,
		minWidth: 50,
		titleSty: DefaultStyle.Bold(true),
		rowSty:   DefaultStyle,
		columns: []OrderColumnSpec{
			{
				Title: "Time",
				Ratio: 0.2,
				Align: lipgloss.Left,
				ValueFn: func(order hl.Order, isCopy bool) string {
					if order.Timestamp <= 0 {
						return "-"
					}
					orderTime := time.UnixMilli(order.Timestamp)
					format := "15:04:05.000"
					return orderTime.Format(format)
				},
			},
			{
				Title: "Coin",
				Ratio: 0.1,
				Align: lipgloss.Left,
				ValueFn: func(o hl.Order, isCopy bool) string {
					if o.Coin == "" {
						return "-"
					}
					return strings.ToUpper(o.Coin)
				},
			},
			{
				Title: "Direction",
				Ratio: 0.15,
				Align: lipgloss.Right,
				ValueFn: func(o hl.Order, isCopy bool) string {
					dir := manager.OrderDir(o, manager.CopyWd2)
					if !isCopy {
						dir = manager.OrderDir(o, manager.PasteWd2)
					}
					return dir
				},
			},
			{
				Title: "Value",
				Ratio: 0.15,
				Align: lipgloss.Right,
				ValueFn: func(o hl.Order, isCopy bool) string {
					val := o.OrigSz * o.LimitPx

					// var accountVal float64
					// if strings.HasPrefix(strings.ToUpper(key), "COPY") {
					// 	accountVal = manager.CopyWd2.AccountValue()
					// } else {
					// 	accountVal = manager.PasteWd2.AccountValue()
					// }
					// var pct float64
					// if accountVal > 0 {
					// 	pct = (val / (accountVal)) * 100
					// }
					return fmt.Sprintf("$%.2f", val)
				},
			},
			{
				Title: "Price",
				Ratio: 0.15,
				Align: lipgloss.Right,
				ValueFn: func(o hl.Order, _ bool) string {
					decimals := manager.Decimals(o.Coin)
					priceStr := hl.PriceToWire(o.LimitPx, 6, decimals)
					return fmt.Sprintf("$%s", priceStr)
				},
			},
			{
				Title: "Cloid",
				Ratio: 0.2,
				Align: lipgloss.Right,
				ValueFn: func(o hl.Order, key bool) string {
					val, _ := hl.HexToInt(o.Cloid)
					return fmt.Sprintf("%v", val)
				},
			},
		},
	}
}

func (r *DefaultOrdersRenderer) RenderPane(isCopy bool, orders map[int]hl.Order, copyOrders map[int]hl.Order, width int) string {
	if width < r.minWidth {
		width = r.minWidth
	}
	head := r.renderHeader(width)
	body := "-"
	if len(orders) != 0 {
		body = r.renderRows(isCopy, orders, copyOrders, width)
	}
	return lipgloss.JoinVertical(lipgloss.Left, head, body)
}

func (r *DefaultOrdersRenderer) renderHeader(totalWidth int) string {
	acc := 0
	var cells []string
	for i, col := range r.columns {
		last := i == len(r.columns)-1
		w := int(math.Round(float64(totalWidth) * col.Ratio))
		if last {
			w = totalWidth - acc
		}
		if w < 5 {
			w = 5
		}
		acc += w
		cells = append(cells,
			DefaultStyle.
				Bold(true).
				Align(col.Align+3).
				Width(w).
				Render(strings.ToUpper(col.Title)),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cells...)
}

func (r *DefaultOrdersRenderer) renderRows(isCopy bool, orders map[int]hl.Order, copyOrders map[int]hl.Order, totalWidth int) string {
	acc := 0
	colWidths := make([]int, len(r.columns))

	for i, c := range r.columns {
		last := i == len(r.columns)-1
		w := int(math.Round(float64(totalWidth) * c.Ratio))
		if last {
			w = totalWidth - acc
		}
		if w < 5 {
			w = 5
		}
		acc += w
		colWidths[i] = w
	}
	var cloids []string
	for _, order := range orders {
		cloids = append(cloids, order.Cloid)
	}
	sort.Slice(cloids, func(i, j int) bool {
		return CloidFull(cloids[i]) < CloidFull(cloids[j])
	})

	var order hl.Order
	var lines []string
	for _, cloid := range cloids {
		order = orders[CloidTruncated(cloid)]
		address := r.manager.CopyAddress
		if !isCopy {
			address = r.manager.PasteAddress
		}
		var dir string
		if address == r.manager.CopyAddress {
			dir = r.manager.OrderDir(order, r.manager.CopyWd2)
		} else {
			dir = r.manager.OrderDir(order, r.manager.PasteWd2)
		}
		isClose := strings.HasPrefix(dir, "Close")
		sideIsBuy := strings.ToUpper(order.Side) == "B"
		lineStyle := DefaultStyle
		if sideIsBuy {
			lineStyle = lineStyle.Foreground(lipgloss.Color("2"))
		} else {
			lineStyle = lineStyle.Foreground(lipgloss.Color("1"))
		}
		if isClose {
			lineStyle = lineStyle.UnderlineSpaces(false).Underline(true)
		}

		var rowCells []string
		for i, col := range r.columns {
			val := col.ValueFn(order, isCopy)
			cell := lineStyle.
				Align(col.Align + 3).
				Width(colWidths[i])
			if !isCopy {
				if _, ok := copyOrders[CloidTruncated(order.Cloid)]; !ok {
					cell = cell.Reverse(true)
				}
			}
			rowCells = append(rowCells, cell.Render(val))

		}
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, rowCells...))
	}
	return strings.Join(lines, "\n")
}
