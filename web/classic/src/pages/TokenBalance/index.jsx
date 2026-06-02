/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';
import { renderQuota } from '../../helpers/render';
import { Button, Descriptions, Input, Tag } from '@douyinfe/semi-ui';

const statusColor = {
  1: 'green',
  2: 'grey',
  3: 'orange',
  4: 'red',
};

const statusText = {
  1: '已启用',
  2: '已禁用',
  3: '已过期',
  4: '已耗尽',
};

const formatExpiredTime = (expiredTime, t) => {
  if (!expiredTime || expiredTime <= 0) return t('永不过期');
  return new Date(expiredTime * 1000).toLocaleString();
};

const TokenBalance = () => {
  const { t } = useTranslation();
  const [token, setToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState(null);

  const handleSubmit = async (event) => {
    event.preventDefault();
    const trimmed = token.trim();
    if (!trimmed) {
      showError(t('请输入令牌'));
      return;
    }

    setLoading(true);
    try {
      const res = await API.post('/api/token/search', { token: trimmed });
      const { success, message, data } = res.data;
      if (success) {
        setResult(data);
      } else {
        setResult(null);
        showError(message);
      }
    } catch (error) {
      setResult(null);
      showError(error.message || t('查询失败'));
    } finally {
      setLoading(false);
    }
  };

  const totalQuota = result
    ? Number(result.remain_quota || 0) + Number(result.used_quota || 0)
    : 0;

  const rows = result
    ? [
        { key: t('令牌名称'), value: result.token_name },
        {
          key: t('状态'),
          value: (
            <Tag color={statusColor[result.status] || 'grey'}>
              {t(statusText[result.status] || '未知')}
            </Tag>
          ),
        },
        {
          key: t('剩余额度'),
          value: result.unlimited_quota
            ? t('无限额度')
            : renderQuota(result.remain_quota),
        },
        { key: t('已用额度'), value: renderQuota(result.used_quota) },
        {
          key: t('总额度'),
          value: result.unlimited_quota
            ? t('无限额度')
            : renderQuota(totalQuota),
        },
        {
          key: t('过期时间'),
          value: formatExpiredTime(result.expired_time, t),
        },
        { key: t('模型倍率'), value: result.model_ratio },
      ]
    : [];

  return (
    <div className='min-h-screen flex items-center justify-center px-4 py-8'>
      <form className='w-full max-w-xl' onSubmit={handleSubmit}>
        <div className='flex flex-col sm:flex-row gap-2'>
          <Input
            value={token}
            onChange={setToken}
            placeholder={t('请输入令牌')}
            showClear
          />
          <Button htmlType='submit' type='primary' loading={loading}>
            {t('查询余额')}
          </Button>
        </div>
        {result && <Descriptions className='mt-4' data={rows} />}
      </form>
    </div>
  );
};

export default TokenBalance;
