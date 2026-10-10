import { createDatasource, DatasourcePing, updateDatasource } from "../../api/datasource"
import {Form, Input, Button, Checkbox, Alert, Drawer, Steps, Card, Row, Col, Typography, Divider, Radio, Select} from "antd"
import React, { useState, useEffect } from "react"
import {
    MinusCircleOutlined,
    PlusOutlined,
    DatabaseOutlined,
    CloudOutlined,
    ApartmentOutlined, 
} from "@ant-design/icons"
import VSCodeEditor from "../../utils/VSCodeEditor";
import { FaultCenterList } from "../../api/faultCenter";
const { Title, Text } = Typography
const MyFormItemContext = React.createContext([])

function toArr(str) {
    return Array.isArray(str) ? str : [str]
}

const MyFormItemGroup = ({ prefix, children }) => {
    const prefixPath = React.useContext(MyFormItemContext)
    const concatPath = React.useMemo(() => [...prefixPath, ...toArr(prefix)], [prefixPath, prefix])
    return <MyFormItemContext.Provider value={concatPath}>{children}</MyFormItemContext.Provider>
}

const MyFormItem = ({ name, ...props }) => {
    const prefixPath = React.useContext(MyFormItemContext)
    const concatName = name !== undefined ? [...prefixPath, ...toArr(name)] : undefined
    return <Form.Item name={concatName} {...props} />
}

const DEFAULT_ZABBIX_CONFIG = {
    apiToken: "",
    faultCenterId: undefined,
    pollInterval: 15,
    timeout: 10,
    initialLookback: 3600,
    overlap: 300,
    insecureSkipVerify: false,
    severityMapping: {
        "5": "P0",
        "4": "P1",
        "3": "P2",
        "2": "P2",
        "1": "P2",
        "0": "P2",
    },
}

// 数据源类型配置
const datasourceTypes = [
    {
        value: "Prometheus",
        label: "Prometheus",
        icon: <DatabaseOutlined />,
        description: "Prometheus 监控系统和时间序列数据库",
    },
    {
        value: "Zabbix",
        label: "Zabbix",
        icon: <DatabaseOutlined />,
        description: "通过只读 JSON-RPC API 同步问题与恢复事件",
    },
    {
        value: "AliCloudSLS",
        label: "阿里云SLS",
        icon: <CloudOutlined />,
        description: "阿里云日志服务",
    },
    {
        value: "Jaeger",
        label: "Jaeger",
        icon: <ApartmentOutlined />,
        description: "分布式追踪系统",
    },
    {
        value: "Loki",
        label: "Loki",
        icon: <DatabaseOutlined />,
        description: "日志聚合系统",
    },
    {
        value: "CloudWatch",
        label: "CloudWatch",
        icon: <CloudOutlined />,
        description: "AWS 监控和可观测性服务",
    },
    
    {
        value: "Kubernetes",
        label: "Kubernetes",
        icon: <ApartmentOutlined />,
        description: "容器编排平台",
    },
    {
        value: "ElasticSearch",
        label: "ElasticSearch",
        icon: <DatabaseOutlined />,
        description: "分布式搜索和分析引擎",
    },
    {
        value: "VictoriaLogs",
        label: "VictoriaLogs",
        icon: <DatabaseOutlined />,
        description: "轻量级日志分析系统",
    },
    {
        value: "ClickHouse",
        label: "ClickHouse",
        icon: <DatabaseOutlined />,
        description: "高性能列式存储数据库",
    },
]

export const CreateDatasourceModal = ({ visible, onClose, selectedRow, type, handleList }) => {
    const [form] = Form.useForm()
    const [enabled, setEnabled] = useState(true)
    const [selectedType, setSelectedType] = useState(null)
    const [submitLoading, setSubmitLoading] = useState(false)
    const [testLoading, setTestLoading] = useState(false)
    const [currentStep, setCurrentStep] = useState(0)
    const [disableStep, setDisableStep] = useState(false)
    const [authState, setAuthState] = useState("Off")
    const [writeState, setWriteState] = useState("Off")
    const [faultCenterOptions, setFaultCenterOptions] = useState([])
    const [faultCenterLoading, setFaultCenterLoading] = useState(false)

    const handleKeyPress = (e) => {
        if (e.key === " ") {
            e.preventDefault()
        }
    }

    useEffect(() => {
        if (!visible) {
            return
        }

        if (selectedRow) {
            const labelsArray = Object.entries(selectedRow.labels || {}).map(([key, value]) => ({
                key,
                value,
            }));

            // 处理HTTP Headers
            const headersArray = selectedRow.http?.headers ? 
                Object.entries(selectedRow.http.headers).map(([key, value]) => ({
                    key,
                    value,
                })) : [];

            const auth = selectedRow.Auth || selectedRow.auth || {}
            setSelectedType(selectedRow.type)
            setWriteState(selectedRow.write?.enabled || "Off")
            setAuthState(auth.user || auth.pass ? "On" : "Off")
            setEnabled(selectedRow.enabled ?? true)
            form.setFieldsValue({
                name: selectedRow.name,
                type: selectedRow.type,
                labels: labelsArray,
                http: {
                    url: selectedRow.http.url,
                    timeout: Number(selectedRow.http.timeout),
                    headers: headersArray  // 添加headers字段
                },
                write: {
                    enabled: selectedRow.write?.enabled || "Off",
                    url: selectedRow.write?.url,
                },
                auth,
                alicloudEndpoint: selectedRow.alicloudEndpoint,
                alicloudAk: selectedRow.alicloudAk,
                alicloudSk: selectedRow.alicloudSk,
                awsCloudwatch: selectedRow.awsCloudwatch,
                description: selectedRow.description,
                kubeConfig: selectedRow.kubeConfig,
                elasticSearch: selectedRow.elasticSearch,
                clickhouseConfig: {
                    addr: selectedRow.clickhouseConfig?.Addr || "",
                    timeout: selectedRow.clickhouseConfig?.Timeout || 10,
                },
                zabbixConfig: {
                    ...DEFAULT_ZABBIX_CONFIG,
                    ...selectedRow.zabbixConfig,
                    apiToken: "",
                    severityMapping: {
                        ...DEFAULT_ZABBIX_CONFIG.severityMapping,
                        ...selectedRow.zabbixConfig?.severityMapping,
                    },
                },
                enabled: selectedRow.enabled
            })

            // 如果是编辑模式，直接跳到第二步
            setCurrentStep(1)
            setDisableStep(true)
        } else {
            form.resetFields()
            form.setFieldsValue({ zabbixConfig: DEFAULT_ZABBIX_CONFIG })
            setSelectedType(null)
            setEnabled(true)
            setAuthState("Off")
            setWriteState("Off")
            setCurrentStep(0)
            setDisableStep(false)
        }
    }, [visible, selectedRow, form])

    useEffect(() => {
        if (!visible || selectedType !== "Zabbix") {
            return
        }

        const loadFaultCenters = async () => {
            setFaultCenterLoading(true)
            try {
                const res = await FaultCenterList()
                setFaultCenterOptions((res?.data || []).map((item) => ({
                    label: item.name,
                    value: item.id,
                })))
            } finally {
                setFaultCenterLoading(false)
            }
        }

        loadFaultCenters()
    }, [visible, selectedType])

    const handleCreate = async (params) => {
        await createDatasource(params)
        await handleList()
    }

    const handleUpdate = async (params) => {
        await updateDatasource(params)
        await handleList()
    }

    const formatParams = (values) => {
        const formattedLabels = values.labels?.reduce((acc, { key, value }) => {
            if (key) {
                acc[key] = value
            }
            return acc
        }, {})

        const formattedHttpHeaders = values?.http?.headers?.reduce((acc, { key, value }) => {
            if (key) {
                acc[key] = value
            }
            return acc
        }, {})

        const zabbixConfig = selectedType === "Zabbix" ? {
            ...DEFAULT_ZABBIX_CONFIG,
            ...values.zabbixConfig,
            pollInterval: Number(values.zabbixConfig?.pollInterval),
            timeout: Number(values.zabbixConfig?.timeout),
            initialLookback: Number(values.zabbixConfig?.initialLookback),
            overlap: Number(values.zabbixConfig?.overlap),
        } : values.zabbixConfig

        return {
            ...values,
            labels: formattedLabels,
            clickhouseConfig: {
                addr: values?.clickhouseConfig?.addr,
                timeout: Number(values?.clickhouseConfig?.timeout),
            },
            http: {
                url: values?.http?.url,
                timeout: selectedType === "Zabbix"
                    ? Number(zabbixConfig?.timeout)
                    : Number(values?.http?.timeout),
                headers: formattedHttpHeaders,
            },
            zabbixConfig,
            write: {
                enabled: writeState,
                url: values?.write?.url,
            },
            enabled,
        }
    }

    const handleFormSubmit = async (values) => {
        const params = formatParams(values)

        if (type === "create") {
            await handleCreate(params)
        } else {
            params.id = selectedRow.id
            await handleUpdate(params)
        }

        onClose()
    }

    const handleSubmit = async () => {
        setSubmitLoading(true)
        try {
            const values = await form.validateFields()
            await handleFormSubmit(values)
        } catch (error) {
            if (!error?.errorFields) {
                console.error(error)
            }
        } finally {
            setSubmitLoading(false)
        }
    }

    const handleSelectType = (type) => {
        setSelectedType(type)
        form.setFieldsValue({ type })
        setCurrentStep(1)
    }

    const handleTestConnection = async () => {
        setTestLoading(true)
        try {
            // 连接测试只依赖连接参数，不应被故障中心、轮询策略等保存项阻塞。
            if (selectedType === "Zabbix") {
                await form.validateFields([
                    ["http", "url"],
                    ["zabbixConfig", "apiToken"],
                ])
            } else {
                await form.validateFields()
            }
            const values = form.getFieldsValue(true)
            const params = formatParams(values)
            if (type === "update") {
                params.id = selectedRow.id
            }
            await DatasourcePing(params)
        } catch (error) {
            if (!error?.errorFields) {
                console.error("连接测试失败:", error)
            }
        } finally {
            setTestLoading(false)
        }
    }

    const handlePrevStep = () => {
        setCurrentStep(0)
    }

    const radioOptions = [
        {
            label: '认证',
            value: 'On',
        },
        {
            label: '不认证',
            value: 'Off',
        },
    ];

    const writeRadioOptions = [
        {
            label: '启用',
            value: 'On',
        },
        {
            label: '禁用',
            value: 'Off',
        },
    ];


    // 渲染数据源类型选择卡片
    const renderDatasourceTypeCards = () => {
        return (
            <div style={{ padding: "20px 0" }}>
                <Row gutter={[16, 16]}>
                    {datasourceTypes.map((dsType) => (
                        <Col xs={24} sm={12} md={8} key={dsType.value}>
                            <Card
                                hoverable
                                onClick={() => handleSelectType(dsType.value)}
                                style={{
                                    height: "100%",
                                    borderColor: selectedType === dsType.value ? "#1890ff" : undefined,
                                    boxShadow: selectedType === dsType.value ? "0 0 0 2px rgba(24,144,255,0.2)" : undefined,
                                }}
                            >
                                <div style={{ display: "flex", alignItems: "center" }}>
                                    <div
                                        style={{
                                            fontSize: "24px",
                                            marginRight: "12px",
                                            width: "40px",
                                            height: "40px",
                                            display: "flex",
                                            alignItems: "center",
                                            justifyContent: "center",
                                            background: "#f0f5ff",
                                            borderRadius: "8px",
                                            color: "#1890ff",
                                        }}
                                    >
                                        {dsType.icon}
                                    </div>
                                    <div>
                                        <Title level={5} style={{ margin: 0 }}>
                                            {dsType.label}
                                        </Title>
                                        <Text type="secondary">{dsType.description}</Text>
                                    </div>
                                </div>
                            </Card>
                        </Col>
                    ))}
                </Row>
            </div>
        )
    }

    // 渲染数据源配置表单
    const renderDatasourceConfigForm = () => {
        return (
            <Form form={form} name="form_item_path" layout="vertical">
                <MyFormItem
                    name="name"
                    label="数据源名称"
                    rules={[{ required: true }]}
                    normalize={(value) => value?.replace(/\s/g, "")}
                >
                    <Input
                        onKeyPress={handleKeyPress}
                    />
                </MyFormItem>

                <label>标签</label>
                <Alert
                    message="提示：可添加外部标签(external labels), 它将会添加到事件当中用于区分数据来源。"
                    type="info"
                    showIcon
                    style={{ marginBottom: 20, marginTop: "10px" }}
                />

                <Form.List name="labels">
                    {(fields, { add, remove }) => (
                        <>
                            {fields.map(({ key, name, ...restField }) => (
                                <div
                                    key={key}
                                    style={{
                                        display: "flex",
                                        marginBottom: 8,
                                        gap: "8px",
                                        alignItems: "center",
                                    }}
                                >
                                    <Form.Item
                                        {...restField}
                                        name={[name, "key"]}
                                        style={{ flex: 3 }}
                                        rules={[{ required: true, message: "请输入标签键（key）" }]}
                                        normalize={(value) => value?.replace(/\s/g, "")}
                                    >
                                        <Input placeholder="键（key）" />
                                    </Form.Item>

                                    <Form.Item
                                        {...restField}
                                        name={[name, "value"]}
                                        style={{ flex: 3 }}
                                        rules={[{ required: true, message: "请输入标签值（value）" }]}
                                        normalize={(value) => value?.replace(/\s/g, "")}
                                    >
                                        <Input placeholder="值（value）" />
                                    </Form.Item>

                                    <MinusCircleOutlined
                                        style={{
                                            marginTop: "-25px",
                                            display: "flex",
                                            justifyContent: "center",
                                            alignItems: "center",
                                            cursor: "pointer",
                                        }}
                                        onClick={() => remove(name)}
                                    />
                                </div>
                            ))}

                            <Form.Item>
                                <Button
                                    type="dashed"
                                    onClick={() => add()}
                                    block
                                    icon={<PlusOutlined />}
                                    disabled={fields.length >= 10}
                                >
                                    添加标签
                                </Button>
                            </Form.Item>
                        </>
                    )}
                </Form.List>

                <MyFormItem name="type" label="数据源类型" hidden rules={[{ required: true }]} />

                {(selectedType === "Prometheus" ||
                    selectedType === "Loki" ||
                    selectedType === "VictoriaLogs" ||
                    selectedType === "Jaeger" ||
                    selectedType === "ElasticSearch" ||
                    selectedType === "ClickHouse" ||
                    selectedType === "Zabbix"
                ) && (
                    <>
                        {selectedType === "ClickHouse" ? (
                            <MyFormItemGroup prefix={["clickhouseConfig"]}>
                                <Divider orientation="left">Server</Divider>
                                <MyFormItem
                                    name="addr"
                                    label="Addr"
                                    rules={[
                                        {
                                            required: true,
                                        },
                                    ]}
                                >
                                    <Input placeholder="127.0.0.1:9000,127.0.0.2:9000"/>
                                </MyFormItem>

                                <MyFormItem
                                    name="timeout"
                                    label="Timeout"
                                    rules={[
                                        {
                                            required: true,
                                        },
                                    ]}
                                >
                                    <Input type={"number"} style={{ width: "100%" }} addonAfter={<span>秒</span>} placeholder="10" min={1} />
                                </MyFormItem>
                            </MyFormItemGroup>
                        ) : (
                            <MyFormItemGroup prefix={["http"]}>
                                <Divider orientation="left">HTTP</Divider>
                                <MyFormItem
                                    name="url"
                                    label="URL"
                                    rules={[
                                        {
                                            required: true,
                                        },
                                        {
                                            pattern: selectedType === "Zabbix"
                                                ? /^https?:\/\/\S+\/?$/
                                                : /^(http|https):\/\/.*[^/]$/,
                                            message: selectedType === "Zabbix"
                                                ? "请输入正确的 Zabbix HTTP/HTTPS 地址"
                                                : '请输入正确的URL格式，且结尾不应包含"/"',
                                        },
                                    ]}
                                >
                                    <Input placeholder={selectedType === "Zabbix" ? "例如：http://zabbix.example.com/zabbix" : undefined} />
                                </MyFormItem>

                                {selectedType !== "Zabbix" && (
                                    <MyFormItem
                                        name="timeout"
                                        label="Timeout"
                                        rules={[{ required: true }]}
                                    >
                                        <Input type={"number"} style={{ width: "100%" }} addonAfter={<span>秒</span>} placeholder="10" min={1} />
                                    </MyFormItem>
                                )}

                                {(selectedType !== "ElasticSearch" && selectedType !== "Zabbix") && (
                                    <Form.List name={["http", "headers"]}>
                                        {(fields, { add, remove }) => (
                                            <>
                                                <label style={{ display: "block", marginTop: "16px", marginBottom: "8px" }}>请求头</label>
                                                {fields.map(({ key, name, ...restField }) => (
                                                    <div
                                                        key={key}
                                                        style={{
                                                            display: "flex",
                                                            marginBottom: 8,
                                                            gap: "8px",
                                                            alignItems: "center",
                                                        }}
                                                    >
                                                        <Form.Item
                                                            {...restField}
                                                            name={[name, "key"]}
                                                            style={{ flex: 1, width: "300px" }}
                                                            rules={[{ required: true, message: "请输入请求头键" }]}
                                                        >
                                                            <Input placeholder="键 (例如: Content-Type)" />
                                                        </Form.Item>
                                                        <Form.Item
                                                            {...restField}
                                                            name={[name, "value"]}
                                                            style={{ flex: 1, width: "300px" }}
                                                            rules={[{ required: true, message: "请输入请求头值" }]}
                                                        >
                                                            <Input placeholder="值 (例如: application/json)" />
                                                        </Form.Item>
                                                        <MinusCircleOutlined
                                                            style={{
                                                                marginTop: "-25px",
                                                                display: "flex",
                                                                justifyContent: "center",
                                                                alignItems: "center",
                                                                cursor: "pointer",
                                                            }}
                                                            onClick={() => remove(name)}
                                                        />
                                                    </div>
                                                ))}
                                                <Form.Item>
                                                    <Button
                                                        type="dashed"
                                                        onClick={() => add()}
                                                        block
                                                        icon={<PlusOutlined />}
                                                        disabled={fields.length >= 10}
                                                    >
                                                        添加请求头
                                                    </Button>
                                                </Form.Item>
                                            </>
                                        )}
                                    </Form.List>
                                )}
                            </MyFormItemGroup>
                        )}

                        {selectedType !== "Zabbix" && (
                            <MyFormItemGroup prefix={["auth"]}>
                                <Divider orientation="left">Auth</Divider>
                                <Radio.Group
                                    block
                                    options={radioOptions}
                                    defaultValue="Off"
                                    value={authState}
                                    onChange={(e)=>setAuthState(e.target.value)}
                                />
                                {authState === "On" && (
                                    <div style={{marginTop: "10px"}}>
                                        <MyFormItem name="user" label="User" rules={[{ required: true }]}>
                                            <Input />
                                        </MyFormItem>
                                        <MyFormItem name="pass" label="Pass" rules={[{ required: true }]}>
                                            <Input.Password />
                                        </MyFormItem>
                                    </div>
                                )}
                            </MyFormItemGroup>
                        )}

                        {selectedType === "Prometheus" && (
                            <MyFormItemGroup prefix={["write"]}>
                                <Divider orientation="left">Write</Divider>
                                <Alert
                                message="提示：用于向该数据源写入指标数据，Prometheus需配置 --web.enable-remote-write-receive 参数。"
                                type="info"
                                showIcon
                                style={{ marginBottom: 20, marginTop: "10px" }}
                            />
                                <Radio.Group
                                    block
                                    options={writeRadioOptions}
                                    defaultValue="Off"
                                    value={writeState}
                                    onChange={(e)=>setWriteState(e.target.value)}
                                />
                                {writeState === "On" && (
                                    <div style={{marginTop: "10px"}}>
                                        <MyFormItem
                                            name="url"
                                            label="URL"
                                            rules={[
                                                {
                                                    required: true,
                                                },
                                                {
                                                    pattern: /^(http|https):\/\/.*[^/]$/,
                                                    message: '请输入正确的URL格式，且结尾不应包含"/"',
                                                },
                                            ]}
                                        >
                                            <Input placeholder="例如: http://localhost:9090/api/v1/write"/>
                                        </MyFormItem>
                                    </div>
                                )}
                            </MyFormItemGroup>
                        )}
                    </>
                )}

                {selectedType === "Zabbix" && (
                    <MyFormItemGroup prefix={["zabbixConfig"]}>
                        <Divider orientation="left">Zabbix API</Divider>
                        <Alert
                            message="使用只读 API Token；WatchAlert 只读取问题、事件、触发器和主机信息，不会确认或关闭 Zabbix 告警。"
                            type="info"
                            showIcon
                            style={{ marginBottom: 20 }}
                        />
                        <MyFormItem
                            name="apiToken"
                            label="API Token"
                            rules={[{ required: type === "create", message: "请输入只读 Zabbix API Token" }]}
                            extra={type === "update" ? "留空表示保持原 Token 不变" : "建议使用仅有主机组读取权限的专用账号"}
                        >
                            <Input.Password autoComplete="new-password" placeholder={type === "update" ? "留空保持不变" : "只读 API Token"} />
                        </MyFormItem>
                        <MyFormItem
                            name="faultCenterId"
                            label="接收告警的故障中心"
                            rules={[{ required: true, message: "请选择故障中心" }]}
                        >
                            <Select
                                showSearch
                                allowClear
                                loading={faultCenterLoading}
                                options={faultCenterOptions}
                                optionFilterProp="label"
                                placeholder="选择故障中心"
                                notFoundContent={faultCenterLoading ? "加载中..." : "暂无故障中心，请先创建"}
                            />
                        </MyFormItem>
                        <Row gutter={16}>
                            <Col span={12}>
                                <MyFormItem
                                    name="pollInterval"
                                    label="轮询间隔"
                                    rules={[{ required: true }]}
                                >
                                    <Input type="number" min={5} addonAfter="秒" />
                                </MyFormItem>
                            </Col>
                            <Col span={12}>
                                <MyFormItem name="timeout" label="请求超时" rules={[{ required: true }]}>
                                    <Input type="number" min={1} addonAfter="秒" />
                                </MyFormItem>
                            </Col>
                            <Col span={12}>
                                <MyFormItem
                                    name="initialLookback"
                                    label="首次回溯窗口"
                                    rules={[{ required: true }]}
                                >
                                    <Input type="number" min={1} addonAfter="秒" />
                                </MyFormItem>
                            </Col>
                            <Col span={12}>
                                <MyFormItem name="overlap" label="增量重叠窗口" rules={[{ required: true }]}>
                                    <Input type="number" min={0} addonAfter="秒" />
                                </MyFormItem>
                            </Col>
                        </Row>
                        <MyFormItem name="insecureSkipVerify" valuePropName="checked">
                            <Checkbox>跳过 HTTPS 证书校验（仅用于可信内网测试环境）</Checkbox>
                        </MyFormItem>
                        <Divider orientation="left">告警等级映射</Divider>
                        <Row gutter={16}>
                            {[
                                ["5", "灾难"],
                                ["4", "严重"],
                                ["3", "一般严重"],
                                ["2", "警告"],
                                ["1", "信息"],
                                ["0", "未分类"],
                            ].map(([severity, label]) => (
                                <Col span={12} key={severity}>
                                    <MyFormItem
                                        name={["severityMapping", severity]}
                                        label={`${label}（${severity}）`}
                                        rules={[{ required: true }]}
                                    >
                                        <Select options={["P0", "P1", "P2"].map((value) => ({ value, label: value }))} />
                                    </MyFormItem>
                                </Col>
                            ))}
                        </Row>
                    </MyFormItemGroup>
                )}

                {selectedType === "AliCloudSLS" && (
                    <div>
                        <Divider orientation="left">Auth</Divider>
                        <MyFormItemGroup prefix={["dsAliCloudConfig"]}>
                            <MyFormItem
                                name="alicloudEndpoint"
                                label="Endpoint"
                                rules={[
                                    {
                                        required: true,
                                    },
                                ]}
                            >
                                <Input />
                            </MyFormItem>

                            <MyFormItem
                                name="alicloudAk"
                                label="AccessKeyId"
                                rules={[
                                    {
                                        required: true,
                                    },
                                ]}
                            >
                                <Input.Password />
                            </MyFormItem>

                            <MyFormItem
                                name="alicloudSk"
                                label="AccessKeySecret"
                                rules={[
                                    {
                                        required: true,
                                    },
                                ]}
                            >
                                <Input.Password />
                            </MyFormItem>
                        </MyFormItemGroup>
                    </div>
                )}

                {selectedType === "CloudWatch" && (
                    <div>
                        <Divider orientation="left">Auth</Divider>
                        <MyFormItemGroup prefix={["awsCloudwatch"]}>
                            <MyFormItem
                                name="region"
                                label="Region"
                                rules={[
                                    {
                                        required: true,
                                    },
                                ]}
                            >
                                <Input />
                            </MyFormItem>

                            <MyFormItem
                                name="accessKey"
                                label="AccessKey"
                                rules={[
                                    {
                                        required: true,
                                    },
                                ]}
                            >
                                <Input.Password />
                            </MyFormItem>

                            <MyFormItem
                                name="secretKey"
                                label="SecretKey"
                                rules={[
                                    {
                                        required: true,
                                    },
                                ]}
                            >
                                <Input.Password />
                            </MyFormItem>
                        </MyFormItemGroup>
                    </div>
                )}

                {selectedType === "Kubernetes" && (
                    <div>
                        <Divider orientation="left">Auth</Divider>
                        <MyFormItem
                            name="kubeConfig"
                            label="认证配置"
                            rules={[
                                {
                                    required: true,
                                },
                            ]}
                        >
                            <VSCodeEditor height={"500px"} language={"Yaml"}/>
                        </MyFormItem>
                    </div>
                )}

                <Divider orientation="left">Other</Divider>
                <MyFormItem name="description" label="描述">
                    <Input />
                </MyFormItem>

                <div style={{display: 'flex', alignItems: 'center'}}>
                    <span style={{marginRight: 8}}>启用/禁用</span>
                    <Checkbox
                        style={{marginTop: '0', marginRight: '10px'}}
                        checked={enabled}
                        onChange={(e) => setEnabled(e.target.checked)}
                    />
                </div>
            </Form>
        )
    }

    // 渲染步骤内容
    const renderStepContent = () => {
        switch (currentStep) {
            case 0:
                return renderDatasourceTypeCards()
            case 1:
                return renderDatasourceConfigForm()
            default:
                return null
        }
    }

    // 渲染底部按钮
    const renderFooterButtons = () => {
        if (currentStep === 0) {
            return (
                <div style={{ display: "flex", justifyContent: "flex-end" }}>
                    <Button onClick={onClose}>取消</Button>
                </div>
            )
        } else {
            return (
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <div>
                        <Button type="default" onClick={handleTestConnection} loading={testLoading}>
                            连接测试
                        </Button>
                    </div>
                    <div>
                        <Button
                            style={{ marginRight: 8 }}
                            onClick={handlePrevStep}
                            disabled={disableStep}
                        >
                            上一步
                        </Button>
                        <Button
                            type="primary"
                            htmlType="submit"
                            loading={submitLoading}
                            onClick={handleSubmit}
                            style={{
                                backgroundColor: "#000000",
                            }}
                        >
                            提交
                        </Button>
                    </div>
                </div>
            )
        }
    }

    return (
        <Drawer title={type === "update" ? "更新数据源" : "创建数据源"} open={visible} onClose={onClose} size="large" footer={renderFooterButtons()}>
            <Steps
                current={currentStep}
                items={[
                    {
                        title: "选择数据源",
                    },
                    {
                        title: "配置数据源",
                    },
                ]}
                style={{ marginBottom: 24 }}
            />

            {renderStepContent()}
        </Drawer>
    )
}
