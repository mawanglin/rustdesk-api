package api

import (
	"github.com/gin-gonic/gin"
	requstform "github.com/lejianwen/rustdesk-api/v2/http/request/api"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"net/http"
)

// Device 对应客户端 RustDesk 1.4.x 的 /api/devices/* 端点
// (设备部署与 CLI 登记，Authorization: Bearer <access_token>)。
type Device struct {
}

// Deploy 部署/接管设备：把客户端上报的 id/uuid/pk 绑定到当前登录用户，返回 {"result":"OK"}。
func (d *Device) Deploy(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	if u.Id == 0 {
		c.JSON(http.StatusOK, gin.H{"result": "Unauthorized"})
		return
	}
	f := &requstform.DeviceDeployForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusOK, gin.H{"result": "ParamsError"})
		return
	}
	if f.Id == "" || f.Uuid == "" {
		c.JSON(http.StatusOK, gin.H{"result": "ParamsError"})
		return
	}
	ps := service.AllService.PeerService
	peer := ps.FindByUuid(f.Uuid)
	if peer.RowId == 0 {
		peer = &model.Peer{Uuid: f.Uuid, Id: f.Id, UserId: u.Id}
		if err := ps.Create(peer); err != nil {
			c.JSON(http.StatusOK, gin.H{"result": "CreateFailed"})
			return
		}
	} else {
		// 已是其他用户的设备则拒绝接管；否则绑定当前用户
		if peer.UserId != 0 && peer.UserId != u.Id {
			c.JSON(http.StatusOK, gin.H{"result": "DeviceBoundToAnotherUser"})
			return
		}
		peer.UserId = u.Id
		if f.Id != "" && peer.Id == "" {
			peer.Id = f.Id
		}
		if err := ps.Update(peer); err != nil {
			c.JSON(http.StatusOK, gin.H{"result": "UpdateFailed"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"result": "OK"})
}

// Cli 设备 CLI 登记：更新设备记录属性（设备名/用户名/备注/设备分组等）。
// 成功返回空 body（客户端视为 Done），失败返回文本。
func (d *Device) Cli(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	if u.Id == 0 {
		c.JSON(http.StatusOK, gin.H{"error": "Unauthorized"})
		return
	}
	f := &requstform.DeviceCliForm{}
	if err := c.ShouldBindJSON(f); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "ParamsError: " + err.Error()})
		return
	}
	if f.Id == "" || f.Uuid == "" {
		c.JSON(http.StatusOK, gin.H{"error": "ParamsError: id and uuid required"})
		return
	}
	ps := service.AllService.PeerService
	peer := ps.FindByUuid(f.Uuid)
	if peer.RowId == 0 {
		peer = &model.Peer{Uuid: f.Uuid, Id: f.Id, UserId: u.Id}
		if err := ps.Create(peer); err != nil {
			c.JSON(http.StatusOK, gin.H{"error": "CreateFailed"})
			return
		}
		c.Status(http.StatusOK) // 空 body 成功
		return
	}
	if peer.UserId != 0 && peer.UserId != u.Id {
		c.JSON(http.StatusOK, gin.H{"error": "DeviceBoundToAnotherUser"})
		return
	}
	peer.UserId = u.Id
	if f.Id != "" {
		peer.Id = f.Id
	}
	if f.DeviceName != "" {
		peer.Hostname = f.DeviceName // peer 无独立"设备名"字段，用 Hostname 承载（客户端展示名）
	}
	if f.DeviceUsername != "" {
		peer.Username = f.DeviceUsername
	}
	if f.Note != "" && peer.Alias == "" {
		peer.Alias = f.Note
	}
	if f.DeviceGroupName != "" {
		peer.GroupId = resolveDeviceGroupId(f.DeviceGroupName)
	}
	if err := ps.Update(peer); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "UpdateFailed"})
		return
	}
	c.Status(http.StatusOK) // 空 body 成功（客户端显示 Done!）
}

// resolveDeviceGroupId 按名称查找设备分组，不存在则创建，返回其 ID。
func resolveDeviceGroupId(name string) uint {
	gs := service.AllService.GroupService
	var g model.DeviceGroup
	err := service.DB.Model(&model.DeviceGroup{}).Where("name = ?", name).Take(&g).Error
	if err == nil {
		return g.Id
	}
	ng := &model.DeviceGroup{Name: name}
	_ = gs.DeviceGroupCreate(ng)
	return ng.Id
}
