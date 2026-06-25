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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerifyExternalPaymentSignature(t *testing.T) {
	body := []byte(`{"externalOrderId":"site-order-1","productName":"pro"}`)
	timestamp := "1781712000"
	nonce := "nonce-1"
	secret := "app-secret"
	signature := SignExternalPaymentPayload(secret, timestamp, nonce, body)

	err := VerifyExternalPaymentSignature(secret, timestamp, nonce, signature, body, time.Unix(1781712000, 0))
	if err != nil {
		t.Fatalf("expected valid signature, got: %v", err)
	}

	err = VerifyExternalPaymentSignature(secret, timestamp, nonce, "bad-signature", body, time.Unix(1781712000, 0))
	if err == nil || !strings.Contains(err.Error(), "invalid signature") {
		t.Fatalf("expected invalid signature error, got: %v", err)
	}
}

func TestVerifyExternalPaymentSignatureCoversCustomAmount(t *testing.T) {
	body := []byte(`{"externalOrderId":"site-order-1","productName":"pay-template","providerName":"provider_pay","amount":88.6}`)
	timestamp := "1781712000"
	nonce := "nonce-1"
	secret := "app-secret"
	signature := SignExternalPaymentPayload(secret, timestamp, nonce, body)

	tamperedBody := []byte(`{"externalOrderId":"site-order-1","productName":"pay-template","providerName":"provider_pay","amount":8.86}`)
	err := VerifyExternalPaymentSignature(secret, timestamp, nonce, signature, tamperedBody, time.Unix(1781712000, 0))
	if err == nil || !strings.Contains(err.Error(), "invalid signature") {
		t.Fatalf("expected tampered amount to fail signature verification, got: %v", err)
	}
}

func TestVerifyExternalPaymentSignatureRejectsExpiredTimestamp(t *testing.T) {
	body := []byte(`{"externalOrderId":"site-order-1"}`)
	timestamp := "1781711000"
	nonce := "nonce-1"
	secret := "app-secret"
	signature := SignExternalPaymentPayload(secret, timestamp, nonce, body)

	err := VerifyExternalPaymentSignature(secret, timestamp, nonce, signature, body, time.Unix(1781712000, 0))
	if err == nil || !strings.Contains(err.Error(), "expired timestamp") {
		t.Fatalf("expected expired timestamp error, got: %v", err)
	}
}

func TestValidateExternalPaymentRequestRequiresEnabledTemplate(t *testing.T) {
	req := &ExternalPaymentRequest{
		ExternalOrderId: "order-1",
		UserId:          "admin/alice",
		ProductName:     "pay-template",
		ProviderName:    "provider_pay",
		Amount:          10,
		Currency:        "USD",
	}
	product := &Product{
		Owner:                     "admin",
		Name:                      "pay-template",
		Currency:                  "USD",
		Providers:                 []string{"provider_pay"},
		AllowExternalCustomAmount: false,
		ExternalMinAmount:         1,
		ExternalMaxAmount:         100,
	}

	err := ValidateExternalPaymentRequest(req, product)
	if err == nil || !strings.Contains(err.Error(), "external custom amount is not enabled") {
		t.Fatalf("expected disabled template rejection, got: %v", err)
	}
}

func TestValidateExternalPaymentRequestChecksAmountCurrencyAndProvider(t *testing.T) {
	product := &Product{
		Owner:                     "admin",
		Name:                      "pay-template",
		Currency:                  "USD",
		Providers:                 []string{"provider_pay"},
		AllowExternalCustomAmount: true,
		ExternalMinAmount:         10,
		ExternalMaxAmount:         100,
	}

	tests := []struct {
		name    string
		req     *ExternalPaymentRequest
		wantErr string
	}{
		{
			name: "zero amount",
			req: &ExternalPaymentRequest{
				ExternalOrderId: "order-1", UserId: "admin/alice", ProductName: "pay-template", ProviderName: "provider_pay", Amount: 0, Currency: "USD",
			},
			wantErr: "amount must be greater than zero",
		},
		{
			name: "below min",
			req: &ExternalPaymentRequest{
				ExternalOrderId: "order-1", UserId: "admin/alice", ProductName: "pay-template", ProviderName: "provider_pay", Amount: 9.99, Currency: "USD",
			},
			wantErr: "amount is below minimum",
		},
		{
			name: "above max",
			req: &ExternalPaymentRequest{
				ExternalOrderId: "order-1", UserId: "admin/alice", ProductName: "pay-template", ProviderName: "provider_pay", Amount: 100.01, Currency: "USD",
			},
			wantErr: "amount is above maximum",
		},
		{
			name: "currency mismatch",
			req: &ExternalPaymentRequest{
				ExternalOrderId: "order-1", UserId: "admin/alice", ProductName: "pay-template", ProviderName: "provider_pay", Amount: 50, Currency: "CNY",
			},
			wantErr: "currency mismatch",
		},
		{
			name: "invalid provider",
			req: &ExternalPaymentRequest{
				ExternalOrderId: "order-1", UserId: "admin/alice", ProductName: "pay-template", ProviderName: "provider_other", Amount: 50, Currency: "USD",
			},
			wantErr: "not valid for the product",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExternalPaymentRequest(tt.req, product)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestBuildExternalPaymentOrderUsesSignedAmountAndSkipsStock(t *testing.T) {
	product := &Product{
		Owner:                     "admin",
		Name:                      "pay-template",
		DisplayName:               "Payment Template",
		Detail:                    "Template detail",
		Currency:                  "USD",
		Providers:                 []string{"provider_pay"},
		AllowExternalCustomAmount: true,
		ExternalMinAmount:         1,
		ExternalMaxAmount:         1000,
	}
	req := &ExternalPaymentRequest{
		ExternalOrderId: "order-1",
		UserId:          "admin/alice",
		ProductName:     "pay-template",
		ProviderName:    "provider_pay",
		Amount:          88.6,
		Currency:        "USD",
		DisplayName:     "VIP package",
		Detail:          "VIP package for third-party product",
	}
	user := &User{Owner: "admin", Name: "alice"}

	order := BuildExternalPaymentOrder(product, req, user)
	if order.Price != req.Amount {
		t.Fatalf("expected order price %v, got %v", req.Amount, order.Price)
	}
	if order.Currency != "USD" {
		t.Fatalf("expected USD currency, got %s", order.Currency)
	}
	if len(order.ProductInfos) != 1 {
		t.Fatalf("expected one product info, got %d", len(order.ProductInfos))
	}
	info := order.ProductInfos[0]
	if info.Price != req.Amount || info.DisplayName != req.DisplayName || info.Detail != req.Detail {
		t.Fatalf("unexpected product info: %+v", info)
	}
	if !info.SkipStock {
		t.Fatalf("expected external custom amount order to skip stock updates")
	}
}

func TestBuildExternalPaymentWebhookPayloadUsesPaymentAmount(t *testing.T) {
	application := &Application{Name: "app"}
	externalPayment := &ExternalPayment{ExternalOrderId: "order-1"}
	payment := &Payment{Owner: "admin", Name: "payment_1", User: "alice", Price: 88.6, Currency: "USD", Provider: "provider_pay"}
	order := &Order{Owner: "admin", Name: "order_1", ProductInfos: []ProductInfo{{
		Name: "pay-template", Price: 88.6,
	}}}

	payload := BuildExternalPaymentWebhookPayload(application, externalPayment, payment, order)
	if payload.Amount != payment.Price {
		t.Fatalf("expected webhook amount %v, got %v", payment.Price, payload.Amount)
	}
	if payload.Currency != payment.Currency {
		t.Fatalf("expected webhook currency %s, got %s", payment.Currency, payload.Currency)
	}
	if payload.ProviderName != payment.Provider {
		t.Fatalf("expected webhook provider %s, got %s", payment.Provider, payload.ProviderName)
	}
}

func TestValidatePaidOrderProductsSkipsStockForExternalTemplate(t *testing.T) {
	order := &Order{
		Currency: "USD",
		ProductInfos: []ProductInfo{{
			Name:      "pay-template",
			SkipStock: true,
		}},
	}
	products := []Product{{
		Name:     "pay-template",
		Currency: "USD",
		Quantity: 0,
	}}

	err := validatePaidOrderProducts(products, order)
	if err != nil {
		t.Fatalf("expected paid external template order to skip stock validation, got: %v", err)
	}
}

func TestValidatePaidOrderProductsRejectsOutOfStockWithoutSkipStock(t *testing.T) {
	order := &Order{
		Currency: "USD",
		ProductInfos: []ProductInfo{{
			Name: "pay-template",
		}},
	}
	products := []Product{{
		Name:     "pay-template",
		Currency: "USD",
		Quantity: 0,
	}}

	err := validatePaidOrderProducts(products, order)
	if err == nil || !strings.Contains(err.Error(), "out of stock") {
		t.Fatalf("expected out of stock rejection, got: %v", err)
	}
}

func TestValidateExternalNativePaymentRequestRejectsUnsafeInputAndAmount(t *testing.T) {
	req := &ExternalNativePaymentRequest{
		ExternalOrderId: "order-1",
		UserId:          "admin/alice",
		ProductName:     "product;drop-table",
		Amount:          1,
	}

	err := ValidateExternalNativePaymentRequest(req)
	if err == nil || !strings.Contains(err.Error(), "amount is not allowed") {
		t.Fatalf("expected amount rejection before unsafe product is used, got: %v", err)
	}

	req.Amount = 0
	err = ValidateExternalNativePaymentRequest(req)
	if err == nil || !strings.Contains(err.Error(), "invalid productName") {
		t.Fatalf("expected unsafe productName rejection, got: %v", err)
	}
}

func TestBuildExternalPaymentWebhookSignature(t *testing.T) {
	payload := []byte(`{"event":"payment.paid","externalOrderId":"site-order-1"}`)
	signature := BuildExternalPaymentWebhookSignature("app-secret", payload)

	if !strings.HasPrefix(signature, "sha256=") {
		t.Fatalf("expected sha256 signature prefix, got: %s", signature)
	}
	if len(strings.TrimPrefix(signature, "sha256=")) != 64 {
		t.Fatalf("expected hex sha256 signature, got: %s", signature)
	}
}

func TestSendRawWebhookDoesNotLetConfiguredHeadersOverrideSignature(t *testing.T) {
	var gotSignature string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-Casdoor-Webhook-Signature")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	payload := `{"event":"payment.paid","externalOrderId":"site-order-1"}`
	webhook := &Webhook{
		Url:         server.URL,
		Method:      http.MethodPost,
		ContentType: "application/json",
		Headers: []*Header{{
			Name:  "X-Casdoor-Webhook-Signature",
			Value: "bad",
		}},
	}

	statusCode, _, err := sendRawWebhook(webhook, payload, "app-secret")
	if err != nil {
		t.Fatalf("expected webhook send success, got: %v", err)
	}
	if statusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got: %d", statusCode)
	}
	if gotBody != payload {
		t.Fatalf("expected body %s, got %s", payload, gotBody)
	}
	if gotSignature == "bad" || gotSignature != BuildExternalPaymentWebhookSignature("app-secret", []byte(payload)) {
		t.Fatalf("expected generated signature, got: %s", gotSignature)
	}
}
