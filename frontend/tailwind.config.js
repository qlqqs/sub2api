// CUSTOM: 色阶改为 CSS 变量，未设置变量时取下列默认值（与原配置一致）
const SLATE = {
  50: '248 250 252', 100: '241 245 249', 200: '226 232 240', 300: '203 213 225', 400: '148 163 184', 500: '100 116 139',
  600: '71 85 105', 700: '51 65 85', 800: '30 41 59', 900: '15 23 42', 950: '2 6 23'
}
// Tailwind 默认 gray
const GRAY = {
  50: '249 250 251', 100: '243 244 246', 200: '229 231 235', 300: '209 213 219', 400: '156 163 175', 500: '107 114 128',
  600: '75 85 99', 700: '55 65 81', 800: '31 41 55', 900: '17 24 39', 950: '3 7 18'
}
function cssVarScale(name, defaults) {
  return Object.fromEntries(
    Object.entries(defaults).map(([k, rgb]) => [k, `rgb(var(--color-${name}-${k}, ${rgb}) / <alpha-value>)`])
  )
}

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色调 - Teal/Cyan 青色系
        // CUSTOM: 经 CSS 变量取值，未设置时即为上述青色；QxAPI 主题在 .qx 作用域内覆盖（themes/qx/theme.css）
        primary: {
          50: 'rgb(var(--color-primary-50, 240 253 250) / <alpha-value>)',
          100: 'rgb(var(--color-primary-100, 204 251 241) / <alpha-value>)',
          200: 'rgb(var(--color-primary-200, 153 246 228) / <alpha-value>)',
          300: 'rgb(var(--color-primary-300, 94 234 212) / <alpha-value>)',
          400: 'rgb(var(--color-primary-400, 45 212 191) / <alpha-value>)',
          500: 'rgb(var(--color-primary-500, 20 184 166) / <alpha-value>)',
          600: 'rgb(var(--color-primary-600, 13 148 136) / <alpha-value>)',
          700: 'rgb(var(--color-primary-700, 15 118 110) / <alpha-value>)',
          800: 'rgb(var(--color-primary-800, 17 94 89) / <alpha-value>)',
          900: 'rgb(var(--color-primary-900, 19 78 74) / <alpha-value>)',
          950: 'rgb(var(--color-primary-950, 4 47 46) / <alpha-value>)'
        },
        // 辅助色 - 深蓝灰
        // CUSTOM: gray / accent / dark 同样经 CSS 变量取值（默认值即原色），QxAPI 全站主题在 html.qx-app 下覆盖（themes/qx/app/tokens.css）
        accent: cssVarScale('accent', SLATE),
        // 深色模式背景
        dark: cssVarScale('dark', SLATE),
        gray: cssVarScale('gray', GRAY)
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 8px 32px rgba(0, 0, 0, 0.08)',
        'glass-sm': '0 4px 16px rgba(0, 0, 0, 0.06)',
        // CUSTOM: 发光、渐变、网格背景经 CSS 变量取值（默认值即原值），主题可覆盖
        glow: 'var(--qx-shadow-glow, 0 0 20px rgba(20, 184, 166, 0.25))',
        'glow-lg': 'var(--qx-shadow-glow-lg, 0 0 40px rgba(20, 184, 166, 0.35))',
        card: '0 1px 3px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06)',
        'card-hover': '0 10px 40px rgba(0, 0, 0, 0.08)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'var(--qx-gradient-primary, linear-gradient(135deg, #14b8a6 0%, #0d9488 100%))',
        'gradient-dark': 'linear-gradient(135deg, #1e293b 0%, #0f172a 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'var(--qx-mesh-gradient, radial-gradient(at 40% 20%, rgba(20, 184, 166, 0.12) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(6, 182, 212, 0.08) 0px, transparent 50%), radial-gradient(at 0% 50%, rgba(20, 184, 166, 0.08) 0px, transparent 50%))'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: 'var(--qx-shadow-glow, 0 0 20px rgba(20, 184, 166, 0.25))' },
          '100%': { boxShadow: 'var(--qx-shadow-glow-pulse, 0 0 30px rgba(20, 184, 166, 0.4))' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
