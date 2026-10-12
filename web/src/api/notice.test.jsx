import { createNotice, updateNotice } from './notice';
import http from '../utils/http';
import { message } from 'antd';
import { HandleApiError } from '../utils/lib';

jest.mock('../utils/http', () => jest.fn());
jest.mock('antd', () => ({ message: { open: jest.fn() } }));
jest.mock('../utils/lib', () => ({ HandleApiError: jest.fn() }));

beforeEach(() => jest.clearAllMocks());
test.each([createNotice, updateNotice])('failed save rejects and never reports success', async save => {
    const failure = new Error('server rejected the save');
    http.mockRejectedValue(failure);
    await expect(save({ name: 'receiver' })).rejects.toBe(failure);
    expect(HandleApiError).toHaveBeenCalledWith(failure);
    expect(message.open).not.toHaveBeenCalled();
});
test.each([createNotice, updateNotice])('successful save returns its result', async save => {
    const result = { code: 200, data: null };
    http.mockResolvedValue(result);
    await expect(save({ name: 'receiver' })).resolves.toBe(result);
    expect(message.open).toHaveBeenCalledWith(expect.objectContaining({ type: 'success' }));
});
