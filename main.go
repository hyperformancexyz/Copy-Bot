package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // registers pprof handlers
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/itay747/hyperformance/config"
	"github.com/itay747/hyperformance/models"
	"github.com/itay747/hyperformance/tui"
	"github.com/itay747/hyperformance/ws"
	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:                        "bot",
	SuggestionsMinimumDistance: 2,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := config.LoadConfigWithOverride(cfgFile)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		f, err := os.OpenFile("block.prof", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return fmt.Errorf("failed to open block.prof: %w", err)
		}
		defer f.Close()
		runtime.SetBlockProfileRate(1)

		// Expose pprof endpoints at http://localhost:6060/debug/pprof/
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
		runtime.SetBlockProfileRate(1)
		// run code here that might block
		pprof.Lookup("block").WriteTo(f, 0)
		logChannel := make(chan string, 10000)
		log.SetOutput(tui.ChannelLogWriter{LogChannel: logChannel})
		ctx, cancel := context.WithCancel(context.Background())
		manager := ws.NewManager(ctx)
		// One line to start one session that handles both copy and paste
		go manager.StartCopyTradingSession(ctx, logChannel)

		copyWd2Pipeline := ws.NewPipeline(manager.CopyWd2Chan)
		pasteWd2Pipeline := ws.NewPipeline(manager.PasteWd2Chan)
		orderUpdatesPipeline := ws.NewPipeline(manager.OrderUpdatesChan)

		copyTees := copyWd2Pipeline.Tee(2)
		pasteTees := pasteWd2Pipeline.Tee(2)

		iocCopyWd2Chan := copyTees[0]
		iocPasteWd2Chan := pasteTees[0]

		aloCopyWd2Chan := copyTees[1]
		aloPasteWd2Chan := pasteTees[1]

		l2BookTUIChan := make(chan *models.L2BookSnapshotMessage)

		manager.StartLogging(10000)
		oldAddLog := manager.AddLogFunc
		manager.AddLogFunc = func(addr, message string) {
			ts := time.Now().Format("15:04:05")
			enriched := fmt.Sprintf("[%s] %s", ts, message)
			oldAddLog(addr, enriched)
		}

		manager.AloEngine.Start(ctx,
			aloCopyWd2Chan.Out(),
			aloPasteWd2Chan.Out(),
			l2BookTUIChan)
		manager.IocEngine.Start(ctx,
			iocCopyWd2Chan.Out(),
			iocPasteWd2Chan.Out(),
			orderUpdatesPipeline.Out())

		go pollLogs(manager, logChannel)

		if err := tui.RunTUI(
			ctx,
			manager,
			logChannel,
			25*time.Millisecond,
		); err != nil {
			fmt.Println("Error in TUI:", err)
		}
		cancel()
		return nil
	},
}

// func init() {
// 	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", "", "Path to config file")
// 	rootCmd.RegisterFlagCompletionFunc("config", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
// 		matchesCur, _ := filepath.Glob(filepath.Join(".", toComplete+"*.json"))
// 		matchesPar, _ := filepath.Glob(filepath.Join("..", toComplete+"*.json"))
// 		candidates := append(matchesCur, matchesPar...)
// 		var valid []string
// 		for _, c := range candidates {
// 			info, err := os.Stat(c)
// 			if err == nil && !info.IsDir() {
// 				_, e := config.ParseConfig(c)
// 				if e == nil {
// 					valid = append(valid, c)
// 				}
// 			}
// 		}
// 		return valid, cobra.ShellCompDirectiveNoFileComp
// 	})
// }

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func pollLogs(managerRef *ws.Manager, output chan<- string) {
	t := time.NewTicker((1000 / 200) * time.Millisecond)
	defer t.Stop()
	for range t.C {
		copyLines := managerRef.GetLogs(managerRef.CopyAddress, 1)
		if len(copyLines) > 0 {
			output <- copyLines[len(copyLines)-1]
		}
		pasteLines := managerRef.GetLogs(managerRef.PasteAddress, 1)
		if len(pasteLines) > 0 {
			output <- pasteLines[len(pasteLines)-1]
		}
	}
}
