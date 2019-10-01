package mxc_smb

import (
	"context"
	"fmt"

	m2m_api "github.com/brocaar/loraserver/api/m2m_server"
	"github.com/brocaar/loraserver/internal/backend/m2m_client"
	"github.com/brocaar/loraserver/internal/storage"
	"github.com/brocaar/lorawan"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func m2mApiDvUsageMode(devEui string) (m2m_api.DvUsageModeResponse, error) {

	fmt.Println("@@ M2M API DvUsageMode begin; devEui: ", devEui)

	// m2mClient, err := m2m_client.GetPool().Get(config.C.M2MServer.M2MServer, []byte(config.C.M2MServer.CACert),
	// 	[]byte(config.C.M2MServer.TLSCert), []byte(config.C.M2MServer.TLSKey))

	m2mClient, err1 := m2m_client.GetPool().Get("mxprotocol-server:4000", []byte{}, []byte{}, []byte{}) // to be changed: get from config file
	if err1 != nil {
		fmt.Println(errors.Wrap(err1, "get m2m-server client error"))
		return m2m_api.DvUsageModeResponse{}, err1
	}

	response, err := m2mClient.DvUsageMode(context.Background(), &m2m_api.DvUsageModeRequest{
		DvEui: devEui,
	})
	if err != nil {
		log.WithError(err).Error("m2m server DvUsageModeResponse error") //@@
		fmt.Println(err, "@@create DvUsageModeResponse")                 //@@
		// return handleGrpcError(err, "create device error")
		return m2m_api.DvUsageModeResponse{}, errors.Wrap(err, "DvUsageModeResponse error") //@@

	}
	// for _, v := range response.FreeGwMac {
	// 	fmt.Println(v.GwMac)
	// }
	return *response, nil
}

func M2mApiDlPktSent(dlPkt m2m_api.DlPkt) error {

	fmt.Println("@@ Calling M2M API DlPktSent begin")

	// m2mClient, err := m2m_client.GetPool().Get(config.C.M2MServer.M2MServer, []byte(config.C.M2MServer.CACert),
	// 	[]byte(config.C.M2MServer.TLSCert), []byte(config.C.M2MServer.TLSKey))

	m2mClient, err1 := m2m_client.GetPool().Get("mxprotocol-server:4000", []byte{}, []byte{}, []byte{}) // to be changed: get from config file
	if err1 != nil {
		fmt.Println(errors.Wrap(err1, "get m2m-server client error")) //@@
		return err1
	}

	// dlPktTest := m2m_api.DlPkt{
	// 	DlIdNs:   2,
	// 	GwMac:    "alkdjs",
	// 	DevEui:   "67",
	// 	Token:    "1231",
	// 	CreateAt: "--time--",
	// 	Nonce:    123,
	// 	Size:     2.1,
	// 	Category: "downlink-cat",
	// }

	_, err := m2mClient.DlPktSent(context.Background(), &m2m_api.DlPktSentRequest{
		DlPkt: &dlPkt})
	if err != nil {
		log.WithError(err).Error("m2m server DlPktSent api error")
		fmt.Println(err, "@@DlPktSent error") //@@
		// return handleGrpcError(err, "create device error")
		return errors.Wrap(err, "DlPktSent error")

	}
	return nil
}

// DeviceGatewayRXInfo[0] to a gateway of organization (if possible).
// Otherwise another available gateway (in case of DeviceMode == WHOLE_NETWORK_USAGE)
// otherwise nil
func SelectSenderGateway(devEui lorawan.EUI64, deviceGatewayRXInfo []storage.DeviceGatewayRXInfo) (reorderedDeviceGatewayRXInfo storage.DeviceGatewayRXInfo, err error) {
	dvUsageModeRes, err := m2mApiDvUsageMode(fmt.Sprintf("%s", devEui))
	if err != nil {
		return storage.DeviceGatewayRXInfo{}, err
	}

	fmt.Println("@@ dvUsageModeRes of devEui: ", fmt.Sprintf("%s", devEui), "    API return: ", dvUsageModeRes) //@@

	reorderedDeviceGatewayRXInfo = storage.DeviceGatewayRXInfo{}

	switch {
	case dvUsageModeRes.DvMode == "INACTIVE" || dvUsageModeRes.DvMode == "DELETED":
		//nothing
	case dvUsageModeRes.DvMode == "FREE_GATEWAYS_LIMITED" || dvUsageModeRes.DvMode == "WHOLE_NETWORK":
		fmt.Println("step: FREE_GATEWAYS_LIMITED")

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
	case dvUsageModeRes.DvMode == "WHOLE_NETWORK":
		if reorderedDeviceGatewayRXInfo.GatewayID == (storage.DeviceGatewayRXInfo{}).GatewayID {
			reorderedDeviceGatewayRXInfo = deviceGatewayRXInfo[0]
		}

	}

	return reorderedDeviceGatewayRXInfo, nil
}
