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
import { useState } from 'react'
import { Search, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatQuota, formatTimestampToDate } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { StatusBadge } from '@/components/status-badge'
import { queryTokenBalance } from '../api'
import { API_KEY_STATUSES, ERROR_MESSAGES } from '../constants'
import { type TokenBalanceInfo } from '../types'

function BalanceRow(props: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex min-h-10 items-center justify-between gap-4 border-b py-2 last:border-b-0'>
      <span className='text-muted-foreground text-sm'>{props.label}</span>
      <span className='text-right text-sm font-medium break-all'>
        {props.value}
      </span>
    </div>
  )
}

export function TokenBalanceLookup() {
  const { t } = useTranslation()
  const [token, setToken] = useState('')
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<TokenBalanceInfo | null>(null)

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const trimmed = token.trim()
    if (!trimmed) {
      toast.error(t('Please enter an API key'))
      return
    }

    setLoading(true)
    try {
      const res = await queryTokenBalance(trimmed)
      if (res.success && res.data) {
        setResult(res.data)
      } else {
        setResult(null)
        toast.error(res.message || t(ERROR_MESSAGES.UNEXPECTED))
      }
    } catch {
      setResult(null)
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setLoading(false)
    }
  }

  const status = result ? API_KEY_STATUSES[result.status] : undefined
  const totalQuota = result
    ? Number(result.remain_quota || 0) + Number(result.used_quota || 0)
    : 0

  return (
    <main className='bg-background min-h-screen'>
      <div className='mx-auto flex min-h-screen w-full max-w-3xl flex-col justify-center px-4 py-10 sm:px-6'>
        <div className='mb-6 flex items-center gap-3'>
          <div className='bg-primary/10 text-primary flex h-10 w-10 items-center justify-center rounded-md'>
            <WalletCards className='h-5 w-5' />
          </div>
          <div>
            <h1 className='text-2xl font-semibold tracking-normal'>
              {t('Token Balance Lookup')}
            </h1>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Paste an API key to check its quota without signing in.')}
            </p>
          </div>
        </div>

        <form className='space-y-4' onSubmit={handleSubmit}>
          <div className='space-y-2'>
            <Label htmlFor='token-balance-key'>{t('API Key')}</Label>
            <div className='flex flex-col gap-2 sm:flex-row'>
              <Input
                id='token-balance-key'
                value={token}
                onChange={(event) => setToken(event.target.value)}
                placeholder={t('Enter API key')}
                autoComplete='off'
              />
              <Button type='submit' disabled={loading} className='sm:w-36'>
                <Search className='h-4 w-4' />
                {loading ? t('Checking...') : t('Check Balance')}
              </Button>
            </div>
          </div>
        </form>

        {result && (
          <section className='mt-6 rounded-md border px-4 py-2'>
            <BalanceRow label={t('Token Name')} value={result.token_name} />
            <BalanceRow
              label={t('Status')}
              value={
                status ? (
                  <StatusBadge
                    label={t(status.label)}
                    variant={status.variant}
                    copyable={false}
                  />
                ) : (
                  result.status
                )
              }
            />
            <BalanceRow
              label={t('Remaining quota')}
              value={
                result.unlimited_quota
                  ? t('Unlimited')
                  : formatQuota(result.remain_quota)
              }
            />
            <BalanceRow
              label={t('Used quota')}
              value={formatQuota(result.used_quota)}
            />
            <BalanceRow
              label={t('Total quota')}
              value={
                result.unlimited_quota
                  ? t('Unlimited')
                  : formatQuota(totalQuota)
              }
            />
            <BalanceRow
              label={t('Expires at')}
              value={
                result.expired_time <= 0
                  ? t('Never expires')
                  : formatTimestampToDate(result.expired_time)
              }
            />
            <BalanceRow
              label={t('Model ratio')}
              value={String(result.model_ratio)}
            />
          </section>
        )}
      </div>
    </main>
  )
}
