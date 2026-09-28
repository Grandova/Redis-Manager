package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"redis-manager/internal/system"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

func (h *SystemHandler) GetInfo(c *gin.Context) {
	info, err := system.GetHostInfo()
	if err != nil {
		ServerError(c, "获取主机系统信息失败", err)
		return
	}
	Success(c, info)
}

func (h *SystemHandler) CheckPort(c *gin.Context) {
	portStr := c.DefaultQuery("port", "80")
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		BadRequest(c, "无效的端口号")
		return
	}

	result, err := system.CheckPort(port)
	if err != nil {
		ServerError(c, "检查端口失败", err)
		return
	}

	Success(c, result)
}

func (h *SystemHandler) CheckDNS(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		BadRequest(c, "域名参数不能为空")
		return
	}

	result, err := system.CheckDomainDNS(domain)
	if err != nil {
		ServerError(c, "DNS 查询出错", err)
		return
	}

	Success(c, result)
}
