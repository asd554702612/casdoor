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
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/casdoor/casdoor/pp"
	"github.com/casdoor/casdoor/util"
	"github.com/xorm-io/core"
)

const (
	ExternalPaymentEventPaid = "payment.paid"

	externalPaymentSignatureTTL = 5 * time.Minute
)

var (
	externalPaymentNameRegexp    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	externalPaymentOrderIdRegexp = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,100}$`)
)

type ExternalPayment struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	Application     string `xorm:"varchar(100) index" json:"application"`
	ExternalOrderId string `xorm:"varchar(100) index" json:"externalOrderId"`
	PaymentOwner    string `xorm:"varchar(100) index" json:"paymentOwner"`
	User            string `xorm:"varchar(100)" json:"user"`
	Order           string `xorm:"varchar(100)" json:"order"`
	Payment         string `xorm:"varchar(100) index" json:"payment"`
	State           string `xorm:"varchar(100)" json:"state"`
	Message         string `xorm:"varchar(2000)" json:"message"`
}

type ExternalNativePaymentRequest struct {
	ExternalOrderId string `json:"externalOrderId"`
	UserId          string `json:"userId"`
	Owner           string `json:"owner"`
	UserName        string `json:"userName"`
	ProductName     string `json:"productName"`
	PricingName     string `json:"pricingName"`
	PlanName        string `json:"planName"`
	Quantity        int    `json:"quantity"`
	CouponCode      string `json:"couponCode"`

	// Amount is intentionally rejected. Prices must come from Casdoor products.
	Amount float64 `json:"amount"`
}

type ExternalNativePaymentResponse struct {
	OrderId         string          `json:"orderId"`
	PaymentId       string          `json:"paymentId"`
	ExternalOrderId string          `json:"externalOrderId"`
	PayUrl          string          `json:"payUrl"`
	State           pp.PaymentState `json:"state"`
}

type ExternalPaymentRequest struct {
	ExternalOrderId string  `json:"externalOrderId"`
	UserId          string  `json:"userId"`
	Owner           string  `json:"owner"`
	UserName        string  `json:"userName"`
	ProductName     string  `json:"productName"`
	ProviderName    string  `json:"providerName"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	DisplayName     string  `json:"displayName"`
	Detail          string  `json:"detail"`
	CouponCode      string  `json:"couponCode"`
}

type ExternalPaymentResponse struct {
	OrderId         string                 `json:"orderId"`
	PaymentId       string                 `json:"paymentId"`
	ExternalOrderId string                 `json:"externalOrderId"`
	PayUrl          string                 `json:"payUrl"`
	State           pp.PaymentState        `json:"state"`
	Amount          float64                `json:"amount"`
	Currency        string                 `json:"currency"`
	ProviderName    string                 `json:"providerName"`
	AttachInfo      map[string]interface{} `json:"attachInfo,omitempty"`
}

type ExternalPaymentWebhookPayload struct {
	Event           string        `json:"event"`
	Application     string        `json:"application"`
	ExternalOrderId string        `json:"externalOrderId"`
	OrderId         string        `json:"orderId"`
	PaymentId       string        `json:"paymentId"`
	UserId          string        `json:"userId"`
	Products        []ProductInfo `json:"products"`
	Amount          float64       `json:"amount"`
	Currency        string        `json:"currency"`
	ProviderName    string        `json:"providerName"`
	PaidTime        string        `json:"paidTime"`
}

func SignExternalPaymentPayload(secret string, timestamp string, nonce string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(nonce))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyExternalPaymentSignature(secret string, timestamp string, nonce string, signature string, body []byte, now time.Time) error {
	if secret == "" {
		return fmt.Errorf("application clientSecret cannot be empty")
	}
	if timestamp == "" || nonce == "" || signature == "" {
		return fmt.Errorf("missing signature headers")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	requestTime := time.Unix(ts, 0)
	if now.Sub(requestTime) > externalPaymentSignatureTTL || requestTime.Sub(now) > externalPaymentSignatureTTL {
		return fmt.Errorf("expired timestamp")
	}

	expected := SignExternalPaymentPayload(secret, timestamp, nonce, body)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) != 1 {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func BuildExternalPaymentWebhookSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}

func ValidateExternalNativePaymentRequest(req *ExternalNativePaymentRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be empty")
	}
	if req.Amount != 0 {
		return fmt.Errorf("amount is not allowed")
	}
	if !externalPaymentOrderIdRegexp.MatchString(req.ExternalOrderId) {
		return fmt.Errorf("invalid externalOrderId")
	}
	if req.UserId == "" {
		if !externalPaymentNameRegexp.MatchString(req.Owner) {
			return fmt.Errorf("invalid owner")
		}
		if !externalPaymentNameRegexp.MatchString(req.UserName) {
			return fmt.Errorf("invalid userName")
		}
	} else {
		owner, name, err := util.GetOwnerAndNameFromIdWithError(req.UserId)
		if err != nil {
			return fmt.Errorf("invalid userId")
		}
		if !externalPaymentNameRegexp.MatchString(owner) || !externalPaymentNameRegexp.MatchString(name) {
			return fmt.Errorf("invalid userId")
		}
	}
	if !externalPaymentNameRegexp.MatchString(req.ProductName) {
		return fmt.Errorf("invalid productName")
	}
	if req.PricingName != "" && !externalPaymentNameRegexp.MatchString(req.PricingName) {
		return fmt.Errorf("invalid pricingName")
	}
	if req.PlanName != "" && !externalPaymentNameRegexp.MatchString(req.PlanName) {
		return fmt.Errorf("invalid planName")
	}
	if req.Quantity < 0 {
		return fmt.Errorf("invalid quantity")
	}
	return nil
}

func ValidateExternalPaymentRequest(req *ExternalPaymentRequest, product *Product) error {
	if req == nil {
		return fmt.Errorf("request cannot be empty")
	}
	if product == nil {
		return fmt.Errorf("product cannot be empty")
	}
	if !product.AllowExternalCustomAmount {
		return fmt.Errorf("external custom amount is not enabled for product: %s", product.Name)
	}
	if req.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if product.ExternalMinAmount > 0 && req.Amount < product.ExternalMinAmount {
		return fmt.Errorf("amount is below minimum: %v", product.ExternalMinAmount)
	}
	if product.ExternalMaxAmount > 0 && req.Amount > product.ExternalMaxAmount {
		return fmt.Errorf("amount is above maximum: %v", product.ExternalMaxAmount)
	}
	if !externalPaymentOrderIdRegexp.MatchString(req.ExternalOrderId) {
		return fmt.Errorf("invalid externalOrderId")
	}
	if req.UserId == "" {
		if !externalPaymentNameRegexp.MatchString(req.Owner) {
			return fmt.Errorf("invalid owner")
		}
		if !externalPaymentNameRegexp.MatchString(req.UserName) {
			return fmt.Errorf("invalid userName")
		}
	} else {
		owner, name, err := util.GetOwnerAndNameFromIdWithError(req.UserId)
		if err != nil {
			return fmt.Errorf("invalid userId")
		}
		if !externalPaymentNameRegexp.MatchString(owner) || !externalPaymentNameRegexp.MatchString(name) {
			return fmt.Errorf("invalid userId")
		}
	}
	if !externalPaymentNameRegexp.MatchString(req.ProductName) {
		return fmt.Errorf("invalid productName")
	}
	if !externalPaymentNameRegexp.MatchString(req.ProviderName) {
		return fmt.Errorf("invalid providerName")
	}
	productCurrency := getProductCurrency(product)
	if req.Currency != "" && req.Currency != productCurrency {
		return fmt.Errorf("currency mismatch: expected %s, got %s", productCurrency, req.Currency)
	}
	if err := validateExternalPaymentProvider(product, req.ProviderName); err != nil {
		return err
	}
	return nil
}

func getProductCurrency(product *Product) string {
	if product == nil || product.Currency == "" {
		return "USD"
	}
	return product.Currency
}

func validateExternalPaymentProvider(product *Product, providerName string) error {
	for _, name := range product.Providers {
		if name == providerName {
			return nil
		}
	}
	return fmt.Errorf("the payment provider: %s is not valid for the product: %s", providerName, product.Name)
}

func getExternalPaymentName(application *Application, externalOrderId string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%s\n%s", application.Owner, application.Name, externalOrderId)))
	return fmt.Sprintf("external_payment_%s", hex.EncodeToString(sum[:])[:32])
}

func getExternalPayment(owner string, name string) (*ExternalPayment, error) {
	if owner == "" || name == "" {
		return nil, nil
	}

	externalPayment := ExternalPayment{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&externalPayment)
	if err != nil {
		return nil, err
	}
	if existed {
		return &externalPayment, nil
	}
	return nil, nil
}

func GetExternalPaymentByPayment(owner string, paymentName string) (*ExternalPayment, error) {
	externalPayment := ExternalPayment{}
	existed, err := ormer.Engine.Where("payment_owner = ? AND payment = ?", owner, paymentName).Get(&externalPayment)
	if err != nil {
		return nil, err
	}
	if existed {
		return &externalPayment, nil
	}
	return nil, nil
}

func AddExternalPayment(externalPayment *ExternalPayment) (bool, error) {
	affected, err := ormer.Engine.Insert(externalPayment)
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func UpdateExternalPayment(externalPayment *ExternalPayment) (bool, error) {
	externalPayment.UpdatedTime = util.GetCurrentTime()
	affected, err := ormer.Engine.ID(core.PK{externalPayment.Owner, externalPayment.Name}).AllCols().Update(externalPayment)
	if err != nil {
		return false, err
	}
	return affected != 0, nil
}

func getExternalPaymentUser(req *ExternalNativePaymentRequest) (*User, error) {
	return getExternalPaymentUserByFields(req.UserId, req.Owner, req.UserName)
}

func getExternalCustomPaymentUser(req *ExternalPaymentRequest) (*User, error) {
	return getExternalPaymentUserByFields(req.UserId, req.Owner, req.UserName)
}

func getExternalPaymentUserByFields(userId string, owner string, userName string) (*User, error) {
	if userId == "" {
		userId = util.GetId(owner, userName)
	}

	user, err := GetUser(userId)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("the user: %s does not exist", userId)
	}
	return user, nil
}

func getExternalPaymentProduct(application *Application, productName string) (*Product, error) {
	product, err := getProduct(application.Organization, productName)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, fmt.Errorf("the product: %s/%s does not exist", application.Organization, productName)
	}
	if product.State != "" && product.State != "Published" {
		return nil, fmt.Errorf("the product: %s is not published", product.Name)
	}
	if product.IsRecharge {
		return nil, fmt.Errorf("recharge product is not supported by external native payment")
	}
	return product, nil
}

func getExternalPaymentWechatProvider(product *Product) (string, error) {
	for _, providerName := range product.Providers {
		provider, err := getProvider(product.Owner, providerName)
		if err != nil {
			return "", err
		}
		if provider != nil && provider.Category == "Payment" && provider.Type == "WeChat Pay" {
			return provider.Name, nil
		}
	}
	return "", fmt.Errorf("no WeChat Pay provider available for product: %s", product.Name)
}

func buildExternalNativePaymentResponse(externalPayment *ExternalPayment) (*ExternalNativePaymentResponse, error) {
	if externalPayment.State == "Failed" {
		if externalPayment.Message != "" {
			return nil, errors.New(externalPayment.Message)
		}
		return nil, fmt.Errorf("external payment failed")
	}
	if externalPayment.Order == "" || externalPayment.Payment == "" {
		return nil, fmt.Errorf("external payment is still being created")
	}

	paymentOwner := externalPayment.PaymentOwner
	if paymentOwner == "" {
		paymentOwner = externalPayment.Owner
	}

	payment, err := getPayment(paymentOwner, externalPayment.Payment)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, fmt.Errorf("the payment: %s does not exist", externalPayment.Payment)
	}

	return &ExternalNativePaymentResponse{
		OrderId:         util.GetId(paymentOwner, externalPayment.Order),
		PaymentId:       util.GetId(paymentOwner, externalPayment.Payment),
		ExternalOrderId: externalPayment.ExternalOrderId,
		PayUrl:          payment.PayUrl,
		State:           payment.State,
	}, nil
}

func buildExternalPaymentResponse(externalPayment *ExternalPayment, attachInfo map[string]interface{}) (*ExternalPaymentResponse, error) {
	if externalPayment.State == "Failed" {
		if externalPayment.Message != "" {
			return nil, errors.New(externalPayment.Message)
		}
		return nil, fmt.Errorf("external payment failed")
	}
	if externalPayment.Order == "" || externalPayment.Payment == "" {
		return nil, fmt.Errorf("external payment is still being created")
	}

	paymentOwner := externalPayment.PaymentOwner
	if paymentOwner == "" {
		paymentOwner = externalPayment.Owner
	}

	payment, err := getPayment(paymentOwner, externalPayment.Payment)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, fmt.Errorf("the payment: %s does not exist", externalPayment.Payment)
	}

	return &ExternalPaymentResponse{
		OrderId:         util.GetId(paymentOwner, externalPayment.Order),
		PaymentId:       util.GetId(paymentOwner, externalPayment.Payment),
		ExternalOrderId: externalPayment.ExternalOrderId,
		PayUrl:          payment.PayUrl,
		State:           payment.State,
		Amount:          payment.Price,
		Currency:        payment.Currency,
		ProviderName:    payment.Provider,
		AttachInfo:      attachInfo,
	}, nil
}

func BuildExternalPaymentOrder(product *Product, req *ExternalPaymentRequest, user *User) *Order {
	currency := getProductCurrency(product)
	displayName := req.DisplayName
	if displayName == "" {
		displayName = product.DisplayName
	}
	detail := req.Detail
	if detail == "" {
		detail = product.Detail
	}

	orderName := fmt.Sprintf("order_%v", util.GenerateTimeId())
	productInfo := ProductInfo{
		Owner:       product.Owner,
		Name:        product.Name,
		DisplayName: displayName,
		Image:       product.Image,
		Detail:      detail,
		Price:       req.Amount,
		Currency:    currency,
		IsRecharge:  false,
		Quantity:    1,
		SkipStock:   true,
	}
	return &Order{
		Owner:        product.Owner,
		Name:         orderName,
		DisplayName:  orderName,
		CreatedTime:  util.GetCurrentTime(),
		Products:     []string{product.Name},
		ProductInfos: []ProductInfo{productInfo},
		User:         user.Name,
		Payment:      "",
		Price:        req.Amount,
		Currency:     currency,
		State:        "Created",
		Message:      "",
		UpdateTime:   "",
	}
}

func PlaceExternalPaymentOrder(product *Product, req *ExternalPaymentRequest, user *User) (*Order, error) {
	order := BuildExternalPaymentOrder(product, req, user)
	affected, err := AddOrder(order)
	if err != nil {
		return nil, err
	}
	if !affected {
		return nil, fmt.Errorf("failed to add order: %s", util.StructToJson(order))
	}
	return order, nil
}

func CreateExternalNativePayment(application *Application, req *ExternalNativePaymentRequest, host string, lang string) (*ExternalNativePaymentResponse, error) {
	if application == nil {
		return nil, fmt.Errorf("application cannot be empty")
	}
	if application.Organization == "" {
		return nil, fmt.Errorf("application organization cannot be empty")
	}
	if err := ValidateExternalNativePaymentRequest(req); err != nil {
		return nil, err
	}

	externalPaymentName := getExternalPaymentName(application, req.ExternalOrderId)
	existing, err := getExternalPayment(application.Owner, externalPaymentName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return buildExternalNativePaymentResponse(existing)
	}

	now := util.GetCurrentTime()
	externalPayment := &ExternalPayment{
		Owner:           application.Owner,
		Name:            externalPaymentName,
		CreatedTime:     now,
		UpdatedTime:     now,
		Application:     application.Name,
		ExternalOrderId: req.ExternalOrderId,
		PaymentOwner:    application.Organization,
		State:           "Creating",
	}
	affected, err := AddExternalPayment(externalPayment)
	if err != nil {
		existing, getErr := getExternalPayment(application.Owner, externalPaymentName)
		if getErr == nil && existing != nil {
			return buildExternalNativePaymentResponse(existing)
		}
		return nil, err
	}
	if !affected {
		return nil, fmt.Errorf("failed to add external payment: %s", externalPayment.Name)
	}

	user, err := getExternalPaymentUser(req)
	if err != nil {
		externalPayment.State = "Failed"
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}
	if user.Owner != application.Organization {
		err = fmt.Errorf("user: %s does not belong to application organization: %s", user.GetId(), application.Organization)
		externalPayment.State = "Failed"
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}

	product, err := getExternalPaymentProduct(application, req.ProductName)
	if err != nil {
		externalPayment.State = "Failed"
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}

	providerName, err := getExternalPaymentWechatProvider(product)
	if err != nil {
		externalPayment.State = "Failed"
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}

	quantity := req.Quantity
	if quantity == 0 {
		quantity = 1
	}
	order, err := PlaceOrder(product.Owner, []ProductInfo{{
		Name:        product.Name,
		Quantity:    quantity,
		PricingName: req.PricingName,
		PlanName:    req.PlanName,
	}}, user, req.CouponCode)
	if err != nil {
		externalPayment.State = "Failed"
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}

	payment, _, err := PayOrder(providerName, host, "", order, lang)
	if err != nil {
		externalPayment.State = "Failed"
		externalPayment.Order = order.Name
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}

	externalPayment.User = user.Name
	externalPayment.PaymentOwner = payment.Owner
	externalPayment.Order = order.Name
	externalPayment.Payment = payment.Name
	externalPayment.State = string(payment.State)
	externalPayment.Message = ""
	_, err = UpdateExternalPayment(externalPayment)
	if err != nil {
		return nil, err
	}

	return buildExternalNativePaymentResponse(externalPayment)
}

func CreateExternalPayment(application *Application, req *ExternalPaymentRequest, host string, lang string) (*ExternalPaymentResponse, error) {
	if application == nil {
		return nil, fmt.Errorf("application cannot be empty")
	}
	if application.Organization == "" {
		return nil, fmt.Errorf("application organization cannot be empty")
	}
	if req == nil {
		return nil, fmt.Errorf("request cannot be empty")
	}
	if !externalPaymentOrderIdRegexp.MatchString(req.ExternalOrderId) {
		return nil, fmt.Errorf("invalid externalOrderId")
	}

	externalPaymentName := getExternalPaymentName(application, req.ExternalOrderId)
	existing, err := getExternalPayment(application.Owner, externalPaymentName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return buildExternalPaymentResponse(existing, nil)
	}

	user, err := getExternalCustomPaymentUser(req)
	if err != nil {
		return nil, err
	}
	if user.Owner != application.Organization {
		return nil, fmt.Errorf("user: %s does not belong to application organization: %s", user.GetId(), application.Organization)
	}

	product, err := getExternalPaymentProduct(application, req.ProductName)
	if err != nil {
		return nil, err
	}
	if err = ValidateExternalPaymentRequest(req, product); err != nil {
		return nil, err
	}

	now := util.GetCurrentTime()
	externalPayment := &ExternalPayment{
		Owner:           application.Owner,
		Name:            externalPaymentName,
		CreatedTime:     now,
		UpdatedTime:     now,
		Application:     application.Name,
		ExternalOrderId: req.ExternalOrderId,
		PaymentOwner:    application.Organization,
		State:           "Creating",
	}
	affected, err := AddExternalPayment(externalPayment)
	if err != nil {
		existing, getErr := getExternalPayment(application.Owner, externalPaymentName)
		if getErr == nil && existing != nil {
			return buildExternalPaymentResponse(existing, nil)
		}
		return nil, err
	}
	if !affected {
		return nil, fmt.Errorf("failed to add external payment: %s", externalPayment.Name)
	}

	order, err := PlaceExternalPaymentOrder(product, req, user)
	if err != nil {
		externalPayment.State = "Failed"
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}

	payment, attachInfo, err := PayOrder(req.ProviderName, host, "", order, lang)
	if err != nil {
		externalPayment.State = "Failed"
		externalPayment.Order = order.Name
		externalPayment.Message = err.Error()
		_, _ = UpdateExternalPayment(externalPayment)
		return nil, err
	}

	externalPayment.User = user.Name
	externalPayment.PaymentOwner = payment.Owner
	externalPayment.Order = order.Name
	externalPayment.Payment = payment.Name
	externalPayment.State = string(payment.State)
	externalPayment.Message = ""
	_, err = UpdateExternalPayment(externalPayment)
	if err != nil {
		return nil, err
	}

	return buildExternalPaymentResponse(externalPayment, attachInfo)
}

func BuildExternalPaymentWebhookPayload(application *Application, externalPayment *ExternalPayment, payment *Payment, order *Order) *ExternalPaymentWebhookPayload {
	return &ExternalPaymentWebhookPayload{
		Event:           ExternalPaymentEventPaid,
		Application:     application.Name,
		ExternalOrderId: externalPayment.ExternalOrderId,
		OrderId:         order.GetId(),
		PaymentId:       payment.GetId(),
		UserId:          util.GetId(payment.Owner, payment.User),
		Products:        order.ProductInfos,
		Amount:          payment.Price,
		Currency:        payment.Currency,
		ProviderName:    payment.Provider,
		PaidTime:        util.GetCurrentTime(),
	}
}

func CreateExternalPaymentPaidWebhookEvents(payment *Payment, order *Order) error {
	if payment == nil || order == nil {
		return nil
	}

	externalPayment, err := GetExternalPaymentByPayment(payment.Owner, payment.Name)
	if err != nil {
		return err
	}
	if externalPayment == nil {
		return nil
	}

	externalPayment.State = string(payment.State)
	_, err = UpdateExternalPayment(externalPayment)
	if err != nil {
		return err
	}

	application, err := getApplication(externalPayment.Owner, externalPayment.Application)
	if err != nil {
		return err
	}
	if application == nil {
		return fmt.Errorf("the application: %s/%s does not exist", externalPayment.Owner, externalPayment.Application)
	}

	webhooks, err := GetPaymentWebhooks(application)
	if err != nil {
		return err
	}
	if len(webhooks) == 0 {
		return nil
	}

	payload := BuildExternalPaymentWebhookPayload(application, externalPayment, payment, order)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	for _, webhook := range webhooks {
		event := &WebhookEvent{
			Owner:        webhook.Owner,
			Name:         util.GenerateId(),
			CreatedTime:  util.GetCurrentTime(),
			UpdatedTime:  util.GetCurrentTime(),
			Webhook:      webhook.GetId(),
			Organization: application.GetId(),
			EventType:    ExternalPaymentEventPaid,
			State:        WebhookEventStatusPending,
			Payload:      string(payloadBytes),
			AttemptCount: 0,
			MaxRetries:   webhook.MaxRetries,
		}
		if event.MaxRetries <= 0 {
			event.MaxRetries = 3
		}
		_, err = AddWebhookEvent(event)
		if err != nil {
			return err
		}
	}
	return nil
}

func IsExternalPaymentEvent(eventType string) bool {
	return strings.HasPrefix(eventType, "payment.")
}
