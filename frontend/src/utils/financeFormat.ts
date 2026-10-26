/**
 * CUSTOM: 财务明细页的金额 / 比例展示。
 *
 * 接口里金额一律是十进制字符串（最多 8 位小数），前端只展示不运算：这里全部按字符串处理，
 * 四舍五入（半数进位）后加千分位，不经过浮点，避免 1.005 → 1.00 之类的误差。
 */

const DECIMAL_RE = /^([+-])?(\d*)(?:\.(\d*))?$/

interface ParsedDecimal {
  negative: boolean
  int: string
  frac: string
}

function parseDecimal(value: string | number | null | undefined): ParsedDecimal | null {
  if (value == null) return null
  const text = String(value).trim()
  if (text === '') return null
  const match = DECIMAL_RE.exec(text)
  if (!match || (match[2] === '' && (match[3] ?? '') === '')) return null
  return {
    negative: match[1] === '-',
    int: match[2].replace(/^0+(?=\d)/, '') || '0',
    frac: match[3] ?? ''
  }
}

/** 整数部分（纯数字字符串）加 1。 */
function incrementDigits(digits: string): string {
  const chars = digits.split('')
  for (let i = chars.length - 1; i >= 0; i--) {
    if (chars[i] === '9') {
      chars[i] = '0'
    } else {
      chars[i] = String(Number(chars[i]) + 1)
      return chars.join('')
    }
  }
  return '1' + chars.join('')
}

/** 按 digits 位小数四舍五入，返回不带千分位的规范字符串（如 "-12.35"）；非法输入返回 null。 */
export function roundDecimalString(value: string | number | null | undefined, digits: number): string | null {
  const parsed = parseDecimal(value)
  if (!parsed) return null
  const padded = parsed.frac.padEnd(digits + 1, '0')
  let kept = parsed.int + padded.slice(0, digits)
  if (Number(padded[digits]) >= 5) kept = incrementDigits(kept)
  const intLen = kept.length - digits
  const intPart = kept.slice(0, intLen).replace(/^0+(?=\d)/, '') || '0'
  const fracPart = kept.slice(intLen)
  const isZero = /^0*$/.test(intPart + fracPart)
  const sign = parsed.negative && !isZero ? '-' : ''
  return digits > 0 ? `${sign}${intPart}.${fracPart}` : `${sign}${intPart}`
}

function groupThousands(intPart: string): string {
  return intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
}

/** 金额展示：默认 2 位小数，带千分位；非法 / 空值显示「—」。 */
export function formatAmount(value: string | number | null | undefined, digits = 2): string {
  const rounded = roundDecimalString(value, digits)
  if (rounded == null) return '—'
  const negative = rounded.startsWith('-')
  const [intPart, fracPart] = (negative ? rounded.slice(1) : rounded).split('.')
  const body = fracPart != null ? `${groupThousands(intPart)}.${fracPart}` : groupThousands(intPart)
  return negative ? `-${body}` : body
}

/** 人民币金额：「¥1,234.50」「-¥3.20」。 */
export function formatCny(value: string | number | null | undefined, digits = 2): string {
  const text = formatAmount(value, digits)
  if (text === '—') return text
  return text.startsWith('-') ? `-¥${text.slice(1)}` : `¥${text}`
}

/** 按展示精度判断是否为负（舍入后为 0 的极小负数不算负）。 */
export function isNegativeAmount(value: string | number | null | undefined, digits = 2): boolean {
  return (roundDecimalString(value, digits) ?? '').startsWith('-')
}

/** 按展示精度判断是否为 0。 */
export function isZeroAmount(value: string | number | null | undefined, digits = 2): boolean {
  const rounded = roundDecimalString(value, digits)
  return rounded != null && /^-?0(?:\.0*)?$/.test(rounded)
}

/** 小数点左右移动 shift 位（正数右移，即 ×10^shift），纯字符串运算。 */
function shiftDecimal(value: string | number | null | undefined, shift: number): string | null {
  const parsed = parseDecimal(value)
  if (!parsed) return null
  let int = parsed.int
  let frac = parsed.frac
  if (shift > 0) {
    frac = frac.padEnd(shift, '0')
    int = int + frac.slice(0, shift)
    frac = frac.slice(shift)
  } else if (shift < 0) {
    const n = -shift
    int = int.padStart(n + 1, '0')
    frac = int.slice(int.length - n) + frac
    int = int.slice(0, int.length - n)
  }
  int = int.replace(/^0+(?=\d)/, '') || '0'
  frac = frac.replace(/0+$/, '')
  const isZero = /^0*$/.test(int + frac)
  const sign = parsed.negative && !isZero ? '-' : ''
  return frac ? `${sign}${int}.${frac}` : `${sign}${int}`
}

/** 比例展示：「0.3215」→「32.15%」；null / 非法显示「—」。 */
export function formatRatio(value: string | number | null | undefined, digits = 2): string {
  const percent = shiftDecimal(value, 2)
  if (percent == null) return '—'
  return `${formatAmount(percent, digits)}%`
}

/**
 * 小数形式的费率 → 百分比输入框里的文本：「0.0320」→「3.2」，「0」/ 空 →「0」。
 * 去掉多余的尾随 0，便于编辑。
 */
export function decimalRateToPercent(value: string | number | null | undefined): string {
  return shiftDecimal(value, 2) ?? '0'
}

export const CHANNEL_FEE_PERCENT_MAX = 20

export type PercentRateError = 'invalid' | 'outOfRange' | 'tooManyDecimals'

/**
 * 百分比输入 → 提交给后端的小数字符串（4 位小数）：「3.2」→「0.0320」，空 →「0」。
 * 后端 numeric(6,4) 存储，所以百分比最多 2 位小数；范围 0–max%。
 */
export function percentToDecimalRate(
  input: string | number | null | undefined,
  maxPercent = CHANNEL_FEE_PERCENT_MAX
): { value: string } | { error: PercentRateError } {
  const text = input == null ? '' : String(input).trim()
  if (text === '') return { value: '0.0000' }
  const parsed = parseDecimal(text)
  if (!parsed) return { error: 'invalid' }
  if (parsed.negative && !/^0*$/.test(parsed.int + parsed.frac)) return { error: 'outOfRange' }
  if (parsed.frac.replace(/0+$/, '').length > 2) return { error: 'tooManyDecimals' }
  const percentNumber = Number(`${parsed.int}.${parsed.frac || '0'}`)
  if (percentNumber > maxPercent) return { error: 'outOfRange' }
  const decimal = shiftDecimal(`${parsed.int}.${parsed.frac}`, -2) ?? '0'
  return { value: roundDecimalString(decimal, 4) ?? '0.0000' }
}
