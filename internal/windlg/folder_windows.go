//go:build windows

package windlg

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
)

var (
	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
	iidIShellItem       = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}
)

const (
	clsctxInprocServer      = 1
	coinitApartmentThreaded = 2
	fosPickFolders          = 0x20
	fosForceFileSystem      = 0x40
	sigdnFileSysPath        = 0x80058000
	rpcEChangedMode         = 0x80010106
	hresultCanceled         = 0x800704C7
)

type iUnknownVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

type iFileOpenDialogVtbl struct {
	iUnknownVtbl
	Show                uintptr
	SetFileTypes        uintptr
	SetFileTypeIndex    uintptr
	GetFileTypeIndex    uintptr
	Advise              uintptr
	Unadvise            uintptr
	SetOptions          uintptr
	GetOptions          uintptr
	SetDefaultFolder    uintptr
	SetFolder           uintptr
	GetFolder           uintptr
	GetCurrentSelection uintptr
	SetFileName         uintptr
	GetFileName         uintptr
	SetTitle            uintptr
	SetOkButtonLabel    uintptr
	SetFileNameLabel    uintptr
	GetResult           uintptr
	AddPlace            uintptr
	SetDefaultExtension uintptr
	Close               uintptr
	SetClientGuid       uintptr
	ClearClientData     uintptr
	SetFilter           uintptr
	GetResults          uintptr
	GetSelectedItems    uintptr
}

type iFileOpenDialog struct {
	vtbl *iFileOpenDialogVtbl
}

type iShellItemVtbl struct {
	iUnknownVtbl
	BindToHandler  uintptr
	GetParent      uintptr
	GetDisplayName uintptr
	GetAttributes  uintptr
	Compare        uintptr
}

type iShellItem struct {
	vtbl *iShellItemVtbl
}

func (d *iFileOpenDialog) Release() {
	syscall.SyscallN(d.vtbl.Release, uintptr(unsafe.Pointer(d)))
}

func (s *iShellItem) Release() {
	syscall.SyscallN(s.vtbl.Release, uintptr(unsafe.Pointer(s)))
}

// PickFolder 弹出系统「选择文件夹」对话框。取消返回 ("", nil)。
func PickFolder(title string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if hr != 0 && hr != 1 && hr != rpcEChangedMode {
		return "", fmt.Errorf("CoInitializeEx: 0x%X", hr)
	}
	if hr != rpcEChangedMode {
		defer procCoUninitialize.Call()
	}

	var dlg *iFileOpenDialog
	hr, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
		uintptr(unsafe.Pointer(&dlg)),
	)
	if hr != 0 || dlg == nil {
		return "", fmt.Errorf("创建文件夹对话框失败: 0x%X", hr)
	}
	defer dlg.Release()

	opts := uintptr(fosPickFolders | fosForceFileSystem)
	syscall.SyscallN(dlg.vtbl.SetOptions, uintptr(unsafe.Pointer(dlg)), opts)
	if title != "" {
		p, err := windows.UTF16PtrFromString(title)
		if err == nil {
			syscall.SyscallN(dlg.vtbl.SetTitle, uintptr(unsafe.Pointer(dlg)), uintptr(unsafe.Pointer(p)))
		}
	}

	hr2, _, _ := syscall.SyscallN(dlg.vtbl.Show, uintptr(unsafe.Pointer(dlg)), 0)
	if hr2 == hresultCanceled || hr2 == 0x800704C7 {
		return "", nil
	}
	if int32(hr2) < 0 {
		return "", fmt.Errorf("选择目录已取消或失败: 0x%X", hr2)
	}

	var item *iShellItem
	hr3, _, _ := syscall.SyscallN(dlg.vtbl.GetResult, uintptr(unsafe.Pointer(dlg)), uintptr(unsafe.Pointer(&item)))
	if hr3 != 0 || item == nil {
		return "", fmt.Errorf("读取所选目录失败: 0x%X", hr3)
	}
	defer item.Release()

	var psz *uint16
	hr4, _, _ := syscall.SyscallN(item.vtbl.GetDisplayName, uintptr(unsafe.Pointer(item)), uintptr(sigdnFileSysPath), uintptr(unsafe.Pointer(&psz)))
	if hr4 != 0 || psz == nil {
		return "", fmt.Errorf("读取路径失败: 0x%X", hr4)
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(psz)))
	return windows.UTF16PtrToString(psz), nil
}
