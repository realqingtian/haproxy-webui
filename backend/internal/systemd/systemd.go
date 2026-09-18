// Package systemd 经 SSH 管理节点 systemd 服务(v0.7 dataplaneapi 服务管理)。
//
// 安全约定:
//   - unit 名必须通过 ValidUnit(排除 shell 元字符),拼装命令时再单引号包裹,双保险防注入;
//   - 重启走 `sudo -n`(非交互),要求节点侧为 SSH 用户配置 sudoers 免密白名单,
//     例如:`sshuser ALL=(root) NOPASSWD: /usr/bin/systemctl restart dataplaneapi`;
//   - host key 指纹校验暂未实现:与节点 dataplaneapi 明文 HTTP 同属当前风险面(见 PLAN
//     已知风险),仅限可信网络使用,后续可加 known_hosts 指纹录入。
package systemd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var unitNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9@._-]{0,127}$`)

// ValidUnit 校验 systemd unit 名:systemd 允许字符的子集,且排除 shell 元字符。
func ValidUnit(unit string) bool { return unitNameRe.MatchString(unit) }

// Config 是一次 SSH 管理操作的连接与目标描述。
type Config struct {
	Host       string
	Port       int
	User       string
	Password   string
	PrivateKey string // PEM 私钥文本;非空时优先于密码
	Unit       string
}

// Status 是 systemctl show 解析出的服务状态。
type Status struct {
	Unit        string `json:"unit"`
	ActiveState string `json:"activeState"` // active / inactive / failed / activating ...
	SubState    string `json:"subState"`    // running / dead / auto-restart ...
	Since       string `json:"since"`       // ExecMainStartTimestamp 原文(节点时区)
	PID         uint32 `json:"pid"`
}

// Status 查询服务状态(systemctl show 无需 root)。
func (c Config) Status(ctx context.Context) (*Status, error) {
	cmd := fmt.Sprintf("systemctl show '%s' --no-pager"+
		" --property ActiveState --property SubState"+
		" --property ExecMainStartTimestamp --property ExecMainPid", c.Unit)
	out, err := c.run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return parseShowOutput(out, c.Unit), nil
}

// parseShowOutput 解析 systemctl show 的 key=value 输出(未出现的字段保持零值)。
func parseShowOutput(out, unit string) *Status {
	st := &Status{Unit: unit}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		switch k {
		case "ActiveState":
			st.ActiveState = v
		case "SubState":
			st.SubState = v
		case "ExecMainStartTimestamp":
			st.Since = v
		case "ExecMainPid":
			if pid, err := strconv.ParseUint(v, 10, 32); err == nil {
				st.PID = uint32(pid)
			}
		}
	}
	return st
}

// Restart 远程重启服务(需要 sudoers 免密白名单),返回命令输出供排障。
func (c Config) Restart(ctx context.Context) (string, error) {
	return c.run(ctx, fmt.Sprintf("sudo -n systemctl restart '%s'", c.Unit))
}

// run 建立 SSH 连接执行单条命令,非零退出码返回含 stderr 的错误。
func (c Config) run(ctx context.Context, cmd string) (string, error) {
	auth, err := c.authMethod()
	if err != nil {
		return "", err
	}
	timeout := 10 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < timeout {
			timeout = d
		}
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), &ssh.ClientConfig{
		User: c.User,
		Auth: []ssh.AuthMethod{auth},
		// 见包注释:暂不对 host key 做指纹校验
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	})
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("open ssh session: %w", err)
	}
	defer session.Close()

	// ctx 提前取消(如 HTTP 客户端断开)时主动关闭连接,避免悬挂
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			client.Close()
		case <-done:
		}
	}()

	var stderr strings.Builder
	session.Stderr = &stderr
	out, err := session.Output(cmd)
	if err != nil {
		// 错误详情优先取 stderr;部分链路(如合并输出的服务端)只在 stdout 里给消息
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(string(out))
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return string(out), nil
}

func (c Config) authMethod() (ssh.AuthMethod, error) {
	if strings.TrimSpace(c.PrivateKey) != "" {
		signer, err := ssh.ParsePrivateKey([]byte(c.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse ssh private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	}
	if c.Password != "" {
		return ssh.Password(c.Password), nil
	}
	return nil, errors.New("no ssh credential configured")
}
