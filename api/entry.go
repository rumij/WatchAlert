package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/zeromicro/go-zero/core/logc"
	"watchAlert/pkg/response"
)

func Service(ctx *gin.Context, fu func() (interface{}, interface{})) {
	// Abort stops Gin's handler chain, but not the calling handler's function.
	// Never execute business operations after a binding failure.
	if ctx.IsAborted() {
		return
	}
	data, err := fu()
	if err != nil {
		logc.Error(context.Background(), err)
		response.Fail(ctx, err.(error).Error(), "failed")
		ctx.Abort()
		return
	} else {
		response.Success(ctx, data, "success")
	}
}

func BindJson(ctx *gin.Context, req interface{}) {
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		response.Fail(ctx, err.Error(), "failed")
		ctx.Abort()
		return
	}
}

func BindQuery(ctx *gin.Context, req interface{}) {
	err := ctx.ShouldBindQuery(req)
	if err != nil {
		response.Fail(ctx, err.Error(), "failed")
		ctx.Abort()
		return
	}
}
