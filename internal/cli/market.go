package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/market"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var auctionCmd = &cobra.Command{
	Use:   "auction",
	Short: "Manage market auctions for task allocation",
}

var auctionOpenCmd = &cobra.Command{
	Use:   "open [task-id]",
	Short: "Open a new auction for a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		strategy, _ := cmd.Flags().GetString("strategy")
		if strategy == "" {
			strategy = string(market.StrategyLowestCost)
		}

		store, cleanup, err := openMarketStore()
		if err != nil {
			return err
		}
		defer cleanup()

		id, err := store.Open(context.Background(), args[0], market.Strategy(strategy))
		if err != nil {
			return err
		}
		fmt.Printf("Auction opened: %s (strategy=%s, task=%s)\n", id, strategy, args[0])
		return nil
	},
}

var auctionCloseCmd = &cobra.Command{
	Use:   "close [auction-id]",
	Short: "Close an auction — picks a winner using the stored strategy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, cleanup, err := openMarketStore()
		if err != nil {
			return err
		}
		defer cleanup()

		ctx := context.Background()
		bids, err := store.Bids(ctx, args[0])
		if err != nil {
			return err
		}
		if len(bids) == 0 {
			return fmt.Errorf("no bids on auction %s", args[0])
		}

		strategyStr, _ := cmd.Flags().GetString("strategy")
		if strategyStr == "" {
			strategyStr = string(market.StrategyLowestCost)
		}
		winner, err := market.NewAuction(nil).SelectWinner(bids, market.Strategy(strategyStr))
		if err != nil {
			return err
		}
		if err := store.Close(ctx, args[0], winner.ID); err != nil {
			return err
		}
		fmt.Printf("Auction closed. Winner: %s (price=%.4f, bid=%s)\n", winner.AgentName, winner.Price, winner.ID)
		return nil
	},
}

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage agent token balances",
}

var walletBalanceCmd = &cobra.Command{
	Use:   "balance [agent-name]",
	Short: "Show token balance for an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, cleanup, err := openMarketStore()
		if err != nil {
			return err
		}
		defer cleanup()
		bal, err := store.Balance(context.Background(), args[0])
		if err != nil {
			return err
		}
		fmt.Printf("%s: %.2f tokens\n", args[0], bal)
		return nil
	},
}

var walletCreditCmd = &cobra.Command{
	Use:   "credit [agent-name] [amount]",
	Short: "Credit tokens to an agent",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		amt, err := strconv.ParseFloat(args[1], 64)
		if err != nil {
			return err
		}
		store, cleanup, err := openMarketStore()
		if err != nil {
			return err
		}
		defer cleanup()
		if err := store.Credit(context.Background(), args[0], amt); err != nil {
			return err
		}
		fmt.Printf("Credited %.2f tokens to %s\n", amt, args[0])
		return nil
	},
}

func openMarketStore() (*market.Store, func(), error) {
	cfg, err := config.Load("hive.yaml")
	if err != nil {
		return nil, nil, err
	}
	st, err := storage.Open(cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}
	return market.NewStore(st.DB), func() { st.Close() }, nil
}

func init() {
	auctionOpenCmd.Flags().String("strategy", "", "selection strategy (lowest-cost|fastest|best-reputation)")
	auctionCloseCmd.Flags().String("strategy", "", "override selection strategy")

	auctionCmd.AddCommand(auctionOpenCmd)
	auctionCmd.AddCommand(auctionCloseCmd)

	walletCmd.AddCommand(walletBalanceCmd)
	walletCmd.AddCommand(walletCreditCmd)

	rootCmd.AddCommand(auctionCmd)
	rootCmd.AddCommand(walletCmd)
}
