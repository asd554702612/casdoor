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

package object

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/casdoor/casdoor/proxy"
)

const (
	iHuyiSmsType              = "IHuyi SMS"
	defaultIHuyiSmsEndpoint   = "https://api.ihuyi.com/sms/Submit.json"
	defaultIHuyiSmsTemplateId = "309190"
)

type ihuyiSmsClient struct {
	account     string
	password    string
	templateId  string
	endpoint    string
	enableProxy bool
}

type ihuyiSmsResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	SmsId string `json:"smsid"`
}

func newIHuyiSmsClient(account string, password string, templateId string, endpoint string, enableProxy bool) (*ihuyiSmsClient, error) {
	account = strings.TrimSpace(account)
	password = strings.TrimSpace(password)
	if account == "" || password == "" {
		return nil, fmt.Errorf("IHuyi SMS requires clientId and clientSecret")
	}

	templateId = strings.TrimSpace(templateId)
	if templateId == "" {
		templateId = defaultIHuyiSmsTemplateId
	}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultIHuyiSmsEndpoint
	}

	return &ihuyiSmsClient{
		account:     account,
		password:    password,
		templateId:  templateId,
		endpoint:    endpoint,
		enableProxy: enableProxy,
	}, nil
}

func (c *ihuyiSmsClient) SendMessage(param map[string]string, targetPhoneNumber ...string) error {
	code := strings.TrimSpace(param["code"])
	if code == "" {
		return fmt.Errorf("IHuyi SMS requires parameter: code")
	}
	if len(targetPhoneNumber) == 0 {
		return fmt.Errorf("IHuyi SMS requires target phone number")
	}

	httpClient := getIHuyiHttpClient(c.enableProxy)

	for _, phoneNumber := range targetPhoneNumber {
		form := url.Values{}
		form.Set("account", c.account)
		form.Set("password", c.password)
		form.Set("mobile", normalizeIHuyiMobile(phoneNumber))
		form.Set("templateid", c.templateId)
		form.Set("content", code)

		req, err := http.NewRequest(http.MethodPost, c.endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}

		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("IHuyi SMS request failed with status: %s", resp.Status)
		}

		var payload ihuyiSmsResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("IHuyi SMS invalid response: %w", err)
		}
		if payload.Code != 2 {
			return fmt.Errorf("IHuyi SMS request failed: code=%d, msg=%s", payload.Code, payload.Msg)
		}
	}

	return nil
}

func normalizeIHuyiMobile(phoneNumber string) string {
	normalized := strings.TrimSpace(phoneNumber)
	var digits strings.Builder
	for _, r := range normalized {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}

	result := digits.String()
	if strings.HasPrefix(result, "86") && len(result) > 11 {
		return result[2:]
	}
	return result
}

func getIHuyiHttpClient(enableProxy bool) *http.Client {
	if enableProxy && proxy.ProxyHttpClient != nil {
		return proxy.ProxyHttpClient
	}
	if proxy.DefaultHttpClient != nil {
		return proxy.DefaultHttpClient
	}
	return http.DefaultClient
}
