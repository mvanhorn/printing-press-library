// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

//go:build !safety_readonly && !safety_agent_safe

package cli

const bakedSafetyProfileName = ""

var bakedAllowCommands []string
var bakedDenyCommands []string
