package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, v := range []struct {
		a, b string
		n    int
	}{{"v1.10.0", "1.9.9", 1}, {"v1.3.2", "1.3.2", 0}, {"1.3.2", "1.3.3", -1}} {
		n, e := compare(v.a, v.b)
		if e != nil || n != v.n {
			t.Fatal(n, e)
		}
	}
	if _, e := compare("v1.4.0-rc1", "1.3.3"); e == nil {
		t.Fatal("prerelease accepted")
	}
}
func TestVerify(t *testing.T) {
	a := Asset{Size: 3, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("abc")))}
	for _, s := range []string{"abc", "axc", "abcx", "ab"} {
		var buf bytes.Buffer
		if _, e := verifiedCopy(&buf, strings.NewReader(s), a); (e == nil) != (s == "abc") {
			t.Fatal(s)
		}
	}
}
func TestPick(t *testing.T) {
	r := release{Tag: "v1.3.3"}
	name := "KVMSwitch-Setup.msi"
	base := "https://github.com/" + repo + "/releases/download/v1.3.3/"
	r.Assets = []Asset{
		{Name: name, Size: 32, State: "uploaded", Digest: "sha256:" + strings.Repeat("ab", 32), URL: base + name},
		{Name: name + SigSuffix, Size: 89, State: "uploaded", URL: base + name + SigSuffix},
	}
	if p, n, e := r.pick("1.3.3", "windows"); e != nil || n != 0 || p.Sig.Name != name+SigSuffix {
		t.Fatal(n, e)
	}
	if _, _, e := r.pick("1.3.2", "mac"); e == nil {
		t.Fatal("missing mac asset")
	}
	r.Assets[1].URL = "https://example.org/x.sig"
	if _, _, e := r.pick("1.3.2", "windows"); e == nil {
		t.Fatal("foreign signature accepted")
	}
	r.Assets = r.Assets[:1]
	if _, _, e := r.pick("1.3.2", "windows"); e == nil || !strings.Contains(e.Error(), "签名") {
		t.Fatal("unsigned release accepted:", e)
	}
	r.Assets[0].URL = "https://example.org/test"
	if _, _, e := r.pick("1.3.2", "windows"); e == nil {
		t.Fatal("foreign asset accepted")
	}
}

func TestBuiltInKeys(t *testing.T) {
	if len(trusted) == 0 {
		t.Fatal("没有内置发布公钥，客户端会拒绝所有更新")
	}
	if !ValidTag("v1.3.18") || ValidTag("1.3.18") || ValidTag("v1.3.18-rc1") {
		t.Fatal("ValidTag")
	}
}

// withTestKey 临时把信任列表换成一把测试密钥。
func withTestKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	old := trusted
	trusted = []ed25519.PublicKey{pub}
	t.Cleanup(func() { trusted = old })
	return priv
}

func TestSignature(t *testing.T) {
	priv := withTestKey(t)
	sum := sha256.Sum256([]byte("pkg"))
	sig := ed25519.Sign(priv, SignedMessage("v1.3.18", "KVMSwitch-mac.zip", 3, sum[:]))
	got, err := DecodeSignature(EncodeSignature(sig))
	if err != nil || VerifySignature("v1.3.18", "KVMSwitch-mac.zip", 3, sum[:], got) != nil {
		t.Fatal("有效签名被拒绝", err)
	}
	other := sha256.Sum256([]byte("evil"))
	for _, c := range []struct {
		tag, name string
		size      int64
		sum       []byte
	}{
		{"v9.9.9", "KVMSwitch-mac.zip", 3, sum[:]},    // 换标签重发旧包
		{"v1.3.18", "KVMSwitch-Setup.msi", 3, sum[:]}, // 跨平台挪用
		{"v1.3.18", "KVMSwitch-mac.zip", 4, sum[:]},   // 大小不符
		{"v1.3.18", "KVMSwitch-mac.zip", 3, other[:]}, // 内容被替换
	} {
		if VerifySignature(c.tag, c.name, c.size, c.sum, sig) == nil {
			t.Fatalf("篡改后的签名被接受：%+v", c)
		}
	}
	_, stranger, _ := ed25519.GenerateKey(rand.Reader)
	if VerifySignature("v1.3.18", "KVMSwitch-mac.zip", 3, sum[:], ed25519.Sign(stranger, SignedMessage("v1.3.18", "KVMSwitch-mac.zip", 3, sum[:]))) == nil {
		t.Fatal("不受信任的密钥被接受")
	}
	if _, err := DecodeSignature([]byte("not-base64!")); err == nil {
		t.Fatal("格式错误的签名文件被接受")
	}
}

// 端到端：模拟 GitHub 提供安装包和签名，只有签名正确时才得到可安装的文件。
func TestDownloadRequiresSignature(t *testing.T) {
	priv := withTestKey(t)
	const tag, name = "v1.3.18", "KVMSwitch-mac.zip"
	body := []byte("real package")
	sum := sha256.Sum256(body)
	good := EncodeSignature(ed25519.Sign(priv, SignedMessage(tag, name, int64(len(body)), sum[:])))
	_, stranger, _ := ed25519.GenerateKey(rand.Reader)
	forged := EncodeSignature(ed25519.Sign(stranger, SignedMessage(tag, name, int64(len(body)), sum[:])))

	sig := good
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, SigSuffix) {
			w.Write(sig)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()
	m := &Manager{client: srv.Client()}
	p := pkg{
		Asset: Asset{Name: name, Size: int64(len(body)), Digest: fmt.Sprintf("sha256:%x", sum), URL: srv.URL + "/" + name},
		Sig:   Asset{Name: name + SigSuffix, URL: srv.URL + "/" + name + SigSuffix},
	}

	path, err := m.download(tag, p)
	if err != nil {
		t.Fatal("正确签名的安装包被拒绝：", err)
	}
	if b, _ := os.ReadFile(path); !bytes.Equal(b, body) {
		t.Fatal("下载内容不符")
	}
	os.RemoveAll(filepath.Dir(path))

	for what, s := range map[string][]byte{"伪造的签名": forged, "空签名": []byte("\n"), "超大签名": bytes.Repeat([]byte("A"), maxSig+1)} {
		sig = s
		if path, err := m.download(tag, p); err == nil {
			os.RemoveAll(filepath.Dir(path))
			t.Fatalf("%s被接受", what)
		}
	}
	sig = good
	if _, err := m.download("v1.3.19", p); err == nil {
		t.Fatal("签名与标签不符却被接受")
	}
}
