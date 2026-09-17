package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// v0.6 会话管理:token 代次吊销(改密 / 重置 / 删除 / 强制下线)与滑动续期。

func createNamedUser(t *testing.T, r *gin.Engine, adminToken, username, password, role string) uint {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/users", adminToken,
		map[string]string{"username": username, "password": password, "role": role})
	if w.Code != http.StatusCreated {
		t.Fatalf("create user %s: %d %s", username, w.Code, w.Body.String())
	}
	var u struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &u)
	return u.ID
}

func TestChangePasswordRevokesOldToken(t *testing.T) {
	r, _ := newTestEnv(t)
	oldToken := loginToken(t, r, "admin", "admin123")

	w := doJSON(t, r, http.MethodPut, "/api/auth/password", oldToken,
		map[string]string{"oldPassword": "admin123", "newPassword": "newpass456"})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"relogin":true`) {
		t.Fatalf("change password: %d %s", w.Code, w.Body.String())
	}

	// 旧 token 已吊销
	if w := doJSON(t, r, http.MethodGet, "/api/auth/me", oldToken, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("old token after change should 401: %d %s", w.Code, w.Body.String())
	}
	// 新密码可登录,旧密码不可
	if w := doJSON(t, r, http.MethodPost, "/api/auth/login", "",
		map[string]string{"username": "admin", "password": "admin123"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("old password should fail: %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/auth/login", "",
		map[string]string{"username": "admin", "password": "newpass456"}); w.Code != http.StatusOK {
		t.Fatalf("new password should work: %d %s", w.Code, w.Body.String())
	}
}

func TestRefreshIssuesNewToken(t *testing.T) {
	r, _ := newTestEnv(t)
	token := loginToken(t, r, "admin", "admin123")

	w := doJSON(t, r, http.MethodPost, "/api/auth/refresh", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Token == "" {
		t.Fatal("refresh returned empty token")
	}
	// 新 token 可用,旧 token 在有效期内仍可用(未被吊销)
	for _, tk := range []string{token, resp.Token} {
		if w := doJSON(t, r, http.MethodGet, "/api/auth/me", tk, nil); w.Code != http.StatusOK {
			t.Fatalf("me with refreshed token: %d %s", w.Code, w.Body.String())
		}
	}
	// 无效 token 不能 refresh
	if w := doJSON(t, r, http.MethodPost, "/api/auth/refresh", "bogus", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh with bad token: %d", w.Code)
	}
}

func TestForceLogoutRevokesSessions(t *testing.T) {
	r, _ := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	uid := createNamedUser(t, r, admin, "op1", "pass1234", "operator")
	opToken := loginToken(t, r, "op1", "pass1234")

	// 不能对自己强制下线
	if w := doJSON(t, r, http.MethodPost, "/api/users/1/force-logout", admin, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("force-logout self should 400: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodPost, fmt.Sprintf("/api/users/%d/force-logout", uid), admin, nil); w.Code != http.StatusOK {
		t.Fatalf("force-logout: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodGet, "/api/auth/me", opToken, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token should 401: %d", w.Code)
	}
	// 重新登录后恢复
	if loginToken(t, r, "op1", "pass1234") == "" {
		t.Fatal("re-login after force logout failed")
	}
}

func TestDeleteUserRevokesSessions(t *testing.T) {
	r, _ := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	uid := createNamedUser(t, r, admin, "tmp1", "pass1234", "viewer")
	token := loginToken(t, r, "tmp1", "pass1234")

	if w := doJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/users/%d", uid), admin, nil); w.Code != http.StatusOK {
		t.Fatalf("delete user: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodGet, "/api/auth/me", token, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("token after user delete should 401: %d", w.Code)
	}
}

func TestAdminPasswordResetRevokesTargetSessions(t *testing.T) {
	r, _ := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	uid := createNamedUser(t, r, admin, "op2", "pass1234", "operator")
	opToken := loginToken(t, r, "op2", "pass1234")

	w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/users/%d", uid), admin,
		map[string]string{"password": "reset5678"})
	if w.Code != http.StatusOK {
		t.Fatalf("reset password: %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, r, http.MethodGet, "/api/auth/me", opToken, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("target token after reset should 401: %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/auth/login", "",
		map[string]string{"username": "op2", "password": "reset5678"}); w.Code != http.StatusOK {
		t.Fatalf("login with reset password: %d %s", w.Code, w.Body.String())
	}
}

func TestRoleChangeTakesEffectWithoutRelogin(t *testing.T) {
	r, fake := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")
	uid := createNamedUser(t, r, admin, "promoted", "pass1234", "viewer")
	token := loginToken(t, r, "promoted", "pass1234")
	id := registerInstance(t, r, admin, fake.url, itDPPass)

	// viewer 写操作被拒
	if w := applyOps(t, r, token, id, []map[string]any{{"kind": "create_backend", "name": "b1"}}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer apply should 403: %d", w.Code)
	}
	// 提升为 operator 后,同一 token 立即可写(角色以 DB 实时为准)
	if w := doJSON(t, r, http.MethodPut, fmt.Sprintf("/api/users/%d", uid), admin,
		map[string]string{"role": "operator"}); w.Code != http.StatusOK {
		t.Fatalf("promote: %d %s", w.Code, w.Body.String())
	}
	if w := applyOps(t, r, token, id, []map[string]any{{"kind": "create_backend", "name": "b1"}}); w.Code != http.StatusOK {
		t.Fatalf("operator apply after promote: %d %s", w.Code, w.Body.String())
	}
}

func TestSettingsRBAC(t *testing.T) {
	r, _ := newTestEnv(t)
	admin := loginToken(t, r, "admin", "admin123")

	w := doJSON(t, r, http.MethodGet, "/api/settings", admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get settings: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"snapshotIntervalMinutes":60`) {
		t.Fatalf("settings defaults: %s", w.Body.String())
	}

	// admin 修改生效
	w = doJSON(t, r, http.MethodPut, "/api/settings", admin,
		map[string]any{"snapshotIntervalMinutes": 30, "monitorIntervalSeconds": 45})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"snapshotIntervalMinutes":30`) {
		t.Fatalf("put settings: %d %s", w.Code, w.Body.String())
	}

	// viewer:可读不可写
	createNamedUser(t, r, admin, "v1", "pass1234", "viewer")
	viewer := loginToken(t, r, "v1", "pass1234")
	if w := doJSON(t, r, http.MethodGet, "/api/settings", viewer, nil); w.Code != http.StatusOK {
		t.Fatalf("viewer get settings: %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPut, "/api/settings", viewer,
		map[string]any{"snapshotIntervalMinutes": 5}); w.Code != http.StatusForbidden {
		t.Fatalf("viewer put settings should 403: %d", w.Code)
	}

	// 越界值 400
	if w := doJSON(t, r, http.MethodPut, "/api/settings", admin,
		map[string]any{"monitorIntervalSeconds": 1}); w.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range monitor interval: %d %s", w.Code, w.Body.String())
	}
}
