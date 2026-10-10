import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { CreateSilenceModal } from './SilenceRuleCreateModal';
import { createSilence, updateSilence } from '../../api/silence';

jest.mock('@ant-design/icons', () => ({ PlusOutlined: () => null, MinusCircleOutlined: () => null }));
jest.mock('../../api/silence', () => ({ createSilence: jest.fn(), updateSilence: jest.fn() }));
jest.mock('../../utils/lib', () => ({ HandleApiError: jest.fn() }));

let root, container;
beforeAll(() => {
    window.matchMedia = () => ({ matches: false, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {} });
    global.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
    global.IS_REACT_ACT_ENVIRONMENT = true;
    // JSDOM has no layout and cannot evaluate the current Ant Design CSS.
    window.getComputedStyle = () => document.documentElement.style;
});
beforeEach(() => {
    jest.clearAllMocks();
    createSilence.mockResolvedValue({});
    updateSilence.mockResolvedValue({});
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
});
afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
});
async function submit() {
    await act(async () => {
        document.querySelector('form').dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        await new Promise(resolve => setTimeout(resolve, 100));
    });
}
const event = { fingerprint: 'ds:42', datasource_id: 'ds', datasource_type: 'Zabbix', rule_name: 'Link down', labels: { zabbix_triggerid: '123' } };

test('single event saves a timed rule and closes without requiring a list callback', async () => {
    const onClose = jest.fn();
    await act(async () => root.render(<CreateSilenceModal visible type="create" onClose={onClose} selectedRow={{ event }} faultCenterId="fc" />));
    await submit();
    expect(createSilence).toHaveBeenCalledTimes(1);
    const params = createSilence.mock.calls[0][0];
    expect(params.labels).toEqual([{ key: 'watchalert_fingerprint', operator: '==', value: 'ds:42' }]);
    expect(params.endsAt - params.startsAt).toBe(3600);
    expect(params.faultCenterId).toBe('fc');
    expect(onClose).toHaveBeenCalledTimes(1);
});
test('failed save keeps the form open', async () => {
    const onClose = jest.fn();
    createSilence.mockRejectedValue({ response: { data: { msg: 'failed', data: 'invalid rule' } } });
    await act(async () => root.render(<CreateSilenceModal visible type="create" onClose={onClose} selectedRow={{ event }} faultCenterId="fc" />));
    await submit();
    expect(createSilence).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
});
test('editing restores the existing time window without reselecting dates', async () => {
    const now = Math.floor(Date.now() / 1000);
    const selectedRow = { id: 's-1', name: 'Maintenance', startsAt: now + 600, endsAt: now + 1200,
        labels: [{ key: 'zabbix_problem_name', operator: '=~', value: 'Link down' }] };
    await act(async () => root.render(<CreateSilenceModal visible type="update" onClose={jest.fn()} selectedRow={selectedRow} faultCenterId="fc" />));
    await submit();
    expect(updateSilence).toHaveBeenCalledWith(expect.objectContaining({ id: 's-1', startsAt: selectedRow.startsAt, endsAt: selectedRow.endsAt }));
});

test('same trigger selection includes new occurrences and respects a shortcut duration', async () => {
    await act(async () => root.render(<CreateSilenceModal visible type="create" onClose={jest.fn()} selectedRow={{ event }} faultCenterId="fc" />));
    await act(async () => {
        document.querySelector('input[type="radio"][value="rule"]').click();
        [...document.querySelectorAll('button')].find(button => button.textContent === '30 分钟').click();
    });
    await submit();
    const params = createSilence.mock.calls[0][0];
    expect(params.labels).toEqual([
        { key: 'watchalert_datasource_id', operator: '==', value: 'ds' },
        { key: 'zabbix_triggerid', operator: '==', value: '123' },
    ]);
    expect(params.endsAt - params.startsAt).toBe(1800);
});
test('class selection submits editable name conditions without an event identifier', async () => {
    await act(async () => root.render(<CreateSilenceModal visible type="create" onClose={jest.fn()} selectedRow={{ event }} faultCenterId="fc" />));
    await act(async () => document.querySelector('input[type="radio"][value="custom"]').click());
    await submit();
    expect(createSilence).toHaveBeenCalledTimes(1);
    expect(createSilence.mock.calls[0][0].labels).toEqual([
        { key: 'watchalert_datasource_id', operator: '==', value: 'ds' },
        { key: 'zabbix_problem_name', operator: 'contains', value: 'Link down' },
    ]);
});

test('bulk defaults to selected events only and device scope covers all selected hosts', async () => {
    const events = [
        { ...event, labels: { zabbix_hostids: '12', instance: 'Switch A' } },
        { ...event, fingerprint: 'ds:43', labels: { zabbix_hostids: '13', instance: 'Switch B' } },
    ];
    const selectedRow = { events };
    await act(async () => root.render(<CreateSilenceModal visible type="create" onClose={jest.fn()} selectedRow={selectedRow} faultCenterId="fc" />));
    await submit();
    const label = createSilence.mock.calls[0][0].labels[0];
    expect(label.key).toBe('watchalert_fingerprint');
    expect(new RegExp(label.value).test('ds:44')).toBe(false);
    createSilence.mockClear();
    await act(async () => document.querySelector('input[type="radio"][value="devices"]').click());
    await submit();
    expect(createSilence.mock.calls[0][0].labels).toEqual([
        { key: 'watchalert_datasource_id', operator: '==', value: 'ds' },
        { key: 'zabbix_hostids', operator: '=~', value: '(^|,)(12|13)(,|$)' },
    ]);
});

test('editing a global rule preserves the global scope even with a center filter', async () => {
    const now = Math.floor(Date.now() / 1000);
    const selectedRow = { id: 'global', name: 'Global', faultCenterId: '', startsAt: now - 60, endsAt: now + 1200,
        labels: [{ key: 'instance', operator: '==', value: 'switch' }] };
    await act(async () => root.render(<CreateSilenceModal visible type="update" onClose={jest.fn()} selectedRow={selectedRow} faultCenterId="filtered-center" />));
    await submit();
    expect(updateSilence.mock.calls[0][0].faultCenterId).toBe('');
});
