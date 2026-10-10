import React, { useState, useEffect } from 'react';
import {Button, Input, Table, Tag, Tooltip, Dropdown, Modal} from 'antd';
import { CreateDatasourceModal } from './DatasourceCreateModal';
import { deleteDatasource, getDatasourceList, getZabbixSyncStatus } from '../../api/datasource';
import { ReactComponent as PrometheusImg } from "../alert/rule/img/Prometheus.svg"
import { ReactComponent as AlicloudImg } from "../alert/rule/img/alicloud.svg"
import { ReactComponent as JaegerImg } from "../alert/rule/img/jaeger.svg"
import { ReactComponent as AwsImg } from "../alert/rule/img/AWSlogo.svg"
import { ReactComponent as LokiImg } from "../alert/rule/img/L.svg"

import { ReactComponent as K8sImg } from "../alert/rule/img/Kubernetes.svg"
import { ReactComponent as ESImg } from "../alert/rule/img/ElasticSearch.svg"
import { ReactComponent as VLogImg } from "../alert/rule/img/victorialogs.svg"
import { ReactComponent as CkImg } from "../alert/rule/img/clickhouse.svg"
import './index.css'
import {ApiOutlined, CopyOutlined, DeleteOutlined, EditOutlined, PlusOutlined, MoreOutlined} from "@ant-design/icons";
import { copyToClipboard } from "../../utils/copyToClipboard";
import {HandleShowTotal} from "../../utils/lib";
import { Breadcrumb } from "../../components/Breadcrumb";

export const Datasources = () => {
    const { Search } = Input
    const [selectedRow, setSelectedRow] = useState(null);
    const [updateVisible, setUpdateVisible] = useState(false);
    const [visible, setVisible] = useState(false);
    const [list, setList] = useState([]);
    const [zabbixStatuses, setZabbixStatuses] = useState({});
    const [height, setHeight] = useState(window.innerHeight);
    const columns = [
        {
            title: '名称',
            dataIndex: 'name',
            key: 'name',
            render: (text, record) => (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                    {text}
                    <Tooltip title="点击复制 ID">
                        <span
                            style={{
                                color: '#8c8c8c',     // 灰色字体
                                fontSize: '12px',
                                cursor: 'pointer',
                                userSelect: 'none',
                                display: 'inline-block',
                                maxWidth: '200px',
                                overflow: 'hidden',
                                textOverflow: 'ellipsis',
                                whiteSpace: 'nowrap'
                            }}
                            onClick={() => copyToClipboard(record.id)}
                        >
                            {record.id}
                            <CopyOutlined style={{ marginLeft: 8 }} />
                        </span>
                    </Tooltip>
                </div>
            ),
        },
        {
            title: '数据源类型',
            dataIndex: 'type',
            key: 'type',
            render: (text, record) => {
                return (
                    <div style={{display: 'flex'}}>
                        {text === "Prometheus" && (
                            <PrometheusImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "CloudWatch" && (
                            <AwsImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "Loki" && (
                            <LokiImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "Jaeger" && (
                            <JaegerImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "AliCloudSLS" && (
                            <AlicloudImg style={{height: "25px", width: "25px"}}/>
                        )}

                        {text === "Kubernetes" && (
                            <K8sImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "ElasticSearch" && (
                            <ESImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "VictoriaLogs" && (
                            <VLogImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "ClickHouse" && (
                            <CkImg style={{height: "25px", width: "25px"}}/>
                        )}
                        {text === "Zabbix" && (
                            <ApiOutlined style={{fontSize: 22, color: "#d40000"}} />
                        )}
                        <div style={{marginLeft: "5px", marginTop: '3px', fontSize: '12px'}}>{text}</div>
                    </div>
                )
            }
        },
        {
            title: '描述',
            dataIndex: 'description',
            key: 'description',
            render: (text, record, index) => {
                if (!text) {
                    return '没有留下任何描述~';
                }
                return text;
            },
        },
        {
            title: "更新时间",
            dataIndex: "updateAt",
            key: "updateAt",
            width: "auto",
            render: (text) => {
                const date = new Date(text * 1000)
                    return (
                        <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                            <span>{date.toLocaleString()}</span>
                        </div>
                    )
            },
        },
        {
            title: "操作人",
            dataIndex: "updateBy",
            key: "updateBy",
            width: "auto",
            render: (text) => {
                return <Tag style={{
                                borderRadius: "12px",
                                padding: "0 10px",
                                fontSize: "12px",
                                fontWeight: "500",
                                display: "inline-flex",
                                alignItems: "center",
                                gap: "4px",
                            }}
                        >
                            {text || "未知用户"}
                        </Tag>
            },
        },
        {
            title: '状态',
            dataIndex: 'enabled',
            key: 'enabled',
            render: enabled => (
                <div className="status-container">
                    <div
                        className={`status-dot ${enabled ? 'status-enabled' : 'status-disabled'}`}
                    />
                    <span>{enabled ? '启用' : '禁用'}</span>
                </div>
            ),
        },
        {
            title: '同步状态',
            key: 'syncStatus',
            render: (_, record) => {
                if (record.type !== 'Zabbix') {
                    return '-'
                }

                const status = zabbixStatuses[record.id]
                if (!status) {
                    return <Tag>未获取</Tag>
                }

                const color = status.lastError ? 'error' : status.running ? 'success' : 'default'
                const text = status.lastError ? '同步异常' : status.running ? '运行中' : '已停止'
                const lastSuccess = status.lastSuccessAt
                    ? new Date(status.lastSuccessAt * 1000).toLocaleString()
                    : '尚未成功同步'

                return (
                    <Tooltip title={(
                        <div>
                            <div>最后成功：{lastSuccess}</div>
                            <div>活动问题：{status.activeProblems || 0}</div>
                            <div>最近恢复：{status.recoveredProblems || 0}</div>
                            <div>历史回补：{status.historyBackfilled || 0}</div>
                            {status.lastError && <div>错误：{status.lastError}</div>}
                        </div>
                    )}>
                        <Tag color={color}>{text}</Tag>
                    </Tooltip>
                )
            },
        },
        {
            title: '操作',
            dataIndex: 'operation',
            fixed: 'right',
            width: 60,
            render: (_, record) => (
                <Dropdown
                    menu={{
                        items: [
                            {
                                key: 'edit',
                                icon: <EditOutlined />,
                                label: '更新',
                                onClick: () => handleUpdateModalOpen(record)
                            },
                            {
                                key: 'delete',
                                icon: <DeleteOutlined />,
                                label: '删除',
                                danger: true,
                                onClick: () => {
                                    Modal.confirm({
                                        title: "确定要删除吗?",
                                        content: `数据源名称: ${record.name}`,
                                        okText: "确定",
                                        cancelText: "取消",
                                        okType: 'danger',
                                        onOk: () => handleDelete(record)
                                    })
                                }
                            }
                        ]
                    }}
                    trigger={['click']}
                    placement="bottomRight"
                >
                    <Button
                        type="text"
                        icon={<MoreOutlined />}
                        style={{ color: "#666" }}
                    />
                </Dropdown>
            ),
        },
    ];

    useEffect(() => {
        // 定义一个处理窗口大小变化的函数
        const handleResize = () => {
            setHeight(window.innerHeight);
        };

        // 监听窗口的resize事件
        window.addEventListener('resize', handleResize);

        // 在组件卸载时移除监听器
        return () => {
            window.removeEventListener('resize', handleResize);
        };
    }, []);

    useEffect(() => {
        try {
            handleList();
        } catch (error) {
            console.error(error)
        }
    }, []);

    const handleList = async () => {
        try {
            const res = await getDatasourceList()
            const records = res?.data || []
            setList(records)
            const zabbixRecords = records.filter((item) => item.type === 'Zabbix')
            const statusEntries = await Promise.all(zabbixRecords.map(async (item) => {
                try {
                    const statusRes = await getZabbixSyncStatus({ id: item.id })
                    return [item.id, statusRes?.data]
                } catch (error) {
                    return [item.id, { running: false, lastError: '同步状态获取失败' }]
                }
            }))
            setZabbixStatuses(Object.fromEntries(statusEntries))
        } catch (error) {
            console.error(error)
        }
    }

    const handleDelete = async (record) => {
        try {
            const params = {
                id: record.id,
                name: record.name,
            }
            await deleteDatasource(params)
            handleList()
        } catch (error) {
            console.error(error)
        }
    };

    const handleModalClose = () => {
        setVisible(false)
    };

    const handleUpdateModalClose = () => {
        setUpdateVisible(false)
    }

    const handleUpdateModalOpen = (record) => {
        setSelectedRow(record)
        setUpdateVisible(true)
    };

    const onSearch = async (value) => {
        try {
            const params = {
                query: value,
            }
            const res = await getDatasourceList(params)
            setList(res?.data)
        } catch (error) {
            console.error(error)
        }
        console.log(value)
    }

    return (
        <>
            <Breadcrumb items={['数据源']} />
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <div>
                    <Search
                        allowClear
                        placeholder="输入搜索关键字"
                        style={{ width: 300 }}
                        onSearch={onSearch}
                    />
                </div>
                <div>
                    <Button
                        type="primary"
                        onClick={() => setVisible(true)}
                        style={{
                            backgroundColor: '#000000'
                        }}
                        icon={<PlusOutlined />}
                    >
                        创建
                    </Button>
                </div>
            </div>

            <CreateDatasourceModal visible={visible} onClose={handleModalClose} type='create' handleList={handleList} />

            <CreateDatasourceModal visible={updateVisible} onClose={handleUpdateModalClose} selectedRow={selectedRow} type="update" handleList={handleList} />

            <div style={{ overflowX: 'auto', marginTop: 10 }}>
                <Table
                    dataSource={list}
                    columns={columns}
                    scroll={{
                        y: height - 250, // 动态设置滚动高度
                        x: 'max-content', // 水平滚动
                    }}
                    style={{
                        backgroundColor: "#fff",
                        borderRadius: "8px",
                        overflow: "hidden",
                    }}
                    pagination={{
                        showTotal: HandleShowTotal,
                        pageSizeOptions: ['10'],
                    }}
                    rowKey={(record) => record.id} // 设置行唯一键
                />
            </div>

        </>
    );
};
