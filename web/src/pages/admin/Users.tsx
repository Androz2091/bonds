import {
  Card,
  Typography,
  Table,
  Tag,
  Button,
  App,
  Popconfirm,
  Space,
  Spin,
  Segmented,
  InputNumber,
  Form,
  Input,
  Modal,
} from "antd";
import {
  CrownOutlined,
  DeleteOutlined,
  StopOutlined,
  CheckCircleOutlined,
  SettingOutlined,
  TeamOutlined,
  DatabaseOutlined,
  KeyOutlined,
  CloudOutlined,
} from "@ant-design/icons";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { api } from "@/api";
import type { AdminUser, APIError, PaginationMeta } from "@/api";
import type { GithubComNaibaBondsInternalDtoAuditEventResponse as AuditEvent } from "@/api/generated/data-contracts";
import { useAuth } from "@/stores/auth";
import { filesize } from "filesize";
import type { ColumnsType } from "antd/es/table";
import { formatContactName, useNameOrder } from "@/utils/nameFormat";
import { useDateFormat, formatDate } from "@/utils/dateFormat";
import { useState } from "react";
import { usePagination } from "@/hooks/usePagination";

const { Title, Text } = Typography;

type AdminIdentityValues = {
  first_name: string;
  last_name: string;
  email: string;
};

function EditAdminIdentityForm({
  user,
  saving,
  onSave,
  onCancel,
}: {
  user: AdminUser;
  saving: boolean;
  onSave: (values: AdminIdentityValues) => Promise<void>;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const [form] = Form.useForm<AdminIdentityValues>();
  return (
    <Form
      form={form}
      layout="vertical"
      initialValues={{
        first_name: user.first_name,
        last_name: user.last_name,
        email: user.email,
      }}
      onFinish={onSave}
    >
      <Form.Item
        label={t("admin.users.first_name")}
        name="first_name"
        rules={[{ required: true }]}
      >
        <Input />
      </Form.Item>
      <Form.Item label={t("admin.users.last_name")} name="last_name">
        <Input />
      </Form.Item>
      <Form.Item
        label={t("admin.users.email")}
        name="email"
        rules={[{ required: true, type: "email" }]}
      >
        <Input />
      </Form.Item>
      <Space style={{ display: "flex", justifyContent: "flex-end" }}>
        <Button onClick={onCancel}>{t("common.cancel")}</Button>
        <Button type="primary" htmlType="submit" loading={saving}>
          {t("common.save")}
        </Button>
      </Space>
    </Form>
  );
}

export default function AdminUsers() {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const { user: currentUser } = useAuth();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const nameOrder = useNameOrder();
  const dateFormats = useDateFormat();
  const pagination = usePagination();
  const qk = ["admin", "users", pagination.page, pagination.pageSize];
  const invalidateKey = ["admin", "users"];
  const [storageLimitModalUser, setStorageLimitModalUser] =
    useState<AdminUser | null>(null);
  const [storageLimitValue, setStorageLimitValue] = useState<number>(0);
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm] = Form.useForm();
  const [editUser, setEditUser] = useState<AdminUser | null>(null);
  const [auditPage, setAuditPage] = useState(1);
  const { data: auditResponse, isLoading: auditLoading } = useQuery({
    queryKey: ["admin", "audit", auditPage],
    queryFn: async (): Promise<{
      events: AuditEvent[];
      meta?: PaginationMeta;
    }> => {
      const result = await api.admin.auditList({
        page: auditPage,
        per_page: 25,
      });
      return {
        events: result.data ?? [],
        meta: result.meta as PaginationMeta | undefined,
      };
    },
  });

  const { data: usersResponse, isLoading } = useQuery({
    queryKey: qk,
    queryFn: async () => {
      const res = await api.admin.usersList(pagination.query);
      return {
        users: (res.data ?? []) as AdminUser[],
        meta: res.meta as PaginationMeta | undefined,
      };
    },
  });
  const users = usersResponse?.users ?? [];
  const meta = usersResponse?.meta;

  const toggleMutation = useMutation({
    mutationFn: ({ id, disabled }: { id: string; disabled: boolean }) =>
      api.admin.usersToggleUpdate(id, { disabled }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: invalidateKey });
      message.success(
        variables.disabled
          ? t("admin.users.disabled_success")
          : t("admin.users.enabled"),
      );
    },
    onError: (e: APIError) => message.error(e.message),
  });

  const adminMutation = useMutation({
    mutationFn: ({
      id,
      is_instance_administrator,
    }: {
      id: string;
      is_instance_administrator: boolean;
    }) => api.admin.usersAdminUpdate(id, { is_instance_administrator }),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: invalidateKey });
      message.success(
        variables.is_instance_administrator
          ? t("admin.users.admin_set")
          : t("admin.users.admin_removed"),
      );
    },
    onError: (e: APIError) => message.error(e.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.admin.usersDelete(id),
    onSuccess: (_data, deletedUserID) => {
      queryClient.setQueriesData<{
        users: AdminUser[];
        meta?: PaginationMeta;
      }>({ queryKey: invalidateKey }, (oldData) => {
        if (!oldData) return oldData;
        const users = oldData.users.filter((user) => user.id !== deletedUserID);
        return {
          ...oldData,
          users,
          meta: oldData.meta
            ? {
                ...oldData.meta,
                total: Math.max(0, (oldData.meta.total ?? users.length) - 1),
              }
            : oldData.meta,
        };
      });
      queryClient.invalidateQueries({ queryKey: invalidateKey });
      message.success(t("admin.users.deleted"));
    },
    onError: (e: APIError) => message.error(e.message),
  });

  const storageLimitMutation = useMutation({
    mutationFn: ({
      id,
      storage_limit_in_mb,
    }: {
      id: string;
      storage_limit_in_mb: number;
    }) => api.admin.usersStorageLimitUpdate(id, { storage_limit_in_mb }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey });
      message.success(t("admin.users.storage_limit_updated"));
      setStorageLimitModalUser(null);
    },
    onError: (e: APIError) => message.error(e.message),
  });
  const createMutation = useMutation({
    mutationFn: (values: {
      email: string;
      first_name: string;
      last_name?: string;
    }) => api.admin.usersCreate(values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey });
      setCreateOpen(false);
      createForm.resetFields();
      message.success(t("admin.users.created_email"));
    },
    onError: (error: APIError) => message.error(error.message),
  });
  const resetPasswordMutation = useMutation({
    mutationFn: (id: string) => api.admin.usersResetPasswordCreate(id),
    onSuccess: () => message.success(t("admin.users.reset_sent")),
    onError: (error: APIError) => message.error(error.message),
  });
  const identityMutation = useMutation({
    mutationFn: (input: {
      id: string;
      first_name: string;
      last_name: string;
    }) =>
      api.admin.usersIdentityUpdate(input.id, {
        first_name: input.first_name,
        last_name: input.last_name,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey });
      message.success(t("common.saved"));
    },
    onError: (error: APIError) => message.error(error.message),
  });
  const emailMutation = useMutation({
    mutationFn: (input: { id: string; email: string }) =>
      api.admin.usersEmailChangeCreate(input.id, { email: input.email }),
    onSuccess: () => message.success(t("admin.users.email_confirmation_sent")),
    onError: (error: APIError) => message.error(error.message),
  });

  const columns: ColumnsType<AdminUser> = [
    {
      title: t("admin.users.name"),
      key: "name",
      render: (_: unknown, record: AdminUser) =>
        formatContactName(nameOrder, record),
    },
    {
      title: t("admin.users.email"),
      dataIndex: "email",
      key: "email",
    },
    {
      title: t("admin.users.storage"),
      key: "storage",
      width: 160,
      render: (_: unknown, record: AdminUser) => {
        const limitMB = record.storage_limit_in_mb ?? 0;
        // storage_limit_in_mb=0 表示使用实例默认限制
        const limitStr =
          limitMB > 0
            ? (filesize(limitMB * 1024 * 1024, { standard: "jedec" }) as string)
            : t("admin.users.instance_default");
        return (
          <Space direction="vertical" size={0}>
            <span>{limitStr}</span>
            <Button
              type="link"
              size="small"
              style={{ padding: 0, height: "auto" }}
              icon={<CloudOutlined />}
              onClick={() => {
                setStorageLimitModalUser(record);
                setStorageLimitValue(limitMB);
              }}
            >
              {t("admin.users.set_storage_limit")}
            </Button>
          </Space>
        );
      },
    },
    {
      title: t("admin.users.role"),
      key: "role",
      width: 100,
      render: (_: unknown, record: AdminUser) =>
        record.is_instance_administrator ? (
          <Tag color="gold" icon={<CrownOutlined />}>
            {t("admin.users.admin")}
          </Tag>
        ) : (
          <Tag>{t("admin.users.user")}</Tag>
        ),
    },
    {
      title: t("admin.users.status"),
      key: "status",
      width: 100,
      render: (_: unknown, record: AdminUser) =>
        record.disabled ? (
          <Tag color="error">{t("admin.users.disabled")}</Tag>
        ) : (
          <Tag color="success">{t("admin.users.active")}</Tag>
        ),
    },
    {
      title: t("admin.users.joined"),
      dataIndex: "created_at",
      key: "created_at",
      width: 160,
      render: (v: string) => (v ? formatDate(v, dateFormats) : "-"),
    },
    {
      title: t("admin.users.actions"),
      key: "actions",
      width: 240,
      render: (_: unknown, record: AdminUser) => {
        const isSelf = record.id === currentUser?.id;
        return (
          <Space size="small">
            {!isSelf && (
              <>
                <Button
                  type="text"
                  size="small"
                  onClick={() => setEditUser(record)}
                >
                  {t("common.edit")}
                </Button>
                <Button
                  type="text"
                  size="small"
                  onClick={() =>
                    record.id && resetPasswordMutation.mutate(record.id)
                  }
                >
                  {t("admin.users.reset_password")}
                </Button>
                <Button
                  type="text"
                  size="small"
                  icon={
                    record.disabled ? <CheckCircleOutlined /> : <StopOutlined />
                  }
                  onClick={() =>
                    toggleMutation.mutate({
                      id: record.id!,
                      disabled: !record.disabled,
                    })
                  }
                >
                  {record.disabled
                    ? t("admin.users.enable")
                    : t("admin.users.disable")}
                </Button>
                <Button
                  type="text"
                  size="small"
                  icon={<CrownOutlined />}
                  onClick={() =>
                    adminMutation.mutate({
                      id: record.id!,
                      is_instance_administrator:
                        !record.is_instance_administrator,
                    })
                  }
                >
                  {record.is_instance_administrator
                    ? t("admin.users.remove_admin")
                    : t("admin.users.set_admin")}
                </Button>
                <Popconfirm
                  title={t("admin.users.delete_confirm")}
                  onConfirm={() => deleteMutation.mutate(record.id!)}
                >
                  <Button
                    type="text"
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                  />
                </Popconfirm>
              </>
            )}
          </Space>
        );
      },
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
    <div style={{ maxWidth: 1100, margin: "0 auto" }}>
      <Segmented
        value="users"
        onChange={(val) => {
          if (val === "settings") navigate("/admin/settings");
          if (val === "backups") navigate("/admin/backups");
          if (val === "oauth-providers") navigate("/admin/oauth-providers");
        }}
        options={[
          {
            label: t("admin.tab_users"),
            value: "users",
            icon: <TeamOutlined />,
          },
          {
            label: t("admin.tab_settings"),
            value: "settings",
            icon: <SettingOutlined />,
          },
          {
            label: t("admin.tab_backups"),
            value: "backups",
            icon: <DatabaseOutlined />,
          },
          {
            label: t("admin.tab_oauth"),
            value: "oauth-providers",
            icon: <KeyOutlined />,
          },
        ]}
        style={{ marginBottom: 24 }}
      />

      <div style={{ marginBottom: 24 }}>
        <Title level={4} style={{ marginBottom: 4 }}>
          <CrownOutlined style={{ marginRight: 8 }} />
          {t("admin.users.title")}
        </Title>
        <Text type="secondary">{t("admin.users.description")}</Text>
        <Button
          type="primary"
          onClick={() => setCreateOpen(true)}
          style={{ marginLeft: 16 }}
        >
          {t("admin.users.create")}
        </Button>
      </div>

      <Modal
        title={t("admin.users.create")}
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => createForm.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Typography.Paragraph type="secondary">
          {t("admin.users.create_hint")}
        </Typography.Paragraph>
        <Form
          form={createForm}
          layout="vertical"
          onFinish={(values: {
            email: string;
            first_name: string;
            last_name?: string;
          }) => createMutation.mutate(values)}
        >
          <Form.Item
            label={t("admin.users.email")}
            name="email"
            rules={[{ required: true, type: "email" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            label={t("admin.users.first_name")}
            name="first_name"
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label={t("admin.users.last_name")} name="last_name">
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t("admin.users.edit_identity")}
        open={!!editUser}
        onCancel={() => setEditUser(null)}
        footer={null}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          {t("admin.users.email_confirmation_hint")}
        </Typography.Paragraph>
        {editUser?.id && (
          <EditAdminIdentityForm
            key={editUser.id}
            user={editUser}
            saving={identityMutation.isPending || emailMutation.isPending}
            onCancel={() => setEditUser(null)}
            onSave={async (values) => {
              if (!editUser?.id) return;
              try {
                await identityMutation.mutateAsync({
                  id: editUser.id,
                  first_name: values.first_name,
                  last_name: values.last_name,
                });
                if (
                  values.email.trim().toLowerCase() !==
                  editUser.email?.toLowerCase()
                ) {
                  await emailMutation.mutateAsync({
                    id: editUser.id,
                    email: values.email,
                  });
                }
                setEditUser(null);
              } catch {
                return;
              }
            }}
          />
        )}
      </Modal>

      <Card>
        <Table
          columns={columns}
          dataSource={users}
          rowKey="id"
          pagination={{
            current: pagination.page,
            pageSize: pagination.pageSize,
            total: meta?.total ?? users.length,
            onChange: pagination.onChange,
            showSizeChanger: true,
            showTotal: (total) => t("pagination.total", { count: total }),
          }}
          size="small"
          scroll={{ x: 900 }}
        />
      </Card>

      <Card title={t("admin.audit.title")} style={{ marginTop: 24 }}>
        <Text type="secondary">{t("admin.audit.description")}</Text>
        <Table<AuditEvent>
          style={{ marginTop: 16 }}
          dataSource={auditResponse?.events ?? []}
          rowKey="id"
          loading={auditLoading}
          columns={[
            {
              title: t("admin.audit.time"),
              dataIndex: "created_at",
              render: (value: string) => formatDate(value, dateFormats),
            },
            {
              title: t("admin.audit.action"),
              key: "action",
              render: (_, row) => `${row.method} ${row.route}`,
            },
            { title: t("admin.audit.status"), dataIndex: "status" },
            {
              title: t("admin.audit.request_id"),
              dataIndex: "request_id",
              ellipsis: true,
            },
          ]}
          pagination={{
            current: auditPage,
            pageSize: 25,
            total: auditResponse?.meta?.total ?? 0,
            onChange: setAuditPage,
          }}
          size="small"
          scroll={{ x: 700 }}
        />
      </Card>

      <Modal
        title={t("admin.users.set_storage_limit")}
        open={!!storageLimitModalUser}
        onCancel={() => setStorageLimitModalUser(null)}
        onOk={() => {
          if (storageLimitModalUser?.id) {
            storageLimitMutation.mutate({
              id: storageLimitModalUser.id,
              storage_limit_in_mb: storageLimitValue,
            });
          }
        }}
        confirmLoading={storageLimitMutation.isPending}
      >
        <Typography.Paragraph type="secondary">
          {t("admin.users.storage_limit_hint")}
        </Typography.Paragraph>
        <InputNumber
          value={storageLimitValue}
          onChange={(v) => setStorageLimitValue(v ?? 0)}
          min={0}
          addonAfter="MB"
          style={{ width: "100%" }}
          placeholder={t("admin.users.storage_limit_placeholder")}
        />
      </Modal>
    </div>
  );
}
