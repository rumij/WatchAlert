import { Alert, AutoComplete, Form, Input, InputNumber, Button, DatePicker, Select, Drawer, Radio, Space, message } from "antd";
import React, { useState, useEffect, useMemo, useRef, useId } from "react";
import dayjs from "dayjs";
import { createSilence, updateSilence } from "../../api/silence";
import { PlusOutlined, MinusCircleOutlined } from "@ant-design/icons";
import { HandleApiError } from "../../utils/lib";
import { silenceLabelsForEvent, silenceLabelsForEvents, silenceTargets, silenceTimeWindow } from "./silencePresets";

const selectorOptions = [
    ["watchalert_fingerprint", "单条告警"],
    ["watchalert_datasource_id", "数据源"],
    ["watchalert_rule_id", "WatchAlert 规则"],
    ["watchalert_rule_name", "WatchAlert 规则名称"],
    ["watchalert_severity", "告警等级 P0 / P1 / P2"],
    ["zabbix_problem_name", "Zabbix 告警名称"],
    ["zabbix_triggerid", "Zabbix 触发器"],
    ["zabbix_hostgroups", "Zabbix 主机组"],
    ["zabbix_hostids", "Zabbix 设备 ID"],
    ["instance", "主机名称"],
].map(([value, label]) => ({ value, label: `${label} (${value})` }));

export const CreateSilenceModal = ({ visible, onClose, selectedRow, type, handleList, faultCenterId }) => {
    const [form] = Form.useForm();
    const formId = useId();
    const [loading, setLoading] = useState(false);
    const saving = useRef(false);
    const timeMode = Form.useWatch("timeMode", form);
    const scope = Form.useWatch("scope", form);
    const events = useMemo(() => selectedRow?.events || (selectedRow?.event ? [selectedRow.event] : []), [selectedRow]);
    const event = events[0];
    const targets = useMemo(() => silenceTargets(events), [events]);
    const effectiveCenter = type === "update"
        ? (selectedRow?.faultCenterId ?? selectedRow?.fault_center_id ?? faultCenterId ?? "") : (faultCenterId || "");

    useEffect(() => {
        if (!visible) return;
        form.resetFields();
        form.setFieldsValue({
            name: selectedRow?.name || (events.length > 1 ? `批量静默：${events.length} 条告警` : event ? `静默：${event.rule_name ?? event.ruleName}` : ""),
            comment: selectedRow?.comment || "",
            scope: "event",
            labels: event ? silenceLabelsForEvents(events, "event") :
                (selectedRow?.labels || [{ key: "", operator: "==", value: "" }]),
            devices: targets.hosts.map(host => host.value),
            groups: [],
            timeMode: type === "update" ? "range" : "duration",
            durationMinutes: 60,
            timeRange: type === "update" && selectedRow?.startsAt && selectedRow?.endsAt
                ? [dayjs.unix(selectedRow.startsAt), dayjs.unix(selectedRow.endsAt)] : undefined,
        });
    }, [visible, selectedRow, events, event, targets, type, form]);

    const changeScope = (value) => {
        form.setFieldValue("scope", value);
        if (value === "custom") form.setFieldValue("labels", silenceLabelsForEvent(event, value));
    };

    const handleFormSubmit = async (values) => {
        if (saving.current) return;
        saving.current = true;
        setLoading(true);
        try {
            const times = silenceTimeWindow(values.timeMode, values.durationMinutes, values.timeRange, dayjs().unix());
            // Keep stable single-event/rule selectors separate from editable class conditions.
            const labels = event && values.scope !== "custom"
                ? silenceLabelsForEvents(events, values.scope, values[values.scope]) : values.labels;
            if (!labels?.length || labels.some(label => !label.key || String(label.value ?? "") === "")) {
                throw new Error("请填写完整的静默条件");
            }
            const params = {
                name: values.name, comment: values.comment || "", labels, ...times,
                faultCenterId: effectiveCenter,
            };
            if (type === "update") {
                await updateSilence({ ...params, id: selectedRow.id });
            } else {
                await createSilence(params);
            }
            onClose();
            if (handleList) await handleList();
        } catch (error) {
            if (error.response) HandleApiError(error);
            else message.error(error.message || "静默保存失败，请稍后重试");
        } finally {
            saving.current = false;
            setLoading(false);
        }
    };

    return (
        <Drawer title={type === "update" ? "编辑静默规则" : "设置告警静默"}
            open={visible} onClose={loading ? undefined : onClose} closable={!loading} maskClosable={!loading} keyboard={!loading} size="large"
            footer={<Space style={{ display: "flex", justifyContent: "flex-end" }}>
                <Button onClick={onClose} disabled={loading}>取消</Button>
                <Button type="primary" htmlType="submit" form={formId} loading={loading}>保存静默</Button>
            </Space>}>
            <Alert type="info" showIcon style={{ marginBottom: 16 }}
                message="静默期间保留告警记录，暂停 WatchAlert 的告警、恢复和升级通知；到期后重新按通知策略处理。"
                description={effectiveCenter ? "仅对当前故障中心生效。" : "对当前租户所有故障中心的匹配告警生效。"} />
            <Form id={formId} form={form} layout="vertical" onFinish={handleFormSubmit}>
                {event && <Form.Item name="scope" label="静默范围">
                    <Radio.Group onChange={e => changeScope(e.target.value)}>
                        <Space direction="vertical">
                            <Radio value="event">{events.length > 1 ? `仅勾选的 ${events.length} 条告警` : "仅此条告警"}</Radio>
                            {events.length === 1 && <Radio value="rule">相同告警规则（包括静默期间新发生的告警）</Radio>}
                            <Radio value="devices" disabled={!targets.hosts.length}>所选设备的全部告警</Radio>
                            <Radio value="groups" disabled={!targets.groups.length}>所选设备组的全部告警</Radio>
                            {events.length === 1 && <Radio value="custom">一类告警（按名称、主机组或标签匹配）</Radio>}
                        </Space>
                    </Radio.Group>
                </Form.Item>}
                {event && scope === "event" && <Alert type="info" style={{ marginBottom: 16 }}
                    message={`仅匹配已选的 ${events.length} 条告警，其他告警不受影响。`} />}
                {event && !targets.zabbix && <Alert type="info" style={{ marginBottom: 16 }}
                    message="设备和设备组快捷静默仅支持同一 Zabbix 数据源的告警；当前选择仍可按勾选告警静默。" />}
                {(scope === "devices" || scope === "groups") && <>
                    <Form.Item name={scope} label={scope === "devices" ? "选择设备" : "选择设备组"}
                        rules={[{ required: true, type: "array", min: 1, message: "请至少选择一项" }]}>
                        <Select mode="multiple" showSearch optionFilterProp="label" allowClear
                            placeholder={scope === "devices" ? "选择要静默的设备" : "选择要静默的设备组"}
                            options={scope === "devices" ? targets.hosts : targets.groups} />
                    </Form.Item>
                    <Alert type="warning" showIcon style={{ marginBottom: 16 }}
                        message="将覆盖所选范围内的全部告警，包括静默期间新发生的告警。"
                        description={scope === "devices" ? "设备选项来自勾选告警；可跨页勾选不同设备上的告警后批量设置。"
                            : "设备组选项来自勾选告警所属的 Zabbix 主机组；组内其他设备也会静默。"} />
                </>}
                {event && scope === "rule" && <Alert type="info" style={{ marginBottom: 16 }}
                    message={(event.datasource_type ?? event.datasourceType) === "Zabbix"
                        ? "匹配同一数据源的同一个 Zabbix 触发器。跨设备的一类告警请选择按条件匹配。"
                        : "匹配同一数据源的同一条 WatchAlert 规则。"} />}
                <Form.Item name="name" label="规则名称" rules={[{ required: true, message: "请输入规则名称" }]}>
                    <Input />
                </Form.Item>
                <Form.Item name="timeMode" label="静默时间">
                    <Radio.Group options={[{ label: "从现在开始", value: "duration" }, { label: "指定起止时间", value: "range" }]} />
                </Form.Item>
                {timeMode === "duration" ? <>
                    <Space wrap style={{ marginBottom: 12 }}>
                        {[[30, "30 分钟"], [60, "1 小时"], [120, "2 小时"], [1440, "1 天"]].map(([minutes, label]) =>
                            <Button key={minutes} onClick={() => form.setFieldValue("durationMinutes", minutes)}>{label}</Button>)}
                    </Space>
                    <Form.Item name="durationMinutes" label="自定义时长（分钟）" rules={[{ required: true, message: "请输入静默时长" }]}>
                        <InputNumber min={1} precision={0} style={{ width: "100%" }} />
                    </Form.Item>
                </> : <Form.Item name="timeRange" label="起止时间" rules={[{ required: true, message: "请选择起止时间" }]}>
                    <DatePicker.RangePicker showTime format="YYYY-MM-DD HH:mm:ss" style={{ width: "100%" }} />
                </Form.Item>}
                {(!event || scope === "custom") && <>
                    <Alert type="info" style={{ marginBottom: 12 }} message="以下条件必须同时满足。"
                        description="例如选择“Zabbix 告警名称”，条件选“包含”，填入 Link down，即可静默这类告警。也可按主机组或已有标签匹配。新发生且符合条件的告警也会静默。" />
                    <Form.List name="labels">
                        {(fields, { add, remove }) => <>
                            {fields.map(({ key, name, ...restField }) => <Space key={key} align="baseline" style={{ display: "flex" }}>
                                <Form.Item {...restField} name={[name, "key"]} rules={[{ required: true, message: "请选择或输入条件" }]}>
                                    <AutoComplete options={selectorOptions} style={{ width: 235 }} placeholder="选择或输入标签名称" />
                                </Form.Item>
                                <Form.Item {...restField} name={[name, "operator"]} rules={[{ required: true }]}>
                                    <Select style={{ width: 125 }} options={[
                                        { value: "==", label: "等于" }, { value: "=", label: "等于（兼容）" },
                                        { value: "!=", label: "不等于" }, { value: "contains", label: "包含" }, { value: "!contains", label: "不包含" }, { value: "=~", label: "正则匹配" }, { value: "!~", label: "正则不匹配" },
                                    ]} />
                                </Form.Item>
                                <Form.Item {...restField} name={[name, "value"]} rules={[{ required: true, message: "请输入匹配值" }]}>
                                    <Input placeholder="匹配值或正则表达式" style={{ width: 210 }} />
                                </Form.Item>
                                {fields.length > 1 && <MinusCircleOutlined aria-label="删除条件" onClick={() => remove(name)} />}
                            </Space>)}
                            <Button block type="dashed" icon={<PlusOutlined />} style={{ marginBottom: 16 }}
                                onClick={() => add({ key: "", operator: "==", value: "" })}>添加条件</Button>
                        </>}
                    </Form.List>
                </>}
                <Form.Item name="comment" label="静默原因（选填）">
                    <Input.TextArea rows={3} maxLength={500} showCount placeholder="例如：链路维护，预计两小时完成" />
                </Form.Item>
            </Form>
        </Drawer>
    );
};
