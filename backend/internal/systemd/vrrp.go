package systemd

import (
	"context"
	"fmt"
	"net"
	"strings"
)

// VRRPStatus 是节点 keepalived 的 VRRP 探测结果(v0.8)。
type VRRPStatus struct {
	KeepalivedRunning bool   `json:"keepalivedRunning"`
	VipPresent        bool   `json:"vipPresent"`
	Role              string `json:"role"` // master / backup / fault
}

// KeepalivedStatus 探测节点 keepalived 进程与 VIP 归属,推导 VRRP 角色。
//
// 实现约定:
//   - 进程存活用 pgrep 而非 systemctl:不依赖 systemd(容器 / alpine 节点同样可用);
//   - VIP 只在 Go 侧与 `ip -o -4 addr show` 输出比对,不进入命令串;
//   - VIP 为空(集群未填)时仍可探测进程与角色,但无法区分主备(恒为 backup 或 fault)。
func (c Config) KeepalivedStatus(ctx context.Context, vip string) (*VRRPStatus, error) {
	procOut, err := c.run(ctx, "pgrep -x keepalived >/dev/null && echo up || echo down")
	if err != nil {
		return nil, fmt.Errorf("check keepalived process: %w", err)
	}
	st := &VRRPStatus{KeepalivedRunning: strings.TrimSpace(procOut) == "up"}

	var addrOut string
	if strings.TrimSpace(vip) != "" {
		addrOut, err = c.run(ctx, "ip -o -4 addr show")
		if err != nil {
			return nil, fmt.Errorf("list node addresses: %w", err)
		}
		st.VipPresent = vipInAddrOutput(vip, addrOut)
	}

	switch {
	case !st.KeepalivedRunning:
		st.Role = "fault"
	case st.VipPresent:
		st.Role = "master"
	default:
		st.Role = "backup"
	}
	return st, nil
}

// vipInAddrOutput 判断 VIP 是否出现在 `ip -o -4 addr show` 输出中(精确 IP 匹配,忽略掩码)。
func vipInAddrOutput(vip, out string) bool {
	want := net.ParseIP(strings.TrimSpace(vip))
	if want == nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, part := range fields {
			if part != "inet" || i+1 >= len(fields) {
				continue
			}
			addr := fields[i+1]
			if j := strings.Index(addr, "/"); j >= 0 {
				addr = addr[:j]
			}
			if got := net.ParseIP(addr); got != nil && got.Equal(want) {
				return true
			}
		}
	}
	return false
}
