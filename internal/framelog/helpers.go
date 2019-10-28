package framelog

import (
	"github.com/pkg/errors"

	"github.com/mxc-foundation/lpwan-server/api/gw"
	"github.com/mxc-foundation/lpwan-server/internal/models"
)

// CreateUplinkFrameSet creates a UplinkFrameSet.
func CreateUplinkFrameSet(rxPacket models.RXPacket) (gw.UplinkFrameSet, error) {
	b, err := rxPacket.PHYPayload.MarshalBinary()
	if err != nil {
		return gw.UplinkFrameSet{}, errors.Wrap(err, "marshal phypayload error")
	}

	return gw.UplinkFrameSet{
		PhyPayload: b,
		TxInfo:     rxPacket.TXInfo,
		RxInfo:     rxPacket.RXInfoSet,
	}, nil
}
