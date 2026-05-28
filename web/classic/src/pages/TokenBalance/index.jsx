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
import {
  Button,
  Card,
  Descriptions,
  Form,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconCreditCard } from '@douyinfe/semi-icons';

const { Title, Text } = Typography;

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
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState(null);

  const handleSubmit = async (values) => {
    const token = String(values.token || '').trim();
    if (!token) {
      showError(t('请输入令牌'));
      return;
    }

    setLoading(true);
    try {
      const res = await API.post('/api/token/search', { token });
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
      <div className='w-full max-w-2xl'>
        <div className='flex items-center gap-3 mb-4'>
          <IconCreditCard size='extra-large' />
          <div>
            <Title heading={3} className='!mb-1'>
              {t('令牌余额查询')}
            </Title>
            <Text type='tertiary'>{t('无需登录，输入令牌即可查询额度。')}</Text>
          </div>
        </div>
        <Card>
          <Form onSubmit={handleSubmit}>
            <Form.Input
              field='token'
              label={t('令牌')}
              placeholder={t('请输入令牌')}
              showClear
            />
            <Space vertical align='start' className='w-full'>
              <Button htmlType='submit' type='primary' loading={loading}>
                {t('查询余额')}
              </Button>
              {result && <Descriptions data={rows} />}
            </Space>
          </Form>
        </Card>
      </div>
    </div>
  );
};

export default TokenBalance;
