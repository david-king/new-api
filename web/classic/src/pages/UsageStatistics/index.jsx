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

import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Empty,
  Form,
  Skeleton,
  TabPane,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useTableCompactMode } from '../../hooks/common/useTableCompactMode';
import { API, renderNumber, renderQuota, showError } from '../../helpers';
import { createCardProPagination } from '../../helpers/utils';

const { Text } = Typography;
const { Title } = Typography;

const getDefaultDates = (period) => {
  const now = new Date();
  if (period === 'monthly') {
    const start = new Date(now);
    start.setMonth(start.getMonth() - 6);
    return {
      start_date: start.toISOString().slice(0, 7),
      end_date: now.toISOString().slice(0, 7),
    };
  }
  const start = new Date(now);
  start.setDate(start.getDate() - 7);
  return {
    start_date: start.toISOString().slice(0, 10),
    end_date: now.toISOString().slice(0, 10),
  };
};

const defaultSummary = {
  total_requests: 0,
  successful_requests: 0,
  failed_requests: 0,
  success_rate: 0,
  total_tokens: 0,
  total_quota: 0,
};

const formatTokenCount = (value) => {
  const n = Number(value || 0);
  if (n === 0) return '-';
  return renderNumber(n);
};

const UsageStatistics = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [period, setPeriod] = useState('daily');
  const [statistics, setStatistics] = useState([]);
  const [summary, setSummary] = useState(defaultSummary);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [totalCount, setTotalCount] = useState(0);
  const [formApi, setFormApi] = useState(null);
  const [compactMode, setCompactMode] = useTableCompactMode(
    'usage_statistics',
  );

  const defaults = useMemo(() => getDefaultDates(period), [period]);

  const formInitValues = useMemo(
    () => ({
      dateRange: [defaults.start_date, defaults.end_date],
      token_id: '',
      model_name: '',
    }),
    [defaults.end_date, defaults.start_date],
  );

  const getFormValues = () => {
    const values = formApi ? formApi.getValues() : formInitValues;
    const dateRange = values.dateRange || formInitValues.dateRange;
    return {
      start_date: dateRange?.[0] || defaults.start_date,
      end_date: dateRange?.[1] || defaults.end_date,
      token_id: values.token_id || '',
      model_name: values.model_name || '',
    };
  };

  const loadStatistics = async (
    page = activePage,
    size = pageSize,
    filterValues = null,
  ) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        p: String(page),
        size: String(size),
        ...(filterValues || getFormValues()),
      });
      const endpoint =
        period === 'monthly'
          ? `/api/usage_statistics_monthly/?${params}`
          : `/api/usage_statistics/?${params}`;
      const res = await API.get(endpoint);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setStatistics(data.items || []);
      setSummary(data.summary || defaultSummary);
      setTotalCount(data.total || 0);
      setActivePage(data.page || page);
      setPageSize(data.page_size || size);
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  };

  const refresh = () => {
    setActivePage(1);
    loadStatistics(1, pageSize);
  };

  useEffect(() => {
    setActivePage(1);
    setStatistics([]);
    setSummary(defaultSummary);
    const nextDefaults = getDefaultDates(period);
    setTimeout(() => {
      loadStatistics(1, pageSize, {
        ...nextDefaults,
        token_id: '',
        model_name: '',
      });
    }, 0);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [period]);

  const columns = useMemo(
    () => [
      {
        title: period === 'monthly' ? t('月份') : t('日期'),
        dataIndex: 'date',
        key: 'date',
        render: (text) => <Tag color='blue'>{text}</Tag>,
      },
      {
        title: t('令牌'),
        dataIndex: 'token_name',
        key: 'token_name',
        render: (text, record) => (
          <div className='min-w-[140px]'>
            <div className='font-medium'>{text || '-'}</div>
            <Text type='tertiary' size='small'>
              ID {record.token_id}
            </Text>
          </div>
        ),
      },
      {
        title: t('模型'),
        dataIndex: 'model_name',
        key: 'model_name',
        render: (text) => <Tag>{text}</Tag>,
      },
      {
        title: t('请求数'),
        dataIndex: 'total_requests',
        key: 'total_requests',
        render: (text) => renderNumber(text || 0),
      },
      {
        title: t('成功'),
        dataIndex: 'successful_requests',
        key: 'successful_requests',
        render: (text) => renderNumber(text || 0),
      },
      {
        title: t('失败'),
        dataIndex: 'failed_requests',
        key: 'failed_requests',
        render: (text) => renderNumber(text || 0),
      },
      {
        title: t('提示词 Tokens'),
        dataIndex: 'prompt_tokens',
        key: 'prompt_tokens',
        render: formatTokenCount,
      },
      {
        title: t('补全 Tokens'),
        dataIndex: 'completion_tokens',
        key: 'completion_tokens',
        render: formatTokenCount,
      },
      {
        title: t('总 Tokens'),
        dataIndex: 'total_tokens',
        key: 'total_tokens',
        render: formatTokenCount,
      },
      {
        title: t('消耗额度'),
        dataIndex: 'total_quota',
        key: 'total_quota',
        render: (text) => renderQuota(text || 0, 6),
      },
    ],
    [period, t],
  );

  const tableColumns = useMemo(
    () => (compactMode ? columns.map(({ fixed, ...rest }) => rest) : columns),
    [columns, compactMode],
  );

  const summaryArea = (
    <Card className='!rounded-2xl shadow-sm border-0 mb-2'>
      <Skeleton loading={loading} active>
        <div className='flex items-center justify-between mb-3'>
          <Title heading={5} className='!m-0'>
            {t('统计摘要')}
          </Title>
          <Button
            size='small'
            type='tertiary'
            theme='outline'
            onClick={() => setCompactMode(!compactMode)}
          >
            {compactMode ? t('宽松模式') : t('紧凑模式')}
          </Button>
        </div>
        <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-6 gap-3'>
          <div className='rounded-lg bg-semi-color-fill-0 p-3'>
            <Text type='tertiary' size='small'>
              {t('总请求数')}
            </Text>
            <div className='text-xl font-semibold mt-1'>
              {renderNumber(summary.total_requests)}
            </div>
          </div>
          <div className='rounded-lg bg-semi-color-fill-0 p-3'>
            <Text type='tertiary' size='small'>
              {t('成功请求数')}
            </Text>
            <div className='text-xl font-semibold mt-1 text-green-600'>
              {renderNumber(summary.successful_requests)}
            </div>
          </div>
          <div className='rounded-lg bg-semi-color-fill-0 p-3'>
            <Text type='tertiary' size='small'>
              {t('失败请求数')}
            </Text>
            <div className='text-xl font-semibold mt-1 text-red-600'>
              {renderNumber(summary.failed_requests)}
            </div>
          </div>
          <div className='rounded-lg bg-semi-color-fill-0 p-3'>
            <Text type='tertiary' size='small'>
              {t('成功率')}
            </Text>
            <div className='text-xl font-semibold mt-1'>
              {(summary.success_rate || 0).toFixed(2)}%
            </div>
          </div>
          <div className='rounded-lg bg-semi-color-fill-0 p-3'>
            <Text type='tertiary' size='small'>
              {t('总 Tokens')}
            </Text>
            <div className='text-xl font-semibold mt-1'>
              {formatTokenCount(summary.total_tokens)}
            </div>
          </div>
          <div className='rounded-lg bg-semi-color-fill-0 p-3'>
            <Text type='tertiary' size='small'>
              {t('总额度消耗')}
            </Text>
            <div className='text-xl font-semibold mt-1'>
              {renderQuota(summary.total_quota || 0, 6)}
            </div>
          </div>
        </div>
      </Skeleton>
    </Card>
  );

  const searchArea = (
    <Form
      key={period}
      initValues={formInitValues}
      getFormApi={setFormApi}
      onSubmit={refresh}
      allowEmpty
      autoComplete='off'
      layout='vertical'
    >
      <div className='flex flex-col gap-2'>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2'>
          <div className='col-span-1 lg:col-span-2'>
            <Form.DatePicker
              field='dateRange'
              className='w-full'
              type={period === 'monthly' ? 'monthRange' : 'dateRange'}
              placeholder={[t('开始日期'), t('结束日期')]}
              showClear={false}
              pure
              size='small'
            />
          </div>
          <Form.Input
            field='token_id'
            prefix={<IconSearch />}
            placeholder={t('令牌 ID')}
            showClear
            pure
            size='small'
          />
          <Form.Input
            field='model_name'
            prefix={<IconSearch />}
            placeholder={t('模型名称')}
            showClear
            pure
            size='small'
          />
        </div>
        <div className='flex gap-2 w-full justify-end'>
          <Button type='tertiary' htmlType='submit' loading={loading} size='small'>
            {t('查询')}
          </Button>
          <Button
            type='tertiary'
            onClick={() => {
              formApi?.reset();
              setTimeout(refresh, 0);
            }}
            size='small'
          >
            {t('重置')}
          </Button>
        </div>
      </div>
    </Form>
  );

  return (
    <div className='mt-[60px] px-2'>
      {summaryArea}
      <div className='mb-2'>
        <Tabs
          type='button'
          activeKey={period}
          onChange={(key) => setPeriod(key)}
        >
          <TabPane tab={t('日统计')} itemKey='daily' />
          <TabPane tab={t('月统计')} itemKey='monthly' />
        </Tabs>
      </div>
      <CardPro
        type='type2'
        searchArea={searchArea}
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize,
          total: totalCount,
          onPageChange: (page) => {
            setActivePage(page);
            loadStatistics(page, pageSize);
          },
          onPageSizeChange: (size) => {
            setPageSize(size);
            setActivePage(1);
            loadStatistics(1, size);
          },
          isMobile,
          t,
        })}
        t={t}
      >
        <CardTable
          columns={tableColumns}
          dataSource={statistics}
          rowKey='id'
          loading={loading}
          scroll={compactMode ? undefined : { x: 'max-content' }}
          pagination={false}
          hidePagination
          className='usage-statistics-table rounded-xl overflow-hidden'
          style={{ width: '100%' }}
          size='small'
          empty={
            <Empty
              image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
              darkModeImage={
                <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
              }
              description={t('暂无用量统计')}
              style={{ padding: 30 }}
            />
          }
        />
      </CardPro>
    </div>
  );
};

export default UsageStatistics;
