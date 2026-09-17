package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/model"
)

// AuditHandler 审计日志查询:列表(后端截断,分页在前端)与 CSV 导出。
type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler { return &AuditHandler{db: db} }

const (
	auditListLimit  = 200
	auditExportLimit = 10000
)

// auditQuery 构造带公共过滤条件的查询;from/to 接受 RFC3339 或 datetime-local(秒/分钟精度)。
func (h *AuditHandler) auditQuery(c *gin.Context) (*gorm.DB, error) {
	q := h.db.Model(&model.AuditLog{})
	if action := c.Query("action"); action != "" {
		q = q.Where("action = ?", action)
	}
	if username := c.Query("username"); username != "" {
		q = q.Where("username = ?", username)
	}
	if target := c.Query("target"); target != "" {
		q = q.Where("target LIKE ?", "%"+target+"%")
	}
	from, err := parseTimeParam(c.Query("from"))
	if err != nil {
		return nil, err
	}
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	to, err := parseTimeParam(c.Query("to"))
	if err != nil {
		return nil, err
	}
	if to != nil {
		q = q.Where("created_at <= ?", *to)
	}
	return q, nil
}

// List GET /api/audit-logs
func (h *AuditHandler) List(c *gin.Context) {
	q, err := h.auditQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var logs []model.AuditLog
	if err := q.Order("id desc").Limit(auditListLimit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}

// ExportCSV GET /api/audit-logs/export:同过滤条件导出 CSV(UTF-8 BOM,便于 Excel 打开中文)。
func (h *AuditHandler) ExportCSV(c *gin.Context) {
	q, err := h.auditQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var logs []model.AuditLog
	if err := q.Order("id desc").Limit(auditExportLimit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="audit-logs-%s.csv"`, time.Now().Format("20060102-150405")))
	// BOM 让 Excel 识别 UTF-8
	c.String(http.StatusOK, "\xEF\xBB\xBF")
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"ID", "时间", "用户", "操作", "对象", "详情", "来源 IP"})
	for _, l := range logs {
		_ = w.Write([]string{
			fmt.Sprintf("%d", l.ID),
			l.CreatedAt.Format("2006-01-02 15:04:05"),
			l.Username, l.Action, l.Target, l.Detail, l.IP,
		})
	}
	w.Flush()
}

// parseTimeParam 解析时间过滤参数;空值返回 nil(不过滤)。
func parseTimeParam(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid time %q (want RFC3339 or 2006-01-02T15:04)", v)
}
