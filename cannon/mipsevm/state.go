package mipsevm

import "github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"

type CpuScalars struct {
	PC     arch.Word `json:"pc"`
	NextPC arch.Word `json:"nextPC"`
	LO     arch.Word `json:"lo"`
	HI     arch.Word `json:"hi"`
}

const (
	VMStatusValid      = 0
	VMStatusInvalid    = 1
	VMStatusPanic      = 2
	VMStatusUnfinished = 3
)

func VmStatus(exited bool, exitCode uint8) uint8 {
	if !exited {
		return VMStatusUnfinished
	}

	switch exitCode {
	case 0:
		return VMStatusValid
	case 1:
		return VMStatusInvalid
	default:
		return VMStatusPanic
	}
}
