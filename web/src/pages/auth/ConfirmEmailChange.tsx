import { useState } from "react";
import { useSearchParams, Link } from "react-router-dom";
import { App, Button, Card, Typography } from "antd";
import { useTranslation } from "react-i18next";
import { api } from "@/api";
import type { APIError } from "@/api";

export default function ConfirmEmailChange() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [params] = useSearchParams();
  const [done, setDone] = useState(false);
  const [saving, setSaving] = useState(false);
  const token = params.get("token") ?? "";

  async function confirm() {
    setSaving(true);
    try {
      await api.auth.confirmEmailChangeCreate({ token });
      setDone(true);
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
        title={t("emailChange.title")}
        style={{ width: "100%", maxWidth: 420 }}
      >
        {done ? (
          <Typography.Paragraph>{t("emailChange.done")}</Typography.Paragraph>
        ) : (
          <Button
            type="primary"
            block
            disabled={!token}
            loading={saving}
            onClick={confirm}
          >
            {t("emailChange.confirm")}
          </Button>
        )}
        <Link to="/login">{t("emailChange.login")}</Link>
      </Card>
    </div>
  );
}
