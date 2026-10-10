import { silenceLabelsForEvent, silenceLabelsForEvents, silenceTargets, silenceTimeWindow } from './silencePresets';

const event = { fingerprint: 'zabbix:ds-1:42:rule-1', datasource_id: 'ds-1', datasource_type: 'Zabbix',
    rule_id: 'rule-1', rule_name: 'Network issues', labels: { zabbix_triggerid: '123', zabbix_problem_name: 'Link down' } };

test('single silence uses a stable fingerprint, not changing problem labels', () => {
    expect(silenceLabelsForEvent(event, 'event')).toEqual([{ key: 'watchalert_fingerprint', operator: '==', value: event.fingerprint }]);
});
test('same trigger and class selectors exclude event ID and include datasource isolation', () => {
    expect(silenceLabelsForEvent(event, 'rule')).toEqual([
        { key: 'watchalert_datasource_id', operator: '==', value: 'ds-1' },
        { key: 'zabbix_triggerid', operator: '==', value: '123' },
    ]);
    expect(silenceLabelsForEvent(event, 'custom')[1]).toEqual({ key: 'zabbix_problem_name', operator: 'contains', value: 'Link down' });
    expect(silenceLabelsForEvent({ ...event, datasource_type: 'Prometheus' }, 'rule')[1].value).toBe('rule-1');
});
test('duration starts when submitted and accepts arbitrary positive minutes', () => {
    expect(silenceTimeWindow('duration', 93, undefined, 1000)).toEqual({ startsAt: 1000, endsAt: 6580 });
});
test('scheduled window and invalid ranges', () => {
    expect(silenceTimeWindow('range', null, [{ unix: () => 1200 }, { unix: () => 1800 }], 1000)).toEqual({ startsAt: 1200, endsAt: 1800 });
    for (const minutes of [0, -1, undefined, NaN, Infinity, Number.MAX_SAFE_INTEGER]) {
        expect(() => silenceTimeWindow('duration', minutes, null, 1000)).toThrow();
    }
    expect(() => silenceTimeWindow('range', 60, null, 1000)).toThrow();
    expect(() => silenceTimeWindow('range', 60, [{ unix: () => 1200 }, { unix: () => 1100 }], 1000)).toThrow();
    expect(() => silenceTimeWindow('range', 60, [{ unix: () => 100 }, { unix: () => 900 }], 1000)).toThrow();
});

test('bulk fingerprints are exact escaped alternatives with no cross-event matches', () => {
    const items = [event, { ...event, fingerprint: 'ds.2:42+(a)' }];
    const [label] = silenceLabelsForEvents(items, 'event');
    const matcher = new RegExp(label.value);
    items.forEach(item => expect(matcher.test(item.fingerprint)).toBe(true));
    ['x' + event.fingerprint, event.fingerprint + '1', 'dsX2:42aaa'].forEach(value => expect(matcher.test(value)).toBe(false));
    expect(silenceLabelsForEvents([event, event], 'event')[0].operator).toBe('==');
});

test('device and group unions match whole members and remain isolated by datasource', () => {
    const item = { ...event, labels: { zabbix_hostids: '12,13', zabbix_hostgroups: 'Office.A,Office+B' } };
    for (const [scope, selected, yes, no] of [
        ['devices', ['12', '13'], ['12', '99,13,88'], ['112', '130', '9']],
        ['groups', ['Office.A'], ['Office.A', 'Other,Office.A'], ['OfficeXA', 'Office.A-extra']],
    ]) {
        const labels = silenceLabelsForEvents([item], scope, selected);
        expect(labels[0]).toEqual({ key: 'watchalert_datasource_id', operator: '==', value: 'ds-1' });
        const matcher = new RegExp(labels[1].value);
        yes.forEach(value => expect(matcher.test(value)).toBe(true));
        no.forEach(value => expect(matcher.test(value)).toBe(false));
    }
    expect(() => silenceLabelsForEvents([item], 'devices', [])).toThrow();
    expect(() => silenceLabelsForEvents([item], 'devices', ['99'])).toThrow();
    expect(() => silenceLabelsForEvents([item, { ...item, datasource_id: 'other' }], 'devices', ['12'])).toThrow();
    expect(() => silenceLabelsForEvents([item, { ...item, datasource_type: 'Prometheus' }], 'groups', ['Office.A'])).toThrow();
    expect(silenceTargets([{ ...item, labels: { zabbix_hostids: '12', instance: 'Switch-12' } }]).hosts[0].label).toContain('Switch-12');
});
