import { formatDate } from '@yuelioi/utils'
import cronstrue from 'cronstrue'
import 'cronstrue/locales/zh_CN'

export { formatDate }

export function randomID(): string {
  const charset = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
  let result = ''
  for (let i = 0; i < 6; i++) {
    const randomIndex = Math.floor(Math.random() * charset.length)
    result += charset[randomIndex]
  }
  return result
}

export function formatCron(expr: string): string {
  const text = cronstrue.toString(expr, { locale: 'zh_CN' })
  return text
}

import AnsiToHtml from 'ansi-to-html'

const ansi = new AnsiToHtml({
  fg: '#e5e7eb', // text-gray-200
  bg: '#0f172a', // slate-950
  newline: true,
  escapeXML: false,
  stream: false,
  colors: {
    0: '#1e293b', // black
    1: '#f87171', // red → ERROR
    2: '#4ade80', // green → INFO
    3: '#fde047', // yellow → WARN
    4: '#60a5fa', // blue → DEBUG（用蓝青色）
    5: '#c084fc', // magenta → FATAL
    6: '#22d3ee', // cyan → DEBUG
    7: '#f9fafb', // white
    8: '#94a3b8', // bright black
    9: '#f87171', // bright red
    10: '#4ade80', // bright green
    11: '#fde047', // bright yellow
    12: '#60a5fa', // bright blue
    13: '#c084fc', // bright magenta
    14: '#22d3ee', // bright cyan
    15: '#ffffff', // bright white
  },
})

// 检测是否包含 ANSI 控制符
function containsAnsi(str: string): boolean {
  // \x1b == ESC
  return /\x1b\[[0-9;]*m/.test(str)
}

// 转义 HTML（防止注入）
function escapeHtml(str: string): string {
  return str.replace(/[&<>]/g, (t: string): string => {
    const map: Record<string, string> = {
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
    }
    return map[t] || t
  })
}

// 统一渲染方法
export function renderOutput(line: string): string {
  if (containsAnsi(line)) {
    return ansi.toHtml(line)
  }
  return escapeHtml(line)
}
