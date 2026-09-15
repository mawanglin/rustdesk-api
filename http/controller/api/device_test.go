package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/config"
	"github.com/lejianwen/rustdesk-api/v2/global"
	"github.com/lejianwen/rustdesk-api/v2/http/middleware"
	"github.com/lejianwen/rustdesk-api/v2/lib/jwt"
	"github.com/lejianwen/rustdesk-api/v2/lib/lock"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	logrus "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// setupDeviceTest 初始化一个内存 sqlite 的最小 API 环境，返回带两个设备端点的 router 与测试用 access token。
func setupDeviceTest(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// 每个测试独立文件库，避免全局 service.DB / 共享内存库互相污染
	dbPath := t.TempDir() + "/test.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserToken{}, &model.Peer{}, &model.DeviceGroup{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 初始化 service 全局（Jwt.Key 留空 → RustAuth 走 InfoByAccessToken，任意 user token 生效）
	cfg := &config.Config{}
	svc := service.New(cfg, db, logrus.New(), jwt.NewJwt("", 0), lock.NewLocal())
	_ = svc
	global.Jwt = jwt.NewJwt("", 0) // rustauth.go 用 global.Jwt.Key 判断是否走 JWT 校验；空 key 跳过

	admin2 := service.AllService.UserService.InfoByUsername("admin")
	if admin2.Id == 0 {
		admin := &model.User{Username: "admin", Password: "x", Status: model.COMMON_STATUS_ENABLE}
		if err := service.AllService.UserService.Create(admin); err != nil {
			t.Fatalf("create user: %v", err)
		}
		var isAdmin bool
		isAdmin = false
		service.DB.Model(admin).Update("is_admin", isAdmin)
	}
	// 重新读以获得 id
	admin2 = service.AllService.UserService.InfoByUsername("admin")

	ut := &model.UserToken{UserId: admin2.Id, Token: "test-token-abc", DeviceUuid: "u1", DeviceId: "dev1", ExpiredAt: time.Now().Unix() + 3600}
	if err := service.DB.Create(ut).Error; err != nil {
		t.Fatalf("create usertoken: %v", err)
	}

	global.Cache = nil // /devices 端点不用 cache；避免依赖

	r := gin.New()
	frg := r.Group("/api")
	frg.Use(middleware.RustAuth())
	d := &Device{}
	frg.POST("/devices/deploy", d.Deploy)
	frg.POST("/devices/cli", d.Cli)
	return r, "test-token-abc"
}

func TestDeviceDeploy_RequiresAuth(t *testing.T) {
	r, _ := setupDeviceTest(t)
	// 无 token → 401
	req := httptest.NewRequest("POST", "/api/devices/deploy",
		bytes.NewBufferString(`{"id":"abc12345","uuid":"uu-1","pk":"pk1"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("deploy 无认证应 401，got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDeviceDeploy_Success(t *testing.T) {
	r, token := setupDeviceTest(t)
	req := httptest.NewRequest("POST", "/api/devices/deploy",
		bytes.NewBufferString(`{"id":"abc12345","uuid":"uu-1","pk":"pk1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("deploy 应 200，got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应非 JSON: %v body=%s", err, w.Body.String())
	}
	if resp["result"] != "OK" {
		t.Fatalf("deploy 期望 result=OK，got %v", resp)
	}
	// peer 已创建且绑定当前用户
	var peer model.Peer
	if err := service.DB.Where("uuid = ?", "uu-1").First(&peer).Error; err != nil {
		t.Fatalf("peer 未创建: %v", err)
	}
	if peer.Id != "abc12345" {
		t.Fatalf("peer.id 期望 abc12345，got %q", peer.Id)
	}
	fmt.Printf("peer 绑定成功 user_id=%d\n", peer.UserId)
}

func TestDeviceCli_Success(t *testing.T) {
	r, token := setupDeviceTest(t)
	// 先 deploy 建 peer，再 cli 更新属性
	req := httptest.NewRequest("POST", "/api/devices/deploy",
		bytes.NewBufferString(`{"id":"cli-001","uuid":"uu-cli","pk":"pk"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("deploy(pre-step) 应 200，got %d", w.Code)
	}

	req = httptest.NewRequest("POST", "/api/devices/cli",
		bytes.NewBufferString(`{"id":"cli-001","uuid":"uu-cli","device_name":"MyMac","device_username":"alice","device_group_name":"办公"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("cli 应 200，got %d body=%s", w.Code, w.Body.String())
	}
	// 成功应为空 body（客户端显示 Done）
	if w.Body.Len() != 0 {
		t.Fatalf("cli 成功应空 body，got %q", w.Body.String())
	}
	var peer model.Peer
	if err := service.DB.Where("uuid = ?", "uu-cli").First(&peer).Error; err != nil {
		t.Fatalf("peer 未找到: %v", err)
	}
	if peer.Hostname != "MyMac" {
		t.Fatalf("Hostname 期望 MyMac，got %q", peer.Hostname)
	}
	if peer.Username != "alice" {
		t.Fatalf("Username 期望 alice，got %q", peer.Username)
	}
	if peer.GroupId == 0 {
		t.Fatal("device_group_name 应解析出 GroupId")
	}
	fmt.Printf("cli 登记成功 group_id=%d\n", peer.GroupId)
}
