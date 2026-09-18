package systemd

import "testing"

func TestValidUnit(t *testing.T) {
	cases := []struct {
		unit string
		want bool
	}{
		{"dataplaneapi", true},
		{"haproxy-dataplaneapi.service", true},
		{"dpapi@1.service", true},
		{"", false},
		{".hidden", false},                 // 不允许首字符为 .
		{"dp api", false},                  // 空格(shell 词元)
		{"dp;reboot", false},               // shell 元字符
		{"dp$(reboot)", false},             // 命令替换
		{"dp`reboot`", false},              // 反引号
		{"dp'quote", false},                // 引号
		{"dp&bg", false},                   // & 符
		{"dp|pipe", false},                 // 管道
		{"../escape", false},               // 路径穿越形态
		{string(make([]byte, 200)), false}, // 超长
	}
	for _, c := range cases {
		if got := ValidUnit(c.unit); got != c.want {
			t.Errorf("ValidUnit(%q) = %v, want %v", c.unit, got, c.want)
		}
	}
}

func TestParseShowOutput(t *testing.T) {
	out := "ActiveState=active\n" +
		"SubState=running\n" +
		"ExecMainStartTimestamp=Tue 2026-09-15 10:00:00 UTC\n" +
		"ExecMainPid=4321\n" +
		"KillMode=control-group\n" + // 未请求的属性行也应被忽略
		"\n"
	st := parseShowOutput(out, "dataplaneapi")
	if st.ActiveState != "active" || st.SubState != "running" || st.PID != 4321 {
		t.Fatalf("parsed = %+v", st)
	}
	if st.Since != "Tue 2026-09-15 10:00:00 UTC" {
		t.Fatalf("since = %q", st.Since)
	}
	// 空输出(inactive 无进程)不报错
	st = parseShowOutput("ActiveState=inactive\nSubState=dead\nExecMainPid=0\n", "x")
	if st.ActiveState != "inactive" || st.PID != 0 {
		t.Fatalf("inactive parsed = %+v", st)
	}
}

func TestVipInAddrOutput(t *testing.T) {
	out := "1: lo    inet 127.0.0.1/8 scope host lo\\       valid_lft forever preferred_lft forever\n" +
		"11: eth0    inet 172.28.255.11/24 brd 172.28.255.255 scope global eth0\\       valid_lft forever\n" +
		"11: eth0    inet 172.28.255.100/24 scope global secondary eth0\\       valid_lft forever\n"
	if !vipInAddrOutput("172.28.255.100", out) {
		t.Fatal("VIP present but not detected")
	}
	if vipInAddrOutput("172.28.255.12", out) {
		t.Fatal("wrong VIP detected")
	}
	if vipInAddrOutput("not-an-ip", out) {
		t.Fatal("invalid VIP should not match")
	}
	if vipInAddrOutput("", out) {
		t.Fatal("empty VIP should not match")
	}
	// IPv4-mapped 写法仍应按同一地址匹配
	if !vipInAddrOutput("::ffff:172.28.255.100", out) {
		t.Fatal("IPv4-mapped form should match same address")
	}
}
