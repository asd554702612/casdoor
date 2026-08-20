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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/beego/beego/v2/core/utils/pagination"
	"github.com/casdoor/casdoor/idv"
	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
)

var getIdvProviderFromProvider = func(provider *object.Provider) idv.IdvProvider {
	return object.GetIdvProviderFromProvider(provider)
}

const identityVerificationLaunchSessionKey = "identityVerificationLaunch"

type identityVerificationLaunchSession struct {
	UserId      string `json:"userId"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	RedirectUri string `json:"redirectUri"`
	State       string `json:"state"`
	ExpiresAt   int64  `json:"expiresAt"`
}

func decodeIdentityVerificationLaunchSession(raw string, now time.Time) (*identityVerificationLaunchSession, error) {
	var session identityVerificationLaunchSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, fmt.Errorf("invalid identity verification launch context")
	}
	if session.UserId == "" || session.Owner == "" || session.Name == "" || session.ExpiresAt <= now.Unix() {
		return nil, fmt.Errorf("expired or invalid identity verification launch context")
	}
	return &session, nil
}

func chooseIdentityVerificationTarget(launchUser *object.User, currentUser *object.User) (*object.User, bool) {
	if launchUser != nil {
		return launchUser, true
	}
	return currentUser, false
}

func (c *ApiController) setIdentityVerificationLaunchSession(info *object.IdentityVerificationLaunchInfo, expiresAt int64) error {
	session := &identityVerificationLaunchSession{
		UserId:      info.UserId,
		Owner:       info.Owner,
		Name:        info.Name,
		RedirectUri: info.RedirectUri,
		State:       info.State,
		ExpiresAt:   expiresAt,
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return c.SetSession(identityVerificationLaunchSessionKey, string(raw))
}

func (c *ApiController) clearIdentityVerificationLaunchSession() {
	_ = c.DelSession(identityVerificationLaunchSessionKey)
}

func (c *ApiController) getIdentityVerificationLaunchSession(now time.Time) (*identityVerificationLaunchSession, bool, error) {
	value := c.GetSession(identityVerificationLaunchSessionKey)
	if value == nil {
		return nil, false, nil
	}
	raw, ok := value.(string)
	if !ok {
		c.clearIdentityVerificationLaunchSession()
		return nil, true, fmt.Errorf("invalid identity verification launch context")
	}
	session, err := decodeIdentityVerificationLaunchSession(raw, now)
	if err != nil {
		c.clearIdentityVerificationLaunchSession()
		return nil, true, err
	}
	return session, true, nil
}

func (c *ApiController) getIdentityVerificationLaunchUser(now time.Time) (*object.User, bool, error) {
	session, exists, err := c.getIdentityVerificationLaunchSession(now)
	if err != nil || !exists {
		return nil, exists, err
	}
	user, err := object.GetUserByUserId(session.Owner, session.UserId)
	if err != nil {
		return nil, true, err
	}
	if user == nil || user.Id != session.UserId || user.Owner != session.Owner || user.Name != session.Name {
		c.clearIdentityVerificationLaunchSession()
		return nil, true, fmt.Errorf("identity verification launch user is no longer valid")
	}
	return user, true, nil
}

func (c *ApiController) getIdentityVerificationTarget(now time.Time) (*object.User, bool, error) {
	launchUser, _, err := c.getIdentityVerificationLaunchUser(now)
	if err != nil {
		return nil, false, err
	}
	var currentUser *object.User
	currentUserId := c.GetSessionUsername()
	if currentUserId != "" {
		currentUser, err = object.GetUser(currentUserId)
		if err != nil {
			return nil, false, err
		}
		if currentUser == nil {
			return nil, false, fmt.Errorf(c.T("general:The user: %s doesn't exist"), currentUserId)
		}
	}
	target, fromLaunch := chooseIdentityVerificationTarget(launchUser, currentUser)
	if target == nil {
		return nil, false, fmt.Errorf("%s", c.T("general:Please login first"))
	}
	return target, fromLaunch, nil
}

// GetIdentityVerification
// @Title GetIdentityVerification
// @Tag User API
// @Description get user's identity verification information
// @Param   id      query    string  false  "The id ( owner/name ) of the user"
// @Param   owner   query    string  false  "The owner of the user"
// @Param   name    query    string  false  "The name of the user"
// @Param   userId  query    string  false  "The userId (UUID) of the user"
// @Success 200 {object} object.IdentityVerificationInfo The Response object
// @router /get-identity-verification [get]
func (c *ApiController) GetIdentityVerification() {
	launchUser, fromLaunch, err := c.getIdentityVerificationLaunchUser(time.Now())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if fromLaunch {
		c.ResponseOk(object.GetIdentityVerificationInfo(launchUser, time.Now()))
		return
	}

	id := c.Ctx.Input.Query("id")
	owner := c.Ctx.Input.Query("owner")
	name := c.Ctx.Input.Query("name")
	userId := c.Ctx.Input.Query("userId")

	var user *object.User
	err = nil
	if userId != "" && owner != "" {
		user, err = object.GetUserByUserId(owner, userId)
	} else {
		if id == "" {
			if owner != "" && name != "" {
				id = util.GetId(owner, name)
			} else {
				id = c.GetSessionUsername()
			}
		}
		if id == "" {
			c.ResponseError(c.T("general:Please login first"))
			return
		}
		user, err = object.GetUser(id)
	}
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if user == nil {
		if id == "" {
			id = userId
		}
		c.ResponseError(fmt.Sprintf(c.T("general:The user: %s doesn't exist"), id))
		return
	}
	currentUser := c.getCurrentUser()
	if !c.IsAdminOrSelf(user) {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return
	}
	if currentUser != nil && currentUser.IsAdmin && !currentUser.IsGlobalAdmin() && user.Owner != currentUser.Owner {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return
	}

	c.ResponseOk(object.GetIdentityVerificationInfo(user, time.Now()))
}

// GetIdentityVerifications
// @Title GetIdentityVerifications
// @Tag User API
// @Description get identity verification information list for administrators
// @Param   owner      query    string  false  "The organization owner"
// @Param   isVerified query    string  false  "Filter by verification status"
// @Success 200 {array} object.IdentityVerificationInfo The Response object
// @router /get-identity-verifications [get]
func (c *ApiController) GetIdentityVerifications() {
	if !c.IsAdmin() {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return
	}

	currentUser := c.getCurrentUser()
	owner := c.Ctx.Input.Query("owner")
	if currentUser != nil && !currentUser.IsGlobalAdmin() {
		if owner == "" {
			owner = currentUser.Owner
		}
		if owner != currentUser.Owner {
			c.ResponseError(c.T("auth:Unauthorized operation"))
			return
		}
	}

	limit := c.Ctx.Input.Query("pageSize")
	page := c.Ctx.Input.Query("p")
	field := c.Ctx.Input.Query("field")
	value := c.Ctx.Input.Query("value")
	sortField := c.Ctx.Input.Query("sortField")
	sortOrder := c.Ctx.Input.Query("sortOrder")
	isVerified := c.Ctx.Input.Query("isVerified")
	status := c.Ctx.Input.Query("status")
	now := time.Now()

	if limit == "" || page == "" {
		count, err := object.GetIdentityVerificationCount(owner, field, value, isVerified, status)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		infos, err := object.GetPaginationIdentityVerifications(owner, 0, int(count), field, value, sortField, sortOrder, isVerified, status, now)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(infos)
		return
	}

	limitInt := util.ParseInt(limit)
	count, err := object.GetIdentityVerificationCount(owner, field, value, isVerified, status)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	paginator := pagination.NewPaginator(c.Ctx.Request, limitInt, count)
	infos, err := object.GetPaginationIdentityVerifications(owner, paginator.Offset(), limitInt, field, value, sortField, sortOrder, isVerified, status, now)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(infos, paginator.Nums())
}

func (c *ApiController) getIdentityVerificationUser(owner string, name string, userId string) (*object.User, error) {
	if userId != "" {
		if owner == "" {
			currentUser := c.getCurrentUser()
			if currentUser != nil {
				owner = currentUser.Owner
			}
		}
		return object.GetUserByUserId(owner, userId)
	}
	if owner != "" && name != "" {
		return object.GetUser(util.GetId(owner, name))
	}
	return nil, fmt.Errorf("%s", c.T("general:Missing parameter"))
}

func (c *ApiController) ensureIdentityAdminForUser(user *object.User) bool {
	if !c.IsAdmin() {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return false
	}
	currentUser := c.getCurrentUser()
	if currentUser != nil && !currentUser.IsGlobalAdmin() && user.Owner != currentUser.Owner {
		c.ResponseError(c.T("auth:Unauthorized operation"))
		return false
	}
	return true
}

func (c *ApiController) evaluateIdentityVerificationRules(user *object.User, application *object.Application, provider *object.Provider, now time.Time) *object.IdentityVerificationRuleResult {
	rules := object.NormalizeIdentityVerificationRules(nil)
	if application != nil {
		rules = object.NormalizeIdentityVerificationRules(application.IdentityVerificationRules)
	}
	result := object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckSkipped, nil, now)
	if !result.ShouldCallProvider {
		return result
	}

	if provider == nil {
		if application == nil {
			return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckError, fmt.Errorf("application is not found"), now)
		}
		var err error
		provider, err = object.GetIdvProviderByApplication(util.GetId(application.Owner, application.Name), "false", c.GetAcceptLanguage())
		if err != nil {
			return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckError, err, now)
		}
	}
	if provider == nil {
		return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckError, fmt.Errorf("ID Verification provider is not configured"), now)
	}
	if !object.IsTrustedIdentityVerificationProvider(rules, provider) {
		return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckError, fmt.Errorf("%s", rules.ProviderUntrustedReason), now)
	}

	idvProvider := getIdvProviderFromProvider(provider)
	if idvProvider == nil {
		return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckError, fmt.Errorf("failed to initialize ID Verification provider"), now)
	}
	verified, err := idvProvider.VerifyIdentity(user.IdCardType, user.IdCard, user.RealName)
	if err != nil {
		return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckError, err, now)
	}
	if verified {
		return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckPassed, nil, now)
	}
	return object.EvaluateIdentityVerificationRules(user, rules, object.IdentityVerificationProviderCheckFailed, nil, now)
}

func (c *ApiController) evaluateVerifyIdentificationRules(user *object.User, application *object.Application, provider *object.Provider, now time.Time) *object.IdentityVerificationRuleResult {
	rules := object.NormalizeIdentityVerificationRules(nil)
	if application != nil {
		rules = object.NormalizeIdentityVerificationRules(application.IdentityVerificationRules)
	}
	if rules.Enabled {
		return c.evaluateIdentityVerificationRules(user, application, provider, now)
	}

	if provider == nil {
		if application == nil {
			return &object.IdentityVerificationRuleResult{
				Status: object.IdentityVerificationStatusRejected,
				Reason: "application is not found",
			}
		}
		var err error
		provider, err = object.GetIdvProviderByApplication(util.GetId(application.Owner, application.Name), "false", c.GetAcceptLanguage())
		if err != nil {
			return &object.IdentityVerificationRuleResult{
				Status: object.IdentityVerificationStatusRejected,
				Reason: err.Error(),
			}
		}
	}
	if provider == nil {
		return &object.IdentityVerificationRuleResult{
			Status: object.IdentityVerificationStatusRejected,
			Reason: "No ID Verification provider configured",
		}
	}

	idvProvider := getIdvProviderFromProvider(provider)
	if idvProvider == nil {
		return &object.IdentityVerificationRuleResult{
			Status: object.IdentityVerificationStatusRejected,
			Reason: "Failed to initialize ID Verification provider",
		}
	}
	verified, err := idvProvider.VerifyIdentity(user.IdCardType, user.IdCard, user.RealName)
	if err != nil {
		return &object.IdentityVerificationRuleResult{
			Status: object.IdentityVerificationStatusRejected,
			Reason: err.Error(),
		}
	}
	if !verified {
		return &object.IdentityVerificationRuleResult{
			Status: object.IdentityVerificationStatusRejected,
			Reason: "Identity verification failed",
		}
	}
	return &object.IdentityVerificationRuleResult{Status: object.IdentityVerificationStatusApproved}
}

func (c *ApiController) applyIdentityVerificationRuleResult(user *object.User, result *object.IdentityVerificationRuleResult, now time.Time) error {
	if result == nil || (result.Status == object.IdentityVerificationStatusPending && result.Reason == "") {
		return nil
	}
	columns, err := object.ApplyIdentityVerificationRuleResult(user, result, "identity verification rules", now)
	if err != nil {
		return err
	}
	_, err = object.UpdateUser(user.GetId(), user, columns, true)
	return err
}

// SubmitIdentityVerification
// @Title SubmitIdentityVerification
// @Tag User API
// @Description submit user's identity verification information
// @Param   body    body   object.IdentityVerificationSubmitRequest  true  "The submit request"
// @Success 200 {object} object.IdentityVerificationInfo The Response object
// @router /submit-identity-verification [post]
func (c *ApiController) SubmitIdentityVerification() {
	user, fromLaunch, err := c.getIdentityVerificationTarget(time.Now())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	id := user.GetId()

	var req object.IdentityVerificationSubmitRequest
	err = json.Unmarshal(c.Ctx.Input.RequestBody, &req)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if req.Owner != "" || req.Name != "" {
		if util.GetId(req.Owner, req.Name) != id {
			c.ResponseError(c.T("auth:Unauthorized operation"))
			return
		}
	}

	req.IdCardType = strings.TrimSpace(req.IdCardType)
	req.IdCard = strings.TrimSpace(req.IdCard)
	req.RealName = strings.TrimSpace(req.RealName)
	if req.IdCardType == object.IdentityIdCardTypeChineseIdCard {
		req.IdCard = object.NormalizeChineseIdCard(req.IdCard)
	}
	if err = object.ValidateIdentityVerificationInput(req.IdCardType, req.IdCard, req.RealName); err != nil {
		c.ResponseError(err.Error())
		return
	}

	user.IdCardType = req.IdCardType
	user.IdCard = req.IdCard
	user.RealName = req.RealName
	columns := []string{"id_card_type", "id_card", "real_name"}
	now := time.Now()
	for _, column := range object.SubmitIdentityVerification(user, now) {
		if !util.InSlice(columns, column) {
			columns = append(columns, column)
		}
	}

	_, err = object.UpdateUser(user.GetId(), user, columns, false)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	application, err := object.GetApplicationByUser(user)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	result := c.evaluateIdentityVerificationRules(user, application, nil, now)
	if err = c.applyIdentityVerificationRuleResult(user, result, now); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if fromLaunch {
		c.clearIdentityVerificationLaunchSession()
	}
	c.ResponseOk(object.GetIdentityVerificationInfo(user, time.Now()))
}

// ReviewIdentityVerification
// @Title ReviewIdentityVerification
// @Tag User API
// @Description review user's identity verification status
// @Param   body    body   object.IdentityVerificationReviewRequest  true  "The review request"
// @Success 200 {object} controllers.Response The Response object
// @router /review-identity-verification [post]
func (c *ApiController) ReviewIdentityVerification() {
	var req object.IdentityVerificationReviewRequest
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &req)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	user, err := c.getIdentityVerificationUser(req.Owner, req.Name, req.UserId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if user == nil {
		c.ResponseError(c.T("general:The user does not exist"))
		return
	}
	if !c.ensureIdentityAdminForUser(user) {
		return
	}

	reviewer := ""
	if currentUser := c.getCurrentUser(); currentUser != nil {
		reviewer = currentUser.GetId()
	}
	columns, err := object.ReviewIdentityVerification(user, req.Status, reviewer, req.Reason, time.Now())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	affected, err := object.UpdateUser(user.GetId(), user, columns, true)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.Data["json"] = wrapActionResponse(affected)
	c.ServeJSON()
}

// ResetIdentityVerification
// @Title ResetIdentityVerification
// @Tag User API
// @Description reset user's identity verification status for re-verification
// @Param   body    body   object.IdentityVerificationResetRequest  true  "The reset request"
// @Success 200 {object} controllers.Response The Response object
// @router /reset-identity-verification [post]
func (c *ApiController) ResetIdentityVerification() {
	var req object.IdentityVerificationResetRequest
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &req)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	user, err := c.getIdentityVerificationUser(req.Owner, req.Name, req.UserId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if user == nil {
		c.ResponseError(c.T("general:The user does not exist"))
		return
	}
	if !c.ensureIdentityAdminForUser(user) {
		return
	}

	columns := object.ResetIdentityVerification(user)
	affected, err := object.UpdateUser(user.GetId(), user, columns, true)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.Data["json"] = wrapActionResponse(affected)
	c.ServeJSON()
}

// GetIdentityVerificationLaunch
// @Title GetIdentityVerificationLaunch
// @Tag User API
// @Description validate a signed identity verification launch URL
// @Param   clientId    query    string  true  "The application clientId"
// @Param   userId      query    string  true  "The user sub"
// @Param   redirectUri query    string  true  "The application callback URL"
// @Param   state       query    string  false "The application state"
// @Param   timestamp   query    string  true  "The signature timestamp"
// @Param   nonce       query    string  true  "The signature nonce"
// @Param   signature   query    string  true  "The launch signature"
// @Success 200 {object} object.IdentityVerificationLaunchInfo The Response object
// @router /get-identity-verification-launch [get]
func (c *ApiController) GetIdentityVerificationLaunch() {
	c.clearIdentityVerificationLaunchSession()

	req := &object.IdentityVerificationLaunchRequest{
		ClientId:    c.Ctx.Input.Query("clientId"),
		UserId:      c.Ctx.Input.Query("userId"),
		RedirectUri: c.Ctx.Input.Query("redirectUri"),
		State:       c.Ctx.Input.Query("state"),
		Timestamp:   c.Ctx.Input.Query("timestamp"),
		Nonce:       c.Ctx.Input.Query("nonce"),
		Signature:   c.Ctx.Input.Query("signature"),
	}
	application, err := object.GetApplicationByClientId(req.ClientId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if application == nil {
		c.ResponseError("invalid application")
		return
	}

	user, err := object.GetUserByUserId(application.Organization, req.UserId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	info, err := object.ValidateIdentityVerificationLaunch(application, user, req, time.Now())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	timestamp, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		c.ResponseError("invalid timestamp")
		return
	}
	if err = c.setIdentityVerificationLaunchSession(info, timestamp+int64(object.IdentityVerificationLaunchTTL/time.Second)); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(info)
}

// SyncExternalUser
// @Title SyncExternalUser
// @Tag User API
// @Description sync trusted application user information from Casdoor
// @Param   body    body   object.ExternalUserSyncRequest  true  "The external user sync request"
// @Success 200 {object} object.ExternalUserSyncResponse The Response object
// @router /external/user/sync [post]
func (c *ApiController) SyncExternalUser() {
	application, body, ok := c.getSignedExternalApplication()
	if !ok {
		return
	}

	var req object.ExternalUserSyncRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if req.UserId == "" {
		c.ResponseError(c.T("general:Missing parameter"))
		return
	}

	now := time.Now()
	user, err := object.GetUserByUserId(application.Organization, req.UserId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if user == nil {
		c.ResponseError("user does not belong to application organization")
		return
	}

	resp := object.GetExternalUserSyncResponse(user, now)
	c.ResponseOk(resp)
}
