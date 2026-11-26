package core

import (
	"testing"
	"time"

	"kvmswitch/internal/proto"
)

// 拨号端（Mac）手动断开：不再自动重连，点“重新连接”后恢复。
func TestPauseFromDialerStopsReconnect(t *testing.T) {
	r := newRig(t, true)
	r.connected()
	r.mac.SetPaused(true)
	r.wait("两端都断开", func() bool { return !r.win.State().Connected && !r.mac.State().Connected })
	if !r.mac.State().Paused || r.win.State().Paused {
		t.Fatalf("只有发起断开的一端应处于手动断开状态：win=%+v mac=%+v", r.win.State(), r.mac.State())
	}
	time.Sleep(2500 * time.Millisecond) // 超过一次拨号重试间隔
	if r.win.State().Connected || r.mac.State().Connected {
		t.Fatal("手动断开后不应自动重连")
	}
	r.mac.SetPaused(false)
	r.connected()
	if r.mac.State().Paused {
		t.Fatal("重新连接后不应再显示手动断开")
	}
}

// 接听端（Windows）在键鼠交给 Mac 时手动断开：键鼠和显示器回到本机，不弹“连接中断”通知，
// 期间拒绝对端拨入，恢复后重新连上。
func TestPauseFromListenerTakesBackAndRejects(t *testing.T) {
	r := newRig(t, true)
	r.connected()
	r.win.Toggle()
	r.active(ActivePeer, ActiveSelf)
	r.fw.get(func() { r.fw.monitor, r.fw.notes = nil, nil })

	r.win.SetPaused(true)
	r.wait("两端都断开", func() bool { return !r.win.State().Connected && !r.mac.State().Connected })
	r.active(ActiveNone, ActiveNone)
	r.wait("显示器切回 Windows", func() bool {
		var n int
		r.fw.get(func() { n = len(r.fw.monitor) })
		return n > 0
	})
	r.fw.get(func() {
		if len(r.fw.notes) != 0 {
			t.Fatalf("手动断开不应弹出连接中断通知：%v", r.fw.notes)
		}
	})
	time.Sleep(2500 * time.Millisecond)
	if r.win.State().Connected || r.mac.State().Connected {
		t.Fatal("接听端手动断开期间不应接受对端连接")
	}
	r.win.SetPaused(false)
	r.connected()
}

// 连续快速的键盘事件被合并写入后，对端仍按原顺序逐个注入、不丢不重。
func TestBatchedInputKeepsOrder(t *testing.T) {
	r := newRig(t, true)
	r.connected()
	r.win.Toggle()
	r.active(ActivePeer, ActiveSelf)
	const n = 300
	for i := 0; i < n; i++ {
		r.win.PushInput(InputEvent{Typ: proto.MsgKey, A: int32(0x41 + i%26), C: proto.KeyFlagDown})
	}
	r.wait("收到全部按键", func() bool {
		var got int
		r.fm.get(func() { got = len(r.fm.keys) })
		return got >= n
	})
	r.fm.get(func() {
		if len(r.fm.keys) != n {
			t.Fatalf("收到 %d 个按键，应为 %d", len(r.fm.keys), n)
		}
		for i, k := range r.fm.keys {
			if k != uint16(0x41+i%26) {
				t.Fatalf("第 %d 个按键是 %#x，顺序错乱", i, k)
			}
		}
	})
}
