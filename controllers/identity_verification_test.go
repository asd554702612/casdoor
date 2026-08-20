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
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"github.com/casdoor/casdoor/idv"
	"github.com/casdoor/casdoor/object"
)

type fakeIdentityVerificationProvider struct {
	called bool
}

func TestDecodeIdentityVerificationLaunchSessionExpires(t *testing.T) {
	raw := `{"userId":"gepin/target","owner":"gepin","name":"target","expiresAt":100}`
	if _, err := decodeIdentityVerificationLaunchSession(raw, time.Unix(101, 0)); err == nil {
		t.Fatal("expired launch context should be rejected")
	}
}

func TestChooseIdentityVerificationTargetPrefersLaunchUser(t *testing.T) {
	launchUser := &object.User{Owner: "gepin", Name: "target"}
	currentUser := &object.User{Owner: "gepin", Name: "other"}
	got, fromLaunch := chooseIdentityVerificationTarget(launchUser, currentUser)
	if got != launchUser || !fromLaunch {
		t.Fatalf("launch target should win: got=%v fromLaunch=%v", got, fromLaunch)
	}
}

func TestChooseIdentityVerificationTargetFallsBackToSessionUser(t *testing.T) {
	currentUser := &object.User{Owner: "gepin", Name: "current"}
	got, fromLaunch := chooseIdentityVerificationTarget(nil, currentUser)
	if got != currentUser || fromLaunch {
		t.Fatalf("session user should be used without launch context: got=%v fromLaunch=%v", got, fromLaunch)
	}
}

func (p *fakeIdentityVerificationProvider) VerifyIdentity(idCardType string, idCard string, realName string) (bool, error) {
	p.called = true
	if idCardType != object.IdentityIdCardTypeChineseIdCard || idCard != "11010519991212345X" || realName != "Alice Real" {
		return false, nil
	}
	return true, nil
}

func TestEvaluateVerifyIdentificationRulesUsesLegacyProviderWhenRulesDisabled(t *testing.T) {
	fakeProvider := &fakeIdentityVerificationProvider{}
	oldGetIdvProviderFromProvider := getIdvProviderFromProvider
	getIdvProviderFromProvider = func(provider *object.Provider) idv.IdvProvider {
		return fakeProvider
	}
	defer func() {
		getIdvProviderFromProvider = oldGetIdvProviderFromProvider
	}()

	user := &object.User{
		IdCardType: object.IdentityIdCardTypeChineseIdCard,
		IdCard:     "11010519991212345X",
		RealName:   "Alice Real",
	}
	application := &object.Application{
		IdentityVerificationRules: &object.IdentityVerificationRules{Enabled: false},
	}
	provider := &object.Provider{Category: "ID Verification", Type: "Alibaba Cloud"}

	result := (&ApiController{}).evaluateVerifyIdentificationRules(user, application, provider, time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC))
	if !fakeProvider.called {
		t.Fatal("expected legacy ID verification provider to be called when rules are disabled")
	}
	if result.Status != object.IdentityVerificationStatusApproved {
		t.Fatalf("expected approved status from legacy provider pass, got %+v", result)
	}
}

func TestVerifyIdentificationUsesLegacyCompatibilityHelper(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "user.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	var verifyIdentification *ast.FuncDecl
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if ok && funcDecl.Name.Name == "VerifyIdentification" {
			verifyIdentification = funcDecl
			break
		}
	}
	if verifyIdentification == nil {
		t.Fatal("VerifyIdentification was not found")
	}

	var callsLegacyCompatibilityHelper bool
	var callsRulesEngineDirectly bool
	ast.Inspect(verifyIdentification.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch selector.Sel.Name {
		case "evaluateVerifyIdentificationRules":
			callsLegacyCompatibilityHelper = true
		case "evaluateIdentityVerificationRules":
			callsRulesEngineDirectly = true
		}
		return true
	})

	if !callsLegacyCompatibilityHelper {
		t.Fatal("VerifyIdentification should call evaluateVerifyIdentificationRules to preserve legacy provider behavior when rules are disabled")
	}
	if callsRulesEngineDirectly {
		t.Fatal("VerifyIdentification should not call evaluateIdentityVerificationRules directly")
	}
}
