package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	dxva2    = windows.NewLazySystemDLL("dxva2.dll")

	pSetWindowsHookExW       = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx     = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx          = user32.NewProc("CallNextHookEx")
	pGetMessageW             = user32.NewProc("GetMessageW")
	pTranslateMessage        = user32.NewProc("TranslateMessage")
	pDispatchMessageW        = user32.NewProc("DispatchMessageW")
	pPostMessageW            = user32.NewProc("PostMessageW")
	pPostQuitMessage         = user32.NewProc("PostQuitMessage")
	pRegisterClassExW        = user32.NewProc("RegisterClassExW")
	pCreateWindowExW         = user32.NewProc("CreateWindowExW")
	pDefWindowProcW          = user32.NewProc("DefWindowProcW")
	pRegisterWindowMessageW  = user32.NewProc("RegisterWindowMessageW")
	pGetCursorPos            = user32.NewProc("GetCursorPos")
	pSetCursorPos            = user32.NewProc("SetCursorPos")
	pGetSystemMetrics        = user32.NewProc("GetSystemMetrics")
	pSendInput               = user32.NewProc("SendInput")
	pMessageBoxW             = user32.NewProc("MessageBoxW")
	pMessageBoxTimeoutW      = user32.NewProc("MessageBoxTimeoutW")
	pMessageBeep             = user32.NewProc("MessageBeep")
	pCreatePopupMenu         = user32.NewProc("CreatePopupMenu")
	pAppendMenuW             = user32.NewProc("AppendMenuW")
	pTrackPopupMenu          = user32.NewProc("TrackPopupMenu")
	pDestroyMenu             = user32.NewProc("DestroyMenu")
	pSetForegroundWindow     = user32.NewProc("SetForegroundWindow")
	pCreateIcon              = user32.NewProc("CreateIcon")
	pEnumDisplayMonitors     = user32.NewProc("EnumDisplayMonitors")
	pOpenClipboard           = user32.NewProc("OpenClipboard")
	pCloseClipboard          = user32.NewProc("CloseClipboard")
	pEmptyClipboard          = user32.NewProc("EmptyClipboard")
	pGetClipboardData        = user32.NewProc("GetClipboardData")
	pSetClipboardData        = user32.NewProc("SetClipboardData")
	pIsClipboardFormatAvail  = user32.NewProc("IsClipboardFormatAvailable")
	pRegisterClipboardFormat = user32.NewProc("RegisterClipboardFormatW")
	pGetClipboardOwner       = user32.NewProc("GetClipboardOwner")
	pGetClipboardSeq         = user32.NewProc("GetClipboardSequenceNumber")
	pSetProcessDpiCtx        = user32.NewProc("SetProcessDpiAwarenessContext")
	pMapVirtualKeyW          = user32.NewProc("MapVirtualKeyW")
	pFindWindowW             = user32.NewProc("FindWindowW")
	pShowWindow              = user32.NewProc("ShowWindow")
	pIsIconic                = user32.NewProc("IsIconic")

	pGlobalAlloc             = kernel32.NewProc("GlobalAlloc")
	pGlobalFree              = kernel32.NewProc("GlobalFree")
	pGlobalLock              = kernel32.NewProc("GlobalLock")
	pGlobalUnlock            = kernel32.NewProc("GlobalUnlock")
	pGlobalSize              = kernel32.NewProc("GlobalSize")
	pSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
	pSetProcessInformation   = kernel32.NewProc("SetProcessInformation")
	pGetCurrentProcess       = kernel32.NewProc("GetCurrentProcess")
	pAttachConsole           = kernel32.NewProc("AttachConsole")
	pGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pDragQueryFileW   = shell32.NewProc("DragQueryFileW")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")

	pGetNumberOfPhysicalMonitors = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	pGetPhysicalMonitors         = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	pDestroyPhysicalMonitors     = dxva2.NewProc("DestroyPhysicalMonitors")
	pGetVCPFeature               = dxva2.NewProc("GetVCPFeatureAndVCPFeatureReply")
	pSetVCPFeature               = dxva2.NewProc("SetVCPFeature")
	pGetCapsLen                  = dxva2.NewProc("GetCapabilitiesStringLength")
	pCapsRequest                 = dxva2.NewProc("CapabilitiesRequestAndCapabilitiesReply")
)

type point struct{ X, Y int32 }

func u16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

const (
	mbOK          = 0x0
	mbYesNo       = 0x4
	mbIconInfo    = 0x40
	mbIconWarn    = 0x30
	mbIconQuest   = 0x20
	mbTopmost     = 0x40000
	mbSetFg       = 0x10000
	mbSystemModal = 0x1000
	idYes         = 6
)

func messageBox(text, title string, flags uintptr) int {
	r, _, _ := pMessageBoxW.Call(0, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), flags|mbTopmost|mbSetFg)
	return int(r)
}

// messageBoxTimeout 超时返回 32000。
func messageBoxTimeout(text, title string, flags uintptr, ms uint32) int {
	if pMessageBoxTimeoutW.Find() != nil {
		return messageBox(text, title, flags)
	}
	r, _, _ := pMessageBoxTimeoutW.Call(0, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), flags|mbTopmost|mbSetFg, 0, uintptr(ms))
	return int(r)
}

func shellOpen(target string) {
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(target))), 0, 0, 1)
}

// disablePowerThrottling 让 Windows 不对本进程启用 EcoQoS（后台降速）。
// 托盘程序没有前台窗口，Windows 11 会把它当作后台进程降低调度优先级与时钟频率，
// 使键鼠钩子回调和网络收发出现几到几十毫秒的抖动。旧系统不支持时静默忽略。
func disablePowerThrottling() {
	if pSetProcessInformation.Find() != nil {
		return
	}
	// PROCESS_POWER_THROTTLING_STATE：Version=1，ControlMask=EXECUTION_SPEED，StateMask=0（关闭）
	state := [3]uint32{1, 1, 0}
	h, _, _ := pGetCurrentProcess.Call()
	const processPowerThrottling = 4
	pSetProcessInformation.Call(h, processPowerThrottling, uintptr(unsafe.Pointer(&state[0])), unsafe.Sizeof(state))
}
