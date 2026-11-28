package update

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// 发布签名：每个安装包都附带一个同名的 .sig 文件，内容是 Base64 编码的 Ed25519 签名。
// 私钥只保存在发布者自己的电脑上，不在 GitHub。即使 GitHub 账号或令牌被盗、有人发布了
// 假的 Release，没有私钥也签不出能通过校验的安装包。
//
// 签名覆盖版本标签、文件名、大小和 SHA-256：不能把旧版本的安装包换个标签重新发布来降级，
// 也不能把一个平台的安装包当作另一个平台的用。

// SigSuffix 是签名文件相对安装包追加的后缀。
const SigSuffix = ".sig"

const maxSig = 4096

// trustedKeyText 是可以签发更新的公钥（Base64）。轮换密钥时，先用旧密钥签发一个
// 同时信任新旧两把公钥的版本，之后再改用新密钥签名。
var trustedKeyText = []string{
	"9tiS/SKILFJ9MagWMH6DJl+6vcl306iaj5zm2KCFTv0=", // 2026-11 发布密钥
}

var trusted = mustKeys(trustedKeyText)

func mustKeys(text []string) []ed25519.PublicKey {
	keys := make([]ed25519.PublicKey, 0, len(text))
	for _, s := range text {
		k, err := base64.StdEncoding.DecodeString(s)
		if err != nil || len(k) != ed25519.PublicKeySize {
			panic(fmt.Sprintf("内置的发布公钥格式错误：%q", s))
		}
		keys = append(keys, ed25519.PublicKey(k))
	}
	return keys
}

// ValidTag 报告 tag 是否为可发布的正式版本标签，例如 v1.3.18。
func ValidTag(tag string) bool {
	return strings.HasPrefix(tag, "v") && semver.MatchString(tag)
}

// SignedMessage 返回发布签名实际签署的内容。
func SignedMessage(tag, name string, size int64, sum []byte) []byte {
	return fmt.Appendf(nil, "KVMSwitch release signature v1\ntag=%s\nname=%s\nsize=%d\nsha256=%x\n", tag, name, size, sum)
}

// EncodeSignature 把签名编码成 .sig 文件的内容。
func EncodeSignature(sig []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(sig) + "\n")
}

// DecodeSignature 解析 .sig 文件的内容。
func DecodeSignature(b []byte) ([]byte, error) {
	sig, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(b)))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, errors.New("发布签名文件格式错误")
	}
	return sig, nil
}

// Trusted 报告 pub 是否在程序内置的信任列表里。
func Trusted(pub ed25519.PublicKey) bool {
	for _, k := range trusted {
		if k.Equal(pub) {
			return true
		}
	}
	return false
}

// VerifySignature 检查 sig 是否由内置的某一把公钥对这个安装包签发。
func VerifySignature(tag, name string, size int64, sum, sig []byte) error {
	if len(trusted) == 0 {
		return errors.New("程序没有内置发布公钥，无法校验更新")
	}
	msg := SignedMessage(tag, name, size, sum)
	for _, k := range trusted {
		if ed25519.Verify(k, msg, sig) {
			return nil
		}
	}
	return errors.New("发布签名无效")
}
