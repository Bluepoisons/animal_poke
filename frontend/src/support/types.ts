/** 客服体系 — 工单 + FAQ */

export type TicketStatus = 'open' | 'in_progress' | 'resolved' | 'closed'
export type TicketPriority = 'low' | 'medium' | 'high' | 'urgent'

export interface SupportTicket {
  id: string
  userId: string
  subject: string
  description: string
  category: 'bug' | 'payment' | 'account' | 'gameplay' | 'other'
  status: TicketStatus
  priority: TicketPriority
  createdAt: number
  updatedAt: number
  messages: TicketMessage[]
}

export interface TicketMessage {
  id: string
  ticketId: string
  authorId: string
  authorType: 'user' | 'support'
  content: string
  createdAt: number
}

export interface FAQItem {
  id: string
  category: string
  question: string
  answer: string
  helpful: number
}

/** 创建工单 */
export function createTicket(
  userId: string,
  subject: string,
  description: string,
  category: SupportTicket['category'],
  priority: TicketPriority = 'medium',
): SupportTicket {
  const now = Date.now()
  return {
    id: `tk-${now}-${Math.random().toString(36).slice(2, 8)}`,
    userId,
    subject,
    description,
    category,
    status: 'open',
    priority,
    createdAt: now,
    updatedAt: now,
    messages: [],
  }
}

/** 添加消息到工单 */
export function addTicketMessage(
  ticket: SupportTicket,
  authorId: string,
  authorType: 'user' | 'support',
  content: string,
): SupportTicket {
  const now = Date.now()
  const message: TicketMessage = {
    id: `msg-${now}-${Math.random().toString(36).slice(2, 8)}`,
    ticketId: ticket.id,
    authorId,
    authorType,
    content,
    createdAt: now,
  }
  return {
    ...ticket,
    messages: [...ticket.messages, message],
    updatedAt: now,
    status: authorType === 'support' ? 'in_progress' : ticket.status,
  }
}

/** 更新工单状态 */
export function updateTicketStatus(
  ticket: SupportTicket,
  status: TicketStatus,
): SupportTicket {
  return { ...ticket, status, updatedAt: Date.now() }
}

/** FAQ 数据 */
export const FAQ_ITEMS: FAQItem[] = [
  {
    id: 'faq-001',
    category: '基础',
    question: '如何发现动物？',
    answer: '点击底部「发现」标签，用摄像头对准动物即可自动检测。',
    helpful: 42,
  },
  {
    id: 'faq-002',
    category: '体力',
    question: '体力多久恢复？',
    answer: '体力每 6 分钟恢复 1 点，每小时恢复 10 点。升级后直接恢复满体力。',
    helpful: 35,
  },
  {
    id: 'faq-003',
    category: '隐私',
    question: '我的照片会被保存吗？',
    answer: '不会。照片仅用于云端瞬时检测，处理后立即销毁，不会保存在服务器上。',
    helpful: 88,
  },
  {
    id: 'faq-004',
    category: '支付',
    question: '购买后没有收到道具？',
    answer: '请尝试重新登录。如果仍未收到，请创建支付类工单，附上订单号。',
    helpful: 21,
  },
]

/** 按分类获取 FAQ */
export function getFAQByCategory(category: string): FAQItem[] {
  return FAQ_ITEMS.filter(f => f.category === category)
}
