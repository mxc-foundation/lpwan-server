package join

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"github.com/brocaar/lorawan"
	"github.com/gofrs/uuid"
	"github.com/golang/protobuf/ptypes"
	"github.com/mxc-foundation/lpwan-server/api/gw"
	m2m_api "github.com/mxc-foundation/lpwan-server/api/m2m_server"
	"github.com/mxc-foundation/lpwan-server/internal/backend/gateway"
	"github.com/mxc-foundation/lpwan-server/internal/band"
	"github.com/mxc-foundation/lpwan-server/internal/config"
	"github.com/mxc-foundation/lpwan-server/internal/downlink/mxc_smb"
	"github.com/mxc-foundation/lpwan-server/internal/framelog"
	"github.com/mxc-foundation/lpwan-server/internal/helpers"
	"github.com/mxc-foundation/lpwan-server/internal/logging"
	"github.com/mxc-foundation/lpwan-server/internal/models"
	"github.com/mxc-foundation/lpwan-server/internal/storage"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var (
	rxWindow        int
	downlinkTXPower int
)

var tasks = []func(*joinContext) error{
	setDeviceGatewayRXInfo,
	smbReorderGateways,
	setTXInfo,
	setToken,
	setDownlinkFrame,
	sendJoinAcceptResponse,
	saveRemainingFrames,
	smbDlSent,
}

type joinContext struct {
	ctx context.Context

	Token               uint16
	DeviceSession       storage.DeviceSession
	DeviceGatewayRXInfo []storage.DeviceGatewayRXInfo
	RXPacket            models.RXPacket
	PHYPayload          lorawan.PHYPayload

	// Downlink frames to be emitted (this can be a slice e.g. to first try
	// using RX1 parameters, failing that RX2 parameters).
	// Only the first item will be emitted, the other(s) will be enqueued
	// and emitted on a scheduling error.
	DownlinkFrames []gw.DownlinkFrame
}

// Setup sets up the join handler.
func Setup(conf config.Config) error {
	nsConfig := conf.NetworkServer.NetworkSettings
	rxWindow = nsConfig.RXWindow
	downlinkTXPower = nsConfig.DownlinkTXPower

	return nil
}

// Handle handles a downlink join-response.
func Handle(ctx context.Context, ds storage.DeviceSession, rxPacket models.RXPacket, phy lorawan.PHYPayload) error {
	jctx := joinContext{
		ctx:           ctx,
		DeviceSession: ds,
		PHYPayload:    phy,
		RXPacket:      rxPacket,
	}

	for _, t := range tasks {
		if err := t(&jctx); err != nil {
			return err
		}
	}

	return nil
}

func setDeviceGatewayRXInfo(ctx *joinContext) error {
	for i := range ctx.RXPacket.RXInfoSet {
		ctx.DeviceGatewayRXInfo = append(ctx.DeviceGatewayRXInfo, storage.DeviceGatewayRXInfo{
			GatewayID: helpers.GetGatewayID(ctx.RXPacket.RXInfoSet[i]),
			RSSI:      int(ctx.RXPacket.RXInfoSet[i].Rssi),
			LoRaSNR:   ctx.RXPacket.RXInfoSet[i].LoraSnr,
			Board:     ctx.RXPacket.RXInfoSet[i].Board,
			Antenna:   ctx.RXPacket.RXInfoSet[i].Antenna,
			Context:   ctx.RXPacket.RXInfoSet[i].Context,
		})
	}

	// this should not happen
	if len(ctx.DeviceGatewayRXInfo) == 0 {
		return errors.New("DeviceGatewayRXInfo is empty!")
	}

	return nil
}

// reorder gateways based on SMB of MXProtcol
func smbReorderGateways(ctx *joinContext) error {

	log.WithFields(log.Fields{
		"ctx.DeviceGatewayRXInfo:": ctx.DeviceGatewayRXInfo,
	}).Info("join/smbReorderGateways: Gateways primary order")

	// ctx.DeviceSession.DevEUI

	SelectedDeviceGatewayRXInfo, err := mxc_smb.SelectSenderGateway(ctx.DeviceSession.DevEUI, ctx.DeviceGatewayRXInfo)
	if err != nil {
		log.Info("join/smbReorderGateways:error reorder ", err)
		return err
	}

	if SelectedDeviceGatewayRXInfo.GatewayID == (storage.DeviceGatewayRXInfo{}).GatewayID {
		log.WithFields(log.Fields{
			"devEui:": ctx.DeviceSession.DevEUI,
		}).Info("join/smbReorderGateways: ErrSmbMxcNotPermittedToSendJoinAns")
		return errors.New("no permission to send downlink join response from SMB of MXC")
	}

	ctx.DeviceGatewayRXInfo = append(ctx.DeviceGatewayRXInfo, storage.DeviceGatewayRXInfo{})
	copy(ctx.DeviceGatewayRXInfo[1:], ctx.DeviceGatewayRXInfo)
	ctx.DeviceGatewayRXInfo[0] = SelectedDeviceGatewayRXInfo

	log.WithFields(log.Fields{
		"ctx.DeviceGatewayRXInfo:": ctx.DeviceGatewayRXInfo,
	}).Info("join/smbReorderGateways: Gateways modified order")
	return nil
}

func setTXInfo(ctx *joinContext) error {
	if rxWindow == 0 || rxWindow == 1 {
		if err := setTXInfoForRX1(ctx); err != nil {
			return err
		}
	}

	if rxWindow == 0 || rxWindow == 2 {
		if err := setTXInfoForRX2(ctx); err != nil {
			return err
		}
	}

	return nil
}

func setTXInfoForRX1(ctx *joinContext) error {
	rxInfo := ctx.DeviceGatewayRXInfo[0]
	txInfo := gw.DownlinkTXInfo{
		GatewayId: rxInfo.GatewayID[:],
		Board:     rxInfo.Board,
		Antenna:   rxInfo.Antenna,
		Context:   rxInfo.Context,
	}

	// get RX1 data-rate
	rx1DR, err := band.Band().GetRX1DataRateIndex(ctx.RXPacket.DR, 0)
	if err != nil {
		return errors.Wrap(err, "get rx1 data-rate index error")
	}

	// set data-rate
	err = helpers.SetDownlinkTXInfoDataRate(&txInfo, rx1DR, band.Band())
	if err != nil {
		return errors.Wrap(err, "set downlink tx-info data-rate error")
	}

	// set frequency
	freq, err := band.Band().GetRX1FrequencyForUplinkFrequency(int(ctx.RXPacket.TXInfo.Frequency))
	if err != nil {
		return errors.Wrap(err, "get rx1 frequency error")
	}
	txInfo.Frequency = uint32(freq)

	// set tx power
	if downlinkTXPower != -1 {
		txInfo.Power = int32(downlinkTXPower)
	} else {
		txInfo.Power = int32(band.Band().GetDownlinkTXPower(int(txInfo.Frequency)))
	}

	// set timestamp
	txInfo.Timing = gw.DownlinkTiming_DELAY
	txInfo.TimingInfo = &gw.DownlinkTXInfo_DelayTimingInfo{
		DelayTimingInfo: &gw.DelayTimingInfo{
			Delay: ptypes.DurationProto(band.Band().GetDefaults().JoinAcceptDelay1),
		},
	}

	ctx.DownlinkFrames = append(ctx.DownlinkFrames, gw.DownlinkFrame{
		TxInfo: &txInfo,
	})

	return nil
}

func setTXInfoForRX2(ctx *joinContext) error {
	rxInfo := ctx.DeviceGatewayRXInfo[0]
	txInfo := gw.DownlinkTXInfo{
		GatewayId: rxInfo.GatewayID[:],
		Board:     rxInfo.Board,
		Antenna:   rxInfo.Antenna,
		Frequency: uint32(band.Band().GetDefaults().RX2Frequency),
		Context:   rxInfo.Context,
	}

	// set data-rate
	err := helpers.SetDownlinkTXInfoDataRate(&txInfo, band.Band().GetDefaults().RX2DataRate, band.Band())
	if err != nil {
		return errors.Wrap(err, "set downlink tx-info data-rate error")
	}

	// set tx power
	if downlinkTXPower != -1 {
		txInfo.Power = int32(downlinkTXPower)
	} else {
		txInfo.Power = int32(band.Band().GetDownlinkTXPower(int(txInfo.Frequency)))
	}

	// set timestamp
	txInfo.Timing = gw.DownlinkTiming_DELAY
	txInfo.TimingInfo = &gw.DownlinkTXInfo_DelayTimingInfo{
		DelayTimingInfo: &gw.DelayTimingInfo{
			Delay: ptypes.DurationProto(band.Band().GetDefaults().JoinAcceptDelay2),
		},
	}

	ctx.DownlinkFrames = append(ctx.DownlinkFrames, gw.DownlinkFrame{
		TxInfo: &txInfo,
	})

	return nil
}

func setToken(ctx *joinContext) error {
	b := make([]byte, 2)
	_, err := rand.Read(b)
	if err != nil {
		return errors.Wrap(err, "read random error")
	}

	var downID uuid.UUID
	if ctxID := ctx.ctx.Value(logging.ContextIDKey); ctxID != nil {
		if id, ok := ctxID.(uuid.UUID); ok {
			downID = id
		}
	}

	for i := range ctx.DownlinkFrames {
		ctx.DownlinkFrames[i].Token = uint32(binary.BigEndian.Uint16(b))
		ctx.DownlinkFrames[i].DownlinkId = downID[:]
	}
	return nil
}

func setDownlinkFrame(ctx *joinContext) error {
	phyB, err := ctx.PHYPayload.MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "marshal phypayload error")
	}

	for i := range ctx.DownlinkFrames {
		ctx.DownlinkFrames[i].PhyPayload = phyB
	}

	return nil
}

func sendJoinAcceptResponse(ctx *joinContext) error {
	if len(ctx.DownlinkFrames) == 0 {
		return nil
	}

	err := gateway.Backend().SendTXPacket(ctx.DownlinkFrames[0])
	if err != nil {
		return errors.Wrap(err, "send downlink frame error")
	}

	// log frame
	if err := framelog.LogDownlinkFrameForGateway(ctx.ctx, storage.RedisPool(), ctx.DownlinkFrames[0]); err != nil {
		log.WithError(err).Error("log downlink frame for gateway error")
	}

	if err := framelog.LogDownlinkFrameForDevEUI(ctx.ctx, storage.RedisPool(), ctx.DeviceSession.DevEUI, ctx.DownlinkFrames[0]); err != nil {
		log.WithError(err).Error("log downlink frame for device error")
	}

	return nil
}

func saveRemainingFrames(ctx *joinContext) error {
	if len(ctx.DownlinkFrames) < 2 {
		return nil
	}

	if err := storage.SaveDownlinkFrames(ctx.ctx, storage.RedisPool(), ctx.DeviceSession.DevEUI, ctx.DownlinkFrames[1:]); err != nil {
		return errors.Wrap(err, "save downlink-frames error")
	}

	return nil
}

func smbDlSent(ctx *joinContext) error {
	dlPkt := m2m_api.DlPkt{
		DlIdNs:      strconv.FormatUint(binary.BigEndian.Uint64(ctx.DownlinkFrames[0].DownlinkId), 10),
		GwMac:       fmt.Sprintf("%s", ctx.DeviceGatewayRXInfo[0].GatewayID),
		DevEui:      fmt.Sprintf("%s", ctx.DeviceSession.DevEUI),
		TokenDlFrm1: int64(ctx.DownlinkFrames[0].Token),
		TokenDlFrm2: int64(ctx.DownlinkFrames[1].Token),
		CreateAt:    time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"), //time.Now().UTC().String(),
		Nonce:       0,
		Size:        0, // will modify for the next phase
		Category:    m2m_api.Category_JOIN_ANS,
	}

	mxc_smb.M2mApiDlPktSent(dlPkt)

	log.WithFields(log.Fields{
		"dlPkt": dlPkt,
	}).Info("join/smbDlSent: join DlPkt sent to M2M wallet")

	return nil
}
