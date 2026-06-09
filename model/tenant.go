package model

import (
	"fmt"
	"strings"

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

// GetDeptIdFromContext 从 gin.Context 获取当前部门 ID
func GetDeptIdFromContext(c *gin.Context) int64 {
	if c == nil {
		return 0
	}
	return c.GetInt64("maas_dept_id")
}

// GetDataScopeFromContext 从 gin.Context 获取当前数据权限范围
// 返回 yudao-cloud DataScopeEnum 的整数值：
// 1=ALL, 2=DEPT_CUSTOM, 3=DEPT_ONLY, 4=DEPT_AND_CHILD, 5=SELF
func GetDataScopeFromContext(c *gin.Context) int {
	if c == nil {
		return 5 // 默认 SELF（最严格）
	}
	str := c.GetString("maas_data_scope")
	if str == "" {
		return 5
	}
	val := 0
	fmt.Sscanf(str, "%d", &val)
	if val == 0 {
		return 5
	}
	return val
}

// DataScope 常量（与 yudao-cloud DataScopeEnum 对齐）
const (
	DataScopeAll          = 1 // 全部数据权限
	DataScopeDeptCustom   = 2 // 指定部门数据权限
	DataScopeDeptOnly     = 3 // 部门数据权限
	DataScopeDeptAndChild = 4 // 部门及以下数据权限
	DataScopeSelf         = 5 // 仅本人数据权限
)

// DeptScopeFilter 根据数据权限范围构建部门过滤条件
// 返回 (condition, args) 对，调用方可以用 db.Where(condition, args...) 追加
//
// data_scope 语义：
//   - ALL(1):          无部门过滤
//   - DEPT_CUSTOM(2):  dept_id IN (dept_scope_ids)，需要从 context 获取 dept_scope_ids
//   - DEPT_ONLY(3):    dept_id = 当前部门 ID
//   - DEPT_AND_CHILD(4): dept_id IN (当前部门 + 子部门 IDs)
//   - SELF(5):         created_by = 当前用户 ID（由具体业务逻辑处理）
func DeptScopeFilter(c *gin.Context, tableName string) (string, []any) {
	if c == nil {
		return "", nil
	}

	dataScope := GetDataScopeFromContext(c)
	deptId := GetDeptIdFromContext(c)

	switch dataScope {
	case DataScopeAll:
		return "", nil

	case DataScopeDeptCustom:
		deptScopeIds := c.GetString("maas_dept_scope_ids")
		if deptScopeIds == "" {
			return "", nil
		}
		// dept_scope_ids 格式: "10,20,30"
		ids := strings.Split(deptScopeIds, ",")
		placeholders := make([]string, len(ids))
		args := make([]any, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			var v int64
			fmt.Sscanf(strings.TrimSpace(id), "%d", &v)
			args[i] = v
		}
		prefix := ""
		if tableName != "" {
			prefix = tableName + "."
		}
		return fmt.Sprintf("%sdept_id IN (%s)", prefix, strings.Join(placeholders, ",")), args

	case DataScopeDeptOnly:
		prefix := ""
		if tableName != "" {
			prefix = tableName + "."
		}
		return fmt.Sprintf("%sdept_id = ?", prefix), []any{deptId}

	case DataScopeDeptAndChild:
		// 简化实现：使用 dept_scope_ids（包含当前部门+子部门）
		deptScopeIds := c.GetString("maas_dept_scope_ids")
		if deptScopeIds != "" {
			ids := strings.Split(deptScopeIds, ",")
			placeholders := make([]string, len(ids))
			args := make([]any, len(ids))
			for i, id := range ids {
				placeholders[i] = "?"
				var v int64
				fmt.Sscanf(strings.TrimSpace(id), "%d", &v)
				args[i] = v
			}
			prefix := ""
			if tableName != "" {
				prefix = tableName + "."
			}
			return fmt.Sprintf("%sdept_id IN (%s)", prefix, strings.Join(placeholders, ",")), args
		}
		// 无 dept_scope_ids 时，退化为 DEPT_ONLY
		prefix := ""
		if tableName != "" {
			prefix = tableName + "."
		}
		return fmt.Sprintf("%sdept_id = ?", prefix), []any{deptId}

	case DataScopeSelf:
		// SELF 由具体业务逻辑处理（通常在查询中添加 user_id 过滤）
		return "", nil

	default:
		return "", nil
	}
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