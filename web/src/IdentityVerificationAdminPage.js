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
import {Button, Input, Popconfirm, Result, Select, Space, Table, Tag} from "antd";
import {SearchOutlined, UserOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as Setting from "./Setting";
import * as UserBackend from "./backend/UserBackend";

const {Option} = Select;
const {TextArea} = Input;

const IdentityStatus = {
  unsubmitted: "unsubmitted",
  pending: "pending",
  approved: "approved",
  rejected: "rejected",
};

class IdentityVerificationAdminPage extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      data: [],
      loading: false,
      pagination: {
        current: 1,
        pageSize: 10,
      },
      searchField: "name",
      searchValue: "",
      statusFilter: "",
      reviewReasons: {},
      sortField: "",
      sortOrder: "",
    };
  }

  componentDidMount() {
    if (this.isAdmin()) {
      this.fetchData({pagination: this.state.pagination});
    }
  }

  componentDidUpdate(prevProps) {
    if (prevProps.account !== this.props.account && this.isAdmin()) {
      this.fetchData({pagination: this.state.pagination});
    }
  }

  isAdmin() {
    return Setting.isLocalAdminUser(this.props.account);
  }

  getAdminOwner() {
    if (Setting.isDefaultOrganizationSelected(this.props.account)) {
      return "";
    }
    return Setting.getRequestOrganization(this.props.account);
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

  fetchData = (params = {}) => {
    const pagination = params.pagination || this.state.pagination;
    const sortField = params.sortField ?? this.state.sortField;
    const sortOrder = params.sortOrder ?? this.state.sortOrder;
    this.setState({loading: true});
    UserBackend.getIdentityVerifications(
      this.getAdminOwner(),
      pagination.current,
      pagination.pageSize,
      this.state.searchValue === "" ? "" : this.state.searchField,
      this.state.searchValue,
      sortField,
      sortOrder,
      "",
      this.state.statusFilter
    ).then((res) => {
      this.setState({loading: false});
      if (res.status !== "ok") {
        Setting.showMessage("error", res.msg);
        return;
      }
      this.setState({
        data: res.data || [],
        pagination: {
          ...pagination,
          total: res.data2,
        },
        sortField,
        sortOrder,
      });
    });
  };

  handleTableChange = (pagination, filters, sorter) => {
    this.fetchData({
      pagination,
      sortField: sorter?.field || "",
      sortOrder: sorter?.order || "",
    });
  };

  search = () => {
    this.fetchData({
      pagination: {
        ...this.state.pagination,
        current: 1,
      },
    });
  };

  resetIdentity = (record) => {
    UserBackend.resetIdentityVerification(record.owner, record.name, record.userId)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", "已重置认证状态");
          this.fetchData({pagination: this.state.pagination});
        } else {
          Setting.showMessage("error", res.msg);
        }
      });
  };

  updateReviewReason(record, value) {
    const key = record.userId || `${record.owner}/${record.name}`;
    this.setState(prevState => ({
      reviewReasons: {
        ...prevState.reviewReasons,
        [key]: value,
      },
    }));
  }

  reviewIdentity = (record, status) => {
    const key = record.userId || `${record.owner}/${record.name}`;
    const reason = (this.state.reviewReasons[key] || "").trim();
    if (status === IdentityStatus.rejected && reason === "") {
      Setting.showMessage("error", "请填写驳回原因");
      return;
    }

    UserBackend.reviewIdentityVerification(record.owner, record.name, record.userId, status, reason)
      .then((res) => {
        if (res.status === "ok") {
          Setting.showMessage("success", status === IdentityStatus.approved ? "已通过实名认证" : "已驳回实名认证");
          this.updateReviewReason(record, "");
          this.fetchData({pagination: this.state.pagination});
        } else {
          Setting.showMessage("error", res.msg);
        }
      });
  };

  renderToolbar() {
    return (
      <div style={{display: "flex", justifyContent: "space-between", gap: 12, flexWrap: "wrap", marginBottom: 16}}>
        <Space>
          <UserOutlined />
          <span style={{fontSize: 16, fontWeight: 600}}>实名认证管理</span>
        </Space>
        <Space wrap>
          <Select virtual={false} style={{width: 120}} value={this.state.searchField} onChange={value => this.setState({searchField: value})}>
            <Option value="name">用户名</Option>
            <Option value="displayName">显示名</Option>
            <Option value="realName">真实姓名</Option>
            <Option value="phone">手机号</Option>
            <Option value="email">邮箱</Option>
          </Select>
          <Input style={{width: 200}} value={this.state.searchValue} allowClear onChange={e => this.setState({searchValue: e.target.value})} onPressEnter={this.search} />
          <Select virtual={false} style={{width: 120}} value={this.state.statusFilter} onChange={value => this.setState({statusFilter: value}, this.search)}>
            <Option value="">全部状态</Option>
            <Option value={IdentityStatus.unsubmitted}>未提交</Option>
            <Option value={IdentityStatus.pending}>待审核</Option>
            <Option value={IdentityStatus.approved}>已通过</Option>
            <Option value={IdentityStatus.rejected}>已驳回</Option>
          </Select>
          <Button type="primary" icon={<SearchOutlined />} onClick={this.search}>筛选</Button>
        </Space>
      </div>
    );
  }

  render() {
    if (!this.isAdmin()) {
      return <Result status="403" title="无权限" subTitle="只有管理员可以管理实名认证" />;
    }

    const columns = [
      {
        title: "用户",
        dataIndex: "name",
        key: "name",
        sorter: true,
        render: (text, record) => `${record.owner}/${record.name}`,
      },
      {
        title: "显示名",
        dataIndex: "displayName",
        key: "displayName",
        sorter: true,
        render: text => text || "-",
      },
      {
        title: "真实姓名",
        dataIndex: "realName",
        key: "realName",
        sorter: true,
        render: text => text || "-",
      },
      {
        title: "身份证号",
        dataIndex: "maskedIdCard",
        key: "maskedIdCard",
        render: text => text || "-",
      },
      {
        title: "实名状态",
        dataIndex: "status",
        key: "status",
        sorter: true,
        render: (value, record) => this.getStatusTag(value, record.isVerified),
      },
      {
        title: "年龄状态",
        dataIndex: "isOver16",
        key: "isOver16",
        render: (value, record) => this.getAgeTag(record),
      },
      {
        title: "审核说明",
        dataIndex: "reason",
        key: "reason",
        width: 180,
        render: text => text || "-",
      },
      {
        title: i18next.t("general:Action"),
        key: "action",
        fixed: Setting.isMobile() ? false : "right",
        render: (text, record) => {
          const key = record.userId || `${record.owner}/${record.name}`;
          return (
            <Space wrap>
              <Button icon={<UserOutlined />} onClick={() => this.props.history.push(`/users/${record.owner}/${record.name}`)}>
                查看用户
              </Button>
              <Button type="primary" disabled={record.status === IdentityStatus.approved && record.isVerified} onClick={() => this.reviewIdentity(record, IdentityStatus.approved)}>
                通过
              </Button>
              <TextArea
                rows={1}
                style={{width: 180}}
                placeholder="驳回原因"
                value={this.state.reviewReasons[key] || ""}
                onChange={e => this.updateReviewReason(record, e.target.value)}
              />
              <Button danger disabled={record.status === IdentityStatus.rejected} onClick={() => this.reviewIdentity(record, IdentityStatus.rejected)}>
                驳回
              </Button>
              <Popconfirm title={`确认重置 ${record.owner}/${record.name} 的实名认证状态？`} onConfirm={() => this.resetIdentity(record)}>
                <Button danger disabled={record.status === IdentityStatus.unsubmitted && !record.isVerified}>重置</Button>
              </Popconfirm>
            </Space>
          );
        },
      },
    ];

    const paginationProps = {
      total: this.state.pagination.total,
      showQuickJumper: true,
      showSizeChanger: true,
      showTotal: () => i18next.t("general:{total} in total").replace("{total}", this.state.pagination.total || 0),
    };

    return (
      <section style={{padding: "24px"}}>
        {this.renderToolbar()}
        <Table
          scroll={{x: "max-content"}}
          columns={columns}
          dataSource={this.state.data}
          rowKey={(record) => record.userId || `${record.owner}/${record.name}`}
          pagination={paginationProps}
          loading={this.state.loading}
          onChange={this.handleTableChange}
          size="middle"
          bordered
        />
      </section>
    );
  }
}

export default IdentityVerificationAdminPage;
