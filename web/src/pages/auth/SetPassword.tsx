import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { App, Button, Card, Form, Input, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { api } from "@/api";
import type { APIError } from "@/api";

export default function SetPassword() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [saving, setSaving] = useState(false);
  const token = params.get("token") ?? "";

  async function submit(values: { password: string; confirm: string }) {
    setSaving(true);
    try {
      await api.auth.setPasswordCreate({ token, password: values.password });
      message.success(t("setPassword.success"));
      navigate("/login", { replace: true });
    } catch (error) {
      message.error((error as APIError).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "grid",
        placeItems: "center",
        padding: 16,
      }}
    >
      <Card
        style={{ width: "100%", maxWidth: 420 }}
        title={t("setPassword.title")}
      >
        {!token ? (
          <Typography.Text type="danger">
            {t("setPassword.invalid")}
          </Typography.Text>
        ) : (
          <Form layout="vertical" onFinish={submit}>
            <Form.Item
              name="password"
              label={t("setPassword.password")}
              rules={[{ required: true, min: 8 }]}
            >
              <Input.Password autoComplete="new-password" />
            </Form.Item>
            <Form.Item
              name="confirm"
              label={t("setPassword.confirm")}
              dependencies={["password"]}
              rules={[
                { required: true },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    return value === getFieldValue("password")
                      ? Promise.resolve()
                      : Promise.reject(new Error(t("setPassword.mismatch")));
                  },
                }),
              ]}
            >
              <Input.Password autoComplete="new-password" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saving} block>
              {t("setPassword.submit")}
            </Button>
          </Form>
        )}
      </Card>
    </div>
  );
}
