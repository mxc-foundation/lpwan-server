package proprietary

import (
	"context"
	"crypto/rand"
	"encoding/binary"

	"github.com/gofrs/uuid"
	"github.com/pkg/errors"

	"github.com/brocaar/lorawan"
	"github.com/mxc-foundation/lpwan-server/api/common"
	"github.com/mxc-foundation/lpwan-server/api/gw"
	"github.com/mxc-foundation/lpwan-server/internal/backend/gateway"
	"github.com/mxc-foundation/lpwan-server/internal/band"
	"github.com/mxc-foundation/lpwan-server/internal/config"
	"github.com/mxc-foundation/lpwan-server/internal/helpers"
	"github.com/mxc-foundation/lpwan-server/internal/logging"
)

const defaultCodeRate = "4/5"

var tasks = []func(*proprietaryContext) error{
	setToken,
	sendProprietaryDown,
}

type proprietaryContext struct {
	ctx context.Context

	Token       uint16
	MACPayload  []byte
	MIC         lorawan.MIC
	GatewayMACs []lorawan.EUI64
	IPol        bool
	Frequency   int
	DR          int
}

var (
	downlinkTXPower int
)

// Setup configures the package.
func Setup(conf config.Config) error {
	downlinkTXPower = conf.NetworkServer.NetworkSettings.DownlinkTXPower

	return nil
}

// Handle handles a proprietary downlink.
func Handle(ctx context.Context, macPayload []byte, mic lorawan.MIC, gwMACs []lorawan.EUI64, iPol bool, frequency, dr int) error {
	pctx := proprietaryContext{
		ctx:         ctx,
		MACPayload:  macPayload,
		MIC:         mic,
		GatewayMACs: gwMACs,
		IPol:        iPol,
		Frequency:   frequency,
		DR:          dr,
	}

	for _, t := range tasks {
		if err := t(&pctx); err != nil {
			return err
		}
	}

	return nil
}

func setToken(ctx *proprietaryContext) error {
	b := make([]byte, 2)
	_, err := rand.Read(b)
	if err != nil {
		return errors.Wrap(err, "read random erro")
	}
	ctx.Token = binary.BigEndian.Uint16(b)
	return nil
}

func sendProprietaryDown(ctx *proprietaryContext) error {
	var txPower int
	if downlinkTXPower != -1 {
		txPower = downlinkTXPower
	} else {
		txPower = band.Band().GetDownlinkTXPower(ctx.Frequency)
	}

	var downID uuid.UUID
	if ctxID := ctx.ctx.Value(logging.ContextIDKey); ctxID != nil {
		if id, ok := ctxID.(uuid.UUID); ok {
			downID = id
		}
	}

	phy := lorawan.PHYPayload{
		MHDR: lorawan.MHDR{
			Major: lorawan.LoRaWANR1,
			MType: lorawan.Proprietary,
		},
		MACPayload: &lorawan.DataPayload{Bytes: ctx.MACPayload},
		MIC:        ctx.MIC,
	}
	phyB, err := phy.MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "marshal phypayload error")
	}

	for _, mac := range ctx.GatewayMACs {
		txInfo := gw.DownlinkTXInfo{
			GatewayId: mac[:],
			Frequency: uint32(ctx.Frequency),
			Power:     int32(txPower),

			Timing: gw.DownlinkTiming_IMMEDIATELY,
			TimingInfo: &gw.DownlinkTXInfo_ImmediatelyTimingInfo{
				ImmediatelyTimingInfo: &gw.ImmediatelyTimingInfo{},
			},
		}

		err = helpers.SetDownlinkTXInfoDataRate(&txInfo, ctx.DR, band.Band())
		if err != nil {
			return errors.Wrap(err, "set downlink tx-info data-rate error")
		}

		// for LoRa, set the iPol value
		if txInfo.Modulation == common.Modulation_LORA {
			modInfo := txInfo.GetLoraModulationInfo()
			if modInfo != nil {
				modInfo.PolarizationInversion = ctx.IPol
			}
		}

		if err := gateway.Backend().SendTXPacket(gw.DownlinkFrame{
			Token:      uint32(ctx.Token),
			DownlinkId: downID[:],
			TxInfo:     &txInfo,
			PhyPayload: phyB,
		}); err != nil {
			return errors.Wrap(err, "send tx packet to gateway error")
		}
	}

	return nil
}
