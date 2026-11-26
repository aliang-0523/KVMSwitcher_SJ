package core

import (
	"testing"
	"time"

	"kvmswitch/internal/proto"
)

func (r *rig) setOneWay(c *Core, on bool) {
	r.t.Helper()
	next := *c.Config()
	next.OneWay = on
	if err := c.ApplyConfig(&next); err != nil {
		r.t.Fatal(err)
	}
}

// oneWayMac 连上后在 Mac 上开启单向控制，并等 Windows 知道这件事。
func oneWayMac(t *testing.T) *rig {
	t.Helper()
	r := newRig(t, true)
	r.connected()
	r.setOneWay(r.mac, true)
	r.wait("Windows 得知 Mac 开启单向控制", func() bool {
		s := r.win.State()
		return s.PeerOneWay && !s.PeerCanControl && r.mac.State().SelfOneWay
	})
	return r
}

// rawFromWin 绕过 Windows 自己的检查，直接往控制通道写消息：模拟被攻破的 Windows。
func (r *rig) rawFromWin(typ byte, payload []byte) {
	r.t.Helper()
	done := make(chan struct{})
	r.win.do(func() {
		defer close(done)
		if r.win.ctrl == nil {
			r.t.Error("控制通道未连接")
			return
		}
		r.win.ctrl.Write(typ, payload)
	})
	<-done
}

func (r *rig) rawActiveFromWin(m proto.Active) {
	r.t.Helper()
	done := make(chan struct{})
	r.win.do(func() {
		defer close(done)
		r.win.ctrl.WriteJSON(proto.MsgActive, m)
	})
	<-done
}

func (r *rig) noMacInjection(what string) {
	r.t.Helper()
	time.Sleep(300 * time.Millisecond)
	r.fm.get(func() {
		if len(r.fm.keys) != 0 || len(r.fm.moves) != 0 || r.fm.buttons != 0 || r.fm.wheels != 0 || r.fm.controlled {
			r.t.Fatalf("%s：Mac 不应接收注入（keys=%v moves=%v buttons=%d wheels=%d controlled=%t）",
				what, r.fm.keys, r.fm.moves, r.fm.buttons, r.fm.wheels, r.fm.controlled)
		}
	})
}

// Mac 控制 Windows 照常工作；Windows 只能把键鼠和显示器交回 Mac，不能再拿走。
func TestOneWayMacControlsWindowsAndWindowsOnlyHandsBack(t *testing.T) {
	r := oneWayMac(t)

	// Windows 主动切向 Mac（尚未被控制时）：在 Windows 本地就被拒绝
	r.win.Hotkey(false)
	r.wait("Windows 本地拒绝并提示", func() bool {
		var ok bool
		r.fw.get(func() { ok = r.fw.beeps == 1 && len(r.fw.notes) == 1 })
		return ok
	})
	r.active(ActiveNone, ActiveNone)

	// Mac 切到 Windows：Mac 的输入到达 Windows
	r.mac.Hotkey(false)
	r.active(ActiveSelf, ActivePeer)
	r.mac.PushInput(InputEvent{Typ: proto.MsgKey, A: 0x42, C: proto.KeyFlagDown})
	r.wait("Mac 的输入到达 Windows", func() bool {
		var ok bool
		r.fw.get(func() { ok = len(r.fw.keys) == 1 && r.fw.keys[0] == 0x42 })
		return ok
	})

	// Windows 交回：两端都把显示器切到 Mac 的输入口，但 Mac 不进入被控制状态
	r.fw.get(func() { r.fw.monitor = nil })
	r.fm.get(func() { r.fm.monitor = nil })
	r.win.Toggle()
	r.active(ActivePeer, ActiveSelf)
	r.wait("两端都把显示器切回 Mac", func() bool {
		var a, b bool
		r.fw.get(func() { a = len(r.fw.monitor) == 1 && r.fw.monitor[0] == 0x0F })
		r.fm.get(func() { b = len(r.fm.monitor) == 1 && r.fm.monitor[0] == 0x0F })
		return a && b
	})
	r.win.PushInput(InputEvent{Typ: proto.MsgKey, A: 0x41, C: proto.KeyFlagDown})
	r.win.PushInput(InputEvent{Typ: proto.MsgMouseMove, A: 3, B: 3})
	r.noMacInjection("交回之后")

	// Windows 再想拿走（切换热键或紧急热键）：本地拒绝，状态不变
	r.win.Hotkey(false)
	r.win.Hotkey(true)
	r.wait("两次拿走都被拒绝", func() bool {
		var ok bool
		r.fw.get(func() { ok = r.fw.beeps == 3 })
		return ok
	})
	r.active(ActivePeer, ActiveSelf)
	r.fm.get(func() {
		if r.fm.forward {
			t.Fatal("Mac 不应把按键转发给 Windows")
		}
	})

	// Mac 仍可随时再切过去
	r.mac.Hotkey(false)
	r.active(ActiveSelf, ActivePeer)
}

// 模拟被攻破的 Windows：绕过它自己的检查直接发消息，Mac 必须自己守住。
func TestOneWayMacResistsCompromisedWindows(t *testing.T) {
	r := oneWayMac(t)

	// 1. 宣布“键鼠归 Mac”，随后注入键鼠：Mac 接受归属，但一个事件都不注入
	r.rawActiveFromWin(proto.Active{Who: "receiver", Seq: 100})
	r.wait("Mac 视为键鼠在本机", func() bool { return r.mac.State().Active == ActiveSelf })
	r.rawFromWin(proto.MsgKey, []byte{0, 0x41, 0, 0, proto.KeyFlagDown})
	r.rawFromWin(proto.MsgMouseMove, []byte{0, 0, 0, 5, 0, 0, 0, 5})
	r.rawFromWin(proto.MsgMouseButton, []byte{proto.BtnLeft, 1})
	r.rawFromWin(proto.MsgWheel, []byte{0, 0, 0, 120, 0, 0, 0, 0})
	r.rawFromWin(proto.MsgInputReset, nil)
	r.noMacInjection("对端宣布键鼠归 Mac 后")

	// 2. 宣布“键鼠归 Windows”并切显示器：Mac 拒绝、不转发，并请 Windows 把显示器切回 Mac
	r.fw.get(func() { r.fw.monitor = nil })
	r.fm.get(func() { r.fm.monitor = nil })
	r.rawActiveFromWin(proto.Active{Who: "sender", Seq: 101, Monitor: true})
	r.wait("Mac 重申键鼠在本机，Windows 随之改为转发", func() bool {
		return r.win.State().Active == ActivePeer && r.mac.State().Active == ActiveSelf
	})
	r.wait("显示器被要求切回 Mac", func() bool {
		var a, b bool
		r.fw.get(func() { a = len(r.fw.monitor) == 1 && r.fw.monitor[0] == 0x0F })
		r.fm.get(func() { b = len(r.fm.monitor) == 1 && r.fm.monitor[0] == 0x0F })
		return a && b
	})
	r.fm.get(func() {
		if r.fm.forward {
			t.Fatal("Mac 不应开始转发本机按键")
		}
	})
	r.noMacInjection("拒绝接管之后")

	// 3. 尚未切换过时宣布“键鼠归 Windows”：Mac 保持各自独立，并让 Windows 也回到独立
	r.rawActiveFromWin(proto.Active{Who: "none", Seq: 102})
	r.active(ActivePeer, ActiveNone)
	r.rawActiveFromWin(proto.Active{Who: "sender", Seq: 103})
	r.active(ActiveNone, ActiveNone)
	r.fm.get(func() {
		if r.fm.forward {
			t.Fatal("Mac 不应开始转发本机按键")
		}
	})
}

// 已在被控制时开启单向控制：立即停止注入。
func TestOneWayTurnedOnWhileControlled(t *testing.T) {
	r := newRig(t, true)
	r.connected()
	r.win.Hotkey(false)
	r.active(ActivePeer, ActiveSelf)
	r.wait("Mac 被控制", func() bool {
		var ok bool
		r.fm.get(func() { ok = r.fm.controlled })
		return ok
	})
	r.setOneWay(r.mac, true)
	r.wait("Windows 得知单向控制", func() bool { return r.win.State().PeerOneWay })
	r.win.PushInput(InputEvent{Typ: proto.MsgKey, A: 0x41, C: proto.KeyFlagDown})
	r.noMacInjection("开启单向控制后")

	// 关闭后恢复双向
	r.setOneWay(r.mac, false)
	r.wait("Windows 得知单向控制已关闭", func() bool { s := r.win.State(); return !s.PeerOneWay && s.PeerCanControl })
	r.wait("Mac 恢复被控制", func() bool {
		var ok bool
		r.fm.get(func() { ok = r.fm.controlled })
		return ok
	})
	r.win.PushInput(InputEvent{Typ: proto.MsgKey, A: 0x43, C: proto.KeyFlagDown})
	r.wait("关闭后输入重新到达 Mac", func() bool {
		var ok bool
		r.fm.get(func() { ok = len(r.fm.keys) == 1 && r.fm.keys[0] == 0x43 })
		return ok
	})
}

// 单向控制以 Mac 的设置为准：Windows 改的设置不生效，并被 Mac 的设置覆盖回来。
func TestOneWayIgnoresPeerConfig(t *testing.T) {
	r := oneWayMac(t)
	next := *r.win.Config()
	next.Hotkey, next.SyncClipboard = "Ctrl+F11", false
	if err := r.win.ApplyConfig(&next); err != nil {
		t.Fatal(err)
	}
	r.wait("Windows 被改回 Mac 的设置", func() bool {
		c := r.win.Config()
		return c.Hotkey == "Ctrl+F12" && c.SyncClipboard
	})
	if c := r.mac.Config(); c.Hotkey != "Ctrl+F12" || !c.SyncClipboard {
		t.Fatalf("Mac 不应采用对端的设置：%+v", c)
	}

	// Mac 自己改的设置照常同步给 Windows
	next = *r.mac.Config()
	next.MouseScale = 1.5
	if err := r.mac.ApplyConfig(&next); err != nil {
		t.Fatal(err)
	}
	r.wait("Windows 采用 Mac 的改动", func() bool { return r.win.Config().MouseScale == 1.5 })
	if !r.mac.Config().OneWay || r.win.Config().OneWay {
		t.Fatal("单向控制只在本机生效，不应同步")
	}
}
