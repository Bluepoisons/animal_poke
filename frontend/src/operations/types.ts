/** 运营工具 — 活动/公告/数据看板 */

export interface Announcement {
  id: string
  title: string
  content: string
  type: 'maintenance' | 'event' | 'update' | 'urgent'
  publishedAt: number
  expiresAt: number | null
  isRead: boolean
}

export interface GameEvent {
  id: string
  name: string
  description: string
  startDate: number
  endDate: number
  type: 'capture' | 'battle' | 'collection' | 'social'
  rewards: { description: string; amount: number }[]
  isActive: boolean
}

export interface DashboardMetric {
  label: string
  value: number
  unit: string
  trend: 'up' | 'down' | 'flat'
  changePercent: number
}

export interface DashboardData {
  metrics: DashboardMetric[]
  updatedAt: number
}

/** 生成 mock 运营数据 */
export function getMockDashboard(): DashboardData {
  return {
    metrics: [
      { label: 'DAU', value: 5234, unit: '人', trend: 'up', changePercent: 12.5 },
      { label: '新增', value: 342, unit: '人', trend: 'up', changePercent: 8.3 },
      { label: '次日留存', value: 52, unit: '%', trend: 'flat', changePercent: 0.5 },
      { label: '日均时长', value: 18, unit: '分钟', trend: 'down', changePercent: -3.2 },
      { label: '付费率', value: 4.2, unit: '%', trend: 'up', changePercent: 0.8 },
      { label: 'ARPU', value: 3.5, unit: '元', trend: 'up', changePercent: 5.1 },
    ],
    updatedAt: Date.now(),
  }
}

/** 生成 mock 公告 */
export function getMockAnnouncements(): Announcement[] {
  const now = Date.now()
  return [
    {
      id: 'ann-001',
      title: 'S1 赛季开启！',
      content: '第一个赛季正式开启，收集限定动物赢取丰厚奖励！',
      type: 'event',
      publishedAt: now - 86400000,
      expiresAt: now + 86400000 * 30,
      isRead: false,
    },
    {
      id: 'ann-002',
      title: 'v2.0 更新公告',
      content: '新增社交系统、PvP 排位、区域排行等功能。',
      type: 'update',
      publishedAt: now - 3600000,
      expiresAt: null,
      isRead: false,
    },
  ]
}

/** 生成 mock 活动 */
export function getMockEvents(): GameEvent[] {
  const now = Date.now()
  return [
    {
      id: 'evt-001',
      name: '猫咪狂欢周',
      description: '本周所有猫类动物出现概率 +50%！',
      startDate: now,
      endDate: now + 7 * 86400000,
      type: 'capture',
      rewards: [
        { description: '捕获 10 只猫', amount: 200 },
        { description: '捕获传说级猫', amount: 1000 },
      ],
      isActive: true,
    },
  ]
}

/** 检查活动是否在进行中 */
export function isEventActive(event: GameEvent, now: number = Date.now()): boolean {
  return now >= event.startDate && now < event.endDate
}

/** 标记公告为已读 */
export function markAnnouncementRead(announcements: Announcement[], id: string): Announcement[] {
  return announcements.map(a => a.id === id ? { ...a, isRead: true } : a)
}
