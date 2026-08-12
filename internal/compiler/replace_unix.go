// SPDX-License-Identifier: AGPL-3.0-only
//go:build !windows

package compiler

import "os"

func replaceFile(replacement, destination string) error {
	return os.Rename(replacement, destination)
}
