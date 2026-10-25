package types

// ChallengeStatus mirrors the proto ChallengeStatus enum.
type ChallengeStatus int32

const (
	ChallengePending   ChallengeStatus = 0
	ChallengeAccepted  ChallengeStatus = 1
	ChallengeDeclined  ChallengeStatus = 2
	ChallengeExpired   ChallengeStatus = 3
	ChallengeCompleted ChallengeStatus = 4
)

// ChallengeRecord represents a peer-to-peer token-swap challenge stored in Redis.
// The proposer pre-signs their TRANSFER leg; the opponent signs theirs on acceptance.
type ChallengeRecord struct {
	ID         string          `json:"id"`
	ProposerID string          `json:"proposer_id"`
	OpponentID string          `json:"opponent_id"`
	OfferRed   uint32          `json:"offer_red"`
	OfferGreen uint32          `json:"offer_green"`
	OfferBlue  uint32          `json:"offer_blue"`
	WantRed    uint32          `json:"want_red"`
	WantGreen  uint32          `json:"want_green"`
	WantBlue   uint32          `json:"want_blue"`
	Status     ChallengeStatus `json:"status"`
	CreatedAt  int64           `json:"created_at"`
	ExpiresAt  int64           `json:"expires_at"`
	// Pre-signed proposer transfer (proposer → opponent).
	RawPayload   []byte `json:"raw_payload"`
	Signature    []byte `json:"signature"`
	SenderPubKey []byte `json:"sender_pub_key"`
}
