// sign 管理发布签名密钥，并给安装包生成 .sig 签名文件。
//
// 用法：
//
//	go run ./tools/sign keygen                 生成私钥，打印要写进 internal/update/sign.go 的公钥
//	go run ./tools/sign sign <标签> <文件>...   为每个文件生成 <文件>.sig，例如 sign v1.3.18 dist/KVMSwitch-mac.zip
//	go run ./tools/sign verify <标签> <文件>... 用程序内置的公钥校验 <文件>.sig
//
// 私钥默认保存在 ~/.kvmswitch-release/ed25519.key，可用环境变量 KVMSWITCH_SIGN_KEY 指定其他路径。
// 私钥不能提交到仓库，也不要上传到 GitHub；请另外离线备份，丢失后已安装的客户端将无法自动更新。
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"kvmswitch/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(keyPath())
	case "sign", "verify":
		if len(os.Args) < 4 {
			usage()
		}
		tag, files := os.Args[2], os.Args[3:]
		if !update.ValidTag(tag) {
			err = fmt.Errorf("标签 %q 不是正式版本标签，应形如 v1.3.18", tag)
		} else if os.Args[1] == "sign" {
			err = sign(keyPath(), tag, files)
		} else {
			err = verify(tag, files)
		}
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "失败：", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法：sign keygen | sign sign <标签> <文件>... | sign verify <标签> <文件>...")
	os.Exit(2)
}

func keyPath() string {
	if p := os.Getenv("KVMSWITCH_SIGN_KEY"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "找不到用户目录：", err)
		os.Exit(1)
	}
	return filepath.Join(home, ".kvmswitch-release", "ed25519.key")
}

func keygen(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s 已存在，不会覆盖已有的私钥", path)
		}
		return err
	}
	_, err = f.WriteString(base64.StdEncoding.EncodeToString(priv.Seed()) + "\n")
	if ce := f.Close(); err == nil {
		err = ce
	}
	if err != nil {
		os.Remove(path)
		return err
	}
	fmt.Println("私钥已保存到：", path)
	fmt.Println("公钥（写进 internal/update/sign.go 的 trustedKeyText）：")
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
	return nil
}

func loadKey(path string) (ed25519.PrivateKey, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败：%w（先运行 keygen，或设置 KVMSWITCH_SIGN_KEY）", err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s 的权限过宽（%v），请先执行 chmod 600", path, st.Mode().Perm())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("%s 不是有效的发布私钥", path)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func digest(path string) (int64, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	return n, h.Sum(nil), err
}

func sign(keyFile, tag string, files []string) error {
	priv, err := loadKey(keyFile)
	if err != nil {
		return err
	}
	if !update.Trusted(priv.Public().(ed25519.PublicKey)) {
		return errors.New("这把私钥对应的公钥不在程序内置的信任列表里，用它签名的更新会被客户端拒绝")
	}
	for _, file := range files {
		size, sum, err := digest(file)
		if err != nil {
			return err
		}
		name := filepath.Base(file)
		sig := ed25519.Sign(priv, update.SignedMessage(tag, name, size, sum))
		// 写出前用客户端完全相同的逻辑再校验一次。
		if err := update.VerifySignature(tag, name, size, sum, sig); err != nil {
			return fmt.Errorf("%s：%w", file, err)
		}
		if err := os.WriteFile(file+update.SigSuffix, update.EncodeSignature(sig), 0o644); err != nil {
			return err
		}
		fmt.Printf("已签名 %s（%s，%d 字节，sha256 %x）\n", file+update.SigSuffix, tag, size, sum)
	}
	return nil
}

func verify(tag string, files []string) error {
	for _, file := range files {
		size, sum, err := digest(file)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(file + update.SigSuffix)
		if err != nil {
			return err
		}
		sig, err := update.DecodeSignature(b)
		if err == nil {
			err = update.VerifySignature(tag, filepath.Base(file), size, sum, sig)
		}
		if err != nil {
			return fmt.Errorf("%s：%w", file, err)
		}
		fmt.Printf("签名有效：%s（%s）\n", file, tag)
	}
	return nil
}
