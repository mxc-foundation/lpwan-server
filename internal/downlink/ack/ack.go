package ack

import (
	"context"
	"encoding/binary"

	"github.com/brocaar/lorawan"
	"github.com/mxc-foundation/lpwan-server/api/gw"
	"github.com/mxc-foundation/lpwan-server/internal/backend/gateway"
	"github.com/mxc-foundation/lpwan-server/internal/storage"
	"github.com/pkg/errors"
)

var (
	errAbort = errors.New("abort")
)

var handleDownlinkTXAckTasks = []func(*ackContext) error{
	// smbPacketPayment,
	abortOnNoError,
	getToken,
	getDownlinkFrame,
	sendDownlinkFrame,
}

type ackContext struct {
	ctx context.Context

	Token         uint16
	DevEUI        lorawan.EUI64
	DownlinkTXAck gw.DownlinkTXAck
	DownlinkFrame gw.DownlinkFrame
}

// HandleDownlinkTXAck handles the given downlink TX acknowledgement.
func HandleDownlinkTXAck(ctx context.Context, downlinkTXAck gw.DownlinkTXAck) error {
	actx := ackContext{
		ctx:           ctx,
		DownlinkTXAck: downlinkTXAck,
	}

	for _, t := range handleDownlinkTXAckTasks {
		if err := t(&actx); err != nil {
			if err == errAbort {
				return nil
			}
			return err
		}
	}

	return nil
}

// to be done: next phase
// func smbPacketPayment(ctx *ackContext) error {
// 	if ctx.DownlinkTXAck.Error != "" {
// 		return nil
// 	}
// 	// retrieve ctx based on Token -
// 	fmt.Println("ack.go/abortOnNoError - Downlink is sent successfully- ctx: ", ctx)

// 	// Not all of the gateways support this ack
// 	// Event after getting the ack there is no guarantee that the downlink is sent
// 	return nil
// }

func abortOnNoError(ctx *ackContext) error {
	if ctx.DownlinkTXAck.Error == "" {
		// no error, nothing to do
		return errAbort
	}
	return nil
}

func getToken(ctx *ackContext) error {
	if ctx.DownlinkTXAck.Token != 0 {
		ctx.Token = uint16(ctx.DownlinkTXAck.Token)
	} else if len(ctx.DownlinkTXAck.DownlinkId) == 16 {
		ctx.Token = binary.BigEndian.Uint16(ctx.DownlinkTXAck.DownlinkId[0:2])
	}
	return nil
}

func getDownlinkFrame(ctx *ackContext) error {
	var err error
	ctx.DevEUI, ctx.DownlinkFrame, err = storage.PopDownlinkFrame(ctx.ctx, storage.RedisPool(), uint32(ctx.Token))
	if err != nil {
		if err == storage.ErrDoesNotExist {
			// no retry is possible, abort
			return errAbort
		}
		return errors.Wrap(err, "pop downlink-frame error")
	}
	return nil
}

func sendDownlinkFrame(ctx *ackContext) error {
	if err := gateway.Backend().SendTXPacket(ctx.DownlinkFrame); err != nil {
		return errors.Wrap(err, "send downlink-frame to gateway error")
	}
	return nil
}
