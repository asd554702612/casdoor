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
	"strings"
	"testing"
	"time"
)

func TestMaskIdCard(t *testing.T) {
	tests := []struct {
		name   string
		idCard string
		want   string
	}{
		{name: "18 digit Chinese ID", idCard: "110105201001012345", want: "110105********2345"},
		{name: "ID ending with X", idCard: "11010519991212345X", want: "110105********345X"},
		{name: "short document", idCard: "1234567", want: "*******"},
		{name: "empty document", idCard: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskIdCard(tt.idCard); got != tt.want {
				t.Fatalf("MaskIdCard(%q) = %q, want %q", tt.idCard, got, tt.want)
			}
		})
	}
}

func TestParseChineseIdCardBirthday(t *testing.T) {
	birthday, ok := ParseChineseIdCardBirthday("110105201001012345")
	if !ok {
		t.Fatal("expected valid Chinese ID birthday")
	}
	if got := birthday.Format("2006-01-02"); got != "2010-01-01" {
		t.Fatalf("birthday = %s, want 2010-01-01", got)
	}

	if _, ok := ParseChineseIdCardBirthday("110105201013012345"); ok {
		t.Fatal("expected invalid month to be rejected")
	}

	if _, ok := ParseChineseIdCardBirthday("123456789012345"); ok {
		t.Fatal("expected non-18-digit ID to be rejected")
	}
}

func TestValidateIdentityVerificationInput(t *testing.T) {
	tests := []struct {
		name      string
		idCard    string
		realName  string
		wantError bool
	}{
		{name: "valid Chinese ID and Chinese name", idCard: "11010519491231002X", realName: "张三", wantError: false},
		{name: "valid lowercase x is accepted", idCard: "11010519491231002x", realName: "张三", wantError: false},
		{name: "invalid checksum is rejected", idCard: "110105194912310021", realName: "张三", wantError: true},
		{name: "invalid birthday is rejected", idCard: "110105199902310029", realName: "张三", wantError: true},
		{name: "short ID is rejected", idCard: "51681199608170916", realName: "张三", wantError: true},
		{name: "English real name is rejected", idCard: "11010519491231002X", realName: "dc", wantError: true},
		{name: "mixed real name is rejected", idCard: "11010519491231002X", realName: "张三A", wantError: true},
		{name: "middle dot Chinese name is accepted", idCard: "11010519491231002X", realName: "阿依古丽·买买提", wantError: false},
		{name: "leading middle dot is rejected", idCard: "11010519491231002X", realName: "·张三", wantError: true},
		{name: "single Chinese character is rejected", idCard: "11010519491231002X", realName: "张", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIdentityVerificationInput(IdentityIdCardTypeChineseIdCard, tt.idCard, tt.realName)
			if (err != nil) != tt.wantError {
				t.Fatalf("ValidateIdentityVerificationInput() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGetIdentityAgeStatus(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		user           *User
		wantAgeChecked bool
		wantIsOver16   bool
	}{
		{name: "verified user exactly 16 today", user: &User{IsVerified: true, IdCard: "110105201007062345"}, wantAgeChecked: true, wantIsOver16: true},
		{name: "verified user turns 16 tomorrow", user: &User{IsVerified: true, IdCard: "110105201007072345"}, wantAgeChecked: true, wantIsOver16: false},
		{name: "unverified user is not age checked", user: &User{IsVerified: false, IdCard: "110105200001012345"}, wantAgeChecked: false, wantIsOver16: false},
		{name: "invalid ID card is not age checked", user: &User{IsVerified: true, IdCard: "bad-id-card"}, wantAgeChecked: false, wantIsOver16: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ageChecked, isOver16 := GetIdentityAgeStatus(tt.user, now)
			if ageChecked != tt.wantAgeChecked || isOver16 != tt.wantIsOver16 {
				t.Fatalf("GetIdentityAgeStatus() = (%v, %v), want (%v, %v)", ageChecked, isOver16, tt.wantAgeChecked, tt.wantIsOver16)
			}
		})
	}
}

func TestGetIdentityVerificationInfoMasksIdCardAndKeepsSelfIdentityData(t *testing.T) {
	user := &User{
		Id:                               "u1",
		Owner:                            "built-in",
		Name:                             "alice",
		DisplayName:                      "Alice",
		RealName:                         "Alice Real",
		IdCardType:                       "CN_ID",
		IdCard:                           "11010519991212345X",
		IsVerified:                       true,
		IdentityVerificationStatus:       IdentityVerificationStatusApproved,
		IdentityVerificationReason:       "manual review passed",
		IdentityVerificationReviewer:     "built-in/admin",
		IdentityVerificationReviewedTime: "2026-07-06 12:00:00",
	}

	got := GetIdentityVerificationInfo(user, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if got.UserId != "u1" || got.Owner != "built-in" || got.Name != "alice" {
		t.Fatalf("unexpected identity user ids: %+v", got)
	}
	if got.MaskedIdCard != "110105********345X" {
		t.Fatalf("expected masked id card, got %s", got.MaskedIdCard)
	}
	if got.Status != IdentityVerificationStatusApproved || got.Reason != "manual review passed" || got.Reviewer != "built-in/admin" || got.ReviewedTime != "2026-07-06 12:00:00" {
		t.Fatalf("unexpected review metadata: %+v", got)
	}
	if !got.AgeChecked || !got.IsOver16 {
		t.Fatalf("expected user to be at least 16")
	}
}

func TestGetExternalUserSyncResponseOmitsSensitiveIdentityData(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	user := &User{
		Id:          "user-id-001",
		Owner:       "gepin",
		Name:        "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Phone:       "13800000000",
		RealName:    "Alice Real",
		IdCard:      "110105201007062345",
		IsVerified:  true,
	}

	resp := GetExternalUserSyncResponse(user, now)
	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if !resp.IsVerified || !resp.AgeChecked || !resp.IsOver16 {
		t.Fatalf("unexpected verification flags: %+v", resp)
	}

	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	bodyText := string(body)
	for _, sensitiveValue := range []string{user.IdCard, user.RealName, "idCard", "realName"} {
		if strings.Contains(bodyText, sensitiveValue) {
			t.Fatalf("response leaked sensitive value %q: %s", sensitiveValue, bodyText)
		}
	}
}

func TestResetIdentityVerificationOnlyResetsVerifiedFlag(t *testing.T) {
	user := &User{
		RealName:                          "Alice Real",
		IdCardType:                        "CN_ID",
		IdCard:                            "11010519991212345X",
		Birthday:                          "1999-12-12",
		IsVerified:                        true,
		DisplayName:                       "Alice",
		IdentityVerificationStatus:        IdentityVerificationStatusApproved,
		IdentityVerificationReason:        "passed",
		IdentityVerificationReviewer:      "built-in/admin",
		IdentityVerificationReviewedTime:  "2026-07-06 12:00:00",
		IdentityVerificationSubmittedTime: "2026-07-06 11:00:00",
	}

	columns := ResetIdentityVerification(user)
	if user.IsVerified {
		t.Fatal("expected verification flag to be reset")
	}
	if user.IdentityVerificationStatus != IdentityVerificationStatusUnsubmitted {
		t.Fatalf("expected status to reset to unsubmitted, got %s", user.IdentityVerificationStatus)
	}
	if user.IdentityVerificationReason != "" || user.IdentityVerificationReviewer != "" || user.IdentityVerificationReviewedTime != "" || user.IdentityVerificationSubmittedTime != "" {
		t.Fatalf("expected review metadata to be cleared, got: %+v", user)
	}
	if user.RealName != "Alice Real" || user.IdCardType != "CN_ID" || user.IdCard != "11010519991212345X" || user.Birthday != "1999-12-12" {
		t.Fatalf("reset should preserve identity data, got: %+v", user)
	}
	wantColumns := []string{
		"is_verified",
		"identity_verification_status",
		"identity_verification_reason",
		"identity_verification_reviewer",
		"identity_verification_reviewed_time",
		"identity_verification_submitted_time",
	}
	if strings.Join(columns, ",") != strings.Join(wantColumns, ",") {
		t.Fatalf("expected reset columns %v, got %v", wantColumns, columns)
	}
}

func TestSubmitIdentityVerificationSetsPendingAndClearsReview(t *testing.T) {
	user := &User{
		RealName:                         "Alice Real",
		IdCardType:                       "CN_ID",
		IdCard:                           "11010519991212345X",
		IsVerified:                       true,
		IdentityVerificationStatus:       IdentityVerificationStatusApproved,
		IdentityVerificationReason:       "old approval",
		IdentityVerificationReviewer:     "built-in/admin",
		IdentityVerificationReviewedTime: "2026-07-06 12:00:00",
	}

	columns := SubmitIdentityVerification(user, time.Date(2026, 7, 6, 13, 0, 0, 0, time.UTC))
	if user.IsVerified {
		t.Fatal("submitting changed identity data should clear verified flag")
	}
	if user.IdentityVerificationStatus != IdentityVerificationStatusPending {
		t.Fatalf("expected pending status, got %s", user.IdentityVerificationStatus)
	}
	if user.IdentityVerificationSubmittedTime != "2026-07-06 13:00:00" {
		t.Fatalf("unexpected submitted time: %s", user.IdentityVerificationSubmittedTime)
	}
	if user.IdentityVerificationReason != "" || user.IdentityVerificationReviewer != "" || user.IdentityVerificationReviewedTime != "" {
		t.Fatalf("expected previous review data to be cleared, got: %+v", user)
	}

	wantColumns := []string{
		"is_verified",
		"identity_verification_status",
		"identity_verification_reason",
		"identity_verification_reviewer",
		"identity_verification_reviewed_time",
		"identity_verification_submitted_time",
	}
	if strings.Join(columns, ",") != strings.Join(wantColumns, ",") {
		t.Fatalf("expected submit columns %v, got %v", wantColumns, columns)
	}
}

func TestApplyIdentityDataChangeInvalidatesApprovedUser(t *testing.T) {
	oldUser := &User{
		RealName:                          "Alice Real",
		IdCardType:                        "CN_ID",
		IdCard:                            "11010519991212345X",
		IsVerified:                        true,
		IdentityVerificationStatus:        IdentityVerificationStatusApproved,
		IdentityVerificationReason:        "passed",
		IdentityVerificationReviewer:      "built-in/admin",
		IdentityVerificationReviewedTime:  "2026-07-06 12:00:00",
		IdentityVerificationSubmittedTime: "2026-07-06 11:00:00",
	}
	newUser := *oldUser
	newUser.IdCard = "110105201007062345"

	columns := ApplyIdentityDataChange(oldUser, &newUser, time.Date(2026, 7, 6, 15, 0, 0, 0, time.UTC))
	if newUser.IsVerified {
		t.Fatal("identity data changes should clear verified flag")
	}
	if newUser.IdentityVerificationStatus != IdentityVerificationStatusPending {
		t.Fatalf("expected pending after identity data change, got %s", newUser.IdentityVerificationStatus)
	}
	if newUser.IdentityVerificationReason != "" || newUser.IdentityVerificationReviewer != "" || newUser.IdentityVerificationReviewedTime != "" {
		t.Fatalf("expected review metadata to be cleared, got: %+v", newUser)
	}
	if newUser.IdentityVerificationSubmittedTime != "2026-07-06 15:00:00" {
		t.Fatalf("unexpected submitted time: %s", newUser.IdentityVerificationSubmittedTime)
	}
	if !strings.Contains(strings.Join(columns, ","), "identity_verification_status") {
		t.Fatalf("expected identity columns, got %v", columns)
	}
}

func TestReviewIdentityVerificationApproveAndReject(t *testing.T) {
	now := time.Date(2026, 7, 6, 14, 0, 0, 0, time.UTC)
	user := &User{
		IdentityVerificationStatus: IdentityVerificationStatusPending,
	}

	columns, err := ReviewIdentityVerification(user, IdentityVerificationStatusApproved, "built-in/admin", "", now)
	if err != nil {
		t.Fatalf("approve should not fail: %v", err)
	}
	if !user.IsVerified || user.IdentityVerificationStatus != IdentityVerificationStatusApproved {
		t.Fatalf("expected approved verified user, got: %+v", user)
	}
	if user.IdentityVerificationReviewer != "built-in/admin" || user.IdentityVerificationReviewedTime != "2026-07-06 14:00:00" {
		t.Fatalf("unexpected approval metadata: %+v", user)
	}
	if !strings.Contains(strings.Join(columns, ","), "is_verified") {
		t.Fatalf("review columns should include is_verified, got %v", columns)
	}

	columns, err = ReviewIdentityVerification(user, IdentityVerificationStatusRejected, "built-in/admin", "证件信息不匹配", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("reject should not fail: %v", err)
	}
	if user.IsVerified || user.IdentityVerificationStatus != IdentityVerificationStatusRejected {
		t.Fatalf("expected rejected unverified user, got: %+v", user)
	}
	if user.IdentityVerificationReason != "证件信息不匹配" {
		t.Fatalf("expected rejection reason, got %q", user.IdentityVerificationReason)
	}
	if !strings.Contains(strings.Join(columns, ","), "identity_verification_reason") {
		t.Fatalf("review columns should include reason, got %v", columns)
	}
}

func TestReviewIdentityVerificationRejectRequiresReason(t *testing.T) {
	user := &User{}
	_, err := ReviewIdentityVerification(user, IdentityVerificationStatusRejected, "built-in/admin", "  ", time.Date(2026, 7, 6, 14, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected rejection without reason to fail")
	}
}

func TestNormalizeIdentityVerificationStatusKeepsLegacyVerifiedUsersApproved(t *testing.T) {
	legacyUser := &User{IsVerified: true}
	if got := NormalizeIdentityVerificationStatus(legacyUser); got != IdentityVerificationStatusApproved {
		t.Fatalf("legacy verified user should be approved, got %s", got)
	}

	newUser := &User{IsVerified: false, IdentityVerificationStatus: IdentityVerificationStatusRejected}
	if got := NormalizeIdentityVerificationStatus(newUser); got != IdentityVerificationStatusRejected {
		t.Fatalf("explicit status should be preserved, got %s", got)
	}
}

func TestGetDefaultIdentityVerificationRules(t *testing.T) {
	rules := GetDefaultIdentityVerificationRules()
	if rules == nil {
		t.Fatal("default identity verification rules should not be nil")
	}
	if !rules.Enabled {
		t.Fatal("default rules should enable public identity verification rules")
	}
	if rules.RequireIdvProvider {
		t.Fatal("default rules should not require a trusted ID Verification provider")
	}
	if !containsIdentityRuleString(rules.AllowedIdCardTypes, IdentityIdCardTypeChineseIdCard) {
		t.Fatalf("default rules should allow %s, got %v", IdentityIdCardTypeChineseIdCard, rules.AllowedIdCardTypes)
	}
	if rules.Under16Action != IdentityVerificationRuleActionManualReview {
		t.Fatalf("under 16 users should default to manual review, got %s", rules.Under16Action)
	}
	if rules.ProviderFailureAction != IdentityVerificationRuleActionReject {
		t.Fatalf("provider mismatches should default to reject, got %s", rules.ProviderFailureAction)
	}
	if rules.ProviderErrorAction != IdentityVerificationRuleActionManualReview {
		t.Fatalf("provider errors should default to manual review, got %s", rules.ProviderErrorAction)
	}
}

func TestNormalizeIdentityVerificationRulesFillsSafeDefaults(t *testing.T) {
	rules := NormalizeIdentityVerificationRules(&IdentityVerificationRules{
		Enabled:            true,
		AllowedIdCardTypes: []string{" CN_ID ", "", "passport"},
		Under16Action:      "bad-action",
	})

	if !rules.Enabled || rules.RequireIdvProvider {
		t.Fatalf("normalization should preserve enabled and keep provider optional by default, got %+v", rules)
	}
	if strings.Join(rules.AllowedIdCardTypes, ",") != "CN_ID,passport" {
		t.Fatalf("unexpected normalized allowed card types: %v", rules.AllowedIdCardTypes)
	}
	if rules.Under16Action != IdentityVerificationRuleActionManualReview {
		t.Fatalf("invalid under 16 action should become manual review, got %s", rules.Under16Action)
	}
	if rules.ProviderFailureAction != IdentityVerificationRuleActionReject {
		t.Fatalf("empty provider failure action should become reject, got %s", rules.ProviderFailureAction)
	}
}

func TestNormalizeIdentityVerificationRulesDisallowsUnsafeAutoApproveActions(t *testing.T) {
	rules := NormalizeIdentityVerificationRules(&IdentityVerificationRules{
		Enabled:                 true,
		Under16Action:           IdentityVerificationRuleActionApprove,
		ProviderFailureAction:   IdentityVerificationRuleActionApprove,
		ProviderErrorAction:     IdentityVerificationRuleActionApprove,
		UnsupportedIdCardAction: IdentityVerificationRuleActionApprove,
		InvalidIdCardAction:     IdentityVerificationRuleActionApprove,
	})

	if rules.Under16Action != IdentityVerificationRuleActionManualReview {
		t.Fatalf("under 16 action must not auto approve, got %s", rules.Under16Action)
	}
	if rules.ProviderFailureAction != IdentityVerificationRuleActionReject {
		t.Fatalf("provider failure action must not auto approve, got %s", rules.ProviderFailureAction)
	}
	if rules.ProviderErrorAction != IdentityVerificationRuleActionManualReview {
		t.Fatalf("provider error action must not auto approve, got %s", rules.ProviderErrorAction)
	}
	if rules.UnsupportedIdCardAction != IdentityVerificationRuleActionReject {
		t.Fatalf("unsupported card action must not auto approve, got %s", rules.UnsupportedIdCardAction)
	}
	if rules.InvalidIdCardAction != IdentityVerificationRuleActionReject {
		t.Fatalf("invalid card action must not auto approve, got %s", rules.InvalidIdCardAction)
	}
}

func TestEvaluateIdentityVerificationRulesDisabledLeavesPending(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "11010519900101234X",
	}

	result := EvaluateIdentityVerificationRules(user, &IdentityVerificationRules{Enabled: false}, IdentityVerificationProviderCheckPassed, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusPending {
		t.Fatalf("disabled rules should keep user pending, got %+v", result)
	}
	if result.Reason != "" {
		t.Fatalf("disabled rules should not set a review reason, got %q", result.Reason)
	}
	if result.ShouldCallProvider {
		t.Fatal("disabled rules should not call provider")
	}
}

func TestEvaluateIdentityVerificationRulesRejectsUnsupportedCardType(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: "passport",
		IdCard:     "P1234567",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckSkipped, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusRejected {
		t.Fatalf("unsupported document should be rejected, got %+v", result)
	}
	if result.Reason == "" {
		t.Fatal("rejected result should include a reason")
	}
	if result.ShouldCallProvider {
		t.Fatal("rule should reject before provider call")
	}
}

func TestEvaluateIdentityVerificationRulesRejectsInvalidChineseIdCard(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "bad-id-card",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckSkipped, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusRejected {
		t.Fatalf("invalid Chinese ID card should be rejected, got %+v", result)
	}
	if result.ShouldCallProvider {
		t.Fatal("invalid local document should not call provider")
	}
}

func TestEvaluateIdentityVerificationRulesRejectsInvalidChineseIdCardChecksum(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "110105194912310021",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckSkipped, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusRejected {
		t.Fatalf("Chinese ID card with bad checksum should be rejected, got %+v", result)
	}
	if result.ShouldCallProvider {
		t.Fatal("invalid local document should not call provider")
	}
}

func TestEvaluateIdentityVerificationRulesRejectsNonChineseRealName(t *testing.T) {
	user := &User{
		RealName:   "dc",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "11010519491231002X",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckSkipped, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusRejected {
		t.Fatalf("non-Chinese real name should be rejected, got %+v", result)
	}
	if result.ShouldCallProvider {
		t.Fatal("invalid local identity data should not call provider")
	}
}

func TestEvaluateIdentityVerificationRulesKeepsUnder16PendingByDefault(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "110105201507062344",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckSkipped, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusPending {
		t.Fatalf("under 16 should default to manual review, got %+v", result)
	}
	if result.ShouldCallProvider {
		t.Fatal("under 16 manual review should not call provider")
	}
}

func TestEvaluateIdentityVerificationRulesApprovesTrustedProviderPass(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "11010519900101234X",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckPassed, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusApproved {
		t.Fatalf("trusted provider pass should approve, got %+v", result)
	}
	if result.ShouldCallProvider {
		t.Fatal("provider result has already been supplied")
	}
}

func TestEvaluateIdentityVerificationRulesAutoApprovesPublicRulesByDefault(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "11010519900101234X",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckSkipped, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusApproved {
		t.Fatalf("public rules should approve valid over-16 Chinese ID without provider, got %+v", result)
	}
	if result.ShouldCallProvider {
		t.Fatal("public rules should not request provider verification by default")
	}
}

func TestEvaluateIdentityVerificationRulesCanRequireProviderBeforeAutoApproval(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "11010519900101234X",
	}
	rules := enabledIdentityVerificationRules()
	rules.RequireIdvProvider = true

	result := EvaluateIdentityVerificationRules(user, rules, IdentityVerificationProviderCheckSkipped, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusPending {
		t.Fatalf("provider-required rules should wait for provider before approval, got %+v", result)
	}
	if !result.ShouldCallProvider {
		t.Fatal("provider-required rules should request provider verification")
	}
}

func TestEvaluateIdentityVerificationRulesProviderFailureAndErrorActions(t *testing.T) {
	user := &User{
		RealName:   "张三",
		IdCardType: IdentityIdCardTypeChineseIdCard,
		IdCard:     "11010519900101234X",
	}

	result := EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckFailed, nil, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusRejected {
		t.Fatalf("provider mismatch should reject by default, got %+v", result)
	}

	result = EvaluateIdentityVerificationRules(user, enabledIdentityVerificationRules(), IdentityVerificationProviderCheckError, errForTest("provider unavailable"), time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if result.Status != IdentityVerificationStatusPending {
		t.Fatalf("provider error should default to manual review, got %+v", result)
	}
	if result.Reason == "" {
		t.Fatal("provider error should include a review reason")
	}
}

func TestApplyIdentityVerificationRuleResult(t *testing.T) {
	now := time.Date(2026, 7, 6, 16, 0, 0, 0, time.UTC)
	user := &User{
		IdentityVerificationStatus:        IdentityVerificationStatusPending,
		IdentityVerificationSubmittedTime: "2026-07-06 15:00:00",
	}

	columns, err := ApplyIdentityVerificationRuleResult(user, &IdentityVerificationRuleResult{
		Status: IdentityVerificationStatusPending,
		Reason: "等待人工审核",
	}, "identity verification rules", now)
	if err != nil {
		t.Fatalf("pending rule result should not fail: %v", err)
	}
	if user.IsVerified || user.IdentityVerificationStatus != IdentityVerificationStatusPending || user.IdentityVerificationReason != "等待人工审核" {
		t.Fatalf("unexpected pending rule application: %+v", user)
	}
	if user.IdentityVerificationReviewedTime != "" || user.IdentityVerificationReviewer != "" {
		t.Fatalf("pending rule should not set review metadata: %+v", user)
	}
	if !containsIdentityRuleString(columns, "identity_verification_reason") {
		t.Fatalf("pending columns should include reason, got %v", columns)
	}

	columns, err = ApplyIdentityVerificationRuleResult(user, &IdentityVerificationRuleResult{
		Status: IdentityVerificationStatusApproved,
		Reason: "自动通过",
	}, "identity verification rules", now)
	if err != nil {
		t.Fatalf("approved rule result should not fail: %v", err)
	}
	if !user.IsVerified || user.IdentityVerificationStatus != IdentityVerificationStatusApproved || user.IdentityVerificationReason != "自动通过" {
		t.Fatalf("unexpected approved rule application: %+v", user)
	}
	if user.IdentityVerificationReviewer != "identity verification rules" || user.IdentityVerificationReviewedTime != "2026-07-06 16:00:00" {
		t.Fatalf("approved rule should set review metadata: %+v", user)
	}
	if !containsIdentityRuleString(columns, "is_verified") {
		t.Fatalf("approved columns should include is_verified, got %v", columns)
	}

	columns, err = ApplyIdentityVerificationRuleResult(user, &IdentityVerificationRuleResult{
		Status: IdentityVerificationStatusRejected,
		Reason: "自动驳回",
	}, "identity verification rules", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("rejected rule result should not fail: %v", err)
	}
	if user.IsVerified || user.IdentityVerificationStatus != IdentityVerificationStatusRejected || user.IdentityVerificationReason != "自动驳回" {
		t.Fatalf("unexpected rejected rule application: %+v", user)
	}
	if !containsIdentityRuleString(columns, "identity_verification_reason") {
		t.Fatalf("rejected columns should include reason, got %v", columns)
	}
}

func TestIsTrustedIdentityVerificationProvider(t *testing.T) {
	rules := GetDefaultIdentityVerificationRules()
	if !IsTrustedIdentityVerificationProvider(rules, &Provider{Category: "ID Verification", Type: "Alibaba Cloud"}) {
		t.Fatal("Alibaba Cloud should be trusted for automatic identity verification by default")
	}
	if IsTrustedIdentityVerificationProvider(rules, &Provider{Category: "ID Verification", Type: "Jumio"}) {
		t.Fatal("Jumio should not be trusted by default because the current provider only starts a verification session")
	}
	if IsTrustedIdentityVerificationProvider(rules, &Provider{Category: "SMS", Type: "Alibaba Cloud"}) {
		t.Fatal("non-ID Verification providers should never be trusted for identity verification")
	}
}

func TestValidateIdentityVerificationLaunch(t *testing.T) {
	now := time.Unix(1781712000, 0)
	application := &Application{
		Name:         "app-gepin",
		ClientId:     "client-id",
		ClientSecret: "client-secret",
		Organization: "gepin",
		RedirectUris: []string{
			"https://child.example.com/identity/callback",
		},
	}
	user := &User{Id: "sub-001", Owner: "gepin", Name: "alice"}
	req := &IdentityVerificationLaunchRequest{
		ClientId:    application.ClientId,
		UserId:      user.Id,
		RedirectUri: "https://child.example.com/identity/callback",
		State:       "state-001",
		Timestamp:   "1781712000",
		Nonce:       "nonce-001",
	}
	req.Signature = SignIdentityVerificationLaunch(application.ClientSecret, req.Timestamp, req.Nonce, req.ClientId, req.UserId, req.RedirectUri, req.State)

	info, err := ValidateIdentityVerificationLaunch(application, user, req, now)
	if err != nil {
		t.Fatalf("expected launch to validate: %v", err)
	}
	if info.ClientId != application.ClientId || info.UserId != user.Id || info.RedirectUri != req.RedirectUri || info.State != req.State {
		t.Fatalf("unexpected launch info: %+v", info)
	}
}

func TestValidateIdentityVerificationLaunchAllowsAnonymousSession(t *testing.T) {
	now := time.Unix(1781712000, 0)
	application := &Application{
		Name:         "app-gepin",
		ClientId:     "client-id",
		ClientSecret: "client-secret",
		Organization: "gepin",
		RedirectUris: []string{"https://child.example.com/identity/callback"},
	}
	user := &User{Id: "sub-001", Owner: "gepin", Name: "alice"}
	req := &IdentityVerificationLaunchRequest{
		ClientId:    application.ClientId,
		UserId:      user.Id,
		RedirectUri: "https://child.example.com/identity/callback",
		State:       "state-001",
		Timestamp:   "1781712000",
		Nonce:       "nonce-001",
	}
	req.Signature = SignIdentityVerificationLaunch(application.ClientSecret, req.Timestamp, req.Nonce, req.ClientId, req.UserId, req.RedirectUri, req.State)

	if _, err := ValidateIdentityVerificationLaunch(application, user, req, now); err != nil {
		t.Fatalf("anonymous signed launch should validate: %v", err)
	}
}

func TestValidateIdentityVerificationLaunchRejectsBadInputs(t *testing.T) {
	now := time.Unix(1781712000, 0)
	application := &Application{
		Name:         "app-gepin",
		ClientId:     "client-id",
		ClientSecret: "client-secret",
		Organization: "gepin",
		RedirectUris: []string{
			"https://child.example.com/identity/callback",
		},
	}
	user := &User{Id: "sub-001", Owner: "gepin", Name: "alice"}
	base := &IdentityVerificationLaunchRequest{
		ClientId:    application.ClientId,
		UserId:      user.Id,
		RedirectUri: "https://child.example.com/identity/callback",
		State:       "state-001",
		Timestamp:   "1781712000",
		Nonce:       "nonce-001",
	}
	base.Signature = SignIdentityVerificationLaunch(application.ClientSecret, base.Timestamp, base.Nonce, base.ClientId, base.UserId, base.RedirectUri, base.State)

	tests := []struct {
		name        string
		application *Application
		user        *User
		mutate      func(*IdentityVerificationLaunchRequest)
		now         time.Time
	}{
		{name: "invalid signature", application: application, user: user, mutate: func(req *IdentityVerificationLaunchRequest) { req.Signature = "bad-signature" }, now: now},
		{name: "expired timestamp", application: application, user: user, now: now.Add(6 * time.Minute)},
		{name: "cross organization user", application: application, user: &User{Id: user.Id, Owner: "other", Name: "alice"}, now: now},
		{name: "invalid redirect uri", application: application, user: user, mutate: func(req *IdentityVerificationLaunchRequest) {
			req.RedirectUri = "https://evil.example.com/callback"
			req.Signature = SignIdentityVerificationLaunch(application.ClientSecret, req.Timestamp, req.Nonce, req.ClientId, req.UserId, req.RedirectUri, req.State)
		}, now: now},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := *base
			if tt.mutate != nil {
				tt.mutate(&req)
			}
			if _, err := ValidateIdentityVerificationLaunch(tt.application, tt.user, &req, tt.now); err == nil {
				t.Fatal("expected launch validation to fail")
			}
		})
	}
}

func TestApplicationHasProvider(t *testing.T) {
	application := &Application{
		Providers: []*ProviderItem{
			{Name: "idv1", Provider: &Provider{Owner: "gepin", Name: "idv1", Category: "ID Verification"}},
			{Owner: "gepin", Name: "idv2"},
			{Name: "legacy-idv"},
		},
	}

	tests := []struct {
		name     string
		provider *Provider
		want     bool
	}{
		{name: "matched hydrated provider", provider: &Provider{Owner: "gepin", Name: "idv1"}, want: true},
		{name: "matched explicit owner provider item", provider: &Provider{Owner: "gepin", Name: "idv2"}, want: true},
		{name: "matched legacy provider item without owner", provider: &Provider{Owner: "gepin", Name: "legacy-idv"}, want: true},
		{name: "same name but wrong owner is rejected when owner is stored", provider: &Provider{Owner: "other", Name: "idv2"}, want: false},
		{name: "missing provider is rejected", provider: &Provider{Owner: "gepin", Name: "idv3"}, want: false},
		{name: "nil provider is rejected", provider: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := application.HasProvider(tt.provider); got != tt.want {
				t.Fatalf("HasProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func containsIdentityRuleString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func enabledIdentityVerificationRules() *IdentityVerificationRules {
	rules := GetDefaultIdentityVerificationRules()
	rules.Enabled = true
	return rules
}

type errForTest string

func (e errForTest) Error() string {
	return string(e)
}
