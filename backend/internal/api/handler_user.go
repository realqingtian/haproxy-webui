package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/auth"
	"haproxy-webui/backend/internal/model"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

// otherAdminExists 除 excludeID 外是否还存在 admin 用户。
func (h *UserHandler) otherAdminExists(excludeID uint) bool {
	var n int64
	h.db.Model(&model.User{}).
		Where("role = ? AND id <> ?", model.RoleAdmin, excludeID).
		Count(&n)
	return n > 0
}

// List GET /api/users(仅 admin)
func (h *UserHandler) List(c *gin.Context) {
	var users []model.User
	if err := h.db.Select("id, username, role, created_at, updated_at").Order("id").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

type createUserRequest struct {
	Username string `json:"username" binding:"required,min=2,max=64"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role" binding:"required,oneof=admin operator viewer"`
}

// Create POST /api/users(仅 admin)
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user := model.User{Username: req.Username, PasswordHash: string(hash), Role: req.Role}
	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名可能已存在: " + err.Error()})
		return
	}
	audit(c, "user.create", req.Username, "role="+req.Role)
	user.PasswordHash = ""
	c.JSON(http.StatusCreated, user)
}

type updateUserRequest struct {
	Role     *string `json:"role,omitempty" binding:"omitempty,oneof=admin operator viewer"`
	Password *string `json:"password,omitempty" binding:"omitempty,min=6"`
}

// Update PUT /api/users/:id(仅 admin):改角色 / 重置密码
func (h *UserHandler) Update(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var user model.User
	if err := h.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 防止系统失去最后一个管理员
	if req.Role != nil && *req.Role != user.Role {
		if user.Role == model.RoleAdmin && *req.Role != model.RoleAdmin && !h.otherAdminExists(user.ID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能降级最后一个管理员"})
			return
		}
		user.Role = *req.Role
	}
	if req.Password != nil && *req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		user.PasswordHash = string(hash)
	}
	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	detail := ""
	if req.Role != nil {
		detail += "role=" + *req.Role + " "
	}
	if req.Password != nil && *req.Password != "" {
		detail += "password reset"
	}
	audit(c, "user.update", user.Username, detail)
	user.PasswordHash = ""
	c.JSON(http.StatusOK, user)
}

// Delete DELETE /api/users/:id(仅 admin):不能删自己、不能删最后一个 admin
func (h *UserHandler) Delete(c *gin.Context) {
	claims := auth.ClaimsFromContext(c)
	var user model.User
	if err := h.db.First(&user, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if claims != nil && user.ID == claims.UserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除当前登录的账号"})
		return
	}
	if user.Role == model.RoleAdmin && !h.otherAdminExists(user.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除最后一个管理员"})
		return
	}
	if err := h.db.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "user.delete", user.Username, "")
	c.JSON(http.StatusOK, gin.H{"deleted": user.ID})
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// ChangePassword PUT /api/auth/password:当前登录用户修改自己的密码
func (h *UserHandler) ChangePassword(c *gin.Context) {
	claims := auth.ClaimsFromContext(c)
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码不正确"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user.PasswordHash = string(hash)
	if err := h.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "user.password", user.Username, "self change")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
