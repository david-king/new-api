/*
Copyright (C) 2023-2026 QuantumNous

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
import { useMemo, useState } from 'react'
import type { ElementType } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  BarChart3,
  CheckCircle2,
  CircleX,
  Hash,
  Search,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { formatNumber, formatQuota } from '@/lib/format'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { SectionPageLayout } from '@/components/layout'
import { getUsageStatistics } from './api'
import type {
  UsageStatisticsFilters,
  UsageStatisticsPeriod,
  UsageStatisticsRecord,
  UsageStatisticsSummary,
} from './types'

const DEFAULT_SUMMARY: UsageStatisticsSummary = {
  total_requests: 0,
  successful_requests: 0,
  failed_requests: 0,
  success_rate: 0,
  total_tokens: 0,
  total_quota: 0,
}

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100]

function getDefaultFilters(
  period: UsageStatisticsPeriod
): UsageStatisticsFilters {
  const now = dayjs()
  if (period === 'monthly') {
    return {
      start_date: now.subtract(6, 'month').format('YYYY-MM'),
      end_date: now.format('YYYY-MM'),
      token_id: '',
      model_name: '',
    }
  }
  return {
    start_date: now.subtract(7, 'day').format('YYYY-MM-DD'),
    end_date: now.format('YYYY-MM-DD'),
    token_id: '',
    model_name: '',
  }
}

function getSuccessRate(record: UsageStatisticsRecord): string {
  if (!record.total_requests) return '0.00%'
  return `${((record.successful_requests / record.total_requests) * 100).toFixed(2)}%`
}

function tokenText(value: number): string {
  return value > 0 ? formatNumber(value) : '-'
}

function SummaryCard({
  title,
  value,
  icon: Icon,
  tone,
}: {
  title: string
  value: string
  icon: ElementType
  tone: string
}) {
  return (
    <Card size='sm'>
      <CardContent className='flex items-center gap-3'>
        <div
          className={`flex size-9 items-center justify-center rounded-lg ${tone}`}
        >
          <Icon className='size-4' />
        </div>
        <div className='min-w-0'>
          <p className='text-muted-foreground text-xs'>{title}</p>
          <p className='truncate text-lg font-semibold'>{value}</p>
        </div>
      </CardContent>
    </Card>
  )
}

function SummarySkeleton() {
  return (
    <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-6'>
      {Array.from({ length: 6 }).map((_, index) => (
        <Skeleton key={index} className='h-[72px]' />
      ))}
    </div>
  )
}

export function UsageStatistics() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<UsageStatisticsPeriod>('daily')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [filters, setFilters] = useState(() => getDefaultFilters('daily'))
  const [draftFilters, setDraftFilters] = useState(() =>
    getDefaultFilters('daily')
  )

  const query = useQuery({
    queryKey: ['usage-statistics', period, page, pageSize, filters],
    queryFn: async () => {
      const res = await getUsageStatistics(period, {
        p: page,
        size: pageSize,
        ...filters,
      })
      if (!res.success) {
        toast.error(res.message || t('Failed to load usage statistics'))
        return null
      }
      return res.data ?? null
    },
    placeholderData: (previousData) => previousData,
  })

  const payload = query.data
  const records = payload?.items ?? []
  const summary = payload?.summary ?? DEFAULT_SUMMARY
  const total = payload?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  const summaryCards = useMemo(
    () => [
      {
        title: t('Total Requests'),
        value: formatNumber(summary.total_requests),
        icon: Hash,
        tone: 'bg-blue-100 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300',
      },
      {
        title: t('Successful Requests'),
        value: formatNumber(summary.successful_requests),
        icon: CheckCircle2,
        tone: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300',
      },
      {
        title: t('Failed Requests'),
        value: formatNumber(summary.failed_requests),
        icon: CircleX,
        tone: 'bg-rose-100 text-rose-700 dark:bg-rose-950/40 dark:text-rose-300',
      },
      {
        title: t('Success Rate'),
        value: `${(summary.success_rate || 0).toFixed(2)}%`,
        icon: BarChart3,
        tone: 'bg-violet-100 text-violet-700 dark:bg-violet-950/40 dark:text-violet-300',
      },
      {
        title: t('Total Tokens'),
        value: tokenText(summary.total_tokens),
        icon: BarChart3,
        tone: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-950/40 dark:text-cyan-300',
      },
      {
        title: t('Total Quota Used'),
        value: formatQuota(summary.total_quota || 0),
        icon: WalletCards,
        tone: 'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300',
      },
    ],
    [summary, t]
  )

  const updateDraft = (key: keyof UsageStatisticsFilters, value: string) => {
    setDraftFilters((prev) => ({ ...prev, [key]: value }))
  }

  const applyFilters = () => {
    setPage(1)
    setFilters(draftFilters)
  }

  const resetFilters = () => {
    const defaults = getDefaultFilters(period)
    setDraftFilters(defaults)
    setFilters(defaults)
    setPage(1)
  }

  const changePeriod = (value: string) => {
    const nextPeriod = value as UsageStatisticsPeriod
    const defaults = getDefaultFilters(nextPeriod)
    setPeriod(nextPeriod)
    setDraftFilters(defaults)
    setFilters(defaults)
    setPage(1)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Usage Statistics')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <Card>
            <CardHeader className='border-b'>
              <CardTitle>{t('Statistics Summary')}</CardTitle>
              <CardDescription>
                {t(
                  'Aggregated request, token, and quota usage by selected period.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {query.isLoading ? (
                <SummarySkeleton />
              ) : (
                <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-6'>
                  {summaryCards.map((card) => (
                    <SummaryCard key={card.title} {...card} />
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='border-b'>
              <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
                <div>
                  <CardTitle>{t('Usage Statistics')}</CardTitle>
                  <CardDescription>
                    {t('Grouped by date, token, and model.')}
                  </CardDescription>
                </div>
                <Tabs value={period} onValueChange={changePeriod}>
                  <TabsList>
                    <TabsTrigger value='daily'>{t('Daily')}</TabsTrigger>
                    <TabsTrigger value='monthly'>{t('Monthly')}</TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
            </CardHeader>
            <CardContent className='space-y-4'>
              <div className='grid gap-2 md:grid-cols-2 xl:grid-cols-6'>
                <Input
                  type={period === 'monthly' ? 'month' : 'date'}
                  value={draftFilters.start_date}
                  onChange={(event) =>
                    updateDraft('start_date', event.target.value)
                  }
                  aria-label={t('Start Date')}
                />
                <Input
                  type={period === 'monthly' ? 'month' : 'date'}
                  value={draftFilters.end_date}
                  onChange={(event) =>
                    updateDraft('end_date', event.target.value)
                  }
                  aria-label={t('End Date')}
                />
                <Input
                  inputMode='numeric'
                  placeholder={t('Token ID')}
                  value={draftFilters.token_id}
                  onChange={(event) =>
                    updateDraft('token_id', event.target.value)
                  }
                />
                <Input
                  placeholder={t('Model Name')}
                  value={draftFilters.model_name}
                  onChange={(event) =>
                    updateDraft('model_name', event.target.value)
                  }
                />
                <Button onClick={applyFilters} disabled={query.isFetching}>
                  <Search className='size-4' />
                  {t('Search')}
                </Button>
                <Button
                  variant='outline'
                  onClick={resetFilters}
                  disabled={query.isFetching}
                >
                  {t('Reset')}
                </Button>
              </div>

              <div className='rounded-lg border'>
                <Table>
                  <TableHeader className='bg-muted/30'>
                    <TableRow>
                      <TableHead>
                        {period === 'monthly' ? t('Month') : t('Date')}
                      </TableHead>
                      <TableHead>{t('Token')}</TableHead>
                      <TableHead>{t('Model')}</TableHead>
                      <TableHead>{t('Requests')}</TableHead>
                      <TableHead>{t('Success Rate')}</TableHead>
                      <TableHead>{t('Prompt Tokens')}</TableHead>
                      <TableHead>{t('Completion Tokens')}</TableHead>
                      <TableHead>{t('Total Tokens')}</TableHead>
                      <TableHead>{t('Quota Used')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {query.isLoading ? (
                      Array.from({ length: 6 }).map((_, index) => (
                        <TableRow key={index}>
                          {Array.from({ length: 9 }).map((__, cellIndex) => (
                            <TableCell key={cellIndex}>
                              <Skeleton className='h-5 w-full' />
                            </TableCell>
                          ))}
                        </TableRow>
                      ))
                    ) : records.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={9} className='h-32 text-center'>
                          <div className='text-muted-foreground text-sm'>
                            {t('No usage statistics found.')}
                          </div>
                        </TableCell>
                      </TableRow>
                    ) : (
                      records.map((record) => (
                        <TableRow
                          key={`${record.date}-${record.token_id}-${record.model_name}`}
                        >
                          <TableCell>
                            <Badge variant='outline'>{record.date}</Badge>
                          </TableCell>
                          <TableCell>
                            <div className='min-w-36'>
                              <div className='font-medium'>
                                {record.token_name || '-'}
                              </div>
                              <div className='text-muted-foreground text-xs'>
                                ID {record.token_id}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant='secondary'>
                              {record.model_name}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <div className='space-y-0.5'>
                              <div>{formatNumber(record.total_requests)}</div>
                              <div className='text-muted-foreground text-xs'>
                                {t('{{success}} success, {{failed}} failed', {
                                  success: formatNumber(
                                    record.successful_requests
                                  ),
                                  failed: formatNumber(record.failed_requests),
                                })}
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>{getSuccessRate(record)}</TableCell>
                          <TableCell>
                            {tokenText(record.prompt_tokens)}
                          </TableCell>
                          <TableCell>
                            {tokenText(record.completion_tokens)}
                          </TableCell>
                          <TableCell>
                            {tokenText(record.total_tokens)}
                          </TableCell>
                          <TableCell>
                            {formatQuota(record.total_quota || 0)}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>

              <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                <div className='text-muted-foreground text-sm'>
                  {t('{{count}} records', { count: formatNumber(total) })}
                </div>
                <div className='flex items-center gap-2'>
                  <select
                    className='border-input bg-background h-8 rounded-lg border px-2 text-sm'
                    value={pageSize}
                    onChange={(event) => {
                      setPageSize(Number(event.target.value))
                      setPage(1)
                    }}
                  >
                    {PAGE_SIZE_OPTIONS.map((size) => (
                      <option key={size} value={size}>
                        {size}
                      </option>
                    ))}
                  </select>
                  <Button
                    variant='outline'
                    disabled={page <= 1 || query.isFetching}
                    onClick={() => setPage((prev) => Math.max(1, prev - 1))}
                  >
                    {t('Previous')}
                  </Button>
                  <span className='text-sm font-medium'>
                    {t('Page {{current}} of {{total}}', {
                      current: page,
                      total: totalPages,
                    })}
                  </span>
                  <Button
                    variant='outline'
                    disabled={page >= totalPages || query.isFetching}
                    onClick={() =>
                      setPage((prev) => Math.min(totalPages, prev + 1))
                    }
                  >
                    {t('Next')}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
