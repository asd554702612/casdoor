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

import React from "react";
import {Alert, Button, Descriptions, Input, Space, Tag, Typography} from "antd";
import {CheckCircleOutlined, ExportOutlined, ReloadOutlined} from "@ant-design/icons";
import * as Setting from "./Setting";
import * as UserBackend from "./backend/UserBackend";
import {getIdentityVerificationSubmitTarget, hasIdentityVerificationLaunchParams} from "./identityVerificationLaunch";

export {getIdentityVerificationSubmitTarget};

const {Text} = Typography;

const IdentityStatus = {
  unsubmitted: "unsubmitted",
  pending: "pending",
  approved: "approved",
  rejected: "rejected",
};

const ChineseNamePattern = /^[\u3400-\u4dbf\u4e00-\u9fff·]{2,30}$/;

class IdentityVerificationPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      identity: null,
      launchInfo: null,
      launchError: "",
      returnUrl: "",
      form: {
        idCardType: "",
        idCard: "",
        realName: "",
      },
      loading: false,
      launchLoading: false,
      verifying: false,
    };
    this.returnTimer = null;
  }

  componentDidMount() {
    this.loadPage();
  }

  componentDidUpdate(prevProps) {
    if (prevProps.account !== this.props.account || prevProps.location.search !== this.props.location.search) {
      this.loadPage();
    }
  }

  componentWillUnmount() {
    if (this.returnTimer !== null) {
      clearTimeout(this.returnTimer);
    }
  }

  getLaunchParams() {
    const query = new URLSearchParams(this.props.location.search || "");
    return {
      clientId: query.get("clientId") || "",
      userId: query.get("userId") || "",
      redirectUri: query.get("redirectUri") || "",
      state: query.get("state") || "",
      timestamp: query.get("timestamp") || "",
      nonce: query.get("nonce") || "",
      signature: query.get("signature") || "",
    };
  }

  hasLaunchParams() {
    return hasIdentityVerificationLaunchParams(this.props.location.search);
  }

  getAgeTag(identity) {
    if (!(identity?.status === IdentityStatus.approved || identity?.isVerified)) {
      return <Tag>未实名</Tag>;
    }
    if (!identity.ageChecked) {
      return <Tag color="orange">年龄未知</Tag>;
    }
    return identity.isOver16 ? <Tag color="green">已满 16 岁</Tag> : <Tag color="red">未满 16 岁</Tag>;
  }

  getStatusTag(status, isVerified) {
    if (status === IdentityStatus.approved || isVerified) {
      return <Tag color="green">已通过</Tag>;
    }
    if (status === IdentityStatus.pending) {
      return <Tag color="blue">待审核</Tag>;
    }
    if (status === IdentityStatus.rejected) {
      return <Tag color="red">已驳回</Tag>;
    }
    return <Tag>未提交</Tag>;
  }

  isApproved(identity) {
    return identity?.status === IdentityStatus.approved || identity?.isVerified;
  }

  updateFormField(field, value) {
    this.setState(prevState => ({
      form: {
        ...prevState.form,
        [field]: value,
      },
    }));
  }

  normalizeChineseIdCard(idCard) {
    return (idCard || "").trim().toUpperCase();
  }

  normalizeChineseIdCardInput(idCard) {
    const cleaned = (idCard || "").toUpperCase().replace(/[^\dX]/g, "");
    const chars = [];
    for (const char of cleaned) {
      if (chars.length >= 18) {
        break;
      }
      if (char === "X" && chars.length !== 17) {
        continue;
      }
      chars.push(char);
    }
    return chars.join("");
  }

  isValidChineseIdCard(idCard) {
    const value = this.normalizeChineseIdCard(idCard);
    if (!/^\d{17}[\dX]$/.test(value)) {
      return false;
    }

    const year = Number(value.slice(6, 10));
    const month = Number(value.slice(10, 12));
    const day = Number(value.slice(12, 14));
    const birthday = new Date(year, month - 1, day);
    if (birthday.getFullYear() !== year || birthday.getMonth() !== month - 1 || birthday.getDate() !== day) {
      return false;
    }

    const weights = [7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2];
    const checkCodes = "10X98765432";
    let sum = 0;
    for (let i = 0; i < 17; i += 1) {
      sum += Number(value[i]) * weights[i];
    }
    return value[17] === checkCodes[sum % 11];
  }

  isValidChineseRealName(realName) {
    const value = (realName || "").trim();
    if (!ChineseNamePattern.test(value) || value.startsWith("·") || value.endsWith("·") || value.includes("··")) {
      return false;
    }
    return Array.from(value).filter(char => char !== "·").length >= 2;
  }

  validateIdentityForm(idCardType, idCard, realName) {
    if (idCardType === "" || idCard === "" || realName === "") {
      Setting.showMessage("error", "请完整填写证件类型、身份证号和真实姓名");
      return false;
    }
    if (!this.isValidChineseRealName(realName)) {
      Setting.showMessage("error", "真实姓名必须为中文姓名");
      return false;
    }
    if (idCardType === "CN_ID" && !this.isValidChineseIdCard(idCard)) {
      Setting.showMessage("error", "请输入有效的18位身份证号");
      return false;
    }
    return true;
  }

  loadPage = async() => {
    if (await this.validateLaunch()) {
      this.refreshSelf();
    }
  };

  validateLaunch = async() => {
    if (!this.hasLaunchParams()) {
      this.setState({launchInfo: null, launchError: "", returnUrl: ""});
      return true;
    }

    this.setState({launchLoading: true, launchError: ""});
    try {
      const res = await UserBackend.getIdentityVerificationLaunch(this.getLaunchParams());
      if (res.status !== "ok") {
        this.setState({launchInfo: null, launchError: res.msg || "子应用实名认证链接校验失败"});
        return false;
      }
      this.setState({launchInfo: res.data, returnUrl: ""}, () => {
        if (this.isApproved(this.state.identity)) {
          this.scheduleReturnToApplication(this.state.identity);
        }
      });
      return true;
    } catch (error) {
      this.setState({launchInfo: null, launchError: error.message});
      return false;
    } finally {
      this.setState({launchLoading: false});
    }
  };

  refreshSelf = () => {
    this.setState({loading: true});
    UserBackend.getIdentityVerification()
      .then((res) => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg);
          this.setState({loading: false});
          return;
        }

        const identity = res.data;
        this.setState({identity}, () => {
          if (this.isApproved(identity) && this.state.launchInfo !== null) {
            this.scheduleReturnToApplication(identity);
          }
        });

        if (!this.isApproved(identity) && !this.hasLaunchParams()) {
          UserBackend.getUser(this.props.account.owner, this.props.account.name)
            .then((userRes) => {
              if (userRes.status === "ok" && userRes.data) {
                this.setState({
                  form: {
                    idCardType: userRes.data.idCardType || identity?.idCardType || "CN_ID",
                    idCard: userRes.data.idCard || "",
                    realName: userRes.data.realName || identity?.realName || "",
                  },
                });
              }
            })
            .finally(() => this.setState({loading: false}));
        } else if (!this.isApproved(identity)) {
          this.setState({
            form: {
              idCardType: identity?.idCardType || "CN_ID",
              idCard: "",
              realName: identity?.realName || "",
            },
          });
          this.setState({loading: false});
        } else {
          this.setState({loading: false});
        }
      })
      .catch((error) => {
        Setting.showMessage("error", error.message);
        this.setState({loading: false});
      });
  };

  submitIdentity = () => {
    const idCardType = this.state.form.idCardType.trim();
    const idCard = idCardType === "CN_ID" ? this.normalizeChineseIdCard(this.state.form.idCard) : this.state.form.idCard.trim();
    const realName = this.state.form.realName.trim();
    if (!this.validateIdentityForm(idCardType, idCard, realName)) {
      return;
    }
    this.setState(prevState => ({
      form: {
        ...prevState.form,
        idCard,
        realName,
      },
    }));

    this.setState({verifying: true});
    const target = getIdentityVerificationSubmitTarget(this.state.launchInfo, this.props.account);
    UserBackend.submitIdentityVerification(target.owner, target.name, idCardType, idCard, realName)
      .then((res) => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg);
          return;
        }

        const identity = res.data;
        Setting.showMessage("success", identity?.status === IdentityStatus.approved ? "实名认证已通过" : "实名认证资料已提交");
        this.setState({identity});
        this.scheduleReturnToApplication(identity);
      })
      .catch(() => null)
      .finally(() => this.setState({verifying: false}));
  };

  buildReturnUrl(identity) {
    const launchInfo = this.state.launchInfo;
    if (!launchInfo?.redirectUri) {
      return "";
    }

    const url = new URL(launchInfo.redirectUri, window.location.origin);
    if (launchInfo.state) {
      url.searchParams.set("state", launchInfo.state);
    }
    url.searchParams.set("status", identity?.status || "");
    url.searchParams.set("isVerified", String(Boolean(identity?.isVerified)));
    url.searchParams.set("ageChecked", String(Boolean(identity?.ageChecked)));
    url.searchParams.set("isOver16", String(Boolean(identity?.isOver16)));
    return url.toString();
  }

  scheduleReturnToApplication(identity) {
    const returnUrl = this.buildReturnUrl(identity);
    if (returnUrl === "") {
      return;
    }

    this.setState({returnUrl});
    if (this.returnTimer !== null) {
      clearTimeout(this.returnTimer);
    }
    this.returnTimer = setTimeout(() => Setting.goToLink(returnUrl), 1800);
  }

  renderLaunchNotice() {
    if (!this.hasLaunchParams()) {
      return null;
    }
    if (this.state.launchLoading) {
      return <Alert style={{marginBottom: 16}} type="info" showIcon message="正在校验子应用实名认证链接" />;
    }
    if (this.state.launchError !== "") {
      return <Alert style={{marginBottom: 16}} type="error" showIcon message="子应用实名认证链接无效" description={this.state.launchError} />;
    }
    if (this.state.returnUrl !== "") {
      return (
        <Alert
          style={{marginBottom: 16}}
          type="success"
          showIcon
          message="实名认证结果已生成"
          description="页面将自动返回子应用。"
          action={<Button size="small" icon={<ExportOutlined />} onClick={() => Setting.goToLink(this.state.returnUrl)}>返回应用</Button>}
        />
      );
    }
    if (this.state.launchInfo !== null) {
      return <Alert style={{marginBottom: 16}} type="info" showIcon message="来自子应用的实名认证请求" description={`应用：${this.state.launchInfo.application}`} />;
    }
    return null;
  }

  renderReadonlyIdentity() {
    const identity = this.state.identity;
    const readonlyItems = [
      {key: "realName", label: "真实姓名", children: identity?.realName || "-"},
      {key: "idCardType", label: "证件类型", children: identity?.idCardType || "-"},
      {key: "maskedIdCard", label: "身份证号", children: identity?.maskedIdCard || "-"},
      {key: "status", label: "审核状态", children: this.getStatusTag(identity?.status, identity?.isVerified)},
      {key: "age", label: "年龄状态", children: this.getAgeTag(identity)},
      {key: "submittedTime", label: "提交时间", children: identity?.submittedTime || "-"},
      {key: "reviewedTime", label: "审核时间", children: identity?.reviewedTime || "-"},
      {key: "reason", label: "审核说明", children: identity?.reason || "-"},
    ];

    return <Descriptions bordered size="small" column={1} items={readonlyItems} />;
  }

  renderForm() {
    const identity = this.state.identity;
    return (
      <div style={{width: "100%"}}>
        <div style={{display: "grid", gridTemplateColumns: "1fr", gap: 10}}>
          <Text type="secondary">证件类型</Text>
          <Input value={this.state.form.idCardType} placeholder="CN_ID" onChange={e => this.updateFormField("idCardType", e.target.value)} />
          <Text type="secondary">身份证号</Text>
          <Input value={this.state.form.idCard} maxLength={18} onChange={e => this.updateFormField("idCard", this.normalizeChineseIdCardInput(e.target.value))} />
          <Text type="secondary">真实姓名</Text>
          <Input value={this.state.form.realName} maxLength={30} onChange={e => this.updateFormField("realName", e.target.value)} />
          <Space wrap style={{marginTop: 6}}>
            <Button block={Setting.isMobile()} type="primary" icon={<CheckCircleOutlined />} loading={this.state.verifying || this.state.loading || this.state.launchLoading} onClick={this.submitIdentity}>
              {identity?.status === IdentityStatus.rejected ? "重新提交" : "提交认证"}
            </Button>
            <Button block={Setting.isMobile()} icon={<ReloadOutlined />} onClick={this.refreshSelf} loading={this.state.loading}>
              刷新
            </Button>
          </Space>
        </div>
      </div>
    );
  }

  render() {
    const identity = this.state.identity;
    const panelClassName = Setting.isDarkTheme(this.props.themeAlgorithm) ? "login-panel-dark" : "login-panel";
    return (
      <div className="login-content" style={{margin: "auto", width: "100%", maxWidth: Setting.isMobile() ? "calc(100vw - 32px)" : 460}}>
        <div className={panelClassName} style={{width: "100%", display: "block"}}>
          <div className="login-form" style={{padding: Setting.isMobile() ? 24 : 32, textAlign: "left"}}>
            <Space style={{marginBottom: 8}}>
              <CheckCircleOutlined />
              <span style={{fontSize: 18, fontWeight: 600}}>实名认证</span>
            </Space>
            <div style={{marginBottom: 18}}>
              <Text type="secondary">请填写本人真实身份信息，认证通过后将同步给已接入应用。</Text>
            </div>
            {this.renderLaunchNotice()}
            {this.isApproved(identity) ? this.renderReadonlyIdentity() : this.renderForm()}
          </div>
        </div>
      </div>
    );
  }
}

export default IdentityVerificationPage;
