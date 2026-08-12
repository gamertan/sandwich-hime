// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows && (amd64 || arm64)

package devserver

import "unsafe"

// jobObjectBasicLimitInfo mirrors JOBOBJECT_BASIC_LIMIT_INFORMATION on the
// supported 64-bit Windows architectures.
type jobObjectBasicLimitInfo struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

var (
	_ [64 - unsafe.Sizeof(jobObjectBasicLimitInfo{})]byte
	_ [unsafe.Sizeof(jobObjectBasicLimitInfo{}) - 64]byte
	_ [144 - unsafe.Sizeof(jobObjectExtendedLimitInfo{})]byte
	_ [unsafe.Sizeof(jobObjectExtendedLimitInfo{}) - 144]byte
)
