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
export type UsageStatisticsPeriod = 'daily' | 'monthly'

export type UsageStatisticsRecord = {
  id: number
  date: string
  token_id: number
  token_name: string
  model_name: string
  total_requests: number
  successful_requests: number
  failed_requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  total_quota: number
  created_time: number
  updated_time: number
}

export type UsageStatisticsSummary = {
  total_requests: number
  successful_requests: number
  failed_requests: number
  success_rate: number
  total_tokens: number
  total_quota: number
}

export type UsageStatisticsFilters = {
  start_date: string
  end_date: string
  token_id?: string
  model_name?: string
}

export type UsageStatisticsParams = UsageStatisticsFilters & {
  p: number
  size: number
}

export type UsageStatisticsPayload = {
  items: UsageStatisticsRecord[]
  total: number
  page: number
  page_size: number
  summary: UsageStatisticsSummary
  start_date: string
  end_date: string
}

export type UsageStatisticsResponse = {
  success: boolean
  message?: string
  data?: UsageStatisticsPayload
}
