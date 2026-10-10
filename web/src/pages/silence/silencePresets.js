const datasourceId = event => event.datasource_id ?? event.datasourceId;
const datasourceType = event => event.datasource_type ?? event.datasourceType;
const splitValues = value => String(value || '').split(',').map(v => v.trim()).filter(Boolean);
const unique = values => [...new Set(values)];
const escapeRegex = value => String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

export function silenceTargets(events) {
    const zabbix = events.length > 0 && events.every(event =>
        datasourceType(event) === 'Zabbix' && datasourceId(event) && datasourceId(event) === datasourceId(events[0]));
    const hosts = new Map();
    const groups = [];
    if (zabbix) events.forEach(event => {
        const ids = splitValues(event.labels?.zabbix_hostids);
        ids.forEach(id => hosts.set(id, {
            value: id,
            label: ids.length === 1 && event.labels?.instance ? `${event.labels.instance}（${id}）` : `设备 ID ${id}`,
        }));
        groups.push(...splitValues(event.labels?.zabbix_hostgroups));
    });
    return { zabbix, hosts: [...hosts.values()], groups: unique(groups).map(value => ({ value, label: value })) };
}

export function silenceLabelsForEvents(events, scope, selectedTargets = []) {
    if (!events?.length || events.some(event => !event?.fingerprint)) throw new Error('请重新选择告警');
    if (scope === 'event') {
        const fingerprints = unique(events.map(event => event.fingerprint));
        return [{ key: 'watchalert_fingerprint', operator: fingerprints.length === 1 ? '==' : '=~',
            value: fingerprints.length === 1 ? fingerprints[0] : `^(${fingerprints.map(escapeRegex).join('|')})$` }];
    }
    if (scope === 'devices' || scope === 'groups') {
        const targets = silenceTargets(events);
        const available = scope === 'devices' ? targets.hosts : targets.groups;
        const selected = unique(selectedTargets);
        if (!targets.zabbix || !selected.length || selected.some(value => !available.some(item => item.value === value))) {
            throw new Error(scope === 'devices' ? '请选择同一 Zabbix 数据源中的设备' : '请选择同一 Zabbix 数据源中的设备组');
        }
        return [
            { key: 'watchalert_datasource_id', operator: '==', value: datasourceId(events[0]) },
            { key: scope === 'devices' ? 'zabbix_hostids' : 'zabbix_hostgroups', operator: '=~',
                value: `(^|,)(${selected.map(escapeRegex).join('|')})(,|$)` },
        ];
    }
    if (events.length !== 1) throw new Error('请从单条告警设置同类条件');
    return silenceLabelsForEvent(events[0], scope);
}

export function silenceLabelsForEvent(event, scope) {
    const equals = (key, value) => ({ key, operator: "==", value: String(value || "") });
    if (scope === "event") {
        return [equals("watchalert_fingerprint", event.fingerprint)];
    }
    const labels = [equals("watchalert_datasource_id", (event.datasource_id ?? event.datasourceId))];
    if (scope === "rule") {
        labels.push((event.datasource_type ?? event.datasourceType) === "Zabbix"
            ? equals("zabbix_triggerid", event.labels?.zabbix_triggerid)
            : equals("watchalert_rule_id", (event.rule_id ?? event.ruleId)));
    } else {
        labels.push((event.datasource_type ?? event.datasourceType) === "Zabbix"
            ? equals("zabbix_problem_name", event.labels?.zabbix_problem_name || (event.rule_name ?? event.ruleName))
            : equals("watchalert_rule_name", (event.rule_name ?? event.ruleName)));
    }
    if (scope === "custom") labels[1].operator = "contains";
    return labels;
}

export function silenceTimeWindow(mode, minutes, range, now) {
    const startsAt = mode === "duration" ? now : range?.[0]?.unix();
    const endsAt = mode === "duration" ? now + Number(minutes) * 60 : range?.[1]?.unix();
    if (!Number.isSafeInteger(startsAt) || !Number.isSafeInteger(endsAt) ||
        startsAt <= 0 || endsAt <= startsAt || endsAt <= now) {
        throw new Error("请选择有效的静默时间，结束时间必须晚于开始时间和当前时间");
    }
    return { startsAt, endsAt };
}
