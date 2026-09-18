// Package systemd 经 SSH 管理节点 systemd 服务与 keepalived 探测
// (v0.7 dataplaneapi 服务管理、v0.8 集群视角、v0.9 host key 指纹校验)。
//
// 安全约定:
//   - unit 名必须通过 ValidUnit(排除 shell 元字符),拼装命令时再单引号包裹,双保险防注入;
//   - host key 指纹:ExpectedFingerprint 非空时严格校验(不匹配拒绝连接);
//     为空时信任首次连接(TOFU),实际指纹记入 CapturedFingerprint 由调用方回写实例,
//     之后即进入钉扎模式——首次连接窗口是与明文 HTTP 同级的已知风险面(见 PLAN 已知风险);
//   - 重启走 `sudo -n`(非交互),要求节点侧为 SSH 用户配置 sudoers 免密白名单,
//     例如:`sshuser ALL=(root) NOPASSWD: /usr/bin/systemctl restart dataplaneapi`。
package systemd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/model"
)

var unitNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9@._-]{0,127}$`)

// ValidUnit 校验 systemd unit 名:systemd 允许字符的子集,且排除 shell 元字符。
func ValidUnit(unit string) bool { return unitNameRe.MatchString(unit) }

// Fingerprint 计算 host key 指纹,格式对齐 `ssh-keygen -lf`(SHA256:<base64>)。
func Fingerprint(key ssh.PublicKey) string {
	sum := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// ConfigFromInstance 从实例记录构造 SSH 管理配置:
// 地址回退 BaseURL host,端口缺省 22,unit 缺省 dataplaneapi;指纹取实例已记录值。
func ConfigFromInstance(inst *model.Instance) (Config, error) {
	host := strings.TrimSpace(inst.SSHHost)
	if host == "" {
		u, err := url.Parse(inst.BaseURL)
		if err != nil || u.Hostname() == "" {
			return Config{}, errors.New("SSH 地址为空且无法从 BaseURL 推导")
		}
		host = u.Hostname()
	}
	port := inst.SSHPort
	if port == 0 {
		port = 22
	}
	unit := strings.TrimSpace(inst.SSHUnit)
	if unit == "" {
		unit = "dataplaneapi"
	}
	if !ValidUnit(unit) {
		return Config{}, errors.New("unit 名包含非法字符(仅允许字母数字与 @ . _ -)")
	}
	return Config{
		Host:                host,
		Port:                port,
		User:                inst.SSHUser,
		Password:            cryptoutil.DecryptStoredOrDefault(inst.SSHPassword),
		PrivateKey:          cryptoutil.DecryptStoredOrDefault(inst.SSHPrivateKey),
		Unit:                unit,
		ExpectedFingerprint: strings.TrimSpace(inst.SSHHostKey),
	}, nil
}

// Hint 把常见 SSH / sudo / 指纹失败翻译成可操作提示;无法识别返回空串。
func Hint(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no ssh credential"):
		return "实例未配置 SSH 凭据:编辑实例填写 SSH 用户与密码或私钥"
	case strings.Contains(msg, "parse ssh private key"):
		return "SSH 私钥格式无法解析,请提供 PEM 格式(OpenSSH 新格式需以 -----BEGIN OPENSSH PRIVATE KEY----- 开头)"
	case strings.Contains(msg, "unable to authenticate"), strings.Contains(msg, "auth failed"):
		return "SSH 认证失败:检查用户名与密码 / 私钥"
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "timed out"), strings.Contains(msg, "no route"):
		return "无法建立 SSH 连接:检查地址 / 端口与节点防火墙、安全组"
	case strings.Contains(msg, "password is required"), strings.Contains(msg, "a password is required"):
		return "sudo 需要密码:请为 SSH 用户配置免密白名单,如 `sshuser ALL=(root) NOPASSWD: /usr/bin/systemctl restart dataplaneapi`"
	case strings.Contains(msg, "not found"), strings.Contains(msg, "Unknown"):
		return "节点上不存在该 unit,确认服务名是否为 dataplaneapi(可在实例设置中修改)"
	case strings.Contains(msg, "host key fingerprint mismatch"):
		return "SSH host key 指纹与实例记录不一致:节点可能重装过系统,也可能存在中间人。确认节点身份无误后,在实例设置中清除已记录的指纹再重连"
	default:
		return ""
	}
}

// Config 是一次 SSH 管理操作的连接与目标描述。
// ExpectedFingerprint 非空时钉扎校验;CapturedFingerprint 在连接建立时被填入
// (TOFU 场景由调用方回写实例),因此各方法使用指针接收者。
type Config struct {
	Host       string
	Port       int
	User       string
	Password   string
	PrivateKey string // PEM 私钥文本;非空时优先于密码
	Unit       string

	ExpectedFingerprint string // 实例已记录的 host key 指纹;空 = TOFU
	CapturedFingerprint string // 最近一次连接实际观察到的指纹(出参)
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
func (c *Config) Status(ctx context.Context) (*Status, error) {
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
func (c *Config) Restart(ctx context.Context) (string, error) {
	return c.run(ctx, fmt.Sprintf("sudo -n systemctl restart '%s'", c.Unit))
}

// run 建立 SSH 连接执行单条命令,非零退出码返回含 stderr 的错误。
func (c *Config) run(ctx context.Context, cmd string) (string, error) {
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
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fp := Fingerprint(key)
			c.CapturedFingerprint = fp // Dial 期间同步回调,写入可见于调用方
			if c.ExpectedFingerprint != "" && c.ExpectedFingerprint != fp {
				return fmt.Errorf("host key fingerprint mismatch: expect %s, got %s",
					c.ExpectedFingerprint, fp)
			}
			return nil
		},
		Timeout: timeout,
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

func (c *Config) authMethod() (ssh.AuthMethod, error) {
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
