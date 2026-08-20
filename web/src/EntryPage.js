// Copyright 2022 The Casdoor Authors. All Rights Reserved.
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

import React from "react";
import {Redirect, Route, Switch} from "react-router-dom";
import i18next from "i18next";
import Loading from "./common/Loading";
import * as ApplicationBackend from "./backend/ApplicationBackend";
import PricingPage from "./pricing/PricingPage";
import * as Setting from "./Setting";
import * as Conf from "./Conf";
import SignupPage from "./auth/SignupPage";
import SelfLoginPage from "./auth/SelfLoginPage";
import LoginPage from "./auth/LoginPage";
import SelfForgetPage from "./auth/SelfForgetPage";
import ForgetPage from "./auth/ForgetPage";
import PromptPage from "./auth/PromptPage";
import ConsentPage from "./auth/ConsentPage";
import ResultPage from "./auth/ResultPage";
import CasLogout from "./auth/CasLogout";
import {authConfig} from "./auth/Auth";
import ProductBuyPage from "./ProductBuyPage";
import PaymentResultPage from "./PaymentResultPage";
import QrCodePage from "./QrCodePage";
import CaptchaPage from "./CaptchaPage";
import CustomHead from "./basic/CustomHead";
import * as Util from "./auth/Util";
import IdentityVerificationPage from "./IdentityVerificationPage";
import {hasIdentityVerificationLaunchParams} from "./identityVerificationLaunch";

export {hasIdentityVerificationLaunchParams};

class EntryPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      application: undefined,
      pricing: undefined,
    };
  }

  renderHomeIfLoggedIn(component) {
    if (this.props.account !== null && this.props.account !== undefined) {
      return <Redirect to={{pathname: "/", state: {from: "/login"}}} />;
    } else {
      return component;
    }
  }

  renderLoginIfNotLoggedIn(component) {
    if (this.props.account === null) {
      sessionStorage.setItem("from", `${window.location.pathname}${window.location.search}`);
      return <Redirect to="/login" />;
    } else if (this.props.account === undefined) {
      return null;
    } else if (this.props.account.needUpdatePassword) {
      Setting.savePostPasswordUpdateRedirect(`${window.location.pathname}${window.location.search}`, this.props.account);
      return <Redirect to="/account" />;
    } else {
      return component;
    }
  }

  isIdentityVerificationPage() {
    const pathname = window.location.pathname;
    return pathname === "/identity-verification" || pathname.startsWith("/identity-verification/submit");
  }

  render() {
    const onUpdateApplication = (application) => {
      this.setState({
        application: application,
      });
      const themeData = application !== null ? Setting.getThemeData(application.organizationObj, application) : Conf.ThemeDefault;
      this.props.updataThemeData(themeData);
      this.props.updateApplication(application);

      if (application) {
        localStorage.setItem("applicationName", application.name);
      }
    };

    const onUpdatePricing = (pricing) => {
      this.setState({
        pricing: pricing,
      });

      ApplicationBackend.getApplication("admin", pricing.application)
        .then((res) => {
          if (res.status === "error") {
            Setting.showMessage("error", res.msg);
            return;
          }
          const application = res.data;
          const themeData = application !== null ? Setting.getThemeData(application.organizationObj, application) : Conf.ThemeDefault;
          this.props.updataThemeData(themeData);
        });
    };

    const isIdentityVerificationPage = this.isIdentityVerificationPage();
    const application = this.state.application ?? (isIdentityVerificationPage ? this.props.application : undefined);

    if (application?.ipRestriction) {
      return Util.renderMessageLarge(this, application.ipRestriction);
    }

    if (application?.organizationObj?.ipRestriction) {
      return Util.renderMessageLarge(this, application.organizationObj.ipRestriction);
    }

    const isDarkMode = this.props.themeAlgorithm.includes("dark");
    const backgroundImage = Setting.inIframe() || application === undefined || application === null ? null :
      (Setting.isMobile() ? `url(${application.formBackgroundUrlMobile})` : `url(${application.formBackgroundUrl})`);

    return (
      <React.Fragment>
        <CustomHead headerHtml={application?.headerHtml} />
        <div className={`${isDarkMode ? "loginBackgroundDark" : "loginBackground"}`}
          style={{backgroundImage}}>
          <Loading
            spinning={!isIdentityVerificationPage && application === undefined && this.state.pricing === undefined}
            tip={i18next.t("login:Loading")}
            style={{position: "absolute", width: "100%", top: 0, bottom: 0}}
          />
          <Switch>
            <Route exact path="/signup" render={(props) => this.renderHomeIfLoggedIn(<SignupPage {...this.props} application={this.state.application} applicationName={authConfig.appName} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/signup/:applicationName" render={(props) => this.renderHomeIfLoggedIn(<SignupPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/login" render={(props) => this.renderHomeIfLoggedIn(<SelfLoginPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/login/:owner" render={(props) => this.renderHomeIfLoggedIn(<SelfLoginPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/signup/oauth/authorize" render={(props) => <SignupPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />} />
            <Route exact path="/login/oauth/authorize" render={(props) => <LoginPage {...this.props} application={this.state.application} type={"code"} mode={"signin"} onUpdateApplication={onUpdateApplication} {...props} />} />
            <Route exact path="/login/oauth/device/:userCode" render={(props) => <LoginPage {...this.props} application={this.state.application} type={"device"} mode={"signin"} onUpdateApplication={onUpdateApplication} {...props} />} />
            <Route exact path="/login/saml/authorize/:owner/:applicationName" render={(props) => <LoginPage {...this.props} application={this.state.application} type={"saml"} mode={"signin"} onUpdateApplication={onUpdateApplication} {...props} />} />
            <Route exact path="/forget" render={(props) => <SelfForgetPage {...this.props} account={this.props.account} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />} />
            <Route exact path="/forget/:applicationName" render={(props) => <ForgetPage {...this.props} account={this.props.account} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />} />
            <Route exact path="/prompt" render={(props) => this.renderLoginIfNotLoggedIn(<PromptPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/prompt/:applicationName" render={(props) => this.renderLoginIfNotLoggedIn(<PromptPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/consent/:applicationName" render={(props) => this.renderLoginIfNotLoggedIn(<ConsentPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/result" render={(props) => this.renderHomeIfLoggedIn(<ResultPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/result/:applicationName" render={(props) => this.renderHomeIfLoggedIn(<ResultPage {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/cas/:owner/:casApplicationName/logout" render={(props) => this.renderHomeIfLoggedIn(<CasLogout {...this.props} application={this.state.application} onUpdateApplication={onUpdateApplication} {...props} />)} />
            <Route exact path="/cas/:owner/:casApplicationName/login" render={(props) => {return (<LoginPage {...this.props} application={this.state.application} type={"cas"} mode={"signin"} onUpdateApplication={onUpdateApplication} {...props} />);}} />
            <Route exact path="/select-plan/:owner/:pricingName" render={(props) => <PricingPage {...this.props} pricing={this.state.pricing} onUpdatePricing={onUpdatePricing} {...props} />} />
            <Route exact path="/buy-plan/:owner/:pricingName" render={(props) => <ProductBuyPage {...this.props} pricing={this.state.pricing} onUpdatePricing={onUpdatePricing} {...props} />} />
            <Route exact path="/buy-plan/:owner/:pricingName/result" render={(props) => <PaymentResultPage {...this.props} pricing={this.state.pricing} onUpdatePricing={onUpdatePricing} {...props} />} />
            <Route exact path="/qrcode/:owner/:paymentName" render={(props) => <QrCodePage {...this.props} onUpdateApplication={onUpdateApplication} {...props} />} />
            <Route exact path="/captcha" render={(props) => <CaptchaPage {...props} />} />
            <Route exact path="/identity-verification" render={(props) => <Redirect to={{pathname: "/identity-verification/submit", search: props.location.search}} />} />
            <Route exact path="/identity-verification/submit" render={(props) => {
              const page = <IdentityVerificationPage {...this.props} account={this.props.account} themeAlgorithm={this.props.themeAlgorithm} application={application} {...props} />;
              return hasIdentityVerificationLaunchParams(props.location.search) ? page : this.renderLoginIfNotLoggedIn(page);
            }} />
          </Switch>
        </div>

      </React.Fragment>
    );
  }
}

export default EntryPage;
