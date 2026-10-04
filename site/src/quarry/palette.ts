import type { LookPatch } from 'mokei/clay'

/**
 * Site-local color roles. Mokei will grow the same names
 * (base, accent1–3, detail). Switch the kit theme later by
 * rewriting this file only.
 */
export const roles = {
  base: '#F7F5F0',
  accent1: '#2B59E8',
  accent2: '#F2B705',
  accent3: '#3B4552',
  detail: '#1C1F24',
  ground: '#E8E2D4',
  rock: '#D2C6B2',
} as const

export type Role = keyof typeof roles

const lamp = '#FFF8EC'

/** Clay look mapped from the roles. Sandstone stays on the ground. */
export const quarryLook = {
  ground: roles.ground,
  road: '#DDD6C8',
  grass: '#E4DED2',
  wall: roles.base,
  roof: roles.accent1,
  accent: roles.accent1,
  yellow: roles.accent2,
  cardboard: roles.base,
  tree: roles.rock,
  tire: roles.detail,
  aoColor: '#C8BDAA',
  skyColor: '#EEF1F4',
  groundBounce: roles.ground,
  sunColor: '#FFFAF4',
  sunAzimuth: -32,
  sunElevation: 48,
  sunIntensity: 0.3,
  skyIntensity: 0.78,
  aoIntensity: 2.4,
  aoRadius: 2.1,
  cameraZoom: 52,
  cameraAzimuth: 32,
  cameraElevation: 36,
} satisfies LookPatch

export const clay = {
  ...roles,
  lamp,
}

/** Push the same roles into page / HUD tokens after applyTheme('quarry'). */
export function applyPagePalette() {
  const root = document.documentElement
  const set = (name: string, value: string) => root.style.setProperty(name, value)

  set('--background', roles.ground)
  set('--foreground', roles.detail)
  set('--ink', roles.detail)
  set('--ink-2', roles.accent3)
  set('--muted', roles.accent3)
  set('--muted-2', '#6B7380')
  set('--hair', 'rgba(59, 69, 82, 0.22)')
  set('--blue', roles.accent1)
  set('--blue-deep', '#1F46C7')

  set('--glass-bg', 'rgba(247, 245, 240, 0.9)')
  set('--glass-border', 'rgba(255, 255, 255, 0.94)')
  set('--glass-shadow', '0 1px 2px rgba(28, 31, 36, 0.04), 0 10px 28px rgba(28, 31, 36, 0.08)')
  set('--topbar-bg', 'rgba(247, 245, 240, 0.82)')
  set('--topbar-border', 'rgba(59, 69, 82, 0.14)')
  set('--topbar-shadow', '0 6px 20px rgba(28, 31, 36, 0.04)')

  set('--card', 'rgba(247, 245, 240, 0.94)')
  set('--card-foreground', roles.detail)
  set('--primary', roles.accent1)
  set('--primary-foreground', '#FFFFFF')
  set('--secondary', roles.base)
  set('--secondary-foreground', roles.accent3)
  set('--muted-surface', '#EDE8DE')
  set('--muted-foreground', roles.accent3)
  set('--accent', '#E4EAFB')
  set('--accent-foreground', roles.accent1)
  set('--border', 'rgba(59, 69, 82, 0.2)')
  set('--input', '#E4DED2')
  set('--ring', roles.accent1)

  set('--success', roles.accent1)
  set('--warning', roles.accent2)
  set('--info', roles.accent1)

  set('--clay-ground', roles.ground)
  set('--clay-road', '#DDD6C8')
  set('--clay-grass', '#E4DED2')
  set('--clay-wall', roles.base)
  set('--clay-roof', roles.accent1)
  set('--clay-accent', roles.accent1)
  set('--clay-yellow', roles.accent2)
  set('--clay-cardboard', roles.base)
  set('--clay-tree', roles.rock)
  set('--clay-tire', roles.detail)
  set('--clay-ao', '#C8BDAA')
  set('--clay-sky', quarryLook.skyColor)
}
