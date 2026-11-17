package agent

import (
	"context"
	"encoding/json"
)

// Tool adalah kontrak untuk semua kemampuan yang dapat dipanggil AI.
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage

	Execute(
		ctx context.Context,
		arguments json.RawMessage,
	) (string, error)
}
