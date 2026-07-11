/**
 * 隐私 scope 目录（AP-068）
 * 展示用途 / 状态 / 版本 / 保留期；analytics 为本地偏好，不进服务端 consent scope。
 */

import {
  SERVER_CONSENT_VERSION,
  type ConsentRecord,
  type ConsentScope,
  getConsent,
  hasScope,
} from '../compliance'
import { getAnalyticsPref } from './analyticsPrefs'

/** 服务端 consent 可接受的 scope + 客户端 analytics */
export type PrivacyScopeId = ConsentScope | 'analytics'

export type ScopeStatus = 'granted' | 'denied' | 'pending' | 'offline_pending' | 'local_only'

export interface PrivacyScopeMeta {
  id: PrivacyScopeId
  /** 是否需同步服务端 /privacy/consent */
  serverBacked: boolean
  /** 同意协议版本（服务端权威或本地策略版本） */
  version: string
  /** 用途说明（中文默认，UI 层可 i18n） */
  purposeKey: string
  /** 保留期说明 key */
  retentionKey: string
}

export const PRIVACY_SCOPES: readonly PrivacyScopeMeta[] = [
  {
    id: 'photo',
    serverBacked: true,
    version: SERVER_CONSENT_VERSION,
    purposeKey: 'privacy.scope.photo.purpose',
    retentionKey: 'privacy.scope.photo.retention',
  },
  {
    id: 'location',
    serverBacked: true,
    version: SERVER_CONSENT_VERSION,
    purposeKey: 'privacy.scope.location.purpose',
    retentionKey: 'privacy.scope.location.retention',
  },
  {
    id: 'precise_location',
    serverBacked: true,
    version: SERVER_CONSENT_VERSION,
    purposeKey: 'privacy.scope.precise.purpose',
    retentionKey: 'privacy.scope.precise.retention',
  },
  {
    id: 'analytics',
    serverBacked: false,
    version: 'local-v1',
    purposeKey: 'privacy.scope.analytics.purpose',
    retentionKey: 'privacy.scope.analytics.retention',
  },
] as const

export interface ScopeViewModel {
  meta: PrivacyScopeMeta
  status: ScopeStatus
  enabled: boolean
  /** 最后本地更新时间（ms） */
  updatedAt: number | null
  /** 服务端是否已确认（server scopes） */
  serverSynced: boolean
}

export function resolveScopeStatus(
  id: PrivacyScopeId,
  consent: ConsentRecord = getConsent(),
): ScopeViewModel {
  const meta = PRIVACY_SCOPES.find((s) => s.id === id)!
  if (id === 'analytics') {
    const pref = getAnalyticsPref()
    const productOk =
      consent.status === 'granted' || consent.status === 'offline_pending'
    const enabled = pref !== 'denied' && productOk
    return {
      meta,
      status: pref === 'denied' ? 'denied' : productOk ? 'local_only' : 'pending',
      enabled,
      updatedAt: consent.updatedAt ?? null,
      serverSynced: false,
    }
  }

  const scope = id as ConsentScope
  const enabled = hasScope(scope, consent)
  let status: ScopeStatus
  if (consent.status === 'pending') status = 'pending'
  else if (consent.status === 'denied' || !consent.scopes.includes(scope)) status = 'denied'
  else if (consent.status === 'offline_pending') status = 'offline_pending'
  else if (enabled) status = 'granted'
  else status = 'denied'

  return {
    meta,
    status,
    enabled,
    updatedAt: consent.updatedAt ?? consent.grantedAt ?? null,
    serverSynced: Boolean(consent.serverSynced && consent.scopes.includes(scope)),
  }
}

export function listScopeViewModels(consent: ConsentRecord = getConsent()): ScopeViewModel[] {
  return PRIVACY_SCOPES.map((s) => resolveScopeStatus(s.id, consent))
}
