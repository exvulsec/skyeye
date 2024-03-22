package controller

import (
	"fmt"
	"github.com/exvulsec/skyeye/model"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type SignatureController struct {
}

func (sc *SignatureController) Routers(routers gin.IRouter) {
	routers.GET("/signatures/:bytesign", sc.GetSignatureByByteSign)
}

func (sc *SignatureController) GetSignatureByByteSign(c *gin.Context) {
	byteSign := strings.ToLower(c.Param("bytesign"))
	textSigns, err := model.GetSignatures([]string{byteSign})
	if err != nil {
		c.JSON(http.StatusOK, model.Message{
			Code: http.StatusInternalServerError,
			Msg:  fmt.Sprintf("get bytesign %s's text signature is err %v", byteSign, err),
		})
		return
	}
	c.JSON(http.StatusOK, model.Message{
		Code: http.StatusOK,
		Data: textSigns,
	})
}
