package ledger

type LedgerModule struct {
	service *LedgerService
}

func NewLedgerModule() *LedgerModule {
	return &LedgerModule{
		service: newLedgerService(),
	}
}

func (m *LedgerModule) Service() *LedgerService {
	return m.service
}
