package maccommand

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan"
	"github.com/mxc-foundation/lpwan-server/internal/logging"
	"github.com/mxc-foundation/lpwan-server/internal/storage"
)

// RequestRXTimingSetup modifies the RX delay between the end of the TX
// and the opening of the first reception slot.
func RequestRXTimingSetup(del int) storage.MACCommandBlock {
	return storage.MACCommandBlock{
		CID: lorawan.RXTimingSetupReq,
		MACCommands: []lorawan.MACCommand{
			{
				CID: lorawan.RXTimingSetupReq,
				Payload: &lorawan.RXTimingSetupReqPayload{
					Delay: uint8(del),
				},
			},
		},
	}
}

func handleRXTimingSetupAns(ctx context.Context, ds *storage.DeviceSession, block storage.MACCommandBlock, pendingBlock *storage.MACCommandBlock) ([]storage.MACCommandBlock, error) {
	if pendingBlock == nil || len(pendingBlock.MACCommands) == 0 {
		return nil, errors.New("expected pending mac-command")
	}
	req := pendingBlock.MACCommands[0].Payload.(*lorawan.RXTimingSetupReqPayload)

	ds.RXDelay = req.Delay

	log.WithFields(log.Fields{
		"dev_eui":  ds.DevEUI,
		"rx_delay": ds.RXDelay,
		"ctx_id":   ctx.Value(logging.ContextIDKey),
	}).Info("rx_timing_setup request acknowledged")

	return nil, nil
}
