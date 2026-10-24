import { describe, expect, it } from 'vitest'
import postcss from 'postcss'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const themeCss = readFileSync(resolve(__dirname, '../theme.css'), 'utf8')

describe('Qx public navigation styles', () => {
  it('pads the public navigation button without shrinking console sidebar icons', () => {
    const fixture = document.createElement('div')
    fixture.innerHTML = `
      <div class="qx"><nav class="qx-nav"><button class="qx-nav-icon"></button></nav></div>
      <div class="qx-shell"><aside class="qx-side"><svg class="qx-nav-icon"></svg></aside></div>
    `
    const button = fixture.querySelector('button')!
    const icon = fixture.querySelector('svg')!
    const paddingRules: string[] = []
    postcss.parse(themeCss).walkRules((rule) => {
      if (!rule.selector.includes('.qx-nav-icon')) return
      rule.walkDecls('padding', () => { paddingRules.push(rule.selector) })
    })
    expect(paddingRules.some((selector) => button.matches(selector))).toBe(true)
    expect(paddingRules.some((selector) => icon.matches(selector))).toBe(false)
  })
})
