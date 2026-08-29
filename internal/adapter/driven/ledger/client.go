package ledgerclient

import (
	"context"
	"fmt"
	"rgb-game/internal/app/port/out"
	"rgb-game/internal/domain/types"
	"rgb-game/pkg/pb"

	"google.golang.org/protobuf/proto"
)

// Client implements out.LedgerClient by calling the Ledger gRPC service.
// All proto marshaling and transaction signing live here, keeping application
// services free of transport-layer concerns.
type Client struct {
	grpc pb.LedgerServiceClient
}

// New creates a LedgerClient wrapping the given gRPC stub.
func New(grpc pb.LedgerServiceClient) *Client {
	return &Client{grpc: grpc}
}

// GetAuthorityNonce fetches the next nonce for the given authority player ID.
func (c *Client) GetAuthorityNonce(ctx context.Context, authorityID string) (uint64, error) {
	resp, err := c.grpc.GetBalance(ctx, &pb.GetBalanceRequest{PlayerId: authorityID})
	if err != nil {
		return 0, fmt.Errorf("ledger GetBalance: %w", err)
	}
	return resp.GetNextNonce(), nil
}

// SubmitMint signs and submits a MINT transaction to the Ledger.
// The signing key is provided via the out.FullAuthority port so the adapter
// never stores credentials itself.
func (c *Client) SubmitMint(
	ctx context.Context,
	auth out.FullAuthority,
	senderID, receiverID string,
	amtRed, amtGreen, amtBlue uint32,
	nonce uint64,
) (txHash string, newBalance *types.PlayerRecord, err error) {
	payload := &pb.TransactionPayload{
		Type:        pb.TransactionPayload_MINT,
		SenderId:    senderID,
		ReceiverId:  receiverID,
		AmountRed:   amtRed,
		AmountGreen: amtGreen,
		AmountBlue:  amtBlue,
		Nonce:       nonce,
	}

	rawPayload, err := proto.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("marshal mint payload: %w", err)
	}

	signature := auth.Sign(rawPayload)

	resp, err := c.grpc.SubmitTransaction(ctx, &pb.SubmitTransactionRequest{
		RawPayload:   rawPayload,
		Signature:    signature,
		SenderPubKey: auth.PubKey(),
	})
	if err != nil {
		return "", nil, fmt.Errorf("ledger SubmitTransaction: %w", err)
	}
	if !resp.GetSuccess() {
		return "", nil, fmt.Errorf("mint rejected by ledger: %s", resp.GetErrorMessage())
	}

	var balance *types.PlayerRecord
	if b := resp.GetNewBalance(); b != nil {
		balance = &types.PlayerRecord{
			ID:    b.GetPlayerId(),
			Red:   b.GetRed(),
			Green: b.GetGreen(),
			Blue:  b.GetBlue(),
			Nonce: b.GetNextNonce(),
		}
	}
	return resp.GetTxHash(), balance, nil
}
