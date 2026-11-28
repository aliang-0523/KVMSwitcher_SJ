// Package update installs only assets from this repository's latest stable release
// that match their published SHA-256 and carry a valid release signature (see sign.go).
package update

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const repo = "aliang-0523/KVMSwitcher_SJ"
const apiURL = "https://api.github.com/repos/" + repo + "/releases/latest"
const maxPackage = 128 << 20

var semver = regexp.MustCompile("^v?(0|[1-9][0-9]{0,8})[.](0|[1-9][0-9]{0,8})[.](0|[1-9][0-9]{0,8})$")

func compare(a, b string) (int, error) {
	parse := func(s string) ([]int, error) {
		m := semver.FindStringSubmatch(s)
		if m == nil {
			return nil, fmt.Errorf("无效的稳定版本号：%q", s)
		}
		var v []int
		for _, p := range m[1:] {
			n, _ := strconv.Atoi(p)
			v = append(v, n)
		}
		return v, nil
	}
	x, e := parse(a)
	if e != nil {
		return 0, e
	}
	y, e := parse(b)
	if e != nil {
		return 0, e
	}
	return slices.CompareFunc(x, y, cmp.Compare[int]), nil
}

type Asset struct {
	Name, Digest, State string
	Size                int64
	URL                 string `json:"browser_download_url"`
}
type release struct {
	Tag               string `json:"tag_name"`
	Draft, Prerelease bool
	Assets            []Asset
}

// pkg 是一个平台的安装包及其签名文件。
type pkg struct {
	Asset
	Sig Asset
}

func (r release) pick(current, platform string) (pkg, int, error) {
	n, e := compare(r.Tag, current)
	if e != nil {
		return pkg{}, 0, e
	}
	if !strings.HasPrefix(r.Tag, "v") || r.Draft || r.Prerelease {
		return pkg{}, 0, errors.New("不是正式发布版本")
	}
	if n < 0 {
		return pkg{}, n, nil
	}
	name := map[string]string{"windows": "KVMSwitch-Setup.msi", "mac": "KVMSwitch-mac.zip"}[platform]
	base := "https://github.com/" + repo + "/releases/download/" + r.Tag + "/"
	var p pkg
	var found, signed bool
	for _, a := range r.Assets {
		switch a.Name {
		case name:
			hash, e := hex.DecodeString(strings.TrimPrefix(a.Digest, "sha256:"))
			if a.State != "uploaded" || a.URL != base+name || a.Size <= 0 || a.Size > maxPackage || !strings.HasPrefix(a.Digest, "sha256:") || e != nil || len(hash) != sha256.Size {
				return pkg{}, 0, errors.New("安装包的地址、大小或 SHA-256 元数据无效")
			}
			p.Asset, found = a, true
		case name + SigSuffix:
			if a.State != "uploaded" || a.URL != base+name+SigSuffix || a.Size <= 0 || a.Size > maxSig {
				return pkg{}, 0, errors.New("发布签名文件的地址或大小无效")
			}
			p.Sig, signed = a, true
		}
	}
	if !found {
		return pkg{}, 0, errors.New("该版本尚未发布本机安装包")
	}
	if !signed {
		return pkg{}, 0, errors.New("该版本没有发布签名，已拒绝更新")
	}
	return p, n, nil
}

type State struct {
	Latest, Message  string
	Busy, Ready, New bool
	Done, Total      int64 // 下载进度（字节），Total 为 0 表示没有在下载
}

// stage 更新面板上显示的安装阶段与下载进度。
func (m *Manager) stage(msg string, done, total int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg != "" {
		m.s.Message = msg
	}
	m.s.Done, m.s.Total = done, total
}

// progress 在写入安装包的同时汇报已下载的字节数。
type progress struct {
	m     *Manager
	n     int64
	total int64
}

func (p *progress) Write(b []byte) (int, error) {
	p.n += int64(len(b))
	p.m.stage("", min(p.n, p.total), p.total)
	return len(b), nil
}

type Manager struct {
	mu                sync.Mutex
	s                 State
	pkg               pkg
	current, platform string
	quit              func()
	client            *http.Client
}

func Start(version, platform string, quit func()) *Manager {
	m := &Manager{current: version, platform: platform, quit: quit, client: &http.Client{Timeout: 4 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		h := req.URL.Hostname()
		if len(via) > 6 || req.URL.Scheme != "https" || !(h == "github.com" || strings.HasSuffix(h, ".githubusercontent.com")) {
			return errors.New("拒绝非 GitHub 下载重定向")
		}
		return nil
	}}}
	go func() {
		m.Check()
		for range time.NewTicker(12 * time.Hour).C {
			m.Check()
		}
	}()
	return m
}
func (m *Manager) State() State { m.mu.Lock(); defer m.mu.Unlock(); return m.s }
func (m *Manager) Check() bool {
	m.mu.Lock()
	if m.s.Busy {
		m.mu.Unlock()
		return false
	}
	m.s = State{Busy: true, Message: "正在检查 GitHub Releases…"}
	m.mu.Unlock()
	go func() {
		r, e := m.latest()
		var p pkg
		var n int
		if e == nil {
			p, n, e = r.pick(m.current, m.platform)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if e != nil {
			m.s = State{Message: "检查失败：" + e.Error()}
			return
		}
		msg := "已是最新版本"
		if n > 0 {
			msg = "发现新版本 " + r.Tag
		} else if n < 0 {
			msg = "本机版本高于最新发布版，不会降级"
		}
		m.pkg = p
		m.s = State{Latest: r.Tag, Message: msg, Ready: n >= 0, New: n > 0}
	}()
	return true
}
func (m *Manager) latest() (r release, err error) {
	client := *m.client
	client.Timeout = 15 * time.Second
	res, e := client.Get(apiURL)
	if e != nil {
		return r, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return r, fmt.Errorf("GitHub: %s", res.Status)
	}
	e = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&r)
	return r, e
}

// verifiedCopy 复制安装包并核对大小和 SHA-256，返回实际算出的 SHA-256。
func verifiedCopy(w io.Writer, r io.Reader, a Asset) ([]byte, error) {
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(w, h), io.LimitReader(r, a.Size+1))
	if e != nil {
		return nil, e
	}
	sum := h.Sum(nil)
	if n != a.Size || "sha256:"+hex.EncodeToString(sum) != a.Digest {
		return nil, errors.New("安装包大小或 SHA-256 不匹配，已停止安装")
	}
	return sum, nil
}
func (m *Manager) Install(tag string) bool {
	m.mu.Lock()
	if m.s.Busy || !m.s.Ready || tag != m.s.Latest {
		m.mu.Unlock()
		return false
	}
	p := m.pkg
	m.s.Busy = true
	m.s.Message = "正在下载发布签名…"
	m.mu.Unlock()
	go func() {
		e := m.downloadInstall(tag, p)
		if e != nil {
			m.mu.Lock()
			m.s.Busy = false
			m.s.Message = "安装失败：" + e.Error()
			m.s.Done, m.s.Total = 0, 0
			m.mu.Unlock()
			return
		}
		m.quit()
	}()
	return true
}
func (m *Manager) downloadInstall(tag string, p pkg) error {
	path, err := m.download(tag, p)
	if err != nil {
		return err
	}
	if err := launch(path); err != nil {
		os.RemoveAll(filepath.Dir(path))
		return err
	}
	return nil
}

func (m *Manager) get(u string) (*http.Response, error) {
	res, e := m.client.Get(u)
	if e != nil {
		var ue *url.Error
		if errors.As(e, &ue) {
			return nil, ue.Err
		}
		return nil, e
	}
	if res.StatusCode != 200 {
		res.Body.Close()
		return nil, fmt.Errorf("下载失败：%s", res.Status)
	}
	return res, nil
}

// download 下载签名和安装包，核对 SHA-256 并校验发布签名，全部通过才返回安装包路径。
func (m *Manager) download(tag string, p pkg) (path string, err error) {
	res, err := m.get(p.Sig.URL)
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxSig+1))
	res.Body.Close()
	if err != nil {
		return "", err
	}
	if len(b) > maxSig {
		return "", errors.New("发布签名文件过大")
	}
	sig, err := DecodeSignature(b)
	if err != nil {
		return "", err
	}

	dir, err := os.MkdirTemp("", "kvmswitch-update-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(dir)
		}
	}()
	m.stage("正在下载安装包…", 0, p.Size)
	res, err = m.get(p.URL)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	path = filepath.Join(dir, p.Name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	sum, err := verifiedCopy(io.MultiWriter(f, &progress{m: m, total: p.Size}), res.Body, p.Asset)
	if ce := f.Close(); err == nil {
		err = ce
	}
	if err != nil {
		return "", err
	}
	m.stage("正在校验发布签名…", p.Size, p.Size)
	if err = VerifySignature(tag, p.Name, p.Size, sum, sig); err != nil {
		return "", fmt.Errorf("%v，已拒绝安装：安装包可能不是官方发布的", err)
	}
	m.stage("校验通过，即将退出并安装…", p.Size, p.Size)
	return path, nil
}
