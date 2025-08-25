package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	hl "github.com/Logarithm-Labs/go-hyperliquid/hyperliquid"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/itay747/hyperformance/models"
	"github.com/itay747/hyperformance/ws"
)

type tickMsg struct{}
type webData2Msg *models.WebData2Message
type l2BookSnapshotMsg *models.L2BookSnapshotMessage
type logMsg string

type TUIModel struct {
	manager           *ws.Manager
	copyPositionsMap  map[string]models.Position
	pastePositionsMap map[string]models.Position
	copyPositions     []hl.AssetPosition
	pastePositions    []hl.AssetPosition

	logChan            <-chan string
	l2BookSnapshotChan <-chan *models.L2BookSnapshotMessage

	splitLog        *SplitLog
	width           int
	height          int
	refreshInterval time.Duration

	copyOrders  map[string]hl.Order
	pasteOrders map[string]hl.Order

	lastCopyUpdate      time.Time
	lastPasteUpdate     time.Time
	prevCopyFunds       float64
	prevPasteFunds      float64
	prevCopyUnrealized  float64
	prevPasteUnrealized float64

	positionsRenderer PositionsRenderer
	ordersRenderer    OrdersRenderer
}

var (
	greenBorder         = lipgloss.Color("#149e82")
	greenAccent         = lipgloss.Color("#149e82")
	highlightText       = lipgloss.Color("#d7fcf0")
	DarkPanelBackground = lipgloss.Color("#151a1e")
	DefaultStyle        = lipgloss.NewStyle().Background(DarkPanelBackground)
	VerticalDivider     = DefaultStyle.
				BorderStyle(lipgloss.ThickBorder()).
				BorderLeft(true).
				BorderRight(false)

	titleBarStyle = DefaultStyle.
			Foreground(highlightText).
			Bold(true)

	statusBarStyle = DefaultStyle.
			Bold(true).
			Padding(0).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenAccent)

	logBoxStyle = DefaultStyle.
			PaddingLeft(2).
			PaddingRight(2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenBorder)

	containerStyle = DefaultStyle.
			Margin(0, 0).
			Padding(1, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(greenBorder)

	splitPaneStyle = DefaultStyle.Padding(0, 1)
)

func RunTUI(
	ctx context.Context,
	managerRef *ws.Manager,
	logStream <-chan string,
	refreshInterval time.Duration,
) error {

	tuiModel := newTUIModel(managerRef, logStream, refreshInterval)

	app := tea.NewProgram(
		tuiModel,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := app.Run()

	return err
}
func newTUIModel(manager *ws.Manager,
	logStream <-chan string,
	refreshInterval time.Duration) *TUIModel {
	split := NewSplitLog(
		manager.CopyAddress,
		manager.PasteAddress,
		0,
		0,
		logStream,
	)
	return &TUIModel{
		manager:           manager,
		logChan:           logStream,
		splitLog:          split,
		refreshInterval:   refreshInterval,
		lastCopyUpdate:    time.Now(),
		lastPasteUpdate:   time.Now(),
		positionsRenderer: NewPositionsRenderer(manager),
		ordersRenderer:    NewOrdersRenderer(manager),
	}
}

// func (tui *TUIModel) Start(
// 	ctx context.Context,
// 	copyWd2Stream <-chan *models.WebData2Message,
// 	pasteWd2Stream <-chan *models.WebData2Message,
// 	copyOrderUpdatesChan <-chan *models.OrderMessage,
// 	pasteOrderUpdatesChan <-chan *models.OrderMessage,
// ) {
// 	go func() {
// 		for {
// 			select {
// 			case <-ctx.Done():
// 				return
// 			case <-copyWd2Stream:
// 				continue
// 			case <-pasteWd2Stream:
// 				continue
// 			case <-copyOrderUpdatesChan:
// 				continue
// 			case <-pasteOrderUpdatesChan:
// 				continue

// 			}
// 		}
// 	}()
// }

func (tui *TUIModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(tui.refreshInterval),
		readLogCmd(tui.logChan),
	)
}

func (tui *TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		tui.width = typed.Width
		tui.height = typed.Height
		return tui, nil
	case tickMsg:
		return tui, tea.Batch(
			tickCmd(tui.refreshInterval),
			readLogCmd(tui.logChan),
		)
	case logMsg:
		return tui, tui.splitLog.UpdateLog(string(typed))
	case tea.KeyMsg:
		if typed.String() == "q" || typed.String() == "ctrl+c" {
			return tui, tea.Quit
		}
	}
	return tui, nil
}

func (tui *TUIModel) View() string {
	if !tui.manager.IsReady() {
		return "Waiting for data..."
	}
	titleBar := titleBarStyle.Render(" Hyperformance Printer v0.0.4 ")
	statusBar := tui.renderStatusBar()
	titleHeight := lipgloss.Height(titleBar)
	statusHeight := lipgloss.Height(statusBar)
	tui.handleWebData2(tui.manager.CopyWd2)
	tui.handleWebData2(tui.manager.PasteWd2)
	availableHeight := tui.height - (titleHeight + statusHeight)
	if availableHeight < 10 {
		availableHeight = 10
	}
	logHeight := 2 * availableHeight / 7
	remainingHeight := availableHeight - logHeight
	ordersHeight := remainingHeight / 2
	positionsHeight := len(tui.copyPositionsMap) + 3

	tui.splitLog.SetSize(tui.width-6, logHeight)

	logRegion := logBoxStyle.
		Width(tui.width - 2).
		Height(logHeight).
		Render(tui.splitLog.Render())

	ordersRegion := containerStyle.
		Width(tui.width - 2).
		Height(ordersHeight).
		Render(tui.renderOrders(ordersHeight))

	positionsRegion := containerStyle.
		Width(tui.width - 2).
		Height(positionsHeight).
		Render(tui.renderPositions(positionsHeight))

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		titleBar,
		logRegion,
		ordersRegion,
		positionsRegion,
		statusBar,
	)
	return body
}

func (tui *TUIModel) renderOrders(h int) string {
	copyOrders := filterOrdersByAllowedSymbols(tui.copyOrders, tui.manager)
	pasteOrders := filterOrdersByAllowedSymbols(tui.pasteOrders, tui.manager)
	left := tui.ordersRenderer.RenderPane(true, copyOrders, copyOrders, tui.width/2)
	right := tui.ordersRenderer.RenderPane(false, pasteOrders, copyOrders, tui.width/2)
	return tui.splitHorizontal(left, right, h-2)
}

func (tui *TUIModel) renderPositions(h int) string {
	if tui.manager.CopyWd2 == nil || tui.manager.PasteWd2 == nil {
		return "Waiting for data..."
	}

	copyPositions := filterPositionsByAllowedSymbols(tui.copyPositionsMap, tui.manager)
	pastePositions := filterPositionsByAllowedSymbols(tui.pastePositionsMap, tui.manager)
	left := tui.positionsRenderer.RenderPane(true, copyPositions, tui.width/2)
	right := tui.positionsRenderer.RenderPane(false, pastePositions, tui.width/2)
	return tui.splitHorizontal(left, right, h-2)
}

func (tui *TUIModel) splitHorizontal(leftContent, rightContent string, h int) string {
	cw := (tui.width - 8) / 2
	l := splitPaneStyle.Width(cw).Height(h).Render(leftContent)
	r := splitPaneStyle.Width(cw).Height(h).Render(rightContent)
	return lipgloss.JoinHorizontal(lipgloss.Top, l, VerticalDivider.Render(), r)
}

func (tui *TUIModel) handleWebData2(wd2 *models.WebData2Message) {
	tui.parsePositions(wd2)
	// Decide if it’s copy side or paste side
	if wd2.Data.User == tui.manager.CopyAddress && tui.manager.CopyWd2 != nil {
		tui.copyOrders = wd2.OrdersByCloid()
		tui.lastCopyUpdate = time.Now()
	} else if tui.manager.PasteWd2 != nil && wd2.Data.User == tui.manager.PasteAddress {
		tui.pasteOrders = wd2.OrdersByCloid()
		tui.lastPasteUpdate = time.Now()
	}

}
func (tui *TUIModel) parsePositions(wd2 *models.WebData2Message) {
	var fresh []hl.AssetPosition
	for _, e := range wd2.Data.ClearinghouseState.AssetPositions {
		fresh = append(fresh, hl.AssetPosition{
			Type: e.Type,
			Position: hl.Position{
				Coin:           e.Position.Coin,
				Szi:            e.Position.Szi,
				PositionValue:  e.Position.PositionValue,
				UnrealizedPnl:  e.Position.UnrealizedPnl,
				ReturnOnEquity: e.Position.ReturnOnEquity,
				MarginUsed:     e.Position.MarginUsed,
				Leverage:       e.Position.Leverage,
				EntryPx:        e.Position.EntryPx,
				LiquidationPx:  e.Position.LiquidationPx,
				MaxLeverage:    e.Position.MaxLeverage,
			},
		})
	}
	if wd2.Data.User == tui.manager.CopyAddress {
		tui.copyPositions = fresh
		tui.copyPositionsMap = wd2.PositionsByCoin()
	} else if wd2.Data.User == tui.manager.PasteAddress {
		tui.pastePositions = fresh
		tui.pastePositionsMap = wd2.PositionsByCoin()

	}
}

// func (tui *TUIModel) handleL2BookSnapshot(l2book *l2BookSnapshotMsg) tea.Cmd {
// 	book := l2book.Data.
// 	fmt.Printf("Book was %#+v", book)
// 	return readL2BookCmd(tui.l2BookSnapshotChan)
// }

func (tui *TUIModel) renderStatusBar() string {
	if tui.manager.CopyWd2 == nil || tui.manager.PasteWd2 == nil {
		return "Waiting for data..."
	}
	copyVal := tui.manager.CopyWd2.AccountValue()
	pasteVal := tui.manager.PasteWd2.AccountValue()
	copyUnreal := tui.sumUnrealized(tui.copyPositions)
	pasteUnreal := tui.sumUnrealized(tui.pastePositions)
	copyStr := flashDeltaWithPnl(copyVal, &tui.prevCopyFunds, copyUnreal, &tui.prevCopyUnrealized, "Copy", len(tui.copyPositionsMap) > 0)
	pasteStr := flashDeltaWithPnl(pasteVal, &tui.prevPasteFunds, pasteUnreal, &tui.prevPasteUnrealized, "Paste", len(tui.pastePositionsMap) > 0)
	leftText := fmt.Sprintf("%s - %.2fs ago - %v", tui.lastCopyUpdate.Format("15:04:05"), time.Since(tui.lastCopyUpdate).Seconds(), tui.manager.CopyWd2.N())
	rightText := fmt.Sprintf("%v - %.2fs ago - %s", tui.manager.PasteWd2.N(), time.Since(tui.lastPasteUpdate).Seconds(), tui.lastPasteUpdate.Format("15:04:05"))
	centerText := fmt.Sprintf("%s | %s", copyStr, pasteStr)
	barWidth := tui.width - 2
	if barWidth < 1 {
		barWidth = 1
	}
	leftWidth := lipgloss.Width(leftText) + 2
	rightWidth := lipgloss.Width(rightText) + 2
	centerWidth := barWidth - (leftWidth + rightWidth)
	if centerWidth < 0 {
		centerWidth = 0
	}
	leftBlock := DefaultStyle.Width(leftWidth).Align(lipgloss.Left).Render(leftText)
	centerBlock := DefaultStyle.Width(centerWidth).Align(lipgloss.Center).Render(centerText)
	rightBlock := DefaultStyle.Width(rightWidth).Align(lipgloss.Right).Render(rightText)
	statusLine := lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, centerBlock, rightBlock)

	return statusBarStyle.Width(barWidth).Render(statusLine)
}

func (tui *TUIModel) sumUnrealized(positions []hl.AssetPosition) float64 {
	var t float64
	for _, p := range positions {
		t += p.Position.UnrealizedPnl
	}
	return t
}

func tickCmd(interval time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(interval)
		return tickMsg{}
	}
}

func readLogCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		select {
		case line := <-ch:
			return logMsg(line)
		default:
			return nil
		}
	}
}

func readWebData2Cmd(ch <-chan *models.WebData2Message) tea.Cmd {
	return func() tea.Msg {
		select {
		case inc := <-ch:
			return webData2Msg(inc)
		default:
			return nil
		}
	}
}
func readL2BookCmd(ch <-chan *models.L2BookSnapshotMessage) tea.Cmd {
	return func() tea.Msg {
		select {
		case inc := <-ch:
			return l2BookSnapshotMsg(inc)
		default:
			return nil
		}
	}
}
func filterPositionsByAllowedSymbols(positions map[string]models.Position, mgr *ws.Manager) map[string]models.Position {
	filteredPositions := make(map[string]models.Position)
	for coin, p := range positions {
		if mgr.IsEnabledCoin(strings.ToUpper(p.Coin)) {
			filteredPositions[coin] = p
		}
	}
	return filteredPositions
}
func CloidFull(cloid string) int {
	cloidValue, _ := hl.HexToInt(cloid)
	return int(cloidValue.Int64())
}
func CloidTruncated(cloid string) int {
	cloidValue := CloidFull(cloid)
	numberStr := strconv.Itoa(cloidValue)
	if len(numberStr) < 2 {
		return 0
	}
	firstChar := numberStr[0]
	numberStr = numberStr[1:]
	numberStr = strings.TrimLeft(numberStr, "0")
	if numberStr == "" {
		return 0
	}
	numberStr = string(firstChar) + numberStr
	result, err := strconv.Atoi(numberStr)
	if err != nil {
		return 0
	}
	return result
}

func filterOrdersByAllowedSymbols(orders map[string]hl.Order, mgr *ws.Manager) map[int]hl.Order {
	filtered := make(map[int]hl.Order)
	for cloid, order := range orders {
		cloidValue := CloidTruncated(cloid)
		if dupedOrder, ok := filtered[cloidValue]; ok {
			if order.Coin == dupedOrder.Coin {
				msg := fmt.Sprintf("TUI meltdown attempting to render dupe cloid orders: \n%#+v\n%#+v", dupedOrder, order)
				panic(msg)
			}

		}
		if mgr.IsEnabledCoin(order.Coin) {
			filtered[cloidValue] = order
		}
	}
	return filtered
}
