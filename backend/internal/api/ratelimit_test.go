package api

import "testing"

func TestLoginLimiterLocksAfterMaxFailures(t *testing.T) {
	l := newLoginLimiter()
	key := "1.2.3.4|alice"
	for i := 0; i < l.maxFailures; i++ {
		if l.Blocked(key) {
			t.Fatalf("locked too early at attempt %d", i+1)
		}
		l.Fail(key)
	}
	if !l.Blocked(key) {
		t.Fatal("should be locked after max failures")
	}
	// 其他 key 不受影响
	if l.Blocked("1.2.3.4|bob") {
		t.Fatal("unrelated key should not be locked")
	}
}

func TestLoginLimiterReset(t *testing.T) {
	l := newLoginLimiter()
	key := "5.6.7.8|bob"
	for i := 0; i < l.maxFailures; i++ {
		l.Fail(key)
	}
	l.Reset(key)
	if l.Blocked(key) {
		t.Fatal("reset should clear lock")
	}
}
