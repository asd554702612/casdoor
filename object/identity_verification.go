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
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xorm-io/xorm"
)

const (
	IdentityVerificationStatusUnsubmitted = "unsubmitted"
	IdentityVerificationStatusPending     = "pending"
	IdentityVerificationStatusApproved    = "approved"
	IdentityVerificationStatusRejected    = "rejected"

	IdentityVerificationRuleActionApprove      = "approve"
	IdentityVerificationRuleActionReject       = "reject"
	IdentityVerificationRuleActionManualReview = "manualReview"

	IdentityVerificationProviderCheckSkipped = "skipped"
	IdentityVerificationProviderCheckPassed  = "passed"
	IdentityVerificationProviderCheckFailed  = "failed"
	IdentityVerificationProviderCheckError   = "error"

	IdentityIdCardTypeChineseIdCard = "CN_ID"

	IdentityVerificationLaunchTTL = 5 * time.Minute
)

type IdentityVerificationInfo struct {
	UserId        string `json:"userId"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	IdCardType    string `json:"idCardType"`
	MaskedIdCard  string `json:"maskedIdCard"`
	RealName      string `json:"realName"`
	IsVerified    bool   `json:"isVerified"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
	Reviewer      string `json:"reviewer"`
	ReviewedTime  string `json:"reviewedTime"`
	SubmittedTime string `json:"submittedTime"`
	AgeChecked    bool   `json:"ageChecked"`
	IsOver16      bool   `json:"isOver16"`
}

type IdentityVerificationRules struct {
	Enabled                 bool     `json:"enabled"`
	RequireIdvProvider      bool     `json:"requireIdvProvider"`
	AllowedIdCardTypes      []string `json:"allowedIdCardTypes"`
	TrustedProviderTypes    []string `json:"trustedProviderTypes"`
	Under16Action           string   `json:"under16Action"`
	ProviderFailureAction   string   `json:"providerFailureAction"`
	ProviderErrorAction     string   `json:"providerErrorAction"`
	UnsupportedIdCardAction string   `json:"unsupportedIdCardAction"`
	InvalidIdCardAction     string   `json:"invalidIdCardAction"`
	UnsupportedIdCardReason string   `json:"unsupportedIdCardReason"`
	InvalidIdCardReason     string   `json:"invalidIdCardReason"`
	Under16Reason           string   `json:"under16Reason"`
	ProviderFailureReason   string   `json:"providerFailureReason"`
	ProviderErrorReason     string   `json:"providerErrorReason"`
	ProviderRequiredReason  string   `json:"providerRequiredReason"`
	ProviderUntrustedReason string   `json:"providerUntrustedReason"`
	AutoApproveReason       string   `json:"autoApproveReason"`
	ManualReviewReason      string   `json:"manualReviewReason"`
}

type IdentityVerificationRuleResult struct {
	Status             string
	Reason             string
	ShouldCallProvider bool
}

type IdentityVerificationLaunchRequest struct {
	ClientId    string `json:"clientId"`
	UserId      string `json:"userId"`
	RedirectUri string `json:"redirectUri"`
	State       string `json:"state"`
	Timestamp   string `json:"timestamp"`
	Nonce       string `json:"nonce"`
	Signature   string `json:"signature"`
}

type IdentityVerificationLaunchInfo struct {
	ClientId    string `json:"clientId"`
	Application string `json:"application"`
	UserId      string `json:"userId"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	RedirectUri string `json:"redirectUri"`
	State       string `json:"state"`
}

type IdentityVerificationResetRequest struct {
	UserId string `json:"userId"`
	Owner  string `json:"owner"`
	Name   string `json:"name"`
}

type IdentityVerificationSubmitRequest struct {
	Owner      string `json:"owner"`
	Name       string `json:"name"`
	IdCardType string `json:"idCardType"`
	IdCard     string `json:"idCard"`
	RealName   string `json:"realName"`
}

type IdentityVerificationReviewRequest struct {
	UserId string `json:"userId"`
	Owner  string `json:"owner"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type ExternalUserSyncRequest struct {
	UserId string `json:"userId"`
}

type ExternalUserSyncResponse struct {
	UserId      string `json:"userId"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	IsVerified  bool   `json:"isVerified"`
	AgeChecked  bool   `json:"ageChecked"`
	IsOver16    bool   `json:"isOver16"`
}

func MaskIdCard(idCard string) string {
	idCard = strings.TrimSpace(idCard)
	if idCard == "" {
		return ""
	}

	runes := []rune(idCard)
	if len(runes) <= 10 {
		return strings.Repeat("*", len(runes))
	}

	return string(runes[:6]) + strings.Repeat("*", len(runes)-10) + string(runes[len(runes)-4:])
}

func ParseChineseIdCardBirthday(idCard string) (time.Time, bool) {
	idCard = strings.TrimSpace(idCard)
	runes := []rune(idCard)
	if len(runes) != 18 {
		return time.Time{}, false
	}

	for i, r := range runes {
		if i == 17 && (r == 'X' || r == 'x') {
			continue
		}
		if !unicode.IsDigit(r) {
			return time.Time{}, false
		}
	}

	birthdayText := string(runes[6:14])
	birthday, err := time.Parse("20060102", birthdayText)
	if err != nil {
		return time.Time{}, false
	}
	if birthday.Format("20060102") != birthdayText {
		return time.Time{}, false
	}

	return birthday, true
}

func NormalizeChineseIdCard(idCard string) string {
	return strings.ToUpper(strings.TrimSpace(idCard))
}

func IsValidChineseIdCardNumber(idCard string) bool {
	idCard = NormalizeChineseIdCard(idCard)
	runes := []rune(idCard)
	if len(runes) != 18 {
		return false
	}

	for i, r := range runes {
		if i == 17 && r == 'X' {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}

	if _, ok := ParseChineseIdCardBirthday(idCard); !ok {
		return false
	}

	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkCodes := "10X98765432"
	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(idCard[i]-'0') * weights[i]
	}
	return idCard[17] == checkCodes[sum%11]
}

func IsValidChineseRealName(realName string) bool {
	runes := []rune(strings.TrimSpace(realName))
	if len(runes) < 2 || len(runes) > 30 {
		return false
	}
	if runes[0] == '·' || runes[len(runes)-1] == '·' {
		return false
	}

	hanCount := 0
	lastMiddleDot := false
	for _, r := range runes {
		if r == '·' {
			if lastMiddleDot {
				return false
			}
			lastMiddleDot = true
			continue
		}
		if !unicode.Is(unicode.Han, r) {
			return false
		}
		hanCount++
		lastMiddleDot = false
	}
	return hanCount >= 2
}

func ValidateIdentityVerificationInput(idCardType string, idCard string, realName string) error {
	idCardType = strings.TrimSpace(idCardType)
	idCard = strings.TrimSpace(idCard)
	realName = strings.TrimSpace(realName)
	if idCardType == "" || idCard == "" || realName == "" {
		return fmt.Errorf("ID card information and real name are required")
	}
	if !IsValidChineseRealName(realName) {
		return fmt.Errorf("real name must be Chinese")
	}
	if idCardType == IdentityIdCardTypeChineseIdCard && !IsValidChineseIdCardNumber(idCard) {
		return fmt.Errorf("invalid Chinese ID card number")
	}
	return nil
}

func GetIdentityAgeStatus(user *User, now time.Time) (bool, bool) {
	if user == nil || !IsIdentityVerified(user) {
		return false, false
	}

	birthday, ok := ParseChineseIdCardBirthday(user.IdCard)
	if !ok {
		return false, false
	}

	return true, !now.Before(birthday.AddDate(16, 0, 0))
}

func IsIdentityVerified(user *User) bool {
	if user == nil {
		return false
	}
	return NormalizeIdentityVerificationStatus(user) == IdentityVerificationStatusApproved && user.IsVerified
}

func NormalizeIdentityVerificationStatus(user *User) string {
	if user == nil {
		return IdentityVerificationStatusUnsubmitted
	}

	switch user.IdentityVerificationStatus {
	case IdentityVerificationStatusPending, IdentityVerificationStatusApproved, IdentityVerificationStatusRejected, IdentityVerificationStatusUnsubmitted:
		return user.IdentityVerificationStatus
	case "":
		if user.IsVerified {
			return IdentityVerificationStatusApproved
		}
		return IdentityVerificationStatusUnsubmitted
	default:
		return IdentityVerificationStatusUnsubmitted
	}
}

func GetIdentityVerificationInfo(user *User, now time.Time) *IdentityVerificationInfo {
	if user == nil {
		return nil
	}

	ageChecked, isOver16 := GetIdentityAgeStatus(user, now)
	status := NormalizeIdentityVerificationStatus(user)
	return &IdentityVerificationInfo{
		UserId:        user.Id,
		Owner:         user.Owner,
		Name:          user.Name,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		Phone:         user.Phone,
		IdCardType:    user.IdCardType,
		MaskedIdCard:  MaskIdCard(user.IdCard),
		RealName:      user.RealName,
		IsVerified:    IsIdentityVerified(user),
		Status:        status,
		Reason:        user.IdentityVerificationReason,
		Reviewer:      user.IdentityVerificationReviewer,
		ReviewedTime:  user.IdentityVerificationReviewedTime,
		SubmittedTime: user.IdentityVerificationSubmittedTime,
		AgeChecked:    ageChecked,
		IsOver16:      isOver16,
	}
}

func GetExternalUserSyncResponse(user *User, now time.Time) *ExternalUserSyncResponse {
	if user == nil {
		return nil
	}

	ageChecked, isOver16 := GetIdentityAgeStatus(user, now)
	return &ExternalUserSyncResponse{
		UserId:      user.Id,
		Owner:       user.Owner,
		Name:        user.Name,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Phone:       user.Phone,
		IsVerified:  IsIdentityVerified(user),
		AgeChecked:  ageChecked,
		IsOver16:    isOver16,
	}
}

func GetDefaultIdentityVerificationRules() *IdentityVerificationRules {
	return &IdentityVerificationRules{
		Enabled:                 true,
		RequireIdvProvider:      false,
		AllowedIdCardTypes:      []string{IdentityIdCardTypeChineseIdCard},
		TrustedProviderTypes:    []string{"Alibaba Cloud"},
		Under16Action:           IdentityVerificationRuleActionManualReview,
		ProviderFailureAction:   IdentityVerificationRuleActionReject,
		ProviderErrorAction:     IdentityVerificationRuleActionManualReview,
		UnsupportedIdCardAction: IdentityVerificationRuleActionReject,
		InvalidIdCardAction:     IdentityVerificationRuleActionReject,
		UnsupportedIdCardReason: "Unsupported identity document type",
		InvalidIdCardReason:     "Invalid identity document number",
		Under16Reason:           "User is under 16 years old",
		ProviderFailureReason:   "Identity verification provider rejected the identity data",
		ProviderErrorReason:     "Identity verification provider is unavailable",
		ProviderRequiredReason:  "No ID Verification provider is configured",
		ProviderUntrustedReason: "ID Verification provider is not trusted for automatic approval",
		AutoApproveReason:       "Automatically approved by identity verification rules",
		ManualReviewReason:      "Manual identity verification review is required",
	}
}

func NormalizeIdentityVerificationRules(rules *IdentityVerificationRules) *IdentityVerificationRules {
	defaultRules := GetDefaultIdentityVerificationRules()
	if rules == nil {
		return defaultRules
	}

	res := *rules
	res.AllowedIdCardTypes = normalizeIdentityStringList(res.AllowedIdCardTypes, defaultRules.AllowedIdCardTypes)
	res.TrustedProviderTypes = normalizeIdentityStringList(res.TrustedProviderTypes, defaultRules.TrustedProviderTypes)
	res.Under16Action = normalizeIdentityUnsafeRuleAction(res.Under16Action, defaultRules.Under16Action)
	res.ProviderFailureAction = normalizeIdentityUnsafeRuleAction(res.ProviderFailureAction, defaultRules.ProviderFailureAction)
	res.ProviderErrorAction = normalizeIdentityUnsafeRuleAction(res.ProviderErrorAction, defaultRules.ProviderErrorAction)
	res.UnsupportedIdCardAction = normalizeIdentityUnsafeRuleAction(res.UnsupportedIdCardAction, defaultRules.UnsupportedIdCardAction)
	res.InvalidIdCardAction = normalizeIdentityUnsafeRuleAction(res.InvalidIdCardAction, defaultRules.InvalidIdCardAction)
	res.UnsupportedIdCardReason = firstNonEmptyIdentityRuleValue(res.UnsupportedIdCardReason, defaultRules.UnsupportedIdCardReason)
	res.InvalidIdCardReason = firstNonEmptyIdentityRuleValue(res.InvalidIdCardReason, defaultRules.InvalidIdCardReason)
	res.Under16Reason = firstNonEmptyIdentityRuleValue(res.Under16Reason, defaultRules.Under16Reason)
	res.ProviderFailureReason = firstNonEmptyIdentityRuleValue(res.ProviderFailureReason, defaultRules.ProviderFailureReason)
	res.ProviderErrorReason = firstNonEmptyIdentityRuleValue(res.ProviderErrorReason, defaultRules.ProviderErrorReason)
	res.ProviderRequiredReason = firstNonEmptyIdentityRuleValue(res.ProviderRequiredReason, defaultRules.ProviderRequiredReason)
	res.ProviderUntrustedReason = firstNonEmptyIdentityRuleValue(res.ProviderUntrustedReason, defaultRules.ProviderUntrustedReason)
	res.AutoApproveReason = firstNonEmptyIdentityRuleValue(res.AutoApproveReason, defaultRules.AutoApproveReason)
	res.ManualReviewReason = firstNonEmptyIdentityRuleValue(res.ManualReviewReason, defaultRules.ManualReviewReason)
	return &res
}

func EvaluateIdentityVerificationRules(user *User, rules *IdentityVerificationRules, providerCheck string, providerErr error, now time.Time) *IdentityVerificationRuleResult {
	rules = NormalizeIdentityVerificationRules(rules)
	if !rules.Enabled {
		return &IdentityVerificationRuleResult{
			Status:             IdentityVerificationStatusPending,
			Reason:             "",
			ShouldCallProvider: false,
		}
	}
	if user == nil {
		return newIdentityVerificationRuleResult(IdentityVerificationStatusRejected, "User is empty", false)
	}
	if strings.TrimSpace(user.RealName) == "" || strings.TrimSpace(user.IdCardType) == "" || strings.TrimSpace(user.IdCard) == "" {
		return newIdentityVerificationRuleResult(IdentityVerificationStatusPending, rules.ManualReviewReason, false)
	}
	if !IsValidChineseRealName(user.RealName) {
		return newIdentityVerificationRuleResult(IdentityVerificationStatusRejected, "Invalid real name", false)
	}

	if !identityStringInSlice(user.IdCardType, rules.AllowedIdCardTypes) {
		return identityRuleActionResult(rules.UnsupportedIdCardAction, rules.UnsupportedIdCardReason, false)
	}

	if user.IdCardType == IdentityIdCardTypeChineseIdCard {
		if !IsValidChineseIdCardNumber(user.IdCard) {
			return identityRuleActionResult(rules.InvalidIdCardAction, rules.InvalidIdCardReason, false)
		}
		birthday, ok := ParseChineseIdCardBirthday(user.IdCard)
		if !ok {
			return identityRuleActionResult(rules.InvalidIdCardAction, rules.InvalidIdCardReason, false)
		}
		if now.IsZero() {
			now = time.Now()
		}
		if now.Before(birthday.AddDate(16, 0, 0)) {
			return identityRuleActionResult(rules.Under16Action, rules.Under16Reason, false)
		}
	}

	switch providerCheck {
	case IdentityVerificationProviderCheckPassed:
		return newIdentityVerificationRuleResult(IdentityVerificationStatusApproved, rules.AutoApproveReason, false)
	case IdentityVerificationProviderCheckFailed:
		return identityRuleActionResult(rules.ProviderFailureAction, rules.ProviderFailureReason, false)
	case IdentityVerificationProviderCheckError:
		reason := rules.ProviderErrorReason
		if providerErr != nil {
			reason = fmt.Sprintf("%s: %s", rules.ProviderErrorReason, providerErr.Error())
		}
		return identityRuleActionResult(rules.ProviderErrorAction, reason, false)
	default:
		if rules.RequireIdvProvider {
			return newIdentityVerificationRuleResult(IdentityVerificationStatusPending, rules.ProviderRequiredReason, true)
		}
		return newIdentityVerificationRuleResult(IdentityVerificationStatusApproved, rules.AutoApproveReason, false)
	}
}

func IsTrustedIdentityVerificationProvider(rules *IdentityVerificationRules, provider *Provider) bool {
	if provider == nil || provider.Category != "ID Verification" {
		return false
	}
	rules = NormalizeIdentityVerificationRules(rules)
	return identityStringInSlice(provider.Type, rules.TrustedProviderTypes)
}

func SignIdentityVerificationLaunch(secret string, timestamp string, nonce string, clientId string, userId string, redirectUri string, state string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(nonce))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(clientId))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(userId))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(redirectUri))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(state))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyIdentityVerificationLaunchSignature(secret string, req *IdentityVerificationLaunchRequest, now time.Time) error {
	if secret == "" {
		return fmt.Errorf("application clientSecret cannot be empty")
	}
	if req == nil {
		return fmt.Errorf("request cannot be empty")
	}
	if req.Timestamp == "" || req.Nonce == "" || req.Signature == "" {
		return fmt.Errorf("missing signature parameters")
	}

	ts, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp")
	}
	requestTime := time.Unix(ts, 0)
	if now.Sub(requestTime) > IdentityVerificationLaunchTTL || requestTime.Sub(now) > IdentityVerificationLaunchTTL {
		return fmt.Errorf("expired timestamp")
	}

	expected := SignIdentityVerificationLaunch(secret, req.Timestamp, req.Nonce, req.ClientId, req.UserId, req.RedirectUri, req.State)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(req.Signature)) != 1 {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func ValidateIdentityVerificationLaunch(application *Application, user *User, req *IdentityVerificationLaunchRequest, now time.Time) (*IdentityVerificationLaunchInfo, error) {
	if application == nil {
		return nil, fmt.Errorf("invalid application")
	}
	if user == nil {
		return nil, fmt.Errorf("user does not belong to application organization")
	}
	if req == nil || strings.TrimSpace(req.ClientId) == "" || strings.TrimSpace(req.UserId) == "" || strings.TrimSpace(req.RedirectUri) == "" {
		return nil, fmt.Errorf("missing launch parameters")
	}
	if req.ClientId != application.ClientId {
		return nil, fmt.Errorf("clientId does not match application")
	}
	if req.UserId != user.Id {
		return nil, fmt.Errorf("userId does not match user")
	}
	if user.Owner != application.Organization {
		return nil, fmt.Errorf("user does not belong to application organization")
	}
	if !application.IsRedirectUriValid(req.RedirectUri) {
		return nil, fmt.Errorf("invalid redirectUri")
	}
	if err := VerifyIdentityVerificationLaunchSignature(application.ClientSecret, req, now); err != nil {
		return nil, err
	}

	return &IdentityVerificationLaunchInfo{
		ClientId:    req.ClientId,
		Application: application.Name,
		UserId:      user.Id,
		Owner:       user.Owner,
		Name:        user.Name,
		RedirectUri: req.RedirectUri,
		State:       req.State,
	}, nil
}

func applyIdentityVerificationFilter(isVerified string, addFilter func(bool)) error {
	if isVerified == "" {
		return nil
	}

	verified, err := strconv.ParseBool(isVerified)
	if err != nil {
		return fmt.Errorf("invalid isVerified filter")
	}
	addFilter(verified)
	return nil
}

func applyIdentityVerificationStatusSessionFilter(session *xorm.Session, status string) error {
	if status == "" {
		return nil
	}

	switch status {
	case IdentityVerificationStatusApproved:
		session.And("(identity_verification_status = ? OR ((identity_verification_status = '' OR identity_verification_status IS NULL) AND is_verified = ?))", status, true)
	case IdentityVerificationStatusUnsubmitted:
		session.And("(identity_verification_status = ? OR ((identity_verification_status = '' OR identity_verification_status IS NULL) AND is_verified = ?))", status, false)
	case IdentityVerificationStatusPending, IdentityVerificationStatusRejected:
		session.And("identity_verification_status = ?", status)
	default:
		return fmt.Errorf("invalid identity verification status filter")
	}
	return nil
}

func GetIdentityVerificationCount(owner string, field string, value string, isVerified string, status string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	err := applyIdentityVerificationFilter(isVerified, func(verified bool) {
		session.And("is_verified = ?", verified)
	})
	if err != nil {
		return 0, err
	}
	err = applyIdentityVerificationStatusSessionFilter(session, status)
	if err != nil {
		return 0, err
	}

	return session.Count(&User{})
}

func GetPaginationIdentityVerifications(owner string, offset int, limit int, field string, value string, sortField string, sortOrder string, isVerified string, status string, now time.Time) ([]*IdentityVerificationInfo, error) {
	users := []*User{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := applyIdentityVerificationFilter(isVerified, func(verified bool) {
		session.And("is_verified = ?", verified)
	})
	if err != nil {
		return nil, err
	}
	err = applyIdentityVerificationStatusSessionFilter(session, status)
	if err != nil {
		return nil, err
	}

	err = session.Find(&users)
	if err != nil {
		return nil, err
	}

	infos := make([]*IdentityVerificationInfo, 0, len(users))
	for _, user := range users {
		infos = append(infos, GetIdentityVerificationInfo(user, now))
	}
	return infos, nil
}

func ResetIdentityVerification(user *User) []string {
	if user == nil {
		return nil
	}

	user.IsVerified = false
	user.IdentityVerificationStatus = IdentityVerificationStatusUnsubmitted
	user.IdentityVerificationReason = ""
	user.IdentityVerificationReviewer = ""
	user.IdentityVerificationReviewedTime = ""
	user.IdentityVerificationSubmittedTime = ""
	return identityVerificationColumns()
}

func ApplyIdentityDataChange(oldUser *User, newUser *User, now time.Time) []string {
	if oldUser == nil || newUser == nil {
		return nil
	}
	if oldUser.IdCard == newUser.IdCard && oldUser.IdCardType == newUser.IdCardType && oldUser.RealName == newUser.RealName {
		return nil
	}

	newUser.IsVerified = false
	newUser.IdentityVerificationStatus = IdentityVerificationStatusPending
	newUser.IdentityVerificationReason = ""
	newUser.IdentityVerificationReviewer = ""
	newUser.IdentityVerificationReviewedTime = ""
	newUser.IdentityVerificationSubmittedTime = utilTimeToString(now)
	return identityVerificationColumns()
}

func SubmitIdentityVerification(user *User, now time.Time) []string {
	if user == nil {
		return nil
	}

	user.IsVerified = false
	user.IdentityVerificationStatus = IdentityVerificationStatusPending
	user.IdentityVerificationReason = ""
	user.IdentityVerificationReviewer = ""
	user.IdentityVerificationReviewedTime = ""
	user.IdentityVerificationSubmittedTime = utilTimeToString(now)
	return identityVerificationColumns()
}

func ReviewIdentityVerification(user *User, status string, reviewer string, reason string, now time.Time) ([]string, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}

	reviewer = strings.TrimSpace(reviewer)
	reason = strings.TrimSpace(reason)
	switch status {
	case IdentityVerificationStatusApproved:
		user.IsVerified = true
		user.IdentityVerificationStatus = IdentityVerificationStatusApproved
		user.IdentityVerificationReason = reason
	case IdentityVerificationStatusRejected:
		if reason == "" {
			return nil, fmt.Errorf("rejection reason is required")
		}
		user.IsVerified = false
		user.IdentityVerificationStatus = IdentityVerificationStatusRejected
		user.IdentityVerificationReason = reason
	default:
		return nil, fmt.Errorf("unsupported identity verification review status: %s", status)
	}

	user.IdentityVerificationReviewer = reviewer
	user.IdentityVerificationReviewedTime = utilTimeToString(now)
	return []string{
		"is_verified",
		"identity_verification_status",
		"identity_verification_reason",
		"identity_verification_reviewer",
		"identity_verification_reviewed_time",
	}, nil
}

func ApplyIdentityVerificationRuleResult(user *User, result *IdentityVerificationRuleResult, reviewer string, now time.Time) ([]string, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if result == nil {
		return nil, fmt.Errorf("identity verification rule result is nil")
	}

	switch result.Status {
	case IdentityVerificationStatusApproved, IdentityVerificationStatusRejected:
		return ReviewIdentityVerification(user, result.Status, reviewer, result.Reason, now)
	case IdentityVerificationStatusPending:
		user.IsVerified = false
		user.IdentityVerificationStatus = IdentityVerificationStatusPending
		user.IdentityVerificationReason = strings.TrimSpace(result.Reason)
		return []string{
			"is_verified",
			"identity_verification_status",
			"identity_verification_reason",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported identity verification rule result status: %s", result.Status)
	}
}

func identityVerificationColumns() []string {
	return []string{
		"is_verified",
		"identity_verification_status",
		"identity_verification_reason",
		"identity_verification_reviewer",
		"identity_verification_reviewed_time",
		"identity_verification_submitted_time",
	}
}

func identityRuleActionResult(action string, reason string, shouldCallProvider bool) *IdentityVerificationRuleResult {
	switch action {
	case IdentityVerificationRuleActionApprove:
		return newIdentityVerificationRuleResult(IdentityVerificationStatusApproved, reason, shouldCallProvider)
	case IdentityVerificationRuleActionReject:
		return newIdentityVerificationRuleResult(IdentityVerificationStatusRejected, reason, shouldCallProvider)
	default:
		return newIdentityVerificationRuleResult(IdentityVerificationStatusPending, reason, shouldCallProvider)
	}
}

func newIdentityVerificationRuleResult(status string, reason string, shouldCallProvider bool) *IdentityVerificationRuleResult {
	reason = strings.TrimSpace(reason)
	if status == IdentityVerificationStatusRejected && reason == "" {
		reason = "Identity verification rejected by rule"
	}
	if status == IdentityVerificationStatusPending && reason == "" {
		reason = "Manual identity verification review is required"
	}
	return &IdentityVerificationRuleResult{
		Status:             status,
		Reason:             reason,
		ShouldCallProvider: shouldCallProvider,
	}
}

func normalizeIdentityStringList(values []string, defaultValues []string) []string {
	res := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		res = append(res, value)
		seen[value] = true
	}
	if len(res) == 0 {
		return append([]string{}, defaultValues...)
	}
	return res
}

func normalizeIdentityRuleAction(action string, defaultAction string) string {
	action = strings.TrimSpace(action)
	switch action {
	case IdentityVerificationRuleActionApprove, IdentityVerificationRuleActionReject, IdentityVerificationRuleActionManualReview:
		return action
	default:
		return defaultAction
	}
}

func normalizeIdentityUnsafeRuleAction(action string, defaultAction string) string {
	action = normalizeIdentityRuleAction(action, defaultAction)
	if action == IdentityVerificationRuleActionApprove {
		return defaultAction
	}
	return action
}

func identityStringInSlice(value string, values []string) bool {
	value = strings.TrimSpace(value)
	for _, item := range values {
		if strings.TrimSpace(item) == value {
			return true
		}
	}
	return false
}

func firstNonEmptyIdentityRuleValue(value string, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return defaultValue
}

func utilTimeToString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
