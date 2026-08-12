// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows && (386 || arm)

package devserver

import "unsafe"

// jobObjectBasicLimitInfo mirrors JOBOBJECT_BASIC_LIMIT_INFORMATION. Windows
// 32-bit ABIs pad this structure to an eight-byte boundary.
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
	_                       uint32
}

var (
	_ [48 - unsafe.Sizeof(jobObjectBasicLimitInfo{})]byte
	_ [unsafe.Sizeof(jobObjectBasicLimitInfo{}) - 48]byte
	_ [112 - unsafe.Sizeof(jobObjectExtendedLimitInfo{})]byte
	_ [unsafe.Sizeof(jobObjectExtendedLimitInfo{}) - 112]byte
)
