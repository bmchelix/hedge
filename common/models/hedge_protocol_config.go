/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

import "errors"

type HedgeSvcConfig struct {
	Protocols []ProtocolDetails
}

type ProtocolDetails struct {
	ProtocolName       string
	ProtocolProperties string
}

/*type HedgeSvcConfig2 struct {
	Protocols []ProtocolDetails2
}

type ProtocolDetails2 struct {
	ProtocolName       string
	ProtocolProperties string
}*/

// Validate ensures your custom configuration has proper values.
func (scs *HedgeSvcConfig) Validate() error {

	if len(scs.Protocols) == 0 {
		return errors.New("Protocols should be specified")
	}

	if scs.Protocols[0].ProtocolName == "" {
		return errors.New("Hedge.Protocols.ProtocolName configuration must not be empty")
	}

	if len(scs.Protocols[0].ProtocolProperties) == 0 {
		return errors.New("PetrolStationId and DomsInterfaceURL configuration is required")
	}

	return nil
}
