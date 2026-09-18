// Package systemdtest 提供进程内 SSH 服务器测试辅助:模拟节点 sshd,
// exec 请求交由回调应答。仅供各包的 _test 文件使用。
package systemdtest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"

	"haproxy-webui/backend/internal/systemd"
)

// NewServer 起一个只支持 exec 的 SSH 服务器,返回监听地址与主机 key 指纹
// (systemd.Fingerprint 格式,可直接填入实例的 ExpectedFingerprint)。
func NewServer(t testing.TB, user, pass string, exec func(cmd string) (string, int)) (addr, fingerprint string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == user && string(password) == pass {
				return nil, nil
			}
			return nil, fmt.Errorf("auth failed for %s", conn.User())
		},
	}
	config.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
			if err != nil {
				continue
			}
			_ = sconn
			go ssh.DiscardRequests(reqs)
			go handleChannels(chans, exec)
		}
	}()
	return ln.Addr().String(), systemd.Fingerprint(signer.PublicKey())
}

func handleChannels(chans <-chan ssh.NewChannel, exec func(string) (string, int)) {
	for nch := range chans {
		if nch.ChannelType() != "session" {
			_ = nch.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		ch, reqs, err := nch.Accept()
		if err != nil {
			continue
		}
		go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
			defer ch.Close()
			for req := range reqs {
				if req.Type != "exec" {
					_ = req.Reply(false, nil)
					continue
				}
				var n uint32
				_ = binary.Read(bytes.NewReader(req.Payload), binary.BigEndian, &n)
				cmd := string(req.Payload[4 : 4+n])
				out, code := exec(cmd)
				_, _ = ch.Write([]byte(out))
				status := make([]byte, 4)
				binary.BigEndian.PutUint32(status, uint32(code))
				_, _ = ch.SendRequest("exit-status", false, status)
				_ = req.Reply(true, nil)
				return
			}
		}(ch, reqs)
	}
}
