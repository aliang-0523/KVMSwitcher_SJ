package update

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/windows"
)

// 旧做法是本程序直接启动 msiexec 后才退出，两者抢同一个 kvmswitch.exe：程序没来得及退出时，
// Windows Installer 只能把替换推迟到重启，MSI 随后打开的仍是旧版，看起来就像“更新没生效”。
//
// 现在交给一个辅助进程（本程序复制到临时目录的副本，不占用安装目录里的 exe）：
// 它等本程序完全退出后，以带进度条、无需点击的被动模式安装，结束后再把程序重新打开。
// 安装取消或失败时重新打开原来的程序，不会让用户突然没有 KVMSwitch 可用。

const helperFlag = "--kvm-install-update"

func launch(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	helper := filepath.Join(filepath.Dir(path), "kvmswitch-update-helper.exe")
	if err := copyFile(exe, helper); err != nil {
		return err
	}
	return exec.Command(helper, helperFlag, strconv.Itoa(os.Getpid()), path, exe).Start()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if ce := out.Close(); err == nil {
		err = ce
	}
	return err
}

// RunInstallHelper 必须在程序入口最先调用。若本进程是安装辅助进程，就完成安装并返回 true。
func RunInstallHelper(args []string) bool {
	if len(args) != 5 || args[1] != helperFlag {
		return false
	}
	pid, err := strconv.Atoi(args[2])
	if err != nil {
		return true
	}
	installHelper(pid, args[3], args[4])
	return true
}

// installedExe 是 MSI 的安装位置（packaging/windows.wxs：LocalAppDataFolder\KVMSwitchApp）。
func installedExe() string {
	d, err := os.UserCacheDir() // %LOCALAPPDATA%
	if err != nil {
		return ""
	}
	return filepath.Join(d, "KVMSwitchApp", "kvmswitch.exe")
}

func installHelper(pid int, msi, oldExe string) {
	// 与主程序写同一个日志文件（cmd/kvmswitch-win 的 logPath），方便在面板里查看。
	logger := log.New(io.Discard, "", log.LstdFlags)
	if d, err := os.UserCacheDir(); err == nil {
		if f, err := os.OpenFile(filepath.Join(d, "KVMSwitch", "kvmswitch.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			logger.SetOutput(f)
		}
	}

	if h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid)); err == nil {
		ev, _ := windows.WaitForSingleObject(h, 30_000)
		windows.CloseHandle(h)
		if ev != windows.WAIT_OBJECT_0 {
			logger.Printf("更新：KVMSwitch 30 秒内没有退出，已取消安装")
			return
		}
	} // 打不开说明已经退出
	time.Sleep(500 * time.Millisecond) // 等系统释放 exe 的文件句柄

	sys, err := windows.GetSystemDirectory()
	if err != nil {
		logger.Printf("更新：找不到系统目录：%v", err)
		restart(logger, oldExe)
		return
	}
	msiLog := filepath.Join(filepath.Dir(msi), "msi-install.log")
	logger.Printf("更新：开始安装 %s", filepath.Base(msi))
	err = exec.Command(filepath.Join(sys, "msiexec.exe"), "/i", msi, "/passive", "/norestart", "/l*v", msiLog).Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	target := oldExe
	switch code {
	case 0:
		logger.Printf("更新：安装完成")
	case 3010:
		logger.Printf("更新：安装完成，但有文件正在被占用，需要重启电脑后新版本才会生效")
	case 1602:
		logger.Printf("更新：安装已取消，继续使用当前版本")
	case 1618:
		logger.Printf("更新：另一个安装程序正在运行，请稍后在面板里重试")
	default:
		logger.Printf("更新：安装失败（%v，退出码 %d），详细日志：%s", err, code, msiLog)
	}
	if code == 0 || code == 3010 {
		if p := installedExe(); p != "" {
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
				target = p
			}
		}
	}
	restart(logger, target)
}

// restart 打开程序。--restart 会等旧实例的单实例锁释放；--background 不弹面板，
// 且在已有实例运行时（例如 MSI 也启动了一份）什么都不做。
func restart(logger *log.Logger, exe string) {
	if err := exec.Command(exe, "--restart", "--background").Start(); err != nil {
		logger.Printf("更新：重新打开 %s 失败：%v", exe, err)
		return
	}
	logger.Printf("更新：已重新打开 %s", exe)
}
