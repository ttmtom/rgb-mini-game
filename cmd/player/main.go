package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"rgb-game/config"
	"rgb-game/pkg/crypto"
	"rgb-game/pkg/logger"
	"rgb-game/pkg/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

// playerAlias returns the alias passed as the first CLI argument, or "" if none.
func playerAlias() string {
	if len(os.Args) > 1 {
		return strings.TrimSpace(os.Args[1])
	}
	return ""
}

func main() {
	logger.Init()

	// ── Configuration ───────────────────────────────────────────────────
	cfg, err := config.InitPlayerFullConfig()
	if err != nil {
		logger.Fatalf("failed to initialize config: %v", err)
	}
	pc := cfg.PlayerConfig

	// Override key path when an alias is provided as the first CLI argument.
	alias := playerAlias()
	if alias != "" {
		pc.PlayerKeyPath = fmt.Sprintf(".key/player_%s_ed25519", alias)
	}

	// ── Player keypair ──────────────────────────────────────────────────
	keypair, err := crypto.LoadOrGenerateKey(pc.PlayerKeyPath)
	if err != nil {
		logger.Fatalf("failed to load/generate player keypair: %v", err)
	}
	playerID := crypto.PubKeyToPlayerID(keypair.PublicKey)
	if alias != "" {
		fmt.Printf("Player Alias : %s\n", alias)
	}
	fmt.Printf("Player ID    : %s\n", playerID)

	// ── gRPC connections ────────────────────────────────────────────────
	ledgerConn, err := grpc.NewClient(pc.LedgerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("failed to connect to Ledger at %s: %v", pc.LedgerAddr, err)
	}
	defer ledgerConn.Close()

	serverConn, err := grpc.NewClient(pc.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatalf("failed to connect to Game Server at %s: %v", pc.ServerAddr, err)
	}
	defer serverConn.Close()

	ledgerClient := pb.NewLedgerServiceClient(ledgerConn)
	gameClient := pb.NewGameServiceClient(serverConn)

	// ── Interactive menu loop ────────────────────────────────────────────
	scanner := bufio.NewScanner(os.Stdin)
	var activeMissionID string

	for {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Println("  RGB Mini-Game Player CLI")
		fmt.Println("========================================")
		fmt.Println("  1. Get Balance")
		fmt.Println("  2. Request Mission")
		fmt.Println("  3. Complete Mission")
		fmt.Println("  4. Transfer")
		fmt.Println("  5. Transaction History")
		fmt.Println("  6. Chain Stats")
		fmt.Println("  7. Block Explorer")
		fmt.Println("  8. Quit")
		fmt.Print("Select option: ")

		if !scanner.Scan() {
			break
		}
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			doGetBalance(ledgerClient, playerID)

		case "2":
			activeMissionID = doRequestMission(gameClient, playerID, scanner)

		case "3":
			if activeMissionID == "" {
				fmt.Print("Enter mission ID: ")
				if scanner.Scan() {
					activeMissionID = strings.TrimSpace(scanner.Text())
				}
			}
			if activeMissionID != "" {
				doCompleteMission(gameClient, activeMissionID, playerID)
				activeMissionID = ""
			}

		case "4":
			doTransfer(ledgerClient, playerID, keypair, scanner)

		case "5":
			doTxHistory(ledgerClient, playerID, scanner)

		case "6":
			doChainStats(ledgerClient)

		case "7":
			doBlockExplorer(ledgerClient, scanner)

		case "8":
			fmt.Println("Goodbye!")
			return

		default:
			fmt.Println("Unknown option. Please enter 1–8.")
		}
	}
}

// doGetBalance fetches and prints the player's current balance.
func doGetBalance(client pb.LedgerServiceClient, playerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: playerID})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	printBalance(resp)
}

// doRequestMission requests a new mission and starts a live countdown.
// Returns the mission ID on success, or "" on failure.
func doRequestMission(client pb.GameServiceClient, playerID string, scanner *bufio.Scanner) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.RequestMission(ctx, &pb.RequestMissionRequest{PlayerId: playerID})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return ""
	}

	fmt.Printf("Mission ID    : %s\n", resp.GetMissionId())
	fmt.Printf("Difficulty    : %s\n", resp.GetDifficulty().String())
	fmt.Printf("Reward        : RED+%d  GREEN+%d  BLUE+%d\n",
		resp.GetRewardRed(), resp.GetRewardGreen(), resp.GetRewardBlue())
	fmt.Printf("Cooldown      : %d seconds\n", resp.GetCooldownSeconds())

	cooldown := int(resp.GetCooldownSeconds())
	if cooldown > 0 {
		fmt.Println("Counting down — press Enter when the countdown finishes to proceed.")
		done := make(chan struct{})
		go func() {
			for remaining := cooldown; remaining > 0; remaining-- {
				fmt.Printf("\r  Cooldown: %d s remaining   ", remaining)
				time.Sleep(time.Second)
			}
			fmt.Printf("\r  Cooldown complete!              \n")
			close(done)
		}()

		// Block until user presses Enter (or countdown finishes, whichever is later)
		scanner.Scan()
		<-done
	}

	return resp.GetMissionId()
}

// doCompleteMission completes a mission and prints the result.
func doCompleteMission(client pb.GameServiceClient, missionID, playerID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.CompleteMission(ctx, &pb.CompleteMissionRequest{
		MissionId: missionID,
		PlayerId:  playerID,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if !resp.GetSuccess() {
		fmt.Printf("Mission failed: %s\n", resp.GetErrorMessage())
		return
	}

	fmt.Printf("Mission completed! TX Hash: %s\n", resp.GetTxHash())
	if resp.GetNewBalance() != nil {
		printBalance(resp.GetNewBalance())
	}
}

// doTransfer performs a peer-to-peer TRANSFER transaction.
func doTransfer(client pb.LedgerServiceClient, playerID string, keypair *crypto.Keypair, scanner *bufio.Scanner) {
	fmt.Print("Receiver player ID: ")
	if !scanner.Scan() {
		return
	}
	receiverID := strings.TrimSpace(scanner.Text())
	if receiverID == "" {
		fmt.Println("Receiver ID cannot be empty.")
		return
	}

	red, ok := promptUint32(scanner, "Amount RED  : ")
	if !ok {
		return
	}
	green, ok := promptUint32(scanner, "Amount GREEN: ")
	if !ok {
		return
	}
	blue, ok := promptUint32(scanner, "Amount BLUE : ")
	if !ok {
		return
	}

	// Fetch current nonce
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	balResp, err := client.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: playerID})
	if err != nil {
		fmt.Printf("Failed to fetch nonce: %v\n", err)
		return
	}
	nonce := balResp.GetNextNonce()

	// Build and sign the transaction payload
	payload := &pb.TransactionPayload{
		Type:        pb.TransactionPayload_TRANSFER,
		SenderId:    playerID,
		ReceiverId:  receiverID,
		AmountRed:   red,
		AmountGreen: green,
		AmountBlue:  blue,
		Nonce:       nonce,
	}

	rawPayload, err := proto.Marshal(payload)
	if err != nil {
		fmt.Printf("Failed to marshal payload: %v\n", err)
		return
	}

	signature := crypto.Sign(keypair.PrivateKey, rawPayload)

	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	txResp, err := client.SubmitTransaction(ctx2, &pb.SubmitTransactionRequest{
		RawPayload:   rawPayload,
		Signature:    signature,
		SenderPubKey: keypair.PublicKey,
	})
	if err != nil {
		fmt.Printf("Transfer failed: %v\n", err)
		return
	}

	fmt.Println("Transfer successful!")
	if txResp.GetNewBalance() != nil {
		printBalance(txResp.GetNewBalance())
	}
}

// printBalance pretty-prints a BalanceResponse.
func printBalance(b *pb.BalanceResponse) {
	fmt.Printf("Balance — RED: %d  GREEN: %d  BLUE: %d  (next nonce: %d)\n",
		b.GetRed(), b.GetGreen(), b.GetBlue(), b.GetNextNonce())
}

// promptUint32 reads a line from the scanner, parses it as uint32.
func promptUint32(scanner *bufio.Scanner, prompt string) (uint32, bool) {
	fmt.Print(prompt)
	if !scanner.Scan() {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(scanner.Text()), 10, 32)
	if err != nil {
		fmt.Printf("Invalid number: %v\n", err)
		return 0, false
	}
	return uint32(v), true
}

// doTxHistory fetches and prints paginated transaction history for the player.
func doTxHistory(client pb.LedgerServiceClient, playerID string, scanner *bufio.Scanner) {
	fmt.Print("Limit (default 10): ")
	var limit uint32 = 10
	if scanner.Scan() {
		if v, err := strconv.ParseUint(strings.TrimSpace(scanner.Text()), 10, 32); err == nil && v > 0 {
			limit = uint32(v)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.GetTransactionHistory(ctx, &pb.GetTransactionHistoryRequest{
		PlayerId: playerID,
		Limit:    limit,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Transaction History (total=%d)\n", resp.GetTotal())
	fmt.Println(strings.Repeat("-", 80))
	for _, t := range resp.GetTransactions() {
		txType := "TRANSFER"
		if t.GetType() == 1 {
			txType = "MINT"
		}
		blockStr := fmt.Sprintf("block %d", t.GetBlockHeight())
		if t.GetBlockHeight() < 0 {
			blockStr = "pending"
		}
		fmt.Printf("[%s] %s  R:%-3d G:%-3d B:%-3d  %s → %s  (%s)\n",
			txType, t.GetHash()[:12]+"...",
			t.GetRed(), t.GetGreen(), t.GetBlue(),
			t.GetSenderId()[:12]+"...", t.GetReceiverId()[:12]+"...",
			blockStr)
	}
}

// doChainStats fetches and prints chain statistics.
func doChainStats(client pb.LedgerServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.GetChainStats(ctx, &pb.GetChainStatsRequest{})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Chain Statistics")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Chain Height    : %d\n", resp.GetChainHeight())
	fmt.Printf("Last Block Hash : %s\n", resp.GetLastBlockHash())
	fmt.Printf("Last Block Time : %s\n", time.Unix(resp.GetLastBlockTimestamp(), 0).Format(time.RFC3339))
	fmt.Printf("Pending TXs     : %d\n", resp.GetPendingTxCount())
	fmt.Printf("Total TXs       : %d\n", resp.GetTotalTxCount())
	fmt.Printf("Total Players   : %d\n", resp.GetTotalPlayers())
}

// doBlockExplorer lets the user inspect a specific block or list recent blocks.
func doBlockExplorer(client pb.LedgerServiceClient, scanner *bufio.Scanner) {
	fmt.Println("Block Explorer")
	fmt.Println("  a. Get block by height")
	fmt.Println("  b. List blocks (page)")
	fmt.Print("Choice: ")
	if !scanner.Scan() {
		return
	}
	choice := strings.TrimSpace(scanner.Text())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch strings.ToLower(choice) {
	case "a":
		height, ok := promptUint64(scanner, "Block height: ")
		if !ok {
			return
		}
		resp, err := client.GetBlock(ctx, &pb.GetBlockRequest{Height: height})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if !resp.GetFound() {
			fmt.Printf("Block %d not found\n", height)
			return
		}
		printBlock(resp.GetBlock())

	case "b":
		page, ok := promptUint32(scanner, "Page (0-indexed): ")
		if !ok {
			return
		}
		resp, err := client.ListBlocks(ctx, &pb.ListBlocksRequest{Page: page, PageSize: 5})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Blocks (total=%d)\n", resp.GetTotalBlocks())
		fmt.Println(strings.Repeat("-", 72))
		for _, b := range resp.GetBlocks() {
			fmt.Printf("Block #%-6d  txs=%-3d  hash=%s...\n",
				b.GetHeight(), len(b.GetTxHashes()), b.GetHash()[:16])
		}
	}
}

func printBlock(b *pb.BlockInfo) {
	fmt.Printf("Block #%d\n", b.GetHeight())
	fmt.Printf("  Hash       : %s\n", b.GetHash())
	fmt.Printf("  Prev Hash  : %s\n", b.GetPrevHash())
	fmt.Printf("  Merkle Root: %s\n", b.GetMerkleRoot())
	fmt.Printf("  Timestamp  : %s\n", time.Unix(b.GetTimestamp(), 0).Format(time.RFC3339))
	fmt.Printf("  Nonce      : %d\n", b.GetNonce())
	fmt.Printf("  Difficulty : %d\n", b.GetDifficulty())
	fmt.Printf("  TXs        : %d\n", len(b.GetTxHashes()))
	for _, h := range b.GetTxHashes() {
		fmt.Printf("    - %s\n", h)
	}
}

// promptUint64 reads a line from the scanner, parses it as uint64.
func promptUint64(scanner *bufio.Scanner, prompt string) (uint64, bool) {
	fmt.Print(prompt)
	if !scanner.Scan() {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(scanner.Text()), 10, 64)
	if err != nil {
		fmt.Printf("Invalid number: %v\n", err)
		return 0, false
	}
	return v, true
}
