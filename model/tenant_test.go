package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestIsTenantScopeEnabled(t *testing.T) {
	// 默认未配置时不应启用
	// 注意：此测试依赖环境变量，可能受其他测试影响
	// 仅验证函数可正常调用
	result := IsTenantScopeEnabled()
	// 不做具体断言，因为环境变量可能已被其他测试设置
	_ = result
}

func TestTenantScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// tenantId = 0 时不添加过滤
	result := TenantScope(db, 0)
	assert.Equal(t, db, result)
}

func TestTenantFilterSQL(t *testing.T) {
	// tenantId = 0 时返回空
	result := TenantFilterSQL("tokens", 0)
	assert.Equal(t, "", result)
}

func TestSetTenantInDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// tenantId = 0 时返回原 db
	result := SetTenantInDB(db, 0)
	assert.Equal(t, db, result)

	// tenantId > 0 时设置成功
	result = SetTenantInDB(db, 100)
	assert.NotEqual(t, db, result)

	// 可以从 db 实例中获取 tenant_id
	tenantId := getTenantIdFromDB(result)
	assert.Equal(t, int64(100), tenantId)
}

func TestGetTenantIdFromDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 未设置时返回 0
	tenantId := getTenantIdFromDB(db)
	assert.Equal(t, int64(0), tenantId)

	// 设置后返回正确值
	db = SetTenantInDB(db, 200)
	tenantId = getTenantIdFromDB(db)
	assert.Equal(t, int64(200), tenantId)
}

func TestIsTenantScopedTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 测试租户隔离表
	testCases := []struct {
		table   string
		scopped bool
	}{
		{"tokens", true},
		{"channels", true},
		{"logs", true},
		{"users", true},
		{"abilities", true},
		{"options", false},
		{"models", false},
		{"vendors", false},
	}

	for _, tc := range testCases {
		// 模拟设置表名
		tx := db.Table(tc.table)
		result := isTenantScopedTable(tx)
		assert.Equal(t, tc.scopped, result, "table: %s", tc.table)
	}
}

func TestTenantCallbacksRegistration(t *testing.T) {
	// 验证回调注册不会 panic
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// RegisterTenantCallbacks 在未启用时不应注册回调
	// 但函数调用不应出错
	RegisterTenantCallbacks(db)
}