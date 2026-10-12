import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { CreateNoticeObjectModal } from './NoticeObjectCreateModal';
import { createNotice, updateNotice } from '../../api/notice';

jest.mock('@ant-design/icons', () => ({ PlusOutlined: () => null, MinusCircleOutlined: () => null }));
jest.mock('../../api/notice', () => ({ createNotice: jest.fn(), updateNotice: jest.fn(), noticeTest: jest.fn() }));
jest.mock('../../api/duty', () => ({ getDutyManagerList: async () => ({ data: [] }) }));
jest.mock('../../api/user', () => ({ getUserList: async () => ({ data: [] }) }));
jest.mock('../../api/noticeTmpl', () => ({ getNoticeTmplList: async () => ({ data: [] }) }));

let root, container, log;
beforeAll(() => {
    window.matchMedia = () => ({ matches: false, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {} });
    global.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
    global.IS_REACT_ACT_ENVIRONMENT = true;
    window.getComputedStyle = () => document.documentElement.style;
});
beforeEach(() => {
    jest.clearAllMocks();
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    log = jest.spyOn(console, 'error').mockImplementation(() => {});
});
afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    log.mockRestore();
});
test.each(['create', 'update'])('%s failure keeps form values and allows a successful retry', async type => {
    const save = type === 'create' ? createNotice : updateNotice;
    save.mockRejectedValueOnce(new Error('save rejected')).mockResolvedValueOnce({ code: 200 });
    const onClose = jest.fn(), handleList = jest.fn();
    const selectedRow = { uuid: 'n-test', name: 'test-receiver', noticeType: 'WebHook',
        routes: [{ noticeType: 'WebHook', hook: 'http://fixture:8080/notify', severitys: ['P0'], noticeTmplId: 'template' }] };
    await act(async () => root.render(<CreateNoticeObjectModal visible type={type} selectedRow={selectedRow} onClose={onClose} handleList={handleList} />));
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 30)); });
    const submit = [...document.querySelectorAll('button')].find(button => button.textContent.replace(/\s/g, '') === '提交');
    await act(async () => { submit.click(); await new Promise(resolve => setTimeout(resolve, 100)); });
    expect(save).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
    expect(handleList).not.toHaveBeenCalled();
    expect(document.querySelector('input[placeholder="请输入通知对象名称"]').value).toBe('test-receiver');
    await act(async () => { submit.click(); await new Promise(resolve => setTimeout(resolve, 100)); });
    expect(save).toHaveBeenCalledTimes(2);
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(handleList).toHaveBeenCalledTimes(1);
});
