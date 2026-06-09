package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// IsTenantScopeEnabled 检查是否启用了租户隔离
func IsTenantScopeEnabled() bool {
	return common.GetEnvOrDefaultString("MAAS_JWT_SECRET", "") != ""
}

// GetTenantIdFromContext 从 gin.Context 获取当前租户 ID
func GetTenantIdFromContext(c *gin.Context) int64 {
	if c == nil {
		return 0
	}
	return c.GetInt64("maas_tenant_id")
}

// TenantScope 返回带租户过滤的 DB 实例
// 使用方式: db := model.TenantScope(DB, tenantId).Where(...)
func TenantScope(db *gorm.DB, tenantId int64) *gorm.DB {
	if tenantId == 0 || !IsTenantScopeEnabled() {
		return db
	}
	return db.Where("tenant_id = ?", tenantId)
}

// TenantFilterSQL 返回租户过滤的 SQL 条件
// 用于需要手动拼接 SQL 的场景
func TenantFilterSQL(tableName string, tenantId int64) string {
	if tenantId == 0 || !IsTenantScopeEnabled() {
		return ""
	}
	return fmt.Sprintf("%s.tenant_id = %d", tableName, tenantId)
}

// RegisterTenantCallbacks 注册 GORM 回调实现自动租户过滤
// 仅在 MaaS 模式（MAAS_JWT_SECRET 已配置）下启用
// 需要通过 SetTenantInDB 传递 tenant_id
func RegisterTenantCallbacks(db *gorm.DB) {
	if !IsTenantScopeEnabled() {
		return
	}

	_ = db.Callback().Query().Before("gorm:query").Register("maas:tenant_query", tenantQueryCallback)
	_ = db.Callback().Create().Before("gorm:create").Register("maas:tenant_create", tenantCreateCallback)
	_ = db.Callback().Update().Before("gorm:update").Register("maas:tenant_update", tenantScopeCallback)
	_ = db.Callback().Delete().Before("gorm:delete").Register("maas:tenant_delete", tenantScopeCallback)
}

// SetTenantInDB 设置 GORM DB 实例中的 tenant_id
func SetTenantInDB(db *gorm.DB, tenantId int64) *gorm.DB {
	if tenantId == 0 {
		return db
	}
	return db.Set("maas:tenant_id", tenantId)
}

// getTenantIdFromDB 从 GORM DB 实例获取 tenant_id
func getTenantIdFromDB(db *gorm.DB) int64 {
	if db == nil {
		return 0
	}
	val, ok := db.Get("maas:tenant_id")
	if !ok {
		return 0
	}
	tenantId, ok := val.(int64)
	if !ok {
		return 0
	}
	return tenantId
}

// 需要租户隔离的表列表
var tenantScopedTables = map[string]bool{
	"tokens":    true,
	"channels":  true,
	"logs":      true,
	"users":     true,
	"abilities": true,
}

// isTenantScopedTable 检查表是否需要租户隔离
func isTenantScopedTable(db *gorm.DB) bool {
	tableName := ""
	if db.Statement != nil {
		if db.Statement.Table != "" {
			tableName = db.Statement.Table
		} else if db.Statement.Schema != nil {
			tableName = db.Statement.Schema.Table
		}
	}
	return tenantScopedTables[tableName]
}

// tenantQueryCallback 查询回调：自动追加 tenant_id WHERE 条件
func tenantQueryCallback(db *gorm.DB) {
	tenantId := getTenantIdFromDB(db)
	if tenantId == 0 || !isTenantScopedTable(db) {
		return
	}
	db.Where("tenant_id = ?", tenantId)
}

// tenantCreateCallback 创建回调：自动填充 tenant_id
func tenantCreateCallback(db *gorm.DB) {
	tenantId := getTenantIdFromDB(db)
	if tenantId == 0 || !isTenantScopedTable(db) {
		return
	}
	db.Statement.SetColumn("tenant_id", tenantId, true)
}

// tenantScopeCallback 更新/删除回调：限制只能操作本租户数据
func tenantScopeCallback(db *gorm.DB) {
	tenantId := getTenantIdFromDB(db)
	if tenantId == 0 || !isTenantScopedTable(db) {
		return
	}
	db.Where("tenant_id = ?", tenantId)
}