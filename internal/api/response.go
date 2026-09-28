package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func SuccessMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: msg,
		Data:    data,
	})
}

func Error(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: msg,
	})
}

func BadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    400,
		Message: msg,
	})
}

func ServerError(c *gin.Context, msg string, err error) {
	errMsg := msg
	if err != nil {
		errMsg = fmt.Sprintf("%s: %v", msg, err)
	}
	c.JSON(http.StatusInternalServerError, Response{
		Code:    500,
		Message: errMsg,
	})
}
