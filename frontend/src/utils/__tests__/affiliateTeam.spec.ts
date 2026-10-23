import { describe, expect, it } from 'vitest'
import { formatTeamMoney, formatTeamShare, teamUserName } from '../affiliateTeam'

describe('affiliateTeam formatting', () => {
  it('shows tiny positive amounts as <¥0.01 instead of 0.00', () => {
    expect(formatTeamMoney(0)).toBe('¥0.00')
    expect(formatTeamMoney(null)).toBe('¥0.00')
    expect(formatTeamMoney(0.00324)).toBe('<¥0.01')
    expect(formatTeamMoney(0.01)).toBe('¥0.01')
    expect(formatTeamMoney(1234.5)).toBe('¥1,234.50')
  })

  it('formats shares and names', () => {
    expect(formatTeamShare(0)).toBe('0%')
    expect(formatTeamShare(0.004)).toBe('<1%')
    expect(formatTeamShare(0.456)).toBe('46%')
    expect(teamUserName({ user_id: 1, nickname: '', email: 'a***@b***.com', deleted: false }, '已删除')).toBe('a***@b***.com')
    expect(teamUserName({ user_id: 1, nickname: '', email: '', deleted: true }, '已删除')).toBe('已删除')
  })
})
