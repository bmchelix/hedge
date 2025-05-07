/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"hedge/common/models"
)

type Executor interface {
	CommandExecutor(ctx interfaces.AppFunctionContext, command models.Command) (bool, models.CommandExecutionLog)
}
