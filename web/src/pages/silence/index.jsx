import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useLocation } from 'react-router-dom';
import { Radio, Table, Popconfirm, Tag, Space, Button, Empty, Tooltip, Input, Select } from 'antd';
import { DeleteOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { CreateSilenceModal } from './SilenceRuleCreateModal';
import { deleteSilence, getSilenceList } from '../../api/silence';
import { FaultCenterList } from '../../api/faultCenter';
import { HandleShowTotal } from '../../utils/lib';
import { Breadcrumb } from '../../components/Breadcrumb';
import '../alert/rule/index.css';

const statusLabels = { 0: ['未生效', 'processing'], 1: ['生效中', 'success'], 2: ['已失效', 'default'] };
const formatDate = timestamp => new Date(timestamp * 1000).toLocaleString('zh-CN', { hour12: false });

export const Silences = () => {
    const location = useLocation();
    const [selectedRow, setSelectedRow] = useState(null);
    const [visible, setVisible] = useState(false);
    const [list, setList] = useState([]);
    const [loading, setLoading] = useState(true);
    const [pagination, setPagination] = useState({ index: 1, size: 10, total: 0 });
    const [searchText, setSearchText] = useState('');
    const [query, setQuery] = useState('');
    const [status, setStatus] = useState('all');
    const [faultCenters, setFaultCenters] = useState([]);
    const [center, setCenter] = useState(() => new URLSearchParams(location.search).get('faultCenterId') || undefined);
    const requestId = useRef(0);

    useEffect(() => {
        FaultCenterList().then(res => setFaultCenters(res?.data || [])).catch(() => {});
    }, []);

    useEffect(() => {
        setCenter(new URLSearchParams(location.search).get('faultCenterId') || undefined);
        setPagination(prev => ({ ...prev, index: 1 }));
    }, [location.search]);

    const handleList = useCallback(async () => {
        const request = ++requestId.current;
        setLoading(true);
        try {
            const res = await getSilenceList({ index: pagination.index, size: pagination.size,
                faultCenterId: center, query: query || undefined, status });
            if (request !== requestId.current) return;
            const data = res.data;
            const lastPage = Math.max(1, Math.ceil(data.total / pagination.size));
            setList(data.list || []);
            setPagination(prev => ({ ...prev, index: Math.min(prev.index, lastPage), total: data.total }));
        } catch {
            // API reports the failure; retain the last successful list and pagination.
        } finally {
            if (request === requestId.current) setLoading(false);
        }
    }, [pagination.index, pagination.size, center, query, status]);

    useEffect(() => {
        handleList();
        return () => { requestId.current += 1; };
    }, [handleList]);

    const handleSaved = () => {
        setSearchText('');
        setQuery('');
        setStatus('all');
        setPagination(prev => ({ ...prev, index: 1 }));
        // If filters already match, the effect does not run again.
        if (!query && status === 'all' && pagination.index === 1) return handleList();
    };

    const handleDelete = async record => {
        await deleteSilence({ id: record.id });
        await handleList();
    };

    const columns = [
        { title: '规则名称', dataIndex: 'name', width: 230, render: (text, record) =>
            <Button type="link" style={{ padding: 0, maxWidth: '100%', whiteSpace: 'normal', height: 'auto', textAlign: 'left' }}
                onClick={() => { setSelectedRow(record); setVisible(true); }}>{text}</Button> },
        { title: '生效范围', key: 'center', width: 190, render: (_, record) =>
            record.faultCenterId ? faultCenters.find(item => item.id === record.faultCenterId)?.name || record.faultCenterId : '当前租户所有故障中心' },
        { title: '匹配条件', key: 'labels', width: 290, render: (_, record) =>
            <Space direction="vertical" size={4} style={{ maxWidth: '100%' }}>{(record.labels || []).map((label, index) => {
                const text = `${label.key}${label.operator}${label.value}`;
                return <Tooltip key={index} title={text}><Tag style={{ maxWidth: 280, overflow: 'hidden', textOverflow: 'ellipsis' }}>{text}</Tag></Tooltip>;
            })}</Space> },
        { title: '时间范围', key: 'time', width: 190, render: (_, record) =>
            <div>{formatDate(record.startsAt)}<br />至 {formatDate(record.endsAt)}</div> },
        { title: '操作人 / 更新时间', key: 'by', width: 180, render: (_, record) =>
            <div>{record.updateBy}<br />{formatDate(record.updateAt)}</div> },
        { title: '状态', dataIndex: 'status', width: 95, render: value =>
            <Tag color={statusLabels[value]?.[1]}>{statusLabels[value]?.[0] || '未知'}</Tag> },
        { title: '操作', key: 'actions', width: 130, fixed: 'right', render: (_, record) =>
            <Popconfirm title={record.status === 2 ? '删除已失效的规则？' : '取消并删除这条静默规则？'}
                description="若告警还匹配其他静默规则，将继续保持静默。"
                onConfirm={() => handleDelete(record)} okText="确定" cancelText="返回" placement="left">
                <Button danger type="text" icon={<DeleteOutlined />}>{record.status === 2 ? '删除' : '取消静默'}</Button>
            </Popconfirm> },
    ];

    return <div>
        <Breadcrumb items={['告警管理', '静默规则']} />
        <Space wrap style={{ marginBottom: 16 }}>
            <Radio.Group optionType="button" value={status} onChange={e => {
                setStatus(e.target.value); setPagination(prev => ({ ...prev, index: 1 }));
            }} options={[{ label: '全部', value: 'all' }, { label: '未生效', value: '0' },
                { label: '生效中', value: '1' }, { label: '已失效', value: '2' }]} />
            <Select aria-label="故障中心筛选" placeholder="全部故障中心" value={center} allowClear showSearch optionFilterProp="label"
                style={{ width: 250 }} options={faultCenters.map(item => ({ value: item.id, label: item.name }))}
                onChange={value => { setCenter(value); setPagination(prev => ({ ...prev, index: 1 })); }} />
            <Input.Search placeholder="搜索规则名称、ID 或原因" value={searchText} allowClear prefix={<SearchOutlined />}
                style={{ width: 280 }} onChange={e => setSearchText(e.target.value)}
                onSearch={value => { setQuery(value); setPagination(prev => ({ ...prev, index: 1 })); }} />
            <Button icon={<ReloadOutlined />} loading={loading} onClick={handleList}>刷新</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { setSelectedRow(null); setVisible(true); }}>添加新规则</Button>
        </Space>
        {center && <div style={{ marginBottom: 12, color: '#666' }}>当前只显示此故障中心专用规则；清除故障中心筛选可查看包括全局规则在内的全部静默。</div>}
        <CreateSilenceModal visible={visible} onClose={() => setVisible(false)} selectedRow={selectedRow}
            type={selectedRow ? 'update' : 'create'} handleList={handleSaved} faultCenterId={center} />
        <Table columns={columns} dataSource={list} loading={loading} rowKey="id" scroll={{ x: 1200 }}
            pagination={{ current: pagination.index, pageSize: pagination.size, total: pagination.total,
                showSizeChanger: true, showTotal: HandleShowTotal, pageSizeOptions: ['10', '20', '50'],
                onChange: (index, size) => setPagination(prev => ({ ...prev, index, size })) }}
            locale={{ emptyText: <Empty description="暂无静默规则" image={Empty.PRESENTED_IMAGE_SIMPLE} /> }} />
    </div>;
};
