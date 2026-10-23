import { describe, expect, it } from 'vitest'
import { applyAnnouncementBanner } from '../announcementBanner'

const items = [{ id: 5 }, { id: 4 }, { id: 3 }, { id: 2 }, { id: 1 }]
const ids = (list: { id: number }[]) => list.map((a) => a.id)

describe('applyAnnouncementBanner', () => {
  it('keeps the incoming order when nothing is configured', () => {
    expect(ids(applyAnnouncementBanner(items, null))).toEqual([5, 4, 3, 2, 1])
    expect(ids(applyAnnouncementBanner(items, { order: [], hidden: [] }))).toEqual([5, 4, 3, 2, 1])
  })

  it('drops hidden ids, puts ordered ids first and keeps the rest in incoming order', () => {
    expect(ids(applyAnnouncementBanner(items, { order: [2, 4], hidden: [3] }))).toEqual([2, 4, 5, 1])
  })

  it('ignores ordered ids the user cannot see', () => {
    expect(ids(applyAnnouncementBanner(items, { order: [99, 1], hidden: [] }))).toEqual([1, 5, 4, 3, 2])
  })
})
