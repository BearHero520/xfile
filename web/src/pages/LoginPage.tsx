import {
  ArrowLeft20Regular,
  CheckmarkCircle20Regular,
  LockClosed20Regular,
  Person20Regular,
  ShieldCheckmark20Regular,
} from "@fluentui/react-icons";
import { FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../api";
import AppearanceControl from "../components/AppearanceControl";
import BrandMark from "../components/BrandMark";
import { Button, Field, Loading } from "../components/ui";
import { useApp, useToast } from "../state";

export default function LoginPage() {
  const { session, site, loading, refresh } = useApp();
  const { show } = useToast();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [captchaID, setCaptchaID] = useState("");
  const [captchaQuestion, setCaptchaQuestion] = useState("");
  const [captchaAnswer, setCaptchaAnswer] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const setupMode = session?.initialized === false;

  useEffect(() => {
    if (session?.authenticated)
      navigate(params.get("redirect") || "/", { replace: true });
  }, [session?.authenticated, navigate, params]);

  useEffect(() => {
    if (!session?.captchaRequired) return;
    api
      .challenge()
      .then((challenge) => {
        if (challenge.required) {
          setCaptchaID(challenge.id || "");
          setCaptchaQuestion(challenge.question || "");
        }
      })
      .catch(() => undefined);
  }, [session?.captchaRequired]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!username.trim() || password.length < 6) {
      show("请输入用户名和至少 6 位密码", "error");
      return;
    }
    if (setupMode && password !== confirmPassword) {
      show("两次输入的密码不一致", "error");
      return;
    }
    setSubmitting(true);
    try {
      if (setupMode) await api.setup(username.trim(), password);
      else
        await api.login({
          username: username.trim(),
          password,
          captchaID,
          captchaAnswer,
        });
      await refresh();
      show(setupMode ? "XFile 初始化完成" : "欢迎回来", "success");
      navigate(params.get("redirect") || "/", { replace: true });
    } catch (error) {
      show(error instanceof Error ? error.message : "登录失败", "error");
      if (!setupMode && session?.captchaRequired) {
        const challenge = await api.challenge().catch(() => null);
        setCaptchaID(challenge?.id || "");
        setCaptchaQuestion(challenge?.question || "");
        setCaptchaAnswer("");
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (loading)
    return (
      <div className="fullscreen-center">
        <Loading />
      </div>
    );
  return (
    <div className="login-page">
      <div className="login-ambient" />
      <div className="login-appearance">
        <AppearanceControl />
      </div>
      <Link to="/" className="back-link">
        <ArrowLeft20Regular />
        返回文件页
      </Link>
      <section className="login-card glass">
        <div className="login-brand">
          <BrandMark />
          <div>
            <strong>XFile</strong>
            <small>{site?.siteName || "文件工作台"}</small>
          </div>
        </div>
        <div className="login-heading">
          <span className="login-icon">
            <ShieldCheckmark20Regular />
          </span>
          <h1>{setupMode ? "初始化工作区" : "登录管理后台"}</h1>
          <p>
            {setupMode
              ? "创建首个超级管理员，随后即可配置存储源。"
              : "使用 XFile 账号访问文件管理与系统设置。"}
          </p>
        </div>
        <form onSubmit={submit} className="login-form">
          <Field label="用户名">
            <div className="input-with-icon">
              <Person20Regular />
              <input
                autoFocus
                autoComplete="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder="请输入用户名"
              />
            </div>
          </Field>
          <Field label="密码">
            <div className="input-with-icon">
              <LockClosed20Regular />
              <input
                type="password"
                autoComplete={setupMode ? "new-password" : "current-password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="至少 6 位"
              />
            </div>
          </Field>
          {setupMode && (
            <Field label="确认密码">
              <div className="input-with-icon">
                <CheckmarkCircle20Regular />
                <input
                  type="password"
                  autoComplete="new-password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                />
              </div>
            </Field>
          )}
          {!setupMode && captchaQuestion && (
            <Field label={`安全验证：${captchaQuestion}`}>
              <input
                value={captchaAnswer}
                onChange={(event) => setCaptchaAnswer(event.target.value)}
                placeholder="请输入答案"
              />
            </Field>
          )}
          <Button variant="primary" disabled={submitting} type="submit">
            {submitting ? "正在验证…" : setupMode ? "创建并进入 XFile" : "登录"}
          </Button>
        </form>
        <div className="login-security">
          <LockClosed20Regular />
          <span>登录会话使用 HttpOnly Cookie，并启用 CSRF 防护。</span>
        </div>
      </section>
    </div>
  );
}
