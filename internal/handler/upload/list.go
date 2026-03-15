package upload

import (
	"sort"

	"github.com/gin-gonic/gin"
)

// ListImages 按日期返回图片列表，支持 ?date=YYYY-MM-DD 过滤。
// @Summary      图片列表
// @Description  按日期分组返回图片列表，可通过 date=YYYY-MM-DD 过滤
// @Tags         images
// @Produce      json
// @Param        date  query  string  false  "日期(YYYY-MM-DD)"
// @Success      200  {object}  APIResponse
// @Failure      500  {object}  APIError
// @Router       /images [get]
func (h *Handler) ListImages(c *gin.Context) {
	date := c.Query("date")

	// storage 层已经按日期分组，这里只做轻量的转换与拼装 URL
	groups, err := h.storage.ListByDate(date)
	if err != nil {
		respServerError(c, err.Error())
		return
	}

	host := c.Request.Host
	var result []ImageGroup
	for day, files := range groups {
		group := ImageGroup{Date: day}
		for _, f := range files {
			// 始终使用 http 协议构造访问地址，便于与上传返回保持一致
			url := "http://" + host + "/files/" + f.Path
			group.Files = append(group.Files, ImageFile{
				Name: f.Name,
				Path: f.Path,
				URL:  url,
				Size: f.Size,
			})
		}
		result = append(result, group)
	}

	// 简单按日期倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date > result[j].Date
	})

	respOK(c, result)
}

