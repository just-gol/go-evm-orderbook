package models

import (
	"math"

	"github.com/gin-gonic/gin"

	"net/http"
)

type Response struct {
	Code int         `json:"code"`           // 状态码：200 成功，500 错误
	Msg  string      `json:"msg"`            // 提示信息
	Data interface{} `json:"data,omitempty"` // 响应数据，可为空时省略
}

type PageResponse struct {
	Code      int         `json:"code"`    // 业务码
	Message   string      `json:"message"` // 消息
	Data      interface{} `json:"data"`    // 列表数据
	PageNum   int         `json:"pageNum"`
	PageSize  int         `json:"pageSize"`
	Total     int64       `json:"total"`
	TotalPage int         `json:"totalPage"`
}

type PageInfo struct {
	Page      int   `json:"page"`      // 当前页码
	PageSize  int   `json:"pageSize"`  // 每页数量
	Total     int64 `json:"total"`     // 总记录数
	TotalPage int   `json:"totalPage"` // 总页数
}

// Success 成功响应（可带 data，可不带）
func Success(ctx *gin.Context, data ...interface{}) {
	resp := Response{
		Code: 200,
		Msg:  "success",
	}
	if len(data) > 0 {
		resp.Data = data[0]
	}
	ctx.JSON(http.StatusOK, resp)
}

func PageSuccess(ctx *gin.Context, msg string, data interface{}, pageNum, pageSize int, total int64) {
	ctx.JSON(http.StatusOK, PageResponse{
		Code:      0,
		Message:   msg,
		Data:      data,
		PageNum:   pageNum,
		PageSize:  pageSize,
		Total:     total,
		TotalPage: int(math.Ceil(float64(total) / float64(pageSize))),
	})
}

// Error 错误响应
func Error(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusInternalServerError, Response{
		Code: 500,
		Msg:  msg,
	})
}
