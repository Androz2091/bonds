import { useState } from "react";
import {
  Card,
  Typography,
  Button,
  Table,
  Modal,
  Form,
  Input,
  Tag,
  Popconfirm,
  Spin,
  App,
} from "antd";
import { DeleteOutlined, SendOutlined } from "@ant-design/icons";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { api } from "@/api";
import type { InvitationType, APIError, PaginationMeta } from "@/api";
import type { ColumnsType } from "antd/es/table";
import { useDateFormat, formatDate } from "@/utils/dateFormat";
import { usePagination } from "@/hooks/usePagination";

const { Title, Text } = Typography;

export default function Invitations() {
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const { t } = useTranslation();
  const dateFormats = useDateFormat();
  const pagination = usePagination();
  const qk = ["settings", "invitations", pagination.page, pagination.pageSize];
  const invalidateKey = ["settings", "invitations"];

  const { data: invitationsResponse, isLoading } = useQuery({
    queryKey: qk,
    queryFn: async () => {
      const res = await api.invitations.invitationsList(pagination.query);
      return {
        invitations: (res.data ?? []) as InvitationType[],
        meta: res.meta as PaginationMeta | undefined,
      };
    },
  });
  const invitations = invitationsResponse?.invitations ?? [];
  const meta = invitationsResponse?.meta;

  const createMutation = useMutation({
    mutationFn: (values: { email: string }) =>
      api.invitations.invitationsCreate(values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey });
      setOpen(false);
      form.resetFields();
    },
    onError: (e: APIError) => message.error(e.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api.invitations.invitationsDelete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey });
    },
    onError: (e: APIError) => message.error(e.message),
  });

  const columns: ColumnsType<InvitationType> = [
    {
      title: t("invitations.email"),
      dataIndex: "email",
      key: "email",
      render: (email: string) => (
        <Text strong>{email}</Text>
      ),
    },
    {
      title: t("invitations.permission"),
      key: "permission",
      render: () => <Tag>{t("invitations.account_member")}</Tag>,
    },
    {
      title: t("common.type"),
      key: "status",
      render: (_, record) =>
        record.accepted_at ? (
          <Tag color="green">{t("invitations.status.accepted")}</Tag>
        ) : (
          <Tag color="orange">{t("invitations.status.pending")}</Tag>
        ),
    },
    {
      title: t("common.created"),
      dataIndex: "created_at",
      key: "created_at",
      render: (val: string) => (
        <Text type="secondary">{formatDate(val, dateFormats)}</Text>
      ),
    },
    {
      title: "",
      key: "actions",
      render: (_, record) =>
        !record.accepted_at ? (
          <Popconfirm
            title={t("invitations.deleteConfirm")}
            onConfirm={() => deleteMutation.mutate(record.id!)}
          >
            <Button
              type="text"
              size="small"
              danger
              icon={<DeleteOutlined />}
            />
          </Popconfirm>
        ) : null,
    },
  ];

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: 80 }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          marginBottom: 24,
        }}
      >
        <div>
          <Title level={4} style={{ marginBottom: 4 }}>
            {t("invitations.title")}
          </Title>
          <Text type="secondary">
            {t("invitations.description")}
          </Text>
        </div>
        <Button
          type="primary"
          icon={<SendOutlined />}
          onClick={() => setOpen(true)}
          style={{ flexShrink: 0, marginTop: 4 }}
        >
          {t("invitations.invite")}
        </Button>
      </div>

      <Card>
        <Table<InvitationType>
          columns={columns}
          dataSource={invitations}
          rowKey="id"
          pagination={{
            current: pagination.page,
            pageSize: pagination.pageSize,
            total: meta?.total ?? invitations.length,
            onChange: pagination.onChange,
            showSizeChanger: true,
            showTotal: (total) => t("pagination.total", { count: total }),
          }}
        />
      </Card>

      <Modal
        title={t("invitations.invite")}
        open={open}
        onCancel={() => {
          setOpen(false);
          form.resetFields();
        }}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={(v) => createMutation.mutate(v)}
        >
          <Form.Item
            name="email"
            label={t("invitations.email")}
            rules={[{ required: true, type: "email" }]}
          >
            <Input placeholder={t("invitations.email")} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
