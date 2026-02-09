//go:build darwin && cgo

package pty

/*
#include <libproc.h>
#include <sys/proc_info.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// GetCwd returns the shell's current working directory
// using libproc proc_pidinfo on macOS.
func (w *Wrapper) GetCwd() (string, error) {
	pid := w.GetPID()
	if pid == 0 {
		return "", fmt.Errorf("no process running")
	}

	var vpi C.struct_proc_vnodepathinfo
	const vpiSize = C.sizeof_struct_proc_vnodepathinfo

	ret, _ := C.proc_pidinfo(C.int(pid), C.PROC_PIDVNODEPATHINFO, 0, unsafe.Pointer(&vpi), vpiSize)
	if ret <= 0 {
		return "", fmt.Errorf("proc_pidinfo failed (ret=%d)", ret)
	}
	if ret != vpiSize {
		return "", fmt.Errorf("proc_pidinfo returned incomplete data: got %d bytes, expected %d", ret, vpiSize)
	}

	return C.GoString(&vpi.pvi_cdir.vip_path[0]), nil
}
