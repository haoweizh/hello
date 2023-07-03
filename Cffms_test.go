package main

import (
	"hello/cfmms"
	"testing"
)

func Test_Cfmms(t *testing.T) {
	//cfmms.Cmd()
	cfmms.GetAbiFromEtherscan("./cfmms/contracts_helper.json", "./cfmms/abi/")
	cfmms.GenerateGoFilesByAbi("./cfmms/abi", "cfmms/abi_go/")

}
