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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIHuyiSmsClientSendMessagePostsNormalizedMobile(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", contentType)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		form = r.PostForm
		_ = json.NewEncoder(w).Encode(ihuyiSmsResponse{Code: 2})
	}))
	defer server.Close()

	provider := &Provider{
		Type:         "IHuyi SMS",
		ClientId:     "account",
		ClientSecret: "password",
		TemplateCode: "309190",
		Endpoint:     server.URL,
	}

	if err := SendSms(provider, "123456", "+8613800138000"); err != nil {
		t.Fatalf("SendSms() error = %v", err)
	}

	if got := form.Get("account"); got != "account" {
		t.Errorf("account = %q, want account", got)
	}
	if got := form.Get("password"); got != "password" {
		t.Errorf("password = %q, want password", got)
	}
	if got := form.Get("mobile"); got != "13800138000" {
		t.Errorf("mobile = %q, want 13800138000", got)
	}
	if got := form.Get("templateid"); got != "309190" {
		t.Errorf("templateid = %q, want 309190", got)
	}
	if got := form.Get("content"); got != "123456" {
		t.Errorf("content = %q, want 123456", got)
	}
}

func TestIHuyiSmsClientReturnsBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ihuyiSmsResponse{Code: 4050, Msg: "mobile error"})
	}))
	defer server.Close()

	provider := &Provider{
		Type:         "IHuyi SMS",
		ClientId:     "account",
		ClientSecret: "password",
		TemplateCode: "309190",
		Endpoint:     server.URL,
	}

	err := SendSms(provider, "123456", "+8613800138000")
	if err == nil {
		t.Fatal("SendSms() error = nil, want business error")
	}
	if !strings.Contains(err.Error(), "code=4050") {
		t.Fatalf("SendSms() error = %q, want code=4050", err.Error())
	}
}

func TestIHuyiSmsClientRejectsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	provider := &Provider{
		Type:         "IHuyi SMS",
		ClientId:     "account",
		ClientSecret: "password",
		TemplateCode: "309190",
		Endpoint:     server.URL,
	}

	err := SendSms(provider, "123456", "+8613800138000")
	if err == nil {
		t.Fatal("SendSms() error = nil, want invalid JSON error")
	}
}

func TestIHuyiSmsClientRequiresCredentials(t *testing.T) {
	provider := &Provider{
		Type:         "IHuyi SMS",
		TemplateCode: "309190",
	}

	err := SendSms(provider, "123456", "+8613800138000")
	if err == nil {
		t.Fatal("SendSms() error = nil, want credential error")
	}
	if !strings.Contains(err.Error(), "clientId") {
		t.Fatalf("SendSms() error = %q, want clientId hint", err.Error())
	}
}
