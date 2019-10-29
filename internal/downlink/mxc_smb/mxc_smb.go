package mxc_smb

import (
	"context"
	"fmt"

	m2m_api "github.com/mxc-foundation/lpwan-server/api/m2m_server"
	"github.com/mxc-foundation/lpwan-server/internal/backend/m2m_client"
	"github.com/mxc-foundation/lpwan-server/internal/config"
	"github.com/mxc-foundation/lpwan-server/internal/storage"
	"github.com/brocaar/lorawan"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func m2mApiDvUsageMode(devEui string) (m2m_api.DvUsageModeResponse, error) {

	m2mClient, err := m2m_client.GetPool().Get(config.C.M2MServer.M2MServer, []byte(config.C.M2MServer.CACert),
		[]byte(config.C.M2MServer.TLSCert), []byte(config.C.M2MServer.TLSKey))
	if err != nil {
		log.WithError(err).Error("get m2m-server client error m2mApiDvUsageMode")
		return m2m_api.DvUsageModeResponse{}, err
	}

	response, err := m2mClient.DvUsageMode(context.Background(), &m2m_api.DvUsageModeRequest{
		DvEui: devEui,
	})
	if err != nil {
		log.WithError(err).Error("m2m server API call DvUsageModeResponse error")
		return m2m_api.DvUsageModeResponse{}, errors.Wrap(err, "m2m server API call DvUsageModeResponse error")

	}

	return *response, nil
}

func M2mApiDlPktSent(dlPkt m2m_api.DlPkt) error {

	m2mClient, err := m2m_client.GetPool().Get(config.C.M2MServer.M2MServer, []byte(config.C.M2MServer.CACert),
		[]byte(config.C.M2MServer.TLSCert), []byte(config.C.M2MServer.TLSKey))
	if err != nil {
		log.WithError(err).Error("get m2m-server API client error M2mApiDlPktSent")
		return err
	}

	_, err = m2mClient.DlPktSent(context.Background(), &m2m_api.DlPktSentRequest{
		DlPkt: &dlPkt})
	if err != nil {
		log.WithError(err).Error("m2m server DlPktSent api error. DlPkt.DlIdNs: ", dlPkt.DlIdNs)
		return errors.Wrap(err, "m2m server DlPktSent api error. DlPkt")

	}
	return nil
}

// Select which gateway should send the downlink packet (will put in  reorderedDeviceGatewayRXInfo)
// A gateway from the Free gateways for the device (dvUsageModeRes.FreeGwMac) will be used preferebly
// If there is no free gateway in the list of gateways receive the uplink (deviceGatewayRXInfo), ...
// ... and the device is enable and willing to pay, a gateway from another org will be able to send the downlink
func SelectSenderGateway(devEui lorawan.EUI64, deviceGatewayRXInfo []storage.DeviceGatewayRXInfo) (reorderedDeviceGatewayRXInfo storage.DeviceGatewayRXInfo, err error) {
	dvUsageModeRes, err := m2mApiDvUsageMode(fmt.Sprintf("%s", devEui))
	if err != nil {
		return storage.DeviceGatewayRXInfo{}, err
	}

	log.WithFields(log.Fields{
		"devEui":         devEui,
		"dvUsageModeRes": dvUsageModeRes,
	}).Info("mxc_smb/selectSenderGateway: API response dvUsageModeRes")

	reorderedDeviceGatewayRXInfo = storage.DeviceGatewayRXInfo{}

	switch {
	case dvUsageModeRes.DvMode == m2m_api.DeviceMode_DV_INACTIVE || dvUsageModeRes.DvMode == m2m_api.DeviceMode_DV_DELETED:
		break
	case dvUsageModeRes.DvMode == m2m_api.DeviceMode_DV_FREE_GATEWAYS_LIMITED || dvUsageModeRes.DvMode == m2m_api.DeviceMode_DV_WHOLE_NETWORK:

		for _, rxInfo := range deviceGatewayRXInfo {
			for _, freeGws := range dvUsageModeRes.FreeGwMac {
				if fmt.Sprintf("%s", rxInfo.GatewayID) == (*freeGws).GwMac {
					reorderedDeviceGatewayRXInfo = rxInfo
					break
				}
			}
			if reorderedDeviceGatewayRXInfo.GatewayID != (storage.DeviceGatewayRXInfo{}).GatewayID {
				break
			}
		}

		if dvUsageModeRes.DvMode == m2m_api.DeviceMode_DV_WHOLE_NETWORK && dvUsageModeRes.EnoughBalance {
			if reorderedDeviceGatewayRXInfo.GatewayID == (storage.DeviceGatewayRXInfo{}).GatewayID {
				reorderedDeviceGatewayRXInfo = deviceGatewayRXInfo[0]
			}
		}

	}

	return reorderedDeviceGatewayRXInfo, nil
}
