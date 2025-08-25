package tui

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type flashEntry struct {
	color     lipgloss.Color
	startTime time.Time
	endTime   time.Time
	duration  time.Duration
}

type FlashTracker struct {
	sync.Mutex
	baseColor      lipgloss.Color
	callTimestamps []time.Time
	flashEntries   map[string]*flashEntry
}

var (
	defaultBaseColorHex  = "#cfd1d4"
	greenColorHex        = "#00af00"
	redColorHex          = "#d70000"
	minimumFlashDuration = 400 * time.Millisecond
	maximumFlashDuration = 800 * time.Millisecond
	flashHistoryLookback = 5 * time.Second
	flashes              = NewFlashTracker()
)

func NewFlashTracker() *FlashTracker {
	return &FlashTracker{
		baseColor:      lipgloss.Color(defaultBaseColorHex),
		flashEntries:   make(map[string]*flashEntry),
		callTimestamps: make([]time.Time, 0, 32),
	}
}

func (tracker *FlashTracker) setFlash(entryKey string, color lipgloss.Color) {
	now := time.Now()
	d := tracker.computeAdaptiveDuration(now)
	entry, exists := tracker.flashEntries[entryKey]
	if !exists {
		tracker.flashEntries[entryKey] = &flashEntry{
			color:     color,
			startTime: now,
			endTime:   now.Add(d),
			duration:  d,
		}
	} else {
		entry.color = color
		entry.startTime = now
		entry.endTime = now.Add(d)
		entry.duration = d
	}
	tracker.recordCall(now)
}

func (tracker *FlashTracker) style(entryKey string) lipgloss.Style {
	entry, found := tracker.flashEntries[entryKey]
	if !found {
		return DefaultStyle
	}
	now := time.Now()
	if now.After(entry.endTime) {
		delete(tracker.flashEntries, entryKey)
		return DefaultStyle
	}
	ratio := now.Sub(entry.startTime).Seconds() / entry.duration.Seconds()
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	eased := easeInOutCubic(ratio)
	blended := blendRGB(string(entry.color), string(tracker.baseColor), eased)
	return DefaultStyle.Foreground(lipgloss.Color(blended))
}

func (tracker *FlashTracker) computeAdaptiveDuration(now time.Time) time.Duration {
	tracker.pruneHistory(now)
	callRate := float64(len(tracker.callTimestamps)) / flashHistoryLookback.Seconds()
	scale := callRate * 0.5
	if scale > 1 {
		scale = 1
	}
	diff := float64(maximumFlashDuration - minimumFlashDuration)
	val := float64(maximumFlashDuration) - scale*diff
	return time.Duration(val)
}

func (tracker *FlashTracker) pruneHistory(now time.Time) {
	cutoff := now.Add(-flashHistoryLookback)
	idx := 0
	for _, ts := range tracker.callTimestamps {
		if ts.After(cutoff) {
			break
		}
		idx++
	}
	if idx > 0 && idx <= len(tracker.callTimestamps) {
		tracker.callTimestamps = tracker.callTimestamps[idx:]
	}
}

func (tracker *FlashTracker) recordCall(now time.Time) {
	tracker.callTimestamps = append(tracker.callTimestamps, now)
}

func easeInOutCubic(x float64) float64 {
	if x < 0.5 {
		return 4 * x * x * x
	}
	return 1 - math.Pow(-2*x+2, 3)/2
}

func blendRGB(fromHex, toHex string, fraction float64) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	r1, g1, b1 := parseHexColor(fromHex)
	r2, g2, b2 := parseHexColor(toHex)
	r := r1 + (r2-r1)*fraction
	g := g1 + (g2-g1)*fraction
	b := b1 + (b2-b1)*fraction
	return rgbToHexString(r, g, b)
}

func parseHexColor(hexStr string) (float64, float64, float64) {
	s := strings.TrimPrefix(hexStr, "#")
	if len(s) != 6 {
		return 0, 0, 0
	}
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return 0, 0, 0
	}
	return float64(decoded[0]), float64(decoded[1]), float64(decoded[2])
}

func rgbToHexString(r, g, b float64) string {
	return fmt.Sprintf("#%02x%02x%02x", clampColor(r), clampColor(g), clampColor(b))
}

func clampColor(v float64) int {
	val := int(math.Round(v))
	if val < 0 {
		val = 0
	} else if val > 255 {
		val = 255
	}
	return val
}

func flashDeltaWithPnl(curFunds float64, prevFunds *float64, curUnreal float64, prevUnreal *float64, label string, hasPositions bool) string {
	fundsKey := label + "_funds"
	unrealKey := label + "_unreal"
	if curFunds != *prevFunds {
		if curFunds > *prevFunds {
			flashes.setFlash(fundsKey, lipgloss.Color(greenColorHex))
		} else {
			flashes.setFlash(fundsKey, lipgloss.Color(redColorHex))
		}
	}
	if curUnreal != *prevUnreal {
		if curUnreal > *prevUnreal {
			flashes.setFlash(unrealKey, lipgloss.Color(greenColorHex))
		} else {
			flashes.setFlash(unrealKey, lipgloss.Color(redColorHex))
		}
	}
	*prevFunds = curFunds
	*prevUnreal = curUnreal
	fundsText := flashes.style(fundsKey).Render(fmt.Sprintf("$%.2f", curFunds))
	if !hasPositions {
		return fmt.Sprintf("%s: %s", label, fundsText)
	}
	sign := "+"
	val := curUnreal
	if val < 0 {
		sign = "-"
		val = -val
	}
	unrealText := flashes.style(unrealKey).Render(fmt.Sprintf("(%s$%.2f)", sign, val))
	return fmt.Sprintf("%s: %s %s", label, fundsText, unrealText)
}
