import React from "react";
import {ApiOutlined, ArrowRightOutlined, PictureOutlined, PlaySquareOutlined} from "@ant-design/icons";

export default function CustomLoginShell(props) {
  const {application, children, mode = "login", nativeSsoOverlay, sidePanels, showSidePanels} = props;
  const themeColor = application?.themeData?.colorPrimary || "#2563eb";
  const cardTitle = mode === "signup" ? "创建账户" : "欢迎回来";
  const cardDescription = mode === "signup" ? "注册您的企业账户以开始" : "登录您的企业账户以继续";
  const modeClass = mode === "signup" ? " custom-login-shell-signup" : "";
  const companyName = "成都格品科技有限公司";

  return (
    <React.Fragment>
      {nativeSsoOverlay}
      <div className={`custom-login-shell${modeClass}`} style={{"--custom-login-primary": themeColor}}>
        <div className="custom-login-ambient custom-login-ambient-one" />
        <div className="custom-login-ambient custom-login-ambient-two" />
        <div className="custom-login-layout">
          <section className="custom-login-brand">
            <div className="custom-login-brand-header">
              <div className="custom-login-brand-logo">
                <img src="/gepin-logo.ico" alt={companyName} />
              </div>
              <h2>{companyName}</h2>
            </div>

            <h1>
              AI 智能服务
              <br />
              一站式解决方案
            </h1>

            <p className="custom-login-copy">
              提供专业的 AI 模型接口中转、图片生成与视频生成服务。安全稳定，开箱即用，助力您的创意项目快速落地。
            </p>

            <div className="custom-login-features">
              <div className="custom-login-feature custom-login-feature-blue">
                <div className="custom-login-feature-icon">
                  <ApiOutlined />
                </div>
                <div>
                  <h3>Token 中转站</h3>
                  <p>统一管理多个 AI 模型 API，支持 OpenAI、Claude、Gemini 等主流模型接口中转</p>
                </div>
              </div>
              <div className="custom-login-feature custom-login-feature-purple">
                <div className="custom-login-feature-icon">
                  <PictureOutlined />
                </div>
                <div>
                  <h3>Image-2 图片生成中心</h3>
                  <p>强大的 AI 图片生成能力，支持文生图、图生图，提供多种风格和尺寸选项</p>
                </div>
              </div>
              <div className="custom-login-feature custom-login-feature-pink">
                <div className="custom-login-feature-icon">
                  <PlaySquareOutlined />
                </div>
                <div>
                  <h3>CV 视频生成中心</h3>
                  <p>AI 驱动的视频生成服务，从文字描述到视频创作，让创意触手可及</p>
                </div>
              </div>
            </div>

            <div className="custom-login-metrics" aria-label="service metrics">
              <div>
                <strong>99.9%</strong>
                <span>服务可用性</span>
              </div>
              <i />
              <div>
                <strong>10+</strong>
                <span>AI 模型</span>
              </div>
              <i />
              <div>
                <strong>24/7</strong>
                <span>在线服务</span>
              </div>
            </div>

          </section>

          <main className="custom-login-main">
            <div className={`custom-login-card${mode === "signup" ? " custom-login-card-signup" : ""}`}>
              <div className="custom-login-card-title">
                <h2>{cardTitle}</h2>
                <p>{cardDescription}</p>
              </div>
              {children}
              <div className="custom-login-card-arrow" aria-hidden="true">
                <ArrowRightOutlined />
              </div>
            </div>
            {showSidePanels && sidePanels?.length > 0 ? (
              <div className="custom-login-side-panels">
                {sidePanels}
              </div>
            ) : null}
          </main>
        </div>
      </div>
    </React.Fragment>
  );
}
