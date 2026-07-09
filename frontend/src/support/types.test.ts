import { describe, it, expect } from 'vitest'
import { createTicket, addTicketMessage, updateTicketStatus, FAQ_ITEMS, getFAQByCategory } from './types'

describe('support', () => {
  it('creates ticket with correct fields', () => {
    const ticket = createTicket('u1', 'Bug', 'Something broke', 'bug')
    expect(ticket.status).toBe('open')
    expect(ticket.priority).toBe('medium')
    expect(ticket.messages).toEqual([])
  })

  it('adds message to ticket', () => {
    let ticket = createTicket('u1', 'Test', 'Help', 'gameplay')
    ticket = addTicketMessage(ticket, 'u1', 'user', 'I need help')
    expect(ticket.messages.length).toBe(1)
    expect(ticket.messages[0].content).toBe('I need help')
    expect(ticket.updatedAt).toBeGreaterThanOrEqual(ticket.createdAt)
  })

  it('support message changes status to in_progress', () => {
    let ticket = createTicket('u1', 'Test', 'Help', 'bug')
    ticket = addTicketMessage(ticket, 'support1', 'support', 'We are looking into it')
    expect(ticket.status).toBe('in_progress')
  })

  it('updates ticket status', () => {
    const ticket = createTicket('u1', 'Test', 'Done', 'bug')
    const updated = updateTicketStatus(ticket, 'resolved')
    expect(updated.status).toBe('resolved')
  })

  it('has FAQ items', () => {
    expect(FAQ_ITEMS.length).toBeGreaterThan(0)
  })

  it('filters FAQ by category', () => {
    const privacy = getFAQByCategory('隐私')
    expect(privacy.length).toBeGreaterThan(0)
    expect(privacy[0].category).toBe('隐私')
  })
})
