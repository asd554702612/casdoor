// Copyright 2026 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/casdoor/casdoor/object"
)

func (c *ApiController) getSignedExternalPaymentApplication() (*object.Application, []byte, bool) {
	clientId := c.Ctx.Request.Header.Get("X-Casdoor-App-Id")
	timestamp := c.Ctx.Request.Header.Get("X-Casdoor-Timestamp")
	nonce := c.Ctx.Request.Header.Get("X-Casdoor-Nonce")
	signature := c.Ctx.Request.Header.Get("X-Casdoor-Signature")
	body := c.Ctx.Input.RequestBody

	application, err := object.GetApplicationByClientId(clientId)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.ResponseError(err.Error())
		return nil, nil, false
	}
	if application == nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.ResponseError("invalid application")
		return nil, nil, false
	}

	err = object.VerifyExternalPaymentSignature(application.ClientSecret, timestamp, nonce, signature, body, time.Now())
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusUnauthorized)
		c.ResponseError(err.Error())
		return nil, nil, false
	}

	return application, body, true
}

// CreateExternalNativePayment
// @Title CreateExternalNativePayment
// @Tag Payment API
// @Description create a WeChat Native payment for a trusted sub-site
// @Param   body    body   object.ExternalNativePaymentRequest  true  "The external payment request"
// @Success 200 {object} object.ExternalNativePaymentResponse The Response object
// @router /external/payment/create-native [post]
func (c *ApiController) CreateExternalNativePayment() {
	application, body, ok := c.getSignedExternalPaymentApplication()
	if !ok {
		return
	}

	var req object.ExternalNativePaymentRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	resp, err := object.CreateExternalNativePayment(application, &req, c.Ctx.Request.Host, c.GetAcceptLanguage())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(resp)
}

// CreateExternalPayment
// @Title CreateExternalPayment
// @Tag Payment API
// @Description create a custom amount payment for a trusted sub-site
// @Param   body    body   object.ExternalPaymentRequest  true  "The external payment request"
// @Success 200 {object} object.ExternalPaymentResponse The Response object
// @router /external/payment/create [post]
func (c *ApiController) CreateExternalPayment() {
	application, body, ok := c.getSignedExternalPaymentApplication()
	if !ok {
		return
	}

	var req object.ExternalPaymentRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	resp, err := object.CreateExternalPayment(application, &req, c.Ctx.Request.Host, c.GetAcceptLanguage())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(resp)
}
